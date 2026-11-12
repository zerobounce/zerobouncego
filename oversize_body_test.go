package zerobouncego

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// oversizeTransport answers every request with one JSON body larger than
// maxJSONBodyBytes, at a fixed status (201 for a file submit when the status
// is 2xx, the only success GenericFileSubmit takes).
type oversizeTransport struct {
	status int
	body   string
	last   int
}

func (f *oversizeTransport) statusFor(req *http.Request) int {
	if f.status == 200 && strings.HasSuffix(req.URL.Path, "/sendfile") {
		return 201
	}
	return f.status
}

func (f *oversizeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.last = f.statusFor(req)
	header := make(http.Header)
	header.Set("Content-Type", CONTENT_TYPE_JSON)
	return &http.Response{
		StatusCode:    f.last,
		Header:        header,
		Body:          io.NopCloser(strings.NewReader(f.body)),
		ContentLength: int64(len(f.body)),
		Request:       req,
	}, nil
}

// A response larger than the JSON size cap keeps its HTTP status: at a 500 it
// is a *ZBApiError with 500 on every call, at a 2xx an API error with that 2xx
// status on every capped call (never a status-less error). The body is valid
// JSON a little over the cap, so a capped call that read past the cap would
// take it as a result, and the count of size-cap errors drops.
func TestABodyOverTheSizeCapKeepsItsStatus(t *testing.T) {
	downloads := downloadCalls(t)
	// The documented cap is 2 MiB (CHANGELOG); the body is just over it.
	big := `{"a":"` + strings.Repeat("x", 2<<20) + `"}`
	for _, status := range []int{200, 500} {
		useFake(t)
		fake := &oversizeTransport{status: status, body: big}
		http.DefaultTransport = fake
		client, warmup := pointEverythingAt(t, "https://api.example.test")
		calls := publicRequestCalls(client, warmup)
		capped := 0
		for _, call := range calls {
			// The file downloads stream to disk and the Warmup client reads
			// without a cap; at a 2xx they are checked by their own tests.
			if status < 300 && (downloads[call.name] || strings.HasPrefix(call.name, "WarmupClient.")) {
				continue
			}
			fake.last = 0
			err := call.call()
			if fake.last == 0 {
				t.Fatalf("%d %s: no request reached the transport", status, call.name)
			}
			// A 2xx call that reads without a cap may take the valid JSON
			// object as its result; anything it reports carries the status.
			if err == nil && status < 300 {
				continue
			}
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) {
				t.Fatalf("%d %s: err = %#v, want a *ZBApiError", status, call.name, err)
			}
			if apiErr.StatusCode != fake.last {
				t.Fatalf("%d %s: status %d", status, call.name, apiErr.StatusCode)
			}
			if apiErr.Message == oversizeBodyMessage {
				capped++
			}
		}
		t.Logf("%d: %d of %d calls reported the size cap", status, capped, len(calls))
		if capped < 25 {
			t.Fatalf("%d: only %d calls reported the size cap", status, capped)
		}
	}
}
