package zerobouncego

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"testing"
)

// r7ValueErr is a value-typed error that carries the dumped request.
type r7ValueErr struct{ dump string }

func (e r7ValueErr) Error() string { return "read failed: " + e.dump }

// r7FailingBody fails every Read with an error that quotes the request.
type r7FailingBody struct{ dump string }

func (b r7FailingBody) Read([]byte) (int, error) { return 0, r7ValueErr{b.dump} }
func (b r7FailingBody) Close() error             { return nil }

type r7BodyTransport struct{}

func (r7BodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, _ := httputil.DumpRequestOut(req, true)
	if req.Body != nil {
		req.Body.Close()
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: r7FailingBody{string(dump)}, Request: req}, nil
}

// A response body whose Read error quotes the request leaks the key on no
// public call (main, Client, batch, multipart, Inbox Placement, Warmup).
func TestBodyReadErrorQuotingTheRequestLeaksNoKey(t *testing.T) {
	useFake(t)
	http.DefaultTransport = r7BodyTransport{}
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	failed := 0
	for _, call := range publicRequestCalls(client, warmup) {
		err := call.call()
		if err == nil {
			continue
		}
		failed++
		for _, layer := range append(r4Layers(err), err.Error()) {
			if strings.Contains(layer, r4Key) {
				t.Errorf("%s leaks the key: %.200s", call.name, layer)
			}
		}
	}
	if failed < 90 {
		t.Fatalf("only %d calls reached the body error", failed)
	}
}

// The Warmup SMTP test's password (not a key the transport knows) is masked
// in a body-read error too.
func TestWarmupBodyReadErrorMasksTheSMTPPassword(t *testing.T) {
	useFake(t)
	http.DefaultTransport = r7BodyTransport{}
	_, warmup := pointEverythingAt(t, "https://api.example.test")
	_, err := warmup.TestSMTPConfiguration("a@example.com", "smtp-user-r7", "smtp-secret-r7-0123", "smtp.example.com", 587)
	if err == nil {
		t.Fatal("no error")
	}
	for _, layer := range append(r4Layers(err), err.Error()) {
		if strings.Contains(layer, "smtp-secret-r7-0123") {
			t.Errorf("leak: %.200s", layer)
		}
	}
}

// r7SOnly prints the request only under %s; %v, %+v, %#v and %q stay clean.
type r7SOnly struct{ dump string }

func (e *r7SOnly) Error() string { return "mock: clean" }

func (e *r7SOnly) Format(f fmt.State, verb rune) {
	switch verb {
	case 's':
		io.WriteString(f, e.dump)
	case 'q':
		io.WriteString(f, `"mock: clean"`)
	default:
		io.WriteString(f, "mock: clean")
	}
}

var r7Sentinel = errors.New("r7 caller sentinel")

// A cause that leaks only under %s is redacted, and a caller's own sentinel
// joined next to a layer that quotes the request still matches errors.Is.
func TestSOnlyCauseAndJoinedSentinel(t *testing.T) {
	useFake(t)
	http.DefaultTransport = r6Transport{build: func(dump string) error { return &r7SOnly{dump: dump} }}
	pointEverythingAt(t, "https://api.example.test")
	_, err := GetCredits()
	if err == nil || strings.Contains(err.Error(), r4Key) {
		t.Fatalf("%%s-only cause: %v", err)
	}
	for _, layer := range r4Layers(err) {
		if strings.Contains(layer, r4Key) {
			t.Errorf("%%s-only cause leaks: %.200s", layer)
		}
	}

	http.DefaultTransport = r6Transport{build: func(dump string) error {
		// A value-typed cause makes the Join layer itself unsafe (its %#v
		// prints the dump), so the sentinel is found only by walking into it.
		return errors.Join(r7Sentinel, r7ValueErr{dump: dump})
	}}
	_, err = GetCredits()
	if !errors.Is(err, r7Sentinel) {
		t.Errorf("caller sentinel lost through errors.Join: %#v", err)
	}
	for _, layer := range r4Layers(err) {
		if strings.Contains(layer, r4Key) {
			t.Errorf("joined cause leaks: %.200s", layer)
		}
	}
}
