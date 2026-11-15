package zerobouncego

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
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
	for _, key := range []string{"abc\u200b", "\uFEFFabc", "abc\u180E"} {
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
	fake := useFake(t)
	t.Setenv("GODEBUG", "http2debug=2")
	Initialize("debug-key")
	if _, err := GetCredits(); err == nil || !strings.Contains(err.Error(), "http2debug") {
		t.Fatalf("GetCredits: %v", err)
	}
	if len(fake.take()) != 0 {
		t.Fatal("request was sent with http2debug on")
	}
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
