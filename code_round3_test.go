package zerobouncego

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// r3Transport answers every request with one status, content type and body.
// The body is read through a counter so a test can tell how much of it the
// SDK consumed.
type r3Transport struct {
	status      int
	contentType string
	size        int
	fill        byte
	read        int
}

type r3Body struct {
	owner *r3Transport
	left  int
	fill  byte
}

func (b *r3Body) Read(p []byte) (int, error) {
	if b.left == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > b.left {
		n = b.left
	}
	for i := 0; i < n; i++ {
		p[i] = b.fill
	}
	b.left -= n
	b.owner.read += n
	return n, nil
}

func (b *r3Body) Close() error { return nil }

func (f *r3Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.read = 0
	header := make(http.Header)
	header.Set("Content-Type", f.contentType)
	return &http.Response{
		StatusCode: f.status,
		Header:     header,
		Body:       &r3Body{owner: f, left: f.size, fill: f.fill},
		Request:    req,
	}, nil
}

// A 3 MiB Warmup CSV download is returned whole (downloads are not capped),
// and so is a 3 MiB getfile written to the caller's writer.
func TestDownloadsOverTheJSONCapAreNotCapped(t *testing.T) {
	const size = 3 << 20
	useFake(t)
	fake := &r3Transport{status: 200, contentType: "text/csv", size: size, fill: 'a'}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	_ = client

	seeds, err := warmup.DownloadSeeds()
	if err != nil || len(seeds) != size {
		t.Fatalf("DownloadSeeds: len %d err %v", len(seeds), err)
	}
	stats, err := warmup.DownloadStatistics("2024-01-01")
	if err != nil || len(stats) != size {
		t.Fatalf("DownloadStatistics: len %d err %v", len(stats), err)
	}
	var out bytes.Buffer
	if err := BulkValidationResult(testing_file_id, &out); err != nil || out.Len() != size {
		t.Fatalf("BulkValidationResult: len %d err %v", out.Len(), err)
	}
	out.Reset()
	if err := BulkEmailFinderResult(testing_file_id, &out); err != nil || out.Len() != size {
		t.Fatalf("BulkEmailFinderResult: len %d err %v", out.Len(), err)
	}
}

// A body over the cap that is not a file (JSON or HTML on a 200) is still
// refused with the size-cap error on the downloads.
func TestOversizeNonFileBodyOnADownloadIsRefused(t *testing.T) {
	for _, contentType := range []string{"application/json", "text/html"} {
		useFake(t)
		fake := &r3Transport{status: 200, contentType: contentType, size: 3 << 20, fill: '{'}
		http.DefaultTransport = fake
		_, warmup := pointEverythingAt(t, "https://api.example.test")
		var out bytes.Buffer
		for name, err := range map[string]error{
			"getfile":       BulkValidationResult(testing_file_id, &out),
			"DownloadSeeds": func() error { _, e := warmup.DownloadSeeds(); return e }(),
		} {
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) || apiErr.Message != oversizeBodyMessage {
				t.Errorf("%s (%s): err = %#v", name, contentType, err)
			}
		}
		if out.Len() != 0 {
			t.Errorf("%s: %d bytes written", contentType, out.Len())
		}
	}
}

// An 8 MiB error body on a download is read up to the 2 MiB limit only, and
// reported as the size-cap error with the response's status.
func TestOversizeErrorBodyOnADownloadIsLimited(t *testing.T) {
	const size = 8 << 20
	for _, status := range []int{500, 404} {
		useFake(t)
		fake := &r3Transport{status: status, contentType: "text/plain", size: size, fill: 'e'}
		http.DefaultTransport = fake
		_, warmup := pointEverythingAt(t, "https://api.example.test")
		calls := map[string]func() error{
			"BulkValidationResult":   func() error { return BulkValidationResult(testing_file_id, io.Discard) },
			"GenericResultFetch":     func() error { return GenericResultFetch(testing_file_id, ENDPOINT_FILE_RESULT, io.Discard) },
			"BulkEmailFinderResult":  func() error { return BulkEmailFinderResult(testing_file_id, io.Discard) },
			"BulkDomainSearchResult": func() error { return BulkDomainSearchResult(testing_file_id, io.Discard) },
			"DownloadSeeds":          func() error { _, e := warmup.DownloadSeeds(); return e },
		}
		for name, call := range calls {
			err := call()
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != status || apiErr.Message != oversizeBodyMessage {
				t.Errorf("%d %s: err = %#v", status, name, err)
			}
			if fake.read > maxJSONBodyBytes+1<<20 {
				t.Errorf("%d %s: read %d bytes of the body", status, name, fake.read)
			}
		}
	}
}

// quotingTransport fails the way httpmock does: its error quotes the URL.
type quotingTransport struct{ cause error }

func (q quotingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		req.Body.Close()
	}
	return nil, fmt.Errorf("no responder found for %q: %w", req.Method+" "+req.URL.String(), q.cause)
}

// A caller's transport that quotes the URL in its error does not leak the key
// through err.Error() or errors.Unwrap, on a GET, a form POST, a JSON POST and
// a multipart upload; errors.Is still sees the cause.
func TestTransportErrorQuotingTheURLDoesNotLeakTheKey(t *testing.T) {
	const key = "zb-secret-KEY-r3-0123456789"
	cause := errors.New("round3 cause")
	useFake(t)
	http.DefaultTransport = quotingTransport{cause: cause}
	Initialize(key)
	URI = "https://api.example.test/v2/"
	BULK_URI = "https://api.example.test/v2/"
	calls := map[string]func() error{
		"GetCredits":     func() error { _, e := GetCredits(); return e },
		"Validate":       func() error { _, e := Validate("a@example.com", ""); return e },
		"ScoreEmailPost": func() error { _, e := ScoreEmailPost("a@example.com"); return e },
		"ValidateBatch":  func() error { _, e := ValidateBatch([]EmailToValidate{{EmailAddress: "a@example.com"}}); return e },
		"FileStatus":     func() error { _, e := GenericFileStatusCheck(testing_file_id, ENDPOINT_FILE_STATUS); return e },
		"Download":       func() error { return BulkValidationResult(testing_file_id, io.Discard) },
	}
	for name, call := range calls {
		err := call()
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		for depth, e := 0, error(err); e != nil && depth < 10; depth, e = depth+1, errors.Unwrap(e) {
			if strings.Contains(e.Error(), key) {
				t.Errorf("%s: layer %d (%T) leaks the key: %v", name, depth, e, e)
			}
		}
		if !errors.Is(err, cause) {
			t.Errorf("%s: errors.Is lost the cause: %v", name, err)
		}
	}
}

// The api_key query value in Client.URI / BulkURI is masked when the client is
// printed, marshalled or formatted, whatever the case of the name.
func TestClientPrintMasksAPIKeyQueryInURI(t *testing.T) {
	const secret = "zb-query-secret-r3"
	for _, name := range []string{"api_key", "API_KEY", "Api_Key"} {
		c := New("k")
		c.URI = "https://api.example.test/v2/?" + name + "=" + secret + "&x=1"
		c.BulkURI = "https://u:pw@bulk.example.test/v2/?" + name + "=" + secret
		outputs := []string{c.String(), c.GoString(), fmt.Sprintf("%v %+v %#v %s", c, c, c, c)}
		if raw, err := json.Marshal(c); err == nil {
			outputs = append(outputs, string(raw))
		}
		for _, out := range outputs {
			if strings.Contains(out, secret) || strings.Contains(out, "pw@") {
				t.Errorf("%s: output leaks: %s", name, out)
			}
		}
		if !strings.Contains(c.String(), "REDACTED") {
			t.Errorf("%s: %s", name, c.String())
		}
	}
}

// The account-level batch failure is recognised whatever the case or the
// padding of the "all" address.
func TestBatchAllRowIsCaseAndSpaceInsensitive(t *testing.T) {
	for _, address := range []string{"ALL", " all ", "All", "\tall\n"} {
		quoted, _ := json.Marshal(address)
		body := `{"email_batch":[],"errors":[{"email_address":` + string(quoted) + `,"error":"Invalid API Key or your account ran out of credits"}]}`
		withStatusResponder(t, 200, "application/json", body)
		_, err := ValidateBatch([]EmailToValidate{{EmailAddress: "a@example.com"}})
		var apiErr *ZBApiError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 200 || apiErr.Message != "Invalid API Key or your account ran out of credits" {
			t.Errorf("address %q: err = %#v", address, err)
		}
	}
}

// Validate's message-only body ({"message":"Invalid key"}) keeps the API's
// text as the error message.
func TestValidateMessageOnlyBodyKeepsTheAPIText(t *testing.T) {
	for _, body := range []string{`{"message":"Invalid key"}`} {
		for _, status := range []int{200, 400} {
			withStatusResponder(t, status, "application/json", body)
			_, err := Validate("a@example.com", "")
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) {
				t.Fatalf("%d %s: err = %#v", status, body, err)
			}
			if !strings.Contains(apiErr.Message, "Invalid key") || apiErr.StatusCode != status {
				t.Errorf("%d %s: %+v", status, body, apiErr)
			}
		}
	}
}
