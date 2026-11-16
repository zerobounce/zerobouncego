package zerobouncego

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func saveGlobalsRound2() func() {
	k, u, b, l, i := API_KEY, URI, BULK_URI, LIST_EVALUATOR_URI, INBOX_PLACEMENT_URI
	return func() { API_KEY, URI, BULK_URI, LIST_EVALUATOR_URI, INBOX_PLACEMENT_URI = k, u, b, l, i }
}

var queryOrFragmentBases = []string{
	"https://api.zerobounce.net/v2?x=1",
	"https://api.zerobounce.net/v2/?x=1",
	"https://api.zerobounce.net/v2?",
	"https://api.zerobounce.net/v2/?",
	"https://api.zerobounce.net/v2#f",
	"https://api.zerobounce.net/v2/#",
	"https://api.zerobounce.net/v2?a=b#f",
}

func TestBaseURLWithQueryOrFragmentIsRefusedAtConfiguration(t *testing.T) {
	defer saveGlobalsRound2()()
	for _, base := range queryOrFragmentBases {
		if err := SetURI(base, ""); err == nil {
			t.Errorf("SetURI(%q) accepted", base)
		}
		if err := SetURI("", base); err == nil {
			t.Errorf("SetURI(\"\", %q) accepted", base)
		}
		if _, err := NewWarmupClient("warmup-key-1", base); err == nil {
			t.Errorf("NewWarmupClient(%q) accepted", base)
		}
		if err := requireHTTPSBase(base); err == nil {
			t.Errorf("requireHTTPSBase(%q) accepted", base)
		}
	}
}

// TestBaseURLWithQueryIsNeverSentAndNeverPrintsTheKey is the regression test
// for a base URL that left "…/getcredits?x=1?api_key=KEY" in a transport error.
func TestBaseURLWithQueryIsNeverSentAndNeverPrintsTheKey(t *testing.T) {
	defer saveGlobalsRound2()()
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte(`{"Credits":"1"}`))
	}))
	defer server.Close()
	const key = "QKEY12345"
	API_KEY = key
	for _, suffix := range []string{"/v2?x=1", "/v2?", "/v2#f", "/v2/?a=b"} {
		base := server.URL + suffix
		URI = base
		BULK_URI = base
		_, err := GetCredits()
		if err == nil || strings.Contains(err.Error(), key) {
			t.Errorf("package URI %q: err = %v", base, err)
		}
		c := New("CKEY12345")
		c.URI = base
		c.BulkURI = base
		_, err = c.Validate("a@b.com", "")
		if err == nil || strings.Contains(err.Error(), "CKEY12345") {
			t.Errorf("Client.URI %q: err = %v", base, err)
		}
		_, err = BulkValidationFileStatus("id")
		if err == nil || strings.Contains(err.Error(), key) {
			t.Errorf("BULK_URI %q: err = %v", base, err)
		}
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("%d requests reached the server", n)
	}
}

func TestWarmupSendRefusesABaseWithQuery(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer server.Close()
	client, err := NewWarmupClient("warmup-key-2", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client.root = server.URL + "?x=1" + warmupBasePath
	if _, _, _, err := client.send("GET", "/emails", nil, nil); err == nil {
		t.Fatal("a Warmup base with a query was sent")
	}
	client.root = server.URL + "#f" + warmupBasePath
	if _, _, _, err := client.send("GET", "/emails", nil, nil); err == nil {
		t.Fatal("a Warmup base with a fragment was sent")
	}
	if hits != 0 {
		t.Fatalf("%d requests reached the server", hits)
	}
}

func TestCheckRequestURLRefusesMalformedQueryAndFragment(t *testing.T) {
	for _, raw := range []string{
		"https://api.zerobounce.net/v2/getcredits?x=1?api_key=KEY",
		"https://api.zerobounce.net/v2/getcredits?a=%zz",
		"https://api.zerobounce.net/v2/getcredits?api_key=KEY#frag",
	} {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if checkRequestURL(parsed) == nil {
			t.Errorf("checkRequestURL(%q) accepted", raw)
		}
	}
	for _, raw := range []string{
		"https://api.zerobounce.net/v2/getcredits",
		"https://api.zerobounce.net/v2/getcredits?api_key=KEY&email=a%40b.com",
	} {
		parsed, _ := url.Parse(raw)
		if err := checkRequestURL(parsed); err != nil {
			t.Errorf("checkRequestURL(%q) = %v", raw, err)
		}
	}
}

func TestRedactAPIKeyBackstopMasksByPattern(t *testing.T) {
	const key = "SECRETKEY99"
	for _, raw := range []string{
		"https://h/v2/getcredits?x=1?api_key=" + key,
		"https://h/v2/getcredits?x=1?API_KEY=" + key + "&a=b",
		"https://h/v2/getcredits?Api_Key=" + key + "#frag",
		"https://h/v2/getcredits#f?api_key=" + key,
		"https://h/v2/getcredits?a=%zz&api_key=" + key,
		"https://h/v2/getcredits?api%5Fkey=" + key,
		"https://h/v2/getcredits?api_key=" + key + "&api_key=" + key,
	} {
		got := redactAPIKey(raw)
		if strings.Contains(got, key) {
			t.Errorf("redactAPIKey(%q) = %q", raw, got)
		}
	}
	if got := redactAPIKey("https://h/v2/getcredits?email=a%40b.com"); got != "https://h/v2/getcredits?email=a%40b.com" {
		t.Errorf("a URL without a key was changed: %q", got)
	}
	if got := redactAPIKey("https://h/v2/x?api_key=" + key + "&a=b"); !strings.Contains(got, "api_key=REDACTED") || !strings.Contains(got, "a=b") {
		t.Errorf("redactAPIKey = %q", got)
	}
}

func TestHTTP2DebugAtInitRefusesAfterGodebugIsCleared(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"address":"a@b.com","status":"valid"}`))
	}))
	defer server.Close()
	saved := http2DebugEnabledAtInit
	defer func() { http2DebugEnabledAtInit = saved }()
	t.Setenv("GODEBUG", "")
	http2DebugEnabledAtInit = godebugEnablesHTTP2FrameLog("http2debug=2")
	if !http2DebugLogsRequests() {
		t.Fatal("a process-start http2debug is not seen after GODEBUG is cleared")
	}
	c := New("DKEY12345")
	c.URI = server.URL + "/v2/"
	if _, err := c.Validate("a@b.com", ""); err == nil || !strings.Contains(err.Error(), "http2debug") {
		t.Fatalf("err = %v", err)
	}
	http2DebugEnabledAtInit = false
	if _, err := c.Validate("a@b.com", ""); err != nil {
		t.Fatalf("with http2debug off: %v", err)
	}
}

func TestFullCaseFoldingMasksEchoedSecrets(t *testing.T) {
	cases := []struct{ secret, echo string }{
		{"straße-key9", "STRASSE-KEY9"},
		{"straße-key9", "strasse-key9"},
		{"STRASSE-KEY9", "straße-key9"},
		{"ﬁle-key9", "FILE-KEY9"},
		{"FILE-KEY9", "ﬁle-key9"},
		{"İx-key9", "i̇x-key9"},
		{"i̇x-key9", "İX-KEY9"},
		{"ẞ-key9", "ss-key9"},
		{"ǰk-key9", "J̌K-KEY9"},
	}
	for _, c := range cases {
		text := "bad key " + c.echo + " for you"
		if !containsFold(text, c.secret) {
			t.Errorf("containsFold(%q, %q) = false", text, c.secret)
		}
		if got := maskSecrets(text, c.secret); got != "bad key *** for you" {
			t.Errorf("maskSecrets(%q, %q) = %q", text, c.secret, got)
		}
		warmup := &WarmupClient{apiKey: keyHolder(c.secret)}
		if got := warmup.redact(text); strings.Contains(strings.ToLower(got), "key9") {
			t.Errorf("Warmup redact(%q, %q) = %q", text, c.secret, got)
		}
	}
	// Two echoes, one of them inside longer text; neighbours stay.
	if got := maskSecrets("a STRASSE-KEY9 b straße-key9 c", "straße-key9"); got != "a *** b *** c" {
		t.Errorf("got %q", got)
	}
	if got := maskSecrets("nothing here", "straße-key9"); got != "nothing here" {
		t.Errorf("got %q", got)
	}
}

func TestFullCaseFoldingReachesRedactKeys(t *testing.T) {
	defer saveGlobalsRound2()()
	API_KEY = "straße-key9"
	if got := redactKeys("Invalid key STRASSE-KEY9."); strings.Contains(strings.ToLower(got), "strasse") {
		t.Errorf("redactKeys = %q", got)
	}
}

// TestSecretFormsKeepUpperAndLowerCopies pins the upper- and lower-cased
// copies of a secret: an echo in another case that is then URL-escaped differs
// from the escaped secret by more than case.
func TestSecretFormsKeepUpperAndLowerCopies(t *testing.T) {
	secret := "keyé1"
	for _, echo := range []string{
		url.QueryEscape(strings.ToUpper(secret)),
		url.PathEscape(strings.ToUpper(secret)),
		url.QueryEscape("KEYé1"),
	} {
		got := maskSecrets("echo "+echo+" end", secret)
		if got != "echo *** end" {
			t.Errorf("maskSecrets(%q) = %q (want masked, not left or omitted)", echo, got)
		}
	}
	mixed := "KeyÉ1"
	if got := maskSecrets("echo "+url.QueryEscape(strings.ToLower(mixed))+" end", mixed); got != "echo *** end" {
		t.Errorf("lower-cased escape: %q", got)
	}
}

type holdsWarmupByValue struct {
	name   string
	client WarmupClient
}

func TestWarmupClientNestedByValueDoesNotPrintTheKey(t *testing.T) {
	const key = "nested-warmup-key-77"
	c, err := NewWarmupClient(key, "")
	if err != nil {
		t.Fatal(err)
	}
	holders := []interface{}{
		holdsWarmupByValue{"x", *c},
		&holdsWarmupByValue{"x", *c},
		[]WarmupClient{*c},
		map[string]WarmupClient{"k": *c},
		struct{ W WarmupClient }{*c},
	}
	for _, holder := range holders {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%d"} {
			if got := fmt.Sprintf(verb, holder); strings.Contains(got, key) {
				t.Errorf("%T with %s printed the key: %s", holder, verb, got)
			}
		}
	}
	// A zero client does not panic.
	var zero WarmupClient
	_ = fmt.Sprintf("%v %+v %#v", zero, zero, zero)
	if got := c.key(); got != key {
		t.Fatalf("key() = %q", got)
	}
}

func TestClientEncodingsRedactTheURIUserinfoPassword(t *testing.T) {
	const password = "hunter2-pw"
	uri := "https://user:" + password + "@api.zerobounce.net/v2/"
	c := Client{APIKey: "enc-key-1234", URI: uri, BulkURI: uri}
	r := RequestConfig{APIKey: "enc-key-1234", URI: uri, BulkURI: uri}
	var outputs []string
	for _, v := range []interface{}{c, r, &c, []Client{c}, map[string]Client{"k": c}} {
		j, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, string(j))
	}
	for _, v := range []interface{}{c, r} {
		x, err := xml.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, string(x))
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(v); err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, buf.String())
	}
	for _, out := range outputs {
		if strings.Contains(out, password) || strings.Contains(out, "enc-key-1234") {
			t.Errorf("encoding leaks a secret: %s", out)
		}
	}
	if !strings.Contains(outputs[0], "api.zerobounce.net") {
		t.Errorf("the host was dropped: %s", outputs[0])
	}
}
