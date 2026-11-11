package zerobouncego

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const formatSecret = "sk-format-secret-123"

// %v, %+v and %#v printed the struct fields, the API key included, so a
// logged client leaked its key.
func TestClientsDoNotPrintTheKey(t *testing.T) {
	client := New(formatSecret)
	warmup, err := NewWarmupClient(formatSecret, "")
	if err != nil {
		t.Fatal(err)
	}
	config := client.config()
	values := []struct {
		name  string
		value interface{}
		want  []string
	}{
		{"*Client", client, []string{"Client", "***", defaultAPIURI, defaultBulkURI}},
		{"Client", *client, []string{"Client", "***", defaultAPIURI, defaultBulkURI}},
		{"*WarmupClient", warmup, []string{"WarmupClient", "***", WarmupDefaultBaseURL}},
		{"WarmupClient", *warmup, []string{"WarmupClient", "***", WarmupDefaultBaseURL}},
		{"RequestConfig", config, []string{"RequestConfig", "***", defaultAPIURI}},
		{"*RequestConfig", &config, []string{"RequestConfig", "***", defaultAPIURI}},
	}
	for _, value := range values {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
			printed := fmt.Sprintf(verb, value.value)
			if strings.Contains(printed, formatSecret) || strings.Contains(printed, fmt.Sprintf("%x", formatSecret)) {
				t.Fatalf("%s with %s prints the key: %s", value.name, verb, printed)
			}
			if verb == "%x" {
				continue
			}
			for _, want := range value.want {
				if !strings.Contains(printed, want) {
					t.Fatalf("%s with %s = %s, missing %q", value.name, verb, printed, want)
				}
			}
		}
	}
	// The key is still there to use.
	if client.APIKey != formatSecret {
		t.Fatal("APIKey changed")
	}
	// Wrapped in an error or another struct, too.
	wrapped := fmt.Errorf("context: %v", client)
	if strings.Contains(wrapped.Error(), formatSecret) || errors.Unwrap(wrapped) != nil {
		t.Fatal(wrapped)
	}
	holder := struct{ C *Client }{client}
	if printed := fmt.Sprintf("%+v", holder); strings.Contains(printed, formatSecret) {
		t.Fatal(printed)
	}
	// An unset key prints as empty, not as "***".
	if printed := fmt.Sprintf("%v", Client{}); !strings.Contains(printed, `APIKey:""`) {
		t.Fatal(printed)
	}
}
