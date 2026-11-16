package zerobouncego

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"testing"
)

// r5HiddenErr has a clean Error() but keeps the dumped request in a field,
// so only %#v (or %+v of a wrapper) shows it.
type r5HiddenErr struct {
	request string
}

func (e r5HiddenErr) Error() string { return "mock: no responder" }

type r5HiddenTransport struct{}

func (r5HiddenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, _ := httputil.DumpRequestOut(req, true)
	if req.Body != nil {
		req.Body.Close()
	}
	return nil, fmt.Errorf("wrap: %w", r5HiddenErr{request: string(dump)})
}

// A transport error whose message is clean but whose cause holds the request
// in a field still leaks the key in no layer and no verb, on every call.
func TestTransportErrorWithTheRequestInAFieldLeaksNoKey(t *testing.T) {
	useFake(t)
	http.DefaultTransport = r5HiddenTransport{}
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	failed := 0
	for _, call := range publicRequestCalls(client, warmup) {
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
	if failed < 60 {
		t.Fatalf("only %d calls reached the transport error", failed)
	}
}

// A Client prints its own key masked wherever it was placed in a base URL.
func TestClientMasksItsOwnKeyAnywhereInItsURLs(t *testing.T) {
	const key = "own-key-in-url-9876"
	for _, uri := range []string{
		"https://h.example/" + key + "/v2/",
		"https://h.example/v2/?apiKey=" + key,
		"https://h.example/v2/?api_key[]=" + key,
		"https://h.example/v2/#" + key,
	} {
		client := &Client{APIKey: key, URI: uri, BulkURI: uri}
		encoded, _ := client.MarshalJSON()
		for _, out := range []string{client.String(), fmt.Sprintf("%v", client), fmt.Sprintf("%+v", *client), fmt.Sprintf("%#v", *client), string(encoded), client.config().String()} {
			if strings.Contains(out, key) {
				t.Errorf("%s: key printed: %s", uri, out)
			}
		}
	}
}

// WarmupClient stays comparable (it was in v3.0.1): == and map keys compile
// and work, and a nested by-value copy still prints no key.
func TestWarmupClientIsComparableAndPrintsNoKeyNested(t *testing.T) {
	const key = "warmup-comparable-key-55"
	a, err := NewWarmupClient(key, "")
	if err != nil {
		t.Fatal(err)
	}
	b := *a
	if !(b == *a) {
		t.Fatal("a copy is not equal to its original")
	}
	seen := map[WarmupClient]bool{*a: true}
	if !seen[b] {
		t.Fatal("map lookup by value failed")
	}
	type holder struct {
		w WarmupClient
		p *WarmupClient
	}
	h := holder{w: *a, p: a}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d"} {
		if out := fmt.Sprintf(verb, h); strings.Contains(out, key) || strings.Contains(out, fmt.Sprintf("%x", key)) {
			t.Errorf("%s prints the key: %s", verb, out)
		}
	}
}

// An oversize 2xx HTML page on a download (not JSON-looking) is refused by the
// HTML part of the cap, not read whole.
func TestOversizeHTMLPageOnADownloadIsRefused(t *testing.T) {
	useFake(t)
	fake := &r3Transport{status: 200, contentType: "text/html", size: 3 << 20, fill: 'x'}
	http.DefaultTransport = fake
	pointEverythingAt(t, "https://api.example.test")
	var out bytes.Buffer
	err := BulkValidationResult(testing_file_id, &out)
	var apiErr *ZBApiError
	if !errors.As(err, &apiErr) || apiErr.Message != oversizeBodyMessage {
		t.Errorf("err = %#v", err)
	}
	if out.Len() != 0 {
		t.Errorf("%d bytes written", out.Len())
	}
}

// Any parameter whose name ends in api_key (x_api_key, my-api_key) is masked.
func TestAPIKeyParamsWithAPrefixAreMasked(t *testing.T) {
	for _, in := range []string{
		"https://h.example/v2/?x_api_key=prefixed-secret-1&a=b",
		"https://h.example/v2/?my-api_key=prefixed-secret-1",
		"https://h.example/v2/?X_API_KEY=prefixed-secret-1",
	} {
		if out := redactAPIKeyParams(in); strings.Contains(out, "prefixed-secret-1") {
			t.Errorf("%s -> %s", in, out)
		}
	}
}

// A Warmup key is remembered once the Warmup client sends, so a transport
// error elsewhere that quotes it is masked too.
func TestWarmupKeyIsRememberedWhenItSends(t *testing.T) {
	useFake(t)
	http.DefaultTransport = r4DumpTransport{}
	const key = "warmup-remembered-key-31"
	warmup, err := NewWarmupClient(key, "")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = warmup.ListSendingAccounts()
	found := false
	for _, k := range knownKeys() {
		if k == key {
			found = true
		}
	}
	if !found {
		t.Fatal("the Warmup key is not remembered after a send")
	}
}
