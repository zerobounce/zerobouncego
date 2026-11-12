package zerobouncego

// Guards shared by every request: the request timeout and truncated bodies,
// swept over publicRequestCalls so a new call cannot escape them; and the
// Warmup client's redirect refusal, secret redaction and CSV check.

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// deadlineTransport records the context deadline each request carries and
// answers like fakeTransport.
type deadlineTransport struct {
	mu        sync.Mutex
	deadlines []time.Time
	missing   int
}

func (d *deadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline, ok := req.Context().Deadline()
	d.mu.Lock()
	if ok {
		d.deadlines = append(d.deadlines, deadline)
	} else {
		d.missing++
	}
	d.mu.Unlock()
	return fakeResponse(recordHTTPRequest(req)), nil
}

func (d *deadlineTransport) take() ([]time.Time, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	deadlines, missing := d.deadlines, d.missing
	d.deadlines, d.missing = nil, 0
	return deadlines, missing
}

// The http.Client timeout (httpTimeout) bounds connecting, sending and
// reading the body. Without it a server that stops answering hangs the caller
// for ever. The client puts it on each request as a context deadline.
func TestEveryRequestCarriesTheRequestTimeout(t *testing.T) {
	if httpTimeout != 120*time.Second {
		t.Fatalf("httpTimeout = %v, want 120s", httpTimeout)
	}
	useFake(t)
	transport := &deadlineTransport{}
	http.DefaultTransport = transport
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	for _, call := range publicRequestCalls(client, warmup) {
		t.Run(call.name, func(t *testing.T) {
			transport.take()
			start := time.Now()
			_ = call.call()
			deadlines, missing := transport.take()
			if missing > 0 {
				t.Fatalf("%d request(s) sent with no timeout", missing)
			}
			if len(deadlines) == 0 {
				t.Fatal("no request reached the transport")
			}
			for _, deadline := range deadlines {
				if left := deadline.Sub(start); left < httpTimeout-10*time.Second || left > httpTimeout+time.Second {
					t.Fatalf("request deadline is %v away, want about %v", left, httpTimeout)
				}
			}
		})
	}
}

// truncatingServer answers each request with the body fakeResponse gives it,
// but with a Content-Length 1000 bytes longer than that body, and then closes
// the connection. With truncate off it sends an honest Content-Length, the
// control that shows every call succeeds on that body when it arrives whole.
func truncatingServer(t *testing.T, truncate *bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := fakeResponse(recordHTTPRequest(r))
		body, _ := io.ReadAll(response.Body)
		contentType := response.Header.Get("Content-Type")
		if strings.Contains(r.URL.Path, warmupBasePath) && strings.Contains(r.URL.Path, "/download") {
			contentType, body = "text/csv", []byte("email\nseed@example.com\n")
		}
		length := len(body)
		if *truncate {
			length += 1000
		}
		conn, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		fmt.Fprintf(buffered, "HTTP/1.1 %d %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n",
			response.StatusCode, http.StatusText(response.StatusCode), contentType, length)
		buffered.Write(body)
		buffered.Flush()
	}))
	t.Cleanup(server.Close)
	return server
}

// A body cut short (the connection closes before Content-Length bytes arrive)
// is an error on every path, JSON calls and downloads alike, even when the
// bytes that did arrive are a complete success body. A download writes
// nothing.
func TestEveryRequestRejectsATruncatedBody(t *testing.T) {
	realTransport := http.DefaultTransport
	useFake(t)
	http.DefaultTransport = realTransport
	truncate := false
	server := truncatingServer(t, &truncate)
	client, warmup := pointEverythingAt(t, server.URL)
	for _, call := range publicRequestCalls(client, warmup) {
		t.Run(call.name, func(t *testing.T) {
			truncate = false
			sweepDownloads.Reset()
			if err := call.call(); err != nil {
				t.Fatalf("control: the whole body failed too, so the truncated case proves nothing: %v", err)
			}
			wrote := sweepDownloads.Len()

			truncate = true
			sweepDownloads.Reset()
			err := call.call()
			if err == nil {
				t.Fatal("a body cut short after its 200 header was reported as success")
			}
			if strings.Contains(err.Error(), "sweep-key") {
				t.Fatalf("error carries the key: %v", err)
			}
			if sweepDownloads.Len() != 0 {
				t.Fatalf("a truncated download wrote %d bytes (a whole one wrote %d)", sweepDownloads.Len(), wrote)
			}
		})
	}
}

// warmupCalls is the Warmup part of publicRequestCalls.
func warmupCalls(warmup *WarmupClient) []requestCall {
	var calls []requestCall
	for _, call := range publicRequestCalls(nil, warmup) {
		if strings.HasPrefix(call.name, "WarmupClient.") {
			calls = append(calls, call)
		}
	}
	return calls
}

// warmupResponder answers every Warmup request through respond.
type warmupResponder struct {
	mu       sync.Mutex
	requests []*http.Request
	respond  func(req *http.Request, body string) (*http.Response, error)
}

func (w *warmupResponder) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}
	w.mu.Lock()
	w.requests = append(w.requests, req)
	w.mu.Unlock()
	return w.respond(req, string(body))
}

func (w *warmupResponder) take() []*http.Request {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := w.requests
	w.requests = nil
	return out
}

func textResponse(req *http.Request, status int, contentType, body string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", contentType)
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

// useWarmupResponder installs responder as the transport and returns a Warmup
// client with the given key, aimed at https://warmup.example.test.
func useWarmupResponder(t *testing.T, key string, respond func(*http.Request, string) (*http.Response, error)) (*warmupResponder, *WarmupClient) {
	t.Helper()
	useFake(t)
	responder := &warmupResponder{respond: respond}
	http.DefaultTransport = responder
	warmup, err := NewWarmupClient(key, "https://warmup.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return responder, warmup
}

// No Warmup request follows a 3xx. The redirect target answers with a
// success, so a client that followed it would report one.
func TestEveryWarmupRequestRefusesARedirect(t *testing.T) {
	responder, warmup := useWarmupResponder(t, "warmup-key", nil)
	for _, status := range []int{301, 302, 303, 307, 308} {
		responder.respond = func(req *http.Request, _ string) (*http.Response, error) {
			if req.URL.Hostname() == "elsewhere.example" {
				if strings.Contains(req.URL.Path, "/download") {
					return textResponse(req, 200, "text/csv", "email\nseed@example.com\n"), nil
				}
				return textResponse(req, 200, CONTENT_TYPE_JSON, `{"success":true}`), nil
			}
			response := textResponse(req, status, "text/html", "<html>moved</html>")
			response.Header.Set("Location", "https://elsewhere.example"+req.URL.RequestURI())
			return response, nil
		}
		for _, call := range warmupCalls(warmup) {
			t.Run(fmt.Sprintf("%d/%s", status, call.name), func(t *testing.T) {
				responder.take()
				err := call.call()
				requests := responder.take()
				if len(requests) != 1 || requests[0].URL.Hostname() != "warmup.example.test" {
					hosts := []string{}
					for _, request := range requests {
						hosts = append(hosts, request.URL.Host)
					}
					t.Fatalf("the redirect was followed: hosts %v", hosts)
				}
				var apiErr *ZBApiError
				if !errors.As(err, &apiErr) {
					t.Fatalf("err = %v, want a *ZBApiError", err)
				}
				if apiErr.StatusCode != status || apiErr.Message != redirectRefusedMessage {
					t.Fatalf("err = %d %q, want %d %q", apiErr.StatusCode, apiErr.Message, status, redirectRefusedMessage)
				}
			})
		}
	}
}

// The Warmup key, and TestSMTPConfiguration's username and password, never
// appear in an error: not when an error body, a non-JSON 2xx body or the
// network layer's own error echoes them, raw or URL-encoded.
func TestWarmupErrorsNeverCarryTheKeyOrSMTPCredentials(t *testing.T) {
	const (
		key      = `wk/se"cret+key%1`
		username = `smtp-"user"@example.com`
		password = `p@ss "word+1/x%`
	)
	secrets := []string{key, username, password}
	var forms []string
	for _, secret := range secrets {
		forms = append(forms, secret, url.QueryEscape(secret), jsonQuoted(secret))
	}
	echo := strings.Join(forms, " | ")
	quoted := jsonQuoted(echo)

	cases := map[string]func(req *http.Request, body string) (*http.Response, error){
		"error body": func(req *http.Request, _ string) (*http.Response, error) {
			return textResponse(req, 500, CONTENT_TYPE_JSON, `{"message":"login failed: `+quoted+`"}`), nil
		},
		"non-JSON 2xx body": func(req *http.Request, _ string) (*http.Response, error) {
			return textResponse(req, 200, "text/plain", "login failed: "+echo), nil
		},
		// The network layer's error quotes what was sent: the key header and
		// the request body, which carries the SMTP credentials.
		"transport error": func(req *http.Request, body string) (*http.Response, error) {
			return nil, fmt.Errorf("proxy refused API-Key %s, body %s, also %s", req.Header.Get("API-Key"), body, echo)
		},
	}
	for name, respond := range cases {
		responder, warmup := useWarmupResponder(t, key, respond)
		csv := map[string]bool{}
		for _, method := range warmupCSVMethods(t) {
			csv["WarmupClient."+method] = true
		}
		calls := append(warmupCalls(warmup), requestCall{"WarmupClient.TestSMTPConfiguration(credentials)", func() error {
			_, err := warmup.TestSMTPConfiguration("a@example.com", username, password, "smtp.example.com", 587)
			return err
		}})
		for _, call := range calls {
			if name == "non-JSON 2xx body" && csv[call.name] {
				continue // text on a 200 is what a CSV download returns
			}
			t.Run(name+"/"+call.name, func(t *testing.T) {
				err := call.call()
				if len(responder.take()) == 0 {
					t.Fatal("no request was sent")
				}
				if err == nil {
					t.Fatal("no error")
				}
				// Only the SMTP call is given the username and password to
				// mask; for the rest just the key is a secret.
				check := []string{key, url.QueryEscape(key), jsonQuoted(key)}
				if strings.HasPrefix(call.name, "WarmupClient.TestSMTPConfiguration(") {
					check = forms
				}
				for _, secret := range check {
					if strings.Contains(err.Error(), secret) {
						t.Fatalf("error carries %q: %v", secret, err)
					}
				}
				if !strings.Contains(err.Error(), "***") {
					t.Fatalf("error does not show the masked secret: %v", err)
				}
			})
		}
	}
}

// warmupCSVMethods are the WarmupClient methods that return a CSV download as
// text: every exported method returning (string, error).
func warmupCSVMethods(t *testing.T) []string {
	t.Helper()
	var names []string
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	clientType := reflect.TypeOf(&WarmupClient{})
	for i := 0; i < clientType.NumMethod(); i++ {
		method := clientType.Method(i)
		if method.Type.NumOut() == 2 && method.Type.Out(0).Kind() == reflect.String && method.Type.Out(1) == errorType {
			names = append(names, method.Name)
		}
	}
	if len(names) == 0 {
		t.Fatal("found no Warmup CSV downloads")
	}
	return names
}

// A JSON body on an HTTP 200 where a CSV belongs is the API's error message,
// not the file. A client that returned it as the CSV would hand the caller an
// error message as data.
func TestWarmupJSONInPlaceOfACSVIsAnError(t *testing.T) {
	const key = "warmup-key"
	responder, warmup := useWarmupResponder(t, key, func(req *http.Request, _ string) (*http.Response, error) {
		return textResponse(req, 200, "application/json; charset=utf-8", `{"message":"No seeds for `+key+`."}`), nil
	})
	byName := map[string]requestCall{}
	for _, call := range warmupCalls(warmup) {
		byName[strings.TrimPrefix(call.name, "WarmupClient.")] = call
	}
	for _, name := range warmupCSVMethods(t) {
		t.Run(name, func(t *testing.T) {
			call, ok := byName[name]
			if !ok {
				t.Fatalf("%s is missing from publicRequestCalls", name)
			}
			err := call.call()
			if len(responder.take()) != 1 {
				t.Fatal("want one request")
			}
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want a *ZBApiError", err)
			}
			if apiErr.StatusCode != 200 || apiErr.Message != "No seeds for ***." {
				t.Fatalf("err = %d %q", apiErr.StatusCode, apiErr.Message)
			}
		})
	}
}

// A CSV download is an error, not the file, when the 200 is an HTML page (by
// content type or by body), a JSON body under a CSV content type, or empty.
// Only a JSON content type and an empty body were checked, so an HTML page or
// a JSON error sent as text/csv came back as the CSV.
func TestWarmupHTMLJSONOrEmptyBodyInPlaceOfACSVIsAnError(t *testing.T) {
	page := `<!DOCTYPE html><html><head><title>Just a moment</title></head><body></body></html>`
	cases := []struct{ name, contentType, body string }{
		{"html content type", "text/html; charset=UTF-8", page},
		{"html body as csv", "text/csv", page},
		{"json object as csv", "text/csv", `{"message":"No seeds for this team."}`},
		{"json list as csv", "text/plain", `["No seeds for this team."]`},
		{"empty", "text/csv", "  \n"},
	}
	for _, tc := range cases {
		responder, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, _ string) (*http.Response, error) {
			return textResponse(req, 200, tc.contentType, tc.body), nil
		})
		byName := map[string]requestCall{}
		for _, call := range warmupCalls(warmup) {
			byName[strings.TrimPrefix(call.name, "WarmupClient.")] = call
		}
		for _, name := range warmupCSVMethods(t) {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				call, ok := byName[name]
				if !ok {
					t.Fatalf("%s is missing from publicRequestCalls", name)
				}
				err := call.call()
				if len(responder.take()) != 1 {
					t.Fatal("want one request")
				}
				var apiErr *ZBApiError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != 200 {
					t.Fatalf("err = %v, want a *ZBApiError with status 200", err)
				}
			})
		}
	}
}
