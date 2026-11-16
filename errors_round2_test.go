package zerobouncego

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// A body that is only an "Error:" prefix is still an error body: never a
// success with zero values, on any typed call.
func TestBlankAfterPrefixErrorBodiesAreErrorsOnEveryCall(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	bodies := []string{
		`{"error":"Error:"}`, `{"Error":"Error: "}`, `{"message":"Error:"}`, `{"Message":"Error:"}`,
		`{"success":false,"message":"Error:"}`, `{"success":false,"message":"[Error]: "}`,
		`{"error":"[Error]:"}`, `{"error":["Error:"]}`,
	}
	checked := 0
	for _, call := range publicRequestCalls(client, warmup) {
		if strings.HasPrefix(call.name, "WarmupClient.") || strings.Contains(call.name, "DoPostFormRequest") {
			continue
		}
		checked++
		for _, body := range bodies {
			// Calls whose result may be a bare message (filters, evaluator,
			// Inbox Placement, validate's greylist) take a message-only body as
			// a result; the error and success:false bodies are errors everywhere.
			if messageOnlyExemptCall(call.name) && !strings.Contains(body, `"error"`) &&
				!strings.Contains(body, `"Error"`) && !strings.Contains(body, `"success"`) {
				continue
			}
			fake.body = body
			err := call.call()
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != fake.status || strings.TrimSpace(apiErr.Message) == "" {
				t.Errorf("%s %s: err = %#v, want a *ZBApiError with status %d and a message", call.name, body, err, fake.status)
			}
		}
	}
	if checked < 30 {
		t.Fatalf("sweep checked %d calls, want at least 30", checked)
	}
}

func TestApiErrorNeverReturnsNil(t *testing.T) {
	for _, message := range []string{"", "  ", "\n\t"} {
		var apiErr *ZBApiError
		err := apiError(200, message)
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 200 || strings.TrimSpace(apiErr.Message) == "" {
			t.Errorf("apiError(200, %q) = %#v, want an error with a fixed message", message, err)
		}
	}
	if got := messageFromBody(200, "application/json", `{"error":"Error:"}`); got == "" {
		t.Error("messageFromBody returned a blank message")
	}
}

func TestErrorPrefixStrippingAndTrimming(t *testing.T) {
	cases := map[string]string{
		"[Error]: boom":         "boom",
		"[Error]:boom":          "boom",
		"  [Error]:  boom  ":    "boom",
		"Error: [Error]: boom":  "boom",
		"[Error]: Error: boom":  "boom",
		"\t\nError:\tboom\r\n":  "boom",
		"[Error]:":              "",
		"Error:  ":              "",
		"not an Error: prefix.": "not an Error: prefix.",
	}
	for in, want := range cases {
		if got := stripErrorPrefix(in); got != want {
			t.Errorf("stripErrorPrefix(%q) = %q, want %q", in, got, want)
		}
	}
	if got := messageFromBody(400, "text/plain", "Error: [Error]: bad key"); got != "bad key" {
		t.Errorf("plain [Error]: body gave %q", got)
	}
	if got := messageFromBody(400, "application/json", `{"message":"[Error]: bad key"}`); got != "bad key" {
		t.Errorf("JSON [Error]: message gave %q", got)
	}
}

// The error-only check is gated on "success": a result that happens to carry
// an error field is a result, an error-only body is not.
func TestErrorOnlyMessageIsGatedOnSuccess(t *testing.T) {
	result := []byte(`{"success":true,"error":"note","address":"a@b.co","status":"valid"}`)
	if err := ensureJSONAPIBody(200, "application/json", result); err != nil {
		t.Errorf("a success:true body with an error field is a result, got %v", err)
	}
	if err := ensureJSONAPIBody(200, "application/json", []byte(`{"error":"note","address":"a@b.co"}`)); err == nil {
		t.Error("an error-only body (no success field) must be an error")
	}
	if got := errorOnlyMessage(map[string]interface{}{"success": true, "error": "x"}); got != "" {
		t.Errorf("errorOnlyMessage with success = %q, want empty", got)
	}
	if got := errorOnlyMessage(map[string]interface{}{"error": "x"}); got != "x" {
		t.Errorf("errorOnlyMessage without success = %q, want x", got)
	}
}

// Only the exact single-key {"Message": "<text>"} body is the greylist result;
// a batch row always needs address and status.
func TestGreylistExemptionIsValidateOnlyAndExact(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	for _, call := range publicRequestCalls(client, warmup) {
		if call.name != "Validate" {
			continue
		}
		expectAccepted(t, call, fake, `{"Message":"x"}`)
		expectStatus200Error(t, call, fake, `{"Message":"x","foo":1}`)
		expectStatus200Error(t, call, fake, `{"Message":"x","status":"valid"}`)
	}
	rows := []string{
		`{"email_batch":[{"Message":"x"}],"errors":[]}`,
		`{"email_batch":[{"address":"a@b.co","status":"valid"},{"Message":"x"}],"errors":[]}`,
		`{"email_batch":[{"address":"a@b.co"}],"errors":[]}`,
	}
	for _, body := range rows {
		var out ValidateBatchResponse
		if err := decodeJSONBody(200, "application/json", []byte(body), &out); err == nil {
			t.Errorf("batch body %s was accepted", body)
		}
	}
	var ok ValidateBatchResponse
	good := `{"email_batch":[{"address":"a@b.co","status":"valid"}],"errors":[]}`
	if err := decodeJSONBody(200, "application/json", []byte(good), &ok); err != nil {
		t.Errorf("good batch rejected: %v", err)
	}
}

// A null body is refused for a typed result whatever whitespace surrounds it,
// and a raw-JSON result still takes it.
func TestNullBodyTrimmingIsPinned(t *testing.T) {
	for _, body := range []string{"null", " null", "null ", "\tnull", "null\t", "\rnull", "null\r", "\nnull", "null\n", " \t\r\nnull \t\r\n"} {
		var out struct{ A int } // no required-field check of its own
		if err := decodeJSONBodyAllowMessage(200, "application/json", []byte(body), &out); err == nil {
			t.Errorf("typed result accepted body %q", body)
		}
		var raw interface{}
		if err := decodeJSONBodyAllowMessage(200, "application/json", []byte(body), &raw); err != nil {
			t.Errorf("raw result refused body %q: %v", body, err)
		}
	}
}

type csvTransport struct{}

func (csvTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	header := make(http.Header)
	header.Set("Content-Type", "text/csv")
	return &http.Response{StatusCode: 200, Header: header, Request: req,
		Body: io.NopCloser(strings.NewReader("a,b\n1,2\n"))}, nil
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestDownloadWriterErrorKeepsItsCause(t *testing.T) {
	useFake(t)
	http.DefaultTransport = csvTransport{}
	API_KEY = "mock_key"
	cause := errors.New("disk full")
	err := BulkValidationResult("0123456789", failingWriter{err: cause})
	if !errors.Is(err, cause) {
		t.Errorf("err = %v, want it to wrap %v", err, cause)
	}
	if err == nil || !strings.Contains(err.Error(), "could not write into given file") {
		t.Errorf("err = %v, lost its message", err)
	}
}
