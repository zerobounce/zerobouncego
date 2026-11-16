package zerobouncego

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// messageOnlySweepExempt are the calls whose answer may be a bare message or a
// free-form object (standard.md §3): validate (greylist), the raw-JSON filter
// calls, the list evaluator result and every Inbox Placement call. Warmup has
// its own rules.
func messageOnlySweepExempt(name string) bool {
	name = strings.TrimPrefix(name, "Client.")
	return strings.HasPrefix(name, "WarmupClient.") ||
		strings.Contains(name, "InboxPlacement") ||
		strings.Contains(name, "Filter") ||
		name == "GetListEvaluatorResult" ||
		strings.HasPrefix(name, "Validate") && !strings.Contains(name, "Batch") &&
			!strings.Contains(name, "File") && !strings.Contains(name, "Bulk") ||
		name == "Validate"
}

func greylistCall(name string) bool {
	name = strings.TrimPrefix(name, "Client.")
	return name == "Validate" || name == "ValidateWithTimeout" || name == "ValidateWithOptions"
}

func expectErrorWithStatus(t *testing.T, call requestCall, fake *fixedBodyTransport, body string) {
	t.Helper()
	fake.body = body
	err := call.call()
	var apiErr *ZBApiError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != fake.status {
		t.Errorf("%s %s: err = %#v, want a *ZBApiError with status %d", call.name, body, err, fake.status)
	}
}

// Every typed call answers a body that is only a message, or an empty object, with
// a *ZBApiError carrying the response's status (E3).
func TestEveryTypedCallRefusesAMessageOnlyOrEmptyAnswer(t *testing.T) {
	downloads := downloadCalls(t)
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	bodies := []string{
		`{"message":"x"}`, `{"Message":"x"}`, `{"error":"x"}`, `{"error":["a"]}`,
		`{"detail":"d"}`, `{}`, `{"Error":"e"}`, `{"error_message":"m"}`,
	}
	swept := 0
	for _, call := range publicRequestCalls(client, warmup) {
		if downloads[call.name] || messageOnlySweepExempt(call.name) {
			continue
		}
		swept++
		for _, body := range bodies {
			expectErrorWithStatus(t, call, fake, body)
		}
	}
	if swept < 50 {
		t.Fatalf("swept %d typed calls, want at least 50", swept)
	}
}

// validate: only {"Message": "<text>"} (capital M, non-blank string) is a
// result: the greylist answer. Any other message-only body is an error.
func TestValidateTakesOnlyTheCapitalMessageGreylistAnswer(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	checked := 0
	for _, call := range publicRequestCalls(client, warmup) {
		if !greylistCall(call.name) {
			continue
		}
		checked++
		expectAccepted(t, call, fake, `{"Message":"Greylist validation still in progress"}`)
		for _, body := range []string{
			`{"message":"x"}`, `{"error":"x"}`, `{"error":["a"]}`, `{"detail":"d"}`, `{}`,
			`{"Message":""}`, `{"Message":"  "}`, `{"Message":null}`, `{"Message":["x"]}`,
			`{"Message":"x","error":"y"}`, `{"Message":"x","message":"y"}`,
		} {
			expectErrorWithStatus(t, call, fake, body)
		}
	}
	if checked != 4 {
		t.Fatalf("checked %d validate calls, want 4", checked)
	}
	// The package-level POST variant.
	fake.body = `{"message":"x"}`
	if _, err := ValidateWithOptions("a@example.com", ValidateOptions{UsePost: true}); err == nil {
		t.Fatal("POST validate took a lowercase message as a result")
	}
}

// Inbox Placement: an empty 2xx body is an error on every call but the delete.
func TestInboxPlacementEmptyBodyIsAnErrorExceptOnDelete(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	errorsSeen, accepted := 0, 0
	for _, call := range publicRequestCalls(client, warmup) {
		if !strings.Contains(call.name, "InboxPlacement") {
			continue
		}
		if strings.Contains(call.name, "DeleteInboxPlacementTest") {
			accepted++
			expectAccepted(t, call, fake, "")
			continue
		}
		errorsSeen++
		expectErrorWithStatus(t, call, fake, "")
	}
	if errorsSeen < 7 || accepted < 1 {
		t.Fatalf("checked %d non-delete and %d delete calls", errorsSeen, accepted)
	}
}

// "success" values a strict boolean reads as false are failures (E4).
func TestFalsySuccessValuesAreFailures(t *testing.T) {
	for _, value := range []string{`false`, `0`, `0.0`, `"false"`, `"False"`, `" FALSE "`, `"0"`, `" 0 "`} {
		if !isFalsyAPISuccess(decodeAny(t, value)) {
			t.Errorf("success %s was not read as false", value)
		}
	}
	for _, value := range []string{`true`, `1`, `"true"`, `"1"`, `null`, `""`, `[]`, `{}`, `"no"`} {
		if isFalsyAPISuccess(decodeAny(t, value)) {
			t.Errorf("success %s was read as false", value)
		}
	}
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	for _, call := range publicRequestCalls(client, warmup) {
		if call.name != "GetCredits" {
			continue
		}
		for _, value := range []string{`0`, `"0"`, `"False"`} {
			expectErrorWithStatus(t, call, fake, `{"success":`+value+`,"message":"nope"}`)
		}
	}
}

func decodeAny(t *testing.T, text string) interface{} {
	t.Helper()
	var value interface{}
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

// A list of messages keeps every non-blank line, joined with a line break.
func TestMessageListsAreJoinedWithLineBreaks(t *testing.T) {
	cases := map[string]string{
		`{"error":["a","b"]}`:         "a\nb",
		`{"error":["a"," ","b ",1]}`:  "a\nb",
		`{"message":["x","y","z"]}`:   "x\ny\nz",
		`{"error":[""],"detail":"d"}`: "d",
	}
	for body, want := range cases {
		if got := messageFromBody(200, "application/json", body); got != want {
			t.Errorf("%s: message %q, want %q", body, got, want)
		}
	}
	err := decodeJSONBody(200, "application/json", []byte(`{"error":["a","b"]}`), &CreditsResponse{})
	var apiErr *ZBApiError
	if !errors.As(err, &apiErr) || apiErr.Message != "a\nb" {
		t.Fatalf("err = %#v, want message a\\nb", err)
	}
}

// An empty ip_address is left out of the request.
func TestBatchEmailWithoutIPOmitsTheField(t *testing.T) {
	encoded, err := json.Marshal(EmailToValidate{EmailAddress: "a@example.com"})
	if err != nil || string(encoded) != `{"email_address":"a@example.com"}` {
		t.Fatalf("encoded %s, %v", encoded, err)
	}
	encoded, _ = json.Marshal(EmailToValidate{EmailAddress: "a@example.com", IPAddress: "1.2.3.4"})
	if string(encoded) != `{"email_address":"a@example.com","ip_address":"1.2.3.4"}` {
		t.Fatalf("encoded %s", encoded)
	}
	useFake(t)
	capture := &bodyCapture{}
	http.DefaultTransport = capture
	Initialize("sweep-key")
	URI = "https://api.example.test/v2/"
	_, _ = ValidateBatch([]EmailToValidate{{EmailAddress: "a@example.com"}})
	if capture.body == "" || strings.Contains(capture.body, "ip_address") {
		t.Fatalf("batch request body = %q, want none with an ip_address", capture.body)
	}
}

type bodyCapture struct{ body string }

func (c *bodyCapture) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		c.body = string(data)
	}
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {CONTENT_TYPE_JSON}},
		Body:       io.NopCloser(strings.NewReader(`{"email_batch":[],"errors":[]}`)),
		Request:    req,
	}, nil
}

// null is not a result: a typed call answers it with the shape error, however
// it is padded; only a raw-JSON result takes it.
func TestNullBodyIsRefusedByTypedResultsOnly(t *testing.T) {
	for _, pad := range []string{"", " ", "\t", "\r", "\n", " \t\r\n"} {
		body := []byte(pad + "null" + pad)
		var typed CreditsResponse
		err := decodeJSONBody(200, "application/json", body, &typed)
		var apiErr *ZBApiError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 200 || apiErr.Message != unexpectedJSONShapeMessage {
			t.Errorf("typed, padding %q: err = %#v", pad, err)
		}
		var raw interface{} = "unchanged"
		if err := decodeJSONBody(200, "application/json", body, &raw); err != nil || raw != nil {
			t.Errorf("raw, padding %q: err = %v, raw = %v", pad, err, raw)
		}
	}
	// Not null: decoded as usual.
	var typed CreditsResponse
	if err := decodeJSONBody(200, "application/json", []byte(`{"credits":"5"}`), &typed); err != nil {
		t.Fatalf("a result was refused: %v", err)
	}
}

// strings.ToLower can change a string's byte length ("İ" grows by one byte), so
// the title is found on the original bytes.
func TestHTMLTitleSurvivesCaseChangesOfByteLength(t *testing.T) {
	cases := map[string]string{
		"İİİ<html><TITLE>Hello İ</TITLE></html>":                        "Hello İ",
		"<html><head>İİİİ</head><title class=\"x\">Bad Gateway</Title>": "Bad Gateway",
		"<TiTlE>plain</tItLe>":                                          "plain",
		"İİİ<title>no close":                                            "",
		"no title İ":                                                    "",
	}
	for body, want := range cases {
		if got := htmlTitle(body); got != want {
			t.Errorf("htmlTitle(%q) = %q, want %q", body, got, want)
		}
	}
	message := messageFromBody(502, "text/html", "İİİİ<html><title>Bad İ Gateway</title></html>")
	if !strings.Contains(message, `"Bad İ Gateway"`) {
		t.Fatalf("message = %q", message)
	}
}
