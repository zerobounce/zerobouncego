package zerobouncego

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func r(code rune) string { return string(code) }

var unsafeBaseURLs = []string{
	"https://api.zerobounce.net@evil.example/v2",
	"https://user:pass@api.zerobounce.net/v2",
	"https://exa mple.com",
	"https://",
	"https:///v2",
	"https://example.com:/v2",
	"https://example.com:0/v2",
	"https://example.com:99999/v2",
	"https://exam\\ple.com/v2",
	"https://example.com/v\x002",
	"https://example.com" + r(0x200b) + "/v2",
	"https://-bad-.example/v2",
	"https://a..b.example/v2",
	"https://[not-ipv6]/v2",
	"https://[::1%25eth0]/v2",
	"https:example.com",
}

// A base URL with userinfo, no host, a bad host or a bad port is refused when
// it is configured, by SetURI and NewWarmupClient.
func TestUnsafeBaseURLsAreRefusedAtConfiguration(t *testing.T) {
	restoreBaseURLs(t)
	for _, raw := range unsafeBaseURLs {
		if err := SetURI(raw, ""); err == nil {
			t.Errorf("SetURI(%q) accepted", raw)
		}
		if err := SetURI("", raw); err == nil {
			t.Errorf("SetURI(bulk %q) accepted", raw)
		}
		if _, err := NewWarmupClient("a-key", raw); err == nil {
			t.Errorf("NewWarmupClient(%q) accepted", raw)
		}
	}
	for _, raw := range []string{"https://api.zerobounce.net/v2/", "https://[2001:db8::1]:8443/v2", "https://example.com:443", "https://localhost/v2"} {
		if err := SetURI(raw, ""); err != nil {
			t.Errorf("SetURI(%q): %v", raw, err)
		}
		if _, err := NewWarmupClient("a-key", raw); err != nil {
			t.Errorf("NewWarmupClient(%q): %v", raw, err)
		}
	}
}

// Fields and package variables set directly skip configuration, so every send
// checks the URL again: nothing reaches the transport.
func TestUnsafeBaseURLsAreRefusedOnEverySend(t *testing.T) {
	for _, raw := range unsafeBaseURLs {
		fake := useFake(t)
		client, warmup := pointEverythingAt(t, "https://api.example.test")
		URI, BULK_URI = raw, raw
		LIST_EVALUATOR_URI, INBOX_PLACEMENT_URI = raw, raw
		client.URI, client.BulkURI = raw, raw
		warmup.root = raw + warmupBasePath
		swept := 0
		for _, call := range publicRequestCalls(client, warmup) {
			swept++
			_ = call.call()
		}
		if n := fake.size(); n != 0 {
			t.Fatalf("%q: %d request(s) reached the transport (%d calls)", raw, n, swept)
		}
	}
}

func TestUserinfoURLReachesNoHost(t *testing.T) {
	fake := useFake(t)
	client := New("a-key")
	client.URI = "https://api.zerobounce.net@evil.example/v2/"
	_, err := client.Validate("a@example.com", "")
	if err == nil || fake.size() != 0 {
		t.Fatalf("err = %v, sent = %d", err, fake.size())
	}
	if strings.Contains(err.Error(), "a-key") {
		t.Fatalf("key in error: %v", err)
	}
}

// Keys with invisible, format or non-ASCII characters are refused by both
// clients with the other SDKs' texts.
func TestKeyWithInvisibleOrNonASCIICharactersIsRefused(t *testing.T) {
	invisible := []string{"ke" + r(0x200b) + "y", r(0xfeff) + "key", "ke" + r(0x180e) + "y", "ke" + r(0x2060) + "y", "ke y", "ke" + r(0xa0) + "y", "ke" + r(0x3000) + "y", "ke" + r(0x85) + "y", "ke" + r(0x2028) + "y"}
	nonASCII := []string{"k" + r(0xe9) + "y", "ke" + r(0x4e2d) + "y"}
	for _, group := range []struct {
		keys []string
		want string
	}{{invisible, "whitespace, control or invisible characters"}, {nonASCII, "cannot be sent in an HTTP header"}} {
		for _, key := range group.keys {
			fake := useFake(t)
			Initialize(key)
			if _, err := GetCredits(); err == nil || !strings.Contains(err.Error(), group.want) {
				t.Errorf("%q GetCredits: %v", key, err)
			}
			if _, err := New(key).Validate("a@example.com", ""); err == nil || !strings.Contains(err.Error(), group.want) {
				t.Errorf("%q Client.Validate: %v", key, err)
			}
			if _, err := NewWarmupClient(key, ""); err == nil || !strings.Contains(err.Error(), group.want) {
				t.Errorf("%q NewWarmupClient: %v", key, err)
			}
			if fake.size() != 0 {
				t.Errorf("%q: request sent", key)
			}
		}
	}
}

// net/http logs :path (with api_key) and the API-Key header under
// GODEBUG=http2debug=1|2, so every send is refused while it is on. net/http
// looks for the substrings, so a later http2debug=0 does not turn it off.
func TestHTTP2DebugRefusesEveryCall(t *testing.T) {
	cases := map[string]bool{
		"http2debug=1":                 true,
		"http2debug=2":                 true,
		"x=1,http2debug=1,y=2":         true,
		"http2debug=1,http2debug=0":    true, // net/http still logs: it tests substrings
		"http2debug=10":                true,
		"http2debug=0,http2debug=2":    true,
		"http2debug=0":                 false,
		"":                             false,
		"other=1":                      false,
		"http2debug=1, http2debug = 0": true,
	}
	for godebug, refused := range cases {
		t.Run(godebug, func(t *testing.T) {
			fake := useFake(t)
			client, warmup := pointEverythingAt(t, "https://api.example.test")
			Initialize("secret-key-123")
			client.APIKey = "secret-key-123"
			t.Setenv("GODEBUG", godebug)
			calls := map[string]func() error{
				"GetCredits":      func() error { _, err := GetCredits(); return err },
				"Client.Validate": func() error { _, err := client.Validate("a@example.com", ""); return err },
				"Warmup":          func() error { _, err := warmup.ListSendingAccounts(); return err },
			}
			for name, call := range calls {
				fake.clear()
				err := call()
				if refused {
					if err == nil || !strings.Contains(err.Error(), "http2debug") {
						t.Fatalf("%s: err = %v, want the http2debug refusal", name, err)
					}
					if strings.Contains(err.Error(), "secret-key-123") {
						t.Fatalf("%s: key in error: %v", name, err)
					}
					if fake.size() != 0 {
						t.Fatalf("%s: request reached the transport", name)
					}
				} else if fake.size() == 0 {
					t.Fatalf("%s: nothing sent (err %v)", name, err)
				}
			}
		})
	}
}

func TestHTTP2DebugRefusalTextHasNoKey(t *testing.T) {
	if strings.Contains(errHTTP2Debug.Error(), "key=") {
		t.Fatal("refusal text carries a key")
	}
	var _ http.RoundTripper = httpsOnlyTransport{}
}

// maskSecrets and the survivor check ignore case and see HTML entities and \'.
func TestRedactionIsCaseInsensitiveAndDecodesHTMLAndEscapedQuote(t *testing.T) {
	secret := "Ab&c'dE-9"
	echoes := map[string]string{
		"upper":         strings.ToUpper(secret),
		"lower":         strings.ToLower(secret),
		"amp":           "Ab&amp;c'dE-9",
		"numeric":       "Ab&#38;c&#39;dE-9",
		"hex":           "Ab&#x26;c&#x27;dE-9",
		"named":         "Ab&amp;c&apos;dE-9",
		"entity-letter": "&#65;b&c'dE-9",
		"backslash":     `Ab&c\'dE-9`,
		"upper-entity":  "AB&AMP;C'DE-9",
	}
	for name, echo := range echoes {
		for shape, body := range map[string]string{
			"json":  `{"error":"bad key ` + jsonQuoted(echo) + ` end"}`,
			"html":  "<html><head><title>bad key " + echo + " end</title></head><body>x</body></html>",
			"plain": "bad key " + echo + " end",
		} {
			rememberKey(secret)
			got := redactKeys(body)
			if containsFold(got, secret) || strings.Contains(strings.ToLower(got), "ab&c'de-9") || strings.Contains(got, echo) {
				t.Errorf("%s/%s leaked: %q", name, shape, got)
			}
		}
	}
}

func TestWarmupSMTPPasswordIsMaskedIgnoringCaseAndEntities(t *testing.T) {
	warmup, err := NewWarmupClient("warm-key", "https://warmup.example.test")
	if err != nil {
		t.Fatal(err)
	}
	password := "Smtp&Pass'9é"
	for _, echo := range []string{
		strings.ToUpper(password), strings.ToLower(password), "Smtp&amp;Pass'9é", `Smtp&Pass\'9é`, "SMTP&AMP;PASS&#39;9É",
	} {
		got := warmup.redact("rejected: "+echo+" end", password)
		if strings.Contains(strings.ToLower(got), "smtp") || strings.Contains(got, "9é") {
			t.Errorf("%q leaked: %q", echo, got)
		}
	}
}

func TestStringRedactsURLUserinfo(t *testing.T) {
	client := Client{APIKey: "k-1", URI: "https://user:hunter2@api.example/v2/", BulkURI: "https://a:b@bulk.example/v2/"}
	for _, out := range []string{client.String(), fmt.Sprintf("%v", client), fmt.Sprintf("%#v", client), RequestConfig{URI: client.URI}.String()} {
		if strings.Contains(out, "hunter2") || strings.Contains(out, "a:b@") || strings.Contains(out, "user:") {
			t.Errorf("userinfo printed: %s", out)
		}
	}
	if got := redactedURL("https://x:y@exa mple.com"); strings.Contains(got, "y@") {
		t.Errorf("unparsable URL printed: %s", got)
	}
}
