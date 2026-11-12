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
// a Warmup key. Each has characters that URL-encode (/ + % ") and a quote
// that JSON escapes, so the raw, URL-encoded and JSON-escaped forms differ.
const (
	echoPackageKey = `pk/echo+"key%1`
	echoClientKey  = `ck/echo+"key%2`
	echoWarmupKey  = `wk/echo+"key%3`
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
		// ValidateBatch's account-level error: an error for the address
		// "all" on an HTTP 200. Only the batch calls fail on it.
		"200 batch all": func(req *http.Request, key string) *http.Response {
			return echoResponse(req, 200, CONTENT_TYPE_JSON,
				`{"email_batch":[],"errors":[{"error":"Invalid API key `+jsonQuoted(key)+`","email_address":"all"}]}`)
		},
	}
	// The calls that must fail on a case, where it is not most of them.
	mustFail := map[string][]string{
		"200 batch all": {"ValidateBatch", "ValidateBatchWithOptions"},
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
		failedCalls := map[string]bool{}
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
				failedCalls[call.name] = true
				for _, text := range errorTexts(err) {
					for _, form := range forms {
						if strings.Contains(text, form) {
							t.Fatalf("error carries the key as %q: %q", form, text)
						}
					}
				}
			})
		}
		if names, ok := mustFail[name]; ok {
			for _, callName := range names {
				if !failedCalls[callName] {
					t.Fatalf("%s: %s returned no error, so nothing was checked", name, callName)
				}
			}
			continue
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

// Every form a server can echo a secret in is masked, each one on its own: as
// sent, URL-encoded (spaces as + and as %20, and path escaping), JSON-escaped
// (plain, and with <, >, & as \u003c, \u003e, \u0026 the way encoding/json
// writes them, each with and without \/), and the URL encoding of each JSON
// form (a server echoing a URL-encoded JSON request body). The secret holds
// every character those encodings treat differently, and the expected forms
// are written out here, so dropping any one form fails.
func TestMaskSecretsMasksEveryEncodingOfASecret(t *testing.T) {
	secret := `a b"c/d+e%f&g=h<i>j'k\l`
	// encoding/json writes &, < and > as these escapes (\x5c is a backslash).
	htmlEscapedJSON := strings.NewReplacer("&", "\x5cu0026", "<", "\x5cu003c", ">", "\x5cu003e").Replace
	forms := []struct{ name, text string }{
		{"raw", `a b"c/d+e%f&g=h<i>j'k\l`},
		{"query", `a+b%22c%2Fd%2Be%25f%26g%3Dh%3Ci%3Ej%27k%5Cl`},
		{"query %20", `a%20b%22c%2Fd%2Be%25f%26g%3Dh%3Ci%3Ej%27k%5Cl`},
		{"path", `a%20b%22c%2Fd+e%25f&g=h%3Ci%3Ej%27k%5Cl`},
		{"json", `a b\"c/d+e%f&g=h<i>j'k\\l`},
		{"query json", `a+b%5C%22c%2Fd%2Be%25f%26g%3Dh%3Ci%3Ej%27k%5C%5Cl`},
		{"query %20 json", `a%20b%5C%22c%2Fd%2Be%25f%26g%3Dh%3Ci%3Ej%27k%5C%5Cl`},
		{"json html", htmlEscapedJSON(`a b\"c/d+e%f&g=h<i>j'k\\l`)},
		{"query json html", `a+b%5C%22c%2Fd%2Be%25f%5Cu0026g%3Dh%5Cu003ci%5Cu003ej%27k%5C%5Cl`},
		{"query %20 json html", `a%20b%5C%22c%2Fd%2Be%25f%5Cu0026g%3Dh%5Cu003ci%5Cu003ej%27k%5C%5Cl`},
		{`json \/`, `a b\"c\/d+e%f&g=h<i>j'k\\l`},
		{`query json \/`, `a+b%5C%22c%5C%2Fd%2Be%25f%26g%3Dh%3Ci%3Ej%27k%5C%5Cl`},
		{`query %20 json \/`, `a%20b%5C%22c%5C%2Fd%2Be%25f%26g%3Dh%3Ci%3Ej%27k%5C%5Cl`},
		{`json html \/`, htmlEscapedJSON(`a b\"c\/d+e%f&g=h<i>j'k\\l`)},
		{`query json html \/`, `a+b%5C%22c%5C%2Fd%2Be%25f%5Cu0026g%3Dh%5Cu003ci%5Cu003ej%27k%5C%5Cl`},
		{`query %20 json html \/`, `a%20b%5C%22c%5C%2Fd%2Be%25f%5Cu0026g%3Dh%5Cu003ci%5Cu003ej%27k%5C%5Cl`},
	}
	for _, form := range forms {
		t.Run(form.name, func(t *testing.T) {
			got := maskSecrets("echo ["+form.text+"] end", secret)
			if got != "echo [***] end" {
				t.Fatalf("maskSecrets left the %s form: %q", form.name, got)
			}
		})
	}
	// The raw form of `"k` sits inside its JSON form `\"k`: masking the
	// shorter form first would leave a stray backslash behind.
	if got := maskSecrets(`{"error":"bad key \"k"}`, `"k`); got != `{"error":"bad key ***"}` {
		t.Fatalf("a shorter form was masked inside a longer one: %q", got)
	}
}

// Every spelling of a secret, composed here from the encoders themselves, is
// masked whole: {raw, JSON (HTML-safe and plain, each with / and with \/)} x
// {as is, URL-encoded with +, with %20}, the path escaping, and each JSON form
// escaped twice. The secret holds every character an encoding changes, so
// each spelling differs from the others and dropping any one generator from
// secretForms fails.
func TestMaskSecretsMasksEveryComposedSpelling(t *testing.T) {
	secret := `a b"c/d+e%f&g=h<i>j'k\l`
	jsonForm := func(htmlSafe, slash bool) func(string) string {
		return func(text string) string {
			var out strings.Builder
			encoder := json.NewEncoder(&out)
			encoder.SetEscapeHTML(htmlSafe)
			_ = encoder.Encode(text)
			quoted := strings.TrimSuffix(out.String(), "\n")
			escaped := quoted[1 : len(quoted)-1]
			if slash {
				escaped = strings.ReplaceAll(escaped, "/", `\/`)
			}
			return escaped
		}
	}
	escapers := map[string]func(string) string{
		"raw":          func(text string) string { return text },
		"json html":    jsonForm(true, false),
		"json":         jsonForm(false, false),
		`json html \/`: jsonForm(true, true),
		`json \/`:      jsonForm(false, true),
	}
	urlEncoders := map[string]func(string) string{
		"":          func(text string) string { return text },
		", query +": url.QueryEscape,
		", query %20": func(text string) string {
			return strings.ReplaceAll(url.QueryEscape(text), "+", "%20")
		},
	}
	spellings := map[string]string{"path": url.PathEscape(secret)}
	for escapeName, escape := range escapers {
		for urlName, encode := range urlEncoders {
			spellings[escapeName+urlName] = encode(escape(secret))
		}
		if escapeName != "raw" {
			spellings[escapeName+", twice"] = escape(escape(secret))
		}
	}
	if len(spellings) != 1+5*3+4 {
		t.Fatalf("composed %d spellings", len(spellings))
	}
	seen := map[string]string{}
	for name, text := range spellings {
		if other, ok := seen[text]; ok {
			t.Fatalf("%s and %s are the same spelling %q", name, other, text)
		}
		seen[text] = name
	}
	for name, text := range spellings {
		if got := maskSecrets("echo ["+text+"] end", secret); got != "echo [***] end" {
			t.Errorf("maskSecrets left the %s spelling: %q", name, got)
		}
	}
}
