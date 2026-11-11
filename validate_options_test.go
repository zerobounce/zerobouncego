package zerobouncego

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestValidateWithOptionsSendsActivityAndVerifyPlus(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", `=~^(.*)`+ENDPOINT_VALIDATE+`(.*)\z`,
		func(req *http.Request) (*http.Response, error) {
			q := req.URL.Query()
			assert.Equal(t, "true", q.Get("activity_data"))
			assert.Equal(t, "false", q.Get("verify_plus"))
			return httpmock.NewStringResponse(200, `{"address":"a@b.com","status":"valid","sub_status":""}`), nil
		},
	)

	ad, vp := true, false
	_, err := ValidateWithOptions("a@b.com", ValidateOptions{
		IPAddress:    SANDBOX_IP,
		ActivityData: &ad,
		VerifyPlus:   &vp,
	})
	assert.Nil(t, err)
}

func TestValidateWithOptionsPost(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", `=~^(.*)`+ENDPOINT_VALIDATE+`\z`,
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "POST", req.Method)
			assert.NoError(t, req.ParseForm())
			assert.Equal(t, "a@b.com", req.FormValue("email"))
			assert.Equal(t, "mock_key", req.FormValue("api_key"))
			return httpmock.NewStringResponse(200, `{"address":"a@b.com","status":"valid","sub_status":""}`), nil
		},
	)

	_, err := ValidateWithOptions("a@b.com", ValidateOptions{
		IPAddress: SANDBOX_IP,
		UsePost:   true,
	})
	assert.Nil(t, err)
}

func TestValidateBatchWithOptionsSendsBodyFields(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", `=~^(.*)`+ENDPOINT_BATCH_VALIDATE+`(.*)\z`,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(200, `{"email_batch":[],"errors":[]}`), nil
		},
	)

	timeout := 60
	ad, vp := true, true
	_, err := ValidateBatchWithOptions(
		[]EmailToValidate{{EmailAddress: "a@b.com", IPAddress: SANDBOX_IP}},
		&ValidateBatchOptions{Timeout: &timeout, ActivityData: &ad, VerifyPlus: &vp},
	)
	assert.Nil(t, err)
}

// The main-API key was only checked for being blank. A key with surrounding
// whitespace or a control character was sent, as the Warmup client refuses to.
func TestKeyWithWhitespaceOrControlCharactersIsRejected(t *testing.T) {
	for _, key := range []string{" key", "key ", "\tkey", "key\n", "ke\x00y", "ke\x7fy", "ke\ry"} {
		fake := useFake(t)
		Initialize(key)
		checks := map[string]func() error{
			"Validate":        func() error { _, err := Validate("a@example.com", ""); return err },
			"GetCredits":      func() error { _, err := GetCredits(); return err },
			"BulkFileStatus":  func() error { _, err := BulkValidationFileStatus("f"); return err },
			"IPT":             func() error { _, err := GetInboxPlacementProviders(); return err },
			"Client.Validate": func() error { _, err := New(key).Validate("a@example.com", ""); return err },
			// The Warmup key is checked once, when the client is built.
			"NewWarmupClient": func() error { _, err := NewWarmupClient(key, ""); return err },
		}
		for name, call := range checks {
			err := call()
			if err == nil || !strings.Contains(err.Error(), "surrounding whitespace or control characters") {
				t.Fatalf("%q %s: err = %v", key, name, err)
			}
		}
		if n := fake.size(); n != 0 {
			t.Fatalf("%q: %d request(s) sent", key, n)
		}
	}
	// An ordinary key still goes through.
	fake := useFake(t)
	Initialize("plain-key")
	if _, err := GetCredits(); err != nil || fake.size() != 1 {
		t.Fatalf("plain key: %v, %d requests", err, fake.size())
	}
	if warmup, err := NewWarmupClient("plain-key", ""); err != nil || warmup == nil {
		t.Fatalf("plain Warmup key: %v", err)
	}
}

// ip_address was always sent, empty when no IP was given.
func TestValidateOmitsEmptyIPAddress(t *testing.T) {
	fake := useFake(t)
	Initialize("ip-key")
	for _, usePost := range []bool{false, true} {
		fake.clear()
		if _, err := ValidateWithOptions("a@example.com", ValidateOptions{UsePost: usePost}); err != nil {
			t.Fatal(err)
		}
		if value, sent := onlyRequest(t, fake).Fields["ip_address"]; sent {
			t.Fatalf("post=%v: ip_address sent as %q", usePost, value)
		}
		fake.clear()
		if _, err := ValidateWithOptions("a@example.com", ValidateOptions{UsePost: usePost, IPAddress: "203.0.113.5"}); err != nil {
			t.Fatal(err)
		}
		if value := onlyRequest(t, fake).Fields["ip_address"]; value != "203.0.113.5" {
			t.Fatalf("post=%v: ip_address = %q", usePost, value)
		}
	}
}
