package zerobouncego

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"
	"time"
)

// realNetwork lets a test reach a local server through the SDK's own
// transport (useFake restores everything afterwards).
func realNetwork(t *testing.T) {
	t.Helper()
	real := http.DefaultTransport
	useFake(t)
	http.DefaultTransport = real
}

// A Warmup body that stalls past the client timeout, or is cut short, keeps
// the error's type (net.Error, and errors.Is as net/http gives it), like the
// main client; the key stays out of every layer.
func TestWarmupBodyReadErrorsKeepTheirType(t *testing.T) {
	realNetwork(t)
	stall := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		fmt.Fprintf(buffered, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{\"a\":")
		buffered.Flush()
		if stall {
			time.Sleep(600 * time.Millisecond)
		}
	}))
	t.Cleanup(server.Close)
	old := http.DefaultClient.Timeout
	http.DefaultClient.Timeout = 200 * time.Millisecond
	t.Cleanup(func() { http.DefaultClient.Timeout = old })
	_, warmup := pointEverythingAt(t, server.URL)

	// What net/http itself gives on this Go version for the same body.
	control := &http.Client{Timeout: 200 * time.Millisecond}
	read := func() error {
		resp, err := control.Get(server.URL + "/x")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		return err
	}

	stall = true
	wantDeadline := errors.Is(read(), context.DeadlineExceeded)
	_, err := warmup.ListSendingAccounts()
	var netErr net.Error
	if err == nil || !errors.As(err, &netErr) || !netErr.Timeout() || errors.Is(err, context.DeadlineExceeded) != wantDeadline {
		t.Errorf("stalled body: %#v (net/http: errors.Is=%v)", err, wantDeadline)
	}
	for _, layer := range r4Layers(err) {
		if strings.Contains(layer, "sweep-key") {
			t.Errorf("leak: %s", layer)
		}
	}

	stall = false
	wantEOF := errors.Is(read(), io.ErrUnexpectedEOF)
	_, err = warmup.ListSendingAccounts()
	if err == nil || errors.Is(err, io.ErrUnexpectedEOF) != wantEOF {
		t.Errorf("cut-short body: %#v (net/http: errors.Is(ErrUnexpectedEOF)=%v)", err, wantEOF)
	}
}

// r6FormatErr prints the request only under %s (as *url.Error formats its
// inner error); Error, %v, %+v and %#v stay clean.
type r6FormatErr struct{ dump string }

func (e *r6FormatErr) Error() string { return "mock: clean" }

func (e *r6FormatErr) Format(f fmt.State, verb rune) {
	if verb == 's' {
		io.WriteString(f, e.dump)
		return
	}
	io.WriteString(f, "mock: clean")
}

// r6Hidden keeps the request in a field behind a pointer.
type r6Hidden struct{ request string }

func (e *r6Hidden) Error() string { return "mock: hidden" }

type r6Transport struct{ build func(dump string) error }

func (tr r6Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, _ := httputil.DumpRequestOut(req, true)
	if req.Body != nil {
		req.Body.Close()
	}
	return nil, tr.build(string(dump))
}

// A cause that prints the key only under %s, or a pointer cause hidden inside
// errors.Join, is redacted too: no key in err.Error() or any layer.
func TestFormatOnlyAndJoinedCausesLeakNoKey(t *testing.T) {
	for name, build := range map[string]func(string) error{
		"format-s": func(dump string) error { return &r6FormatErr{dump: dump} },
		"join-ptr": func(dump string) error { return errors.Join(errors.New("a"), &r6Hidden{request: dump}) },
	} {
		useFake(t)
		http.DefaultTransport = r6Transport{build: build}
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
					t.Errorf("%s/%s leaks the key: %.200s", name, call.name, layer)
				}
			}
		}
		if failed < 60 {
			t.Fatalf("%s: only %d calls reached the transport error", name, failed)
		}
	}
}

// r6HTMLBody is an HTML page larger than the cap, sent as text/csv.
type r6HTMLTransport struct{}

func (r6HTMLTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		req.Body.Close()
	}
	body := "<!DOCTYPE html><html><head><title>x</title></head><body>" + strings.Repeat("a", 3<<20) + "</body></html>"
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/csv"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

// An oversize HTML page under a non-HTML content type is refused by the body
// check of the download cap.
func TestOversizeHTMLBodyUnderCSVIsRefused(t *testing.T) {
	useFake(t)
	http.DefaultTransport = r6HTMLTransport{}
	pointEverythingAt(t, "https://api.example.test")
	sweepDownloads.Reset()
	err := BulkValidationResult(testing_file_id, &sweepDownloads)
	var apiErr *ZBApiError
	if !errors.As(err, &apiErr) || apiErr.Message != oversizeBodyMessage {
		t.Errorf("err = %#v", err)
	}
	if sweepDownloads.Len() != 0 {
		t.Errorf("%d bytes written", sweepDownloads.Len())
	}
}

// A chunked body cut mid-stream is an error on every call, and a download
// writes nothing.
func TestEveryRequestRejectsAChunkedBodyCutMidStream(t *testing.T) {
	realNetwork(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		writer := bufio.NewWriter(buffered)
		fmt.Fprintf(writer, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\n\r\n5\r\n{\"Cre\r\n40\r\nonly part")
		writer.Flush()
		buffered.Flush()
	}))
	t.Cleanup(server.Close)
	client, warmup := pointEverythingAt(t, server.URL)
	for _, call := range publicRequestCalls(client, warmup) {
		sweepDownloads.Reset()
		err := call.call()
		if err == nil {
			t.Errorf("%s: a chunked body cut mid-stream was a success", call.name)
			continue
		}
		if strings.Contains(err.Error(), "sweep-key") {
			t.Errorf("%s: error carries the key: %v", call.name, err)
		}
		if sweepDownloads.Len() != 0 {
			t.Errorf("%s: wrote %d bytes", call.name, sweepDownloads.Len())
		}
	}
}

// A refused connection (a real closed loopback port) is a plain error on every
// call: never a *ZBApiError, never the key.
func TestEveryRequestOnAClosedPortIsATransportError(t *testing.T) {
	realNetwork(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + listener.Addr().String()
	listener.Close()
	client, warmup := pointEverythingAt(t, base)
	for _, call := range publicRequestCalls(client, warmup) {
		err := call.call()
		if err == nil {
			t.Errorf("%s: no error on a closed port", call.name)
			continue
		}
		var apiErr *ZBApiError
		if errors.As(err, &apiErr) {
			t.Errorf("%s: a refused connection is a *ZBApiError: %v", call.name, err)
		}
		for _, layer := range r4Layers(err) {
			if strings.Contains(layer, "sweep-key") {
				t.Errorf("%s: leak: %s", call.name, layer)
			}
		}
	}
}
