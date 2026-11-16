package zerobouncego

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"testing"
	"time"
)

const r4Key = "sweep-key"

// r4DumpTransport fails the way a debugging transport does: its error is the
// whole request, headers and body included.
type r4DumpTransport struct{ cause error }

func (d r4DumpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, _ := httputil.DumpRequestOut(req, true)
	if req.Body != nil {
		req.Body.Close()
	}
	if d.cause != nil {
		return nil, fmt.Errorf("mock: no responder for request:\n%s: %w", dump, d.cause)
	}
	return nil, fmt.Errorf("mock: no responder for request:\n%s", dump)
}

func r4Layers(err error) []string {
	var out []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		out = append(out, fmt.Sprintf("%v|%+v|%#v|%s", e, e, e, e.Error()))
		if multi, ok := e.(interface{ Unwrap() []error }); ok {
			for _, inner := range multi.Unwrap() {
				out = append(out, r4Layers(inner)...)
			}
		}
	}
	return out
}

// Every public call, with a transport whose error is the full request dump
// (the key in the query, the API-Key header, the bearer header, a form, JSON
// or multipart body), leaks the key in no layer of the error.
func TestDumpingTransportErrorLeaksNoKeyOnAnyCall(t *testing.T) {
	cause := errors.New("r4 cause")
	useFake(t)
	http.DefaultTransport = r4DumpTransport{cause: cause}
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	calls := publicRequestCalls(client, warmup)
	if len(calls) < 100 {
		t.Fatalf("only %d calls swept", len(calls))
	}
	failed := 0
	for _, call := range calls {
		err := call.call()
		if err == nil {
			continue
		}
		failed++
		for _, layer := range r4Layers(err) {
			if strings.Contains(layer, r4Key) {
				t.Errorf("%s leaks the key: %s", call.name, layer)
			}
		}
	}
	if failed < 100 {
		t.Fatalf("only %d calls reached the transport error", failed)
	}
}

// A body that is not replayable (multipart streams through a pipe) is covered
// by the keys the SDK remembers.
func TestDumpingTransportErrorMasksMultipartAndBearer(t *testing.T) {
	useFake(t)
	http.DefaultTransport = r4DumpTransport{}
	Initialize("mp-bearer-key-4242")
	URI, BULK_URI = "https://api.example.test/v2/", "https://api.example.test/v2/"
	INBOX_PLACEMENT_URI = "https://api.example.test/api/ipt"
	for name, call := range map[string]func() error{
		"multipart": func() error {
			_, e := BulkValidationSubmit(CsvFile{File: strings.NewReader("a@example.com\n"), FileName: "f.csv", EmailAddressColumn: 1}, false)
			return e
		},
		"bearer": func() error { _, e := GetInboxPlacementProviders(); return e },
		"json":   func() error { _, e := ValidateBatch([]EmailToValidate{{EmailAddress: "a@example.com"}}); return e },
	} {
		err := call()
		if err == nil || strings.Contains(strings.Join(r4Layers(err), "\n"), "mp-bearer-key-4242") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// requestSecrets itself finds every place the SDK puts the key.
func TestRequestSecretsFindsEveryPlace(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, s := range list {
			if s == want {
				return true
			}
		}
		return false
	}
	query, _ := http.NewRequest("GET", "https://h/?%61pi_KEY=q-secret", nil)
	header, _ := http.NewRequest("GET", "https://h/", nil)
	header.Header.Set("API-Key", "h-secret")
	bearer, _ := http.NewRequest("GET", "https://h/", nil)
	bearer.Header.Set("Authorization", "Bearer b-secret")
	form, _ := http.NewRequest("POST", "https://h/", strings.NewReader("a=1&api_key=f-secret"))
	jsonBody, _ := http.NewRequest("POST", "https://h/", strings.NewReader(`{"api_key":"j-secret","email_batch":[]}`))
	multi, _ := http.NewRequest("POST", "https://h/", strings.NewReader("--B\r\nContent-Disposition: form-data; name=\"api_key\"\r\n\r\nm-secret\r\n--B--\r\n"))
	multi.Header.Set("Content-Type", "multipart/form-data; boundary=B")
	for want, req := range map[string]*http.Request{"q-secret": query, "h-secret": header, "b-secret": bearer, "f-secret": form, "j-secret": jsonBody, "m-secret": multi} {
		if !has(requestSecrets(req), want) {
			t.Errorf("%s not found in %v", want, requestSecrets(req))
		}
	}
}

type r4ValueErr struct{ URL string }

func (e r4ValueErr) Error() string { return "no responder for " + e.URL }

// %#v of the redacted error (and of every layer) holds nothing of a value-typed
// cause that carried the key.
func TestRedactedTransportErrorKeepsNoRawCause(t *testing.T) {
	useFake(t)
	http.DefaultTransport = rtFunc4(func(r *http.Request) (*http.Response, error) {
		return nil, r4ValueErr{URL: r.URL.String()}
	})
	Initialize("value-cause-key-99")
	URI = "https://api.example.test/v2/"
	_, err := GetCredits()
	if err == nil {
		t.Fatal("no error")
	}
	for _, layer := range r4Layers(err) {
		if strings.Contains(layer, "value-cause-key-99") {
			t.Errorf("leak: %s", layer)
		}
	}
	var redacted *redactedTransportError
	if !errors.As(err, &redacted) {
		t.Fatalf("not a redactedTransportError: %T", err)
	}
	if got := fmt.Sprintf("%#v", redacted); strings.Contains(got, "value-cause-key-99") {
		t.Errorf("%%#v leaks: %s", got)
	}
}

type rtFunc4 func(*http.Request) (*http.Response, error)

func (f rtFunc4) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type r4TimeoutErr struct{ msg string }

func (e *r4TimeoutErr) Error() string   { return e.msg }
func (e *r4TimeoutErr) Timeout() bool   { return true }
func (e *r4TimeoutErr) Temporary() bool { return true }

// Timeout, Temporary and errors.Is for the standard sentinels survive the
// redaction.
func TestRedactedTransportErrorKeepsTimeoutTemporaryAndSentinels(t *testing.T) {
	Initialize("tmo-key-0123")
	URI = "https://api.example.test/v2/"
	for _, sentinel := range []error{context.DeadlineExceeded, context.Canceled, os.ErrDeadlineExceeded, net.ErrClosed} {
		useFake(t)
		Initialize("tmo-key-0123")
		URI = "https://api.example.test/v2/"
		http.DefaultTransport = rtFunc4(func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("%w: %w", sentinel, &r4TimeoutErr{msg: "timed out " + r.URL.String()})
		})
		_, err := GetCredits()
		if err == nil || strings.Contains(err.Error(), "tmo-key-0123") {
			t.Fatalf("err = %v", err)
		}
		var netErr net.Error
		if !errors.Is(err, sentinel) {
			t.Errorf("errors.Is(%v) lost", sentinel)
		}
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Errorf("%v: Timeout() lost", sentinel)
		}
		var temp interface{ Temporary() bool }
		if !errors.As(err, &temp) || !temp.Temporary() {
			t.Errorf("%v: Temporary() lost", sentinel)
		}
	}
	// A cause that is not temporary stays not temporary, not a timeout.
	useFake(t)
	Initialize("tmo-key-0123")
	URI = "https://api.example.test/v2/"
	http.DefaultTransport = rtFunc4(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("plain " + r.URL.String())
	})
	_, err := GetCredits()
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Errorf("a plain error became a timeout: %v", err)
	}
	var temp interface{ Temporary() bool }
	if errors.As(err, &temp) && temp.Temporary() {
		t.Errorf("a plain error became temporary: %v", err)
	}
}

// A transport error without a secret is returned untouched.
func TestTransportErrorWithoutSecretPassesThrough(t *testing.T) {
	useFake(t)
	Initialize("pass-key-0123")
	URI = "https://api.example.test/v2/"
	cause := &r4TimeoutErr{msg: "plain failure"}
	http.DefaultTransport = rtFunc4(func(r *http.Request) (*http.Response, error) { return nil, cause })
	_, err := GetCredits()
	var got *r4TimeoutErr
	if !errors.As(err, &got) || got != cause {
		t.Errorf("err = %#v", err)
	}
}

// A Warmup client timeout keeps errors.Is(DeadlineExceeded) and net.Error.
func TestWarmupTimeoutKeepsTheCauseAndMasksTheKey(t *testing.T) {
	useFake(t)
	old := http.DefaultClient.Timeout
	http.DefaultClient.Timeout = 50 * time.Millisecond
	t.Cleanup(func() { http.DefaultClient.Timeout = old })
	http.DefaultTransport = rtFunc4(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	warmup, err := NewWarmupClient("warmup-key-r4-0123", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = warmup.ListSendingAccounts()
	if err == nil {
		t.Fatal("no error")
	}
	// Before Go 1.23 net/http's client-timeout error does not match
	// context.DeadlineExceeded, so compare with what net/http itself gives
	// for the same timeout on this Go version.
	control := &http.Client{Timeout: 50 * time.Millisecond, Transport: http.DefaultTransport}
	_, controlErr := control.Get("https://control.example.test/")
	wantIs := errors.Is(controlErr, context.DeadlineExceeded)
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) != wantIs || !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("timeout lost: %#v (net/http itself: errors.Is=%v)", err, wantIs)
	}
	// The same through a dumping transport: the key and the call's secret are
	// masked and the timeout still answers.
	http.DefaultTransport = rtFunc4(func(r *http.Request) (*http.Response, error) {
		dump, _ := httputil.DumpRequestOut(r, true)
		<-r.Context().Done()
		return nil, fmt.Errorf("%s: %w", dump, r.Context().Err())
	})
	_, err = warmup.TestSMTPConfiguration("a@example.com", "user", "smtp-secret-r4", "smtp.example.com", 587)
	if err == nil {
		t.Fatal("no error")
	}
	for _, layer := range r4Layers(err) {
		if strings.Contains(layer, "warmup-key-r4-0123") || strings.Contains(layer, "smtp-secret-r4") {
			t.Errorf("leak: %s", layer)
		}
	}
	if errors.Is(err, context.DeadlineExceeded) != wantIs || !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("timeout lost after redaction: %#v", err)
	}
}

// The API-Key header and a form body are masked on their own, with the key
// unknown to the SDK's registry.
func TestRedactingTransportMasksHeaderAndFormSecrets(t *testing.T) {
	for name, build := range map[string]func() *http.Request{
		"header": func() *http.Request {
			r, _ := http.NewRequest("GET", "https://h/", nil)
			r.Header.Set("API-Key", "only-in-header-r4")
			return r
		},
		"form": func() *http.Request {
			r, _ := http.NewRequest("POST", "https://h/", strings.NewReader("api_key=only-in-form-r4&x=1"))
			return r
		},
	} {
		secret := "only-in-" + name + "-r4"
		transport := redactingTransport{inner: rtFunc4(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("echo " + secret)
		})}
		_, err := transport.RoundTrip(build())
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// An error that echoes the key only in an encoded form is replaced whole.
func TestRedactingTransportDropsAnEncodedEcho(t *testing.T) {
	r, _ := http.NewRequest("GET", "https://h/?api_key=enc+key/r4", nil)
	transport := redactingTransport{inner: rtFunc4(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("echo enc%2bkey%2fr4")
	})}
	_, err := transport.RoundTrip(r)
	if err == nil || err.Error() != credentialOmittedMessage {
		t.Errorf("err = %v", err)
	}
}

// redactedURL masks by the decoded parameter name, and an unparsable URL is
// handled (masked, or not printed when it holds an @).
func TestRedactedURLMasksDecodedNamesAndUnparsableURLs(t *testing.T) {
	const secret = "r4-url-secret"
	for _, raw := range []string{
		"https://h/v2/?%61pi_key=" + secret,
		"https://h/v2/?api%5Fkey=" + secret,
		"https://h/v2/?API_KEY=" + secret + "&a=1",
		"https://h/v2/#api_key=" + secret,
		"http://[::1/?api_key=" + secret,
	} {
		got := redactedURL(raw)
		if strings.Contains(got, secret) || !strings.Contains(got, "REDACTED") {
			t.Errorf("redactedURL(%q) = %q", raw, got)
		}
	}
	if got := redactedURL("http://[::1/u:p@x?api_key=" + secret); got != "(url redacted)" {
		t.Errorf("unparsable URL with @ = %q", got)
	}
	if got := redactedURL("http://[::1/?other=1"); got != "http://[::1/?other=1" {
		t.Errorf("unparsable URL kept as is = %q", got)
	}
}

// The masking stops at the end of the value: the ':' after it stays.
func TestAPIKeyMaskKeepsTheColonAfterTheValue(t *testing.T) {
	r, _ := http.NewRequest("GET", "https://h/?api_key=other-key-r4", nil)
	transport := redactingTransport{inner: rtFunc4(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("api_key=SomeUnknownValue: context deadline exceeded")
	})}
	_, err := transport.RoundTrip(r)
	if err == nil || err.Error() != "api_key=REDACTED: context deadline exceeded" {
		t.Errorf("err = %v", err)
	}
	if got := redactAPIKeyParams("x=1?api_key=K9&y=2;z"); got != "x=1?api_key=REDACTED&y=2;z" {
		t.Errorf("got %q", got)
	}
}

var _ = io.Discard

// A key the SDK sent earlier is masked even when the failing request does not
// carry it (a body that cannot be replayed).
func TestRedactingTransportMasksRememberedKeys(t *testing.T) {
	const remembered = "remembered-key-r4-77"
	rememberKey(remembered)
	r, _ := http.NewRequest("POST", "https://h/", io.NopCloser(strings.NewReader("streamed")))
	transport := redactingTransport{inner: rtFunc4(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("echo " + remembered)
	})}
	_, err := transport.RoundTrip(r)
	if err == nil || strings.Contains(err.Error(), remembered) {
		t.Errorf("err = %v", err)
	}
}
