package zerobouncego

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The keys the body-echo sweep sends: the package key, a Client's own key and
// a Warmup key. Each has characters that URL-encode, so the raw and the
// encoded form differ.
const (
	echoPackageKey = "pk/echo+key1"
	echoClientKey  = "ck/echo+key2"
	echoWarmupKey  = "wk/echo+key3"
)

// keyEchoTransport answers every request with a body that quotes the key the
// request carried (api_key parameter or field, Bearer header, or Warmup
// API-Key header), the way a proxy or an error page might.
type keyEchoTransport struct {
	respond func(req *http.Request, key string) *http.Response
	sent    int
}

func (e *keyEchoTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorded := recordHTTPRequest(req)
	key := recorded.APIKey
	if key == "" {
		key = req.Header.Get("API-Key")
	}
	e.sent++
	return e.respond(req, key), nil
}

func echoResponse(req *http.Request, status int, contentType, body string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", contentType)
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

// jsonQuoted is text as it sits inside a JSON string.
func jsonQuoted(text string) string {
	encoded, _ := json.Marshal(text)
	return string(encoded[1 : len(encoded)-1])
}

// errorTexts is every text an error exposes: its message, the message and
// fields of a *ZBApiError, and the same for each error it wraps.
func errorTexts(err error) []string {
	var texts []string
	for current := err; current != nil; current = errors.Unwrap(current) {
		texts = append(texts, current.Error())
		var apiErr *ZBApiError
		if errors.As(current, &apiErr) {
			texts = append(texts, apiErr.Message)
		}
		if urlErr, ok := current.(*url.Error); ok {
			texts = append(texts, urlErr.URL, urlErr.Op)
		}
	}
	return texts
}

// No request returns the API key in an error, whatever form of it the
// response body echoes: an error-only 200, a {"success": false} 200, a text
// 400 quoting the request URL, or a JSON 400. Messages built from a response
// body used to be passed on as they came, so a body that quoted the key
// handed it to the caller on every call but the Warmup ones.
func TestEveryRequestRedactsTheKeyFromBodyMessages(t *testing.T) {
	cases := map[string]func(req *http.Request, key string) *http.Response{
		"200 error-only": func(req *http.Request, key string) *http.Response {
			return echoResponse(req, 200, CONTENT_TYPE_JSON, `{"error":"Invalid API key `+jsonQuoted(key)+`"}`)
		},
		"200 success false": func(req *http.Request, key string) *http.Response {
			return echoResponse(req, 200, CONTENT_TYPE_JSON, `{"success":false,"message":"Key `+jsonQuoted(key)+` is not allowed"}`)
		},
		"400 text quoting the URL": func(req *http.Request, key string) *http.Response {
			quoted := *req.URL
			query := quoted.Query()
			query.Set("api_key", key)
			quoted.RawQuery = query.Encode()
			return echoResponse(req, 400, "text/plain", "Bad request: "+quoted.String()+" (key "+key+")")
		},
		"400 JSON error": func(req *http.Request, key string) *http.Response {
			return echoResponse(req, 400, CONTENT_TYPE_JSON, `{"error":"Invalid API key `+jsonQuoted(key)+`"}`)
		},
	}
	var forms []string
	for _, key := range []string{echoPackageKey, echoClientKey, echoWarmupKey} {
		forms = append(forms, key, url.QueryEscape(key), url.PathEscape(key), jsonQuoted(key))
	}
	for name, respond := range cases {
		useFake(t)
		echo := &keyEchoTransport{respond: respond}
		http.DefaultTransport = echo
		client, _ := pointEverythingAt(t, "https://api.example.test")
		API_KEY = echoPackageKey
		client.APIKey = echoClientKey
		warmup, err := NewWarmupClient(echoWarmupKey, "https://warmup.example.test")
		if err != nil {
			t.Fatal(err)
		}
		failed := 0
		calls := publicRequestCalls(client, warmup)
		for _, call := range calls {
			t.Run(name+"/"+call.name, func(t *testing.T) {
				before := echo.sent
				err := call.call()
				if echo.sent == before {
					t.Fatal("no request reached the transport")
				}
				if err == nil {
					return // a result, not an error: nothing to leak
				}
				failed++
				for _, text := range errorTexts(err) {
					for _, form := range forms {
						if strings.Contains(text, form) {
							t.Fatalf("error carries the key as %q: %q", form, text)
						}
					}
				}
			})
		}
		// Most calls fail on each of these bodies; if none did, the sweep
		// checked nothing.
		if failed < len(calls)/2 {
			t.Fatalf("%s: only %d of %d calls returned an error", name, failed, len(calls))
		}
	}
}

// The masking itself: every form of a key, and any api_key query value.
func TestRedactKeysMasksEveryFormOfASentKey(t *testing.T) {
	rememberKey("ab/cd+ef")
	got := redactKeys(`raw ab/cd+ef, query ab%2Fcd%2Bef, path ab%2Fcd+ef, json ab\/cd+ef, url https://x.test/v2?api_key=other-key&email=a`)
	want := `raw ***, query ***, path ***, json ***, url https://x.test/v2?api_key=***&email=a`
	if got != want {
		t.Fatalf("redactKeys = %q\nwant %q", got, want)
	}
}
