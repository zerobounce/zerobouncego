package zerobouncego

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBaseURLWithUserinfoIsRefused(t *testing.T) {
	const evil = "https://api.zerobounce.net@127.0.0.1:9/v2/"
	if err := SetURI(evil, ""); err == nil || !strings.Contains(err.Error(), "userinfo") {
		t.Fatalf("SetURI: %v", err)
	}
	if _, err := NewWarmupClient("warmup-key", "https://warmup-api.zerobounce.net@127.0.0.1:9"); err == nil || !strings.Contains(err.Error(), "userinfo") {
		t.Fatalf("NewWarmupClient: %v", err)
	}
	if _, err := NewWarmupClient("warmup-key", "http://127.0.0.1:9"); err != nil {
		t.Fatalf("loopback http: %v", err)
	}
	fake := useFake(t)
	client := New("userinfo-key")
	client.URI = evil
	if _, err := client.Validate("a@b.com", ""); err == nil || !strings.Contains(err.Error(), "userinfo") {
		t.Fatalf("Client.URI: %v", err)
	}
	if len(fake.take()) != 0 {
		t.Fatal("userinfo URL was sent")
	}
}

func TestFormatCharactersInAKeyAreRefused(t *testing.T) {
	fake := useFake(t)
	for _, key := range []string{
		"abc\u200b", "\uFEFFabc", "abc\u180E",
		"abc\u3164def", "abc\u2800", "abc\u034Fdef", "abc\u115F",
		"abc\u0085", "abc\uFE00def",
		"ab\u00A0c", "ab\u2028c", "ab\u2029c", "ab\u1680c", "ab\u3000c",
		"ab\u180Bc", "ab\u180Fc", "ab\u2065c", "ab\uFFF0c", "ab\U000E0002c", "ab\U000E01F0c",
	} {
		client := New(key)
		if _, err := client.Validate("a@b.com", ""); err == nil || !strings.Contains(err.Error(), "whitespace or control") {
			t.Fatalf("%q: %v", key, err)
		}
		if _, err := NewWarmupClient(key, ""); err == nil {
			t.Fatalf("%q warmup accepted", key)
		}
	}
	if len(fake.take()) != 0 {
		t.Fatal("padded key was sent")
	}
}

func TestCaseFoldedSecretIsNotInTheError(t *testing.T) {
	const secret = "zb-test-KEY-abcDEF123456"
	previous := API_KEY
	API_KEY = secret
	t.Cleanup(func() { API_KEY = previous })
	rememberKey(secret)
	for _, echoed := range []string{strings.ToUpper(secret), strings.ToLower(secret)} {
		masked := redactKeys("Invalid API key " + echoed)
		if strings.Contains(strings.ToLower(masked), strings.ToLower(secret)) {
			t.Fatalf("case-folded echo survived: %s", masked)
		}
	}
}

func TestClientEncodingOmitsTheKey(t *testing.T) {
	const secret = "zb-test-KEY-abcDEF123456"
	client := New(secret)
	encoded, err := json.Marshal([]interface{}{client, client.config(), map[string]Client{"x": *client}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("json holds the key: %s", encoded)
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(client); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte(secret)) {
		t.Fatal("gob holds the key")
	}
	type clientWithExtra struct {
		Client
		Extra string
	}
	for _, value := range []any{client, *client, client.config(), []Client{*client}, clientWithExtra{Client: *client, Extra: "x"}} {
		xmlEncoded, err := xml.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(xmlEncoded, []byte(secret)) {
			t.Fatalf("xml holds the key: %s", xmlEncoded)
		}
	}
}

func TestNegativeTimeoutIsRefused(t *testing.T) {
	useFake(t)
	previous := http.DefaultClient.Timeout
	http.DefaultClient.Timeout = -time.Second
	t.Cleanup(func() { http.DefaultClient.Timeout = previous })
	Initialize("timeout-key")
	if _, err := GetCredits(); err == nil || !strings.Contains(err.Error(), "must not be negative") {
		t.Fatalf("GetCredits: %v", err)
	}
}

func TestHTTP2DebugRefusesToSend(t *testing.T) {
	for _, value := range []string{"http2debug=2", "http2debug=0,http2debug=2", "http2debug=1", "xhttp2debug=2", "x=http2debug=2"} {
		t.Run(value, func(t *testing.T) {
			fake := useFake(t)
			t.Setenv("GODEBUG", value)
			Initialize("debug-key")
			if _, err := GetCredits(); err == nil || !strings.Contains(err.Error(), "http2debug") {
				t.Fatalf("GetCredits: %v", err)
			}
			if len(fake.take()) != 0 {
				t.Fatal("request was sent with http2debug on")
			}
		})
	}
	t.Run("enabled at startup", func(t *testing.T) {
		fake := useFake(t)
		previous := http2DebugEnabledAtInit
		http2DebugEnabledAtInit = true
		t.Cleanup(func() { http2DebugEnabledAtInit = previous })
		t.Setenv("GODEBUG", "")
		Initialize("debug-key")
		if _, err := GetCredits(); err == nil || !strings.Contains(err.Error(), "http2debug") {
			t.Fatalf("GetCredits: %v", err)
		}
		if len(fake.take()) != 0 {
			t.Fatal("request was sent after http2debug was cleared")
		}
	})
}

func TestInitializeWithURIRejectsAnUnknownConstant(t *testing.T) {
	previousKey, previousURI := API_KEY, URI
	t.Cleanup(func() {
		API_KEY = previousKey
		URI = previousURI
	})
	API_KEY = "kept-key"
	URI = "https://kept.example/v2/"
	if err := InitializeWithURI("other-key", ZbApiURL(99)); err == nil {
		t.Fatal("unknown constant was accepted")
	}
	if API_KEY != "kept-key" || URI != "https://kept.example/v2/" {
		t.Fatalf("state changed: key %q uri %q", API_KEY, URI)
	}
}

func TestLoadEnvFromFileDoesNotPrintTheLine(t *testing.T) {
	dir := t.TempDir()
	const secret = "zb-test-KEY-abcDEF123456"
	if err := os.WriteFile(dir+"/.env", []byte("ZEROBOUNCE_API_KEY "+secret+" oops\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	ok := LoadEnvFromFile()
	writer.Close()
	os.Stdout = old
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("malformed .env was loaded")
	}
	if strings.Contains(string(out), secret) {
		t.Fatalf("stdout quoted the line: %s", out)
	}
	if !strings.Contains(string(out), "could not be read") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestLoadEnvFromFileDoesNotPrintABadBaseURL(t *testing.T) {
	dir := t.TempDir()
	const secret = "zb-test-KEY-abcDEF123456"
	body := "ZEROBOUNCE_API_KEY=other-key\nZERO_BOUNCE_URI=https://x.invalid/%zz" + secret + "\n"
	if err := os.WriteFile(dir+"/.env", []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	previousKey, previousURI := API_KEY, URI
	t.Cleanup(func() {
		os.Chdir(previous)
		API_KEY = previousKey
		URI = previousURI
		os.Unsetenv("ZEROBOUNCE_API_KEY")
		os.Unsetenv("ZERO_BOUNCE_API_KEY")
		os.Unsetenv("ZERO_BOUNCE_URI")
		os.Unsetenv("ZERO_BOUNCE_BULK_URI")
	})
	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	ok := LoadEnvFromFile()
	writer.Close()
	os.Stdout = old
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("env file was not loaded")
	}
	if strings.Contains(string(out), secret) {
		t.Fatalf("stdout quoted the URL: %s", out)
	}
	if !strings.Contains(string(out), "base URL was not applied") {
		t.Fatalf("stdout = %q", out)
	}
	if URI != previousURI {
		t.Fatalf("bad base URL was applied: %s", URI)
	}
}

func TestWarmupTypesOfEmailsAcceptsAJSONArray(t *testing.T) {
	var sent string
	responder, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, body string) (*http.Response, error) {
		sent = body
		return textResponse(req, 200, CONTENT_TYPE_JSON, `{"success":true}`), nil
	})
	_, err := warmup.AddSendingAccount("a@b.com", "zb_managed", &WarmupSendingAccountOptions{
		EmailContent: map[string]interface{}{
			"types_of_emails": []interface{}{"transactional", "newsletter"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sent, "transactional") || len(responder.take()) != 1 {
		t.Fatalf("body %s", sent)
	}
	rule := 1
	if _, err := warmup.AddSendingAccount("a@b.com", "zb_managed", &WarmupSendingAccountOptions{EngagementRuleID: &rule}); err == nil || !strings.Contains(err.Error(), "self_managed") {
		t.Fatalf("engagement rule on zb_managed: %v", err)
	}
}

func TestQuotedSecretEchoFromAResponseIsOmitted(t *testing.T) {
	const secret = "zb-test-KEY-abc'DEF123456"
	bodies := []string{
		`bad key zb-test-KEY-abc\'DEF123456`,
		"bad key 'zb-test-KEY-abc''DEF123456'",
		`bad key zb-test-KEY-abc&#39;DEF123456`,
		`bad key zb-test-KEY-abc\x27DEF123456`,
	}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			_, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, _ string) (*http.Response, error) {
				return textResponse(req, 400, "text/plain", body), nil
			})
			_, err := warmup.TestSMTPConfiguration("a@example.com", "user", secret, "smtp.example.com", 587)
			if err == nil || strings.Contains(err.Error(), "DEF123456") || !strings.Contains(err.Error(), "omitted") {
				t.Fatalf("smtp: %v", err)
			}
			client := New(secret)
			client.URI = "https://api.example.test/v2/"
			_, err = client.Validate("a@example.com", "")
			if err == nil || strings.Contains(err.Error(), "DEF123456") || !strings.Contains(err.Error(), "omitted") {
				t.Fatalf("validate: %v", err)
			}
		})
	}
}

func TestWarmupDownloadRejectsJSONBodies(t *testing.T) {
	cases := []struct {
		name, contentType, body, message string
	}{
		{"bom object", "text/csv", "\uFEFF{\"error\":\"x\"}", ""},
		{"two boms", "text/csv", "\uFEFF\uFEFF{\"message\":\"File not found\"}", "File not found"},
		{"plus json", "application/problem+json", "seed,provider\na,b\n", ""},
		{"trailing", "text/csv", "{\"error\":\"x\"} trailing", ""},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			_, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, _ string) (*http.Response, error) {
				return textResponse(req, 200, cs.contentType, cs.body), nil
			})
			got, err := warmup.DownloadSeeds()
			if err == nil || got != "" {
				t.Fatalf("got %q err %v", got, err)
			}
			if strings.Contains(err.Error(), "\uFEFF") {
				t.Fatalf("BOM in the message: %v", err)
			}
			if cs.message != "" && !strings.Contains(err.Error(), cs.message) {
				t.Fatalf("message %v", err)
			}
		})
	}
	_, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, _ string) (*http.Response, error) {
		return textResponse(req, 200, "text/csv", "seed,provider\na@b.c,gmail\n"), nil
	})
	got, err := warmup.DownloadSeeds()
	if err != nil || !strings.Contains(got, "seed,provider") {
		t.Fatalf("csv %q err %v", got, err)
	}
}

func TestFileStatusKeepsErrorReason(t *testing.T) {
	var status FileStatusResponse
	body := []byte(`{"success":true,"file_id":"1","file_name":"f.csv","file_status":"Complete","complete_percentage":"0%","error_reason":"bad file"}`)
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if status.ErrorReason == nil || *status.ErrorReason != "bad file" {
		t.Fatalf("%v", status.ErrorReason)
	}
}
