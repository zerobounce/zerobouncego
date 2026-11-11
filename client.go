package zerobouncego

import (
	"fmt"
	"io"
)

const (
	defaultAPIURI  = "https://api.zerobounce.net/v2/"
	defaultBulkURI = "https://bulkapi.zerobounce.net/v2/"
)

// Client calls the API with its own key and base URLs.
// It does not read or write the package-level API_KEY, URI, or BULK_URI.
type Client struct {
	APIKey  string
	URI     string
	BulkURI string
}

// New returns a client for one API key. The package-level key is left unchanged.
func New(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		URI:     defaultAPIURI,
		BulkURI: defaultBulkURI,
	}
}

func (c *Client) config() RequestConfig {
	if c == nil {
		return RequestConfig{}
	}
	uri := c.URI
	if uri == "" {
		uri = defaultAPIURI
	}
	bulk := c.BulkURI
	if bulk == "" {
		bulk = defaultBulkURI
	}
	return RequestConfig{APIKey: c.APIKey, URI: uri, BulkURI: bulk}
}

// Validate validates one email with this client's key.
func (c *Client) Validate(email, ipAddress string) (*ValidateResponse, error) {
	return validateWithOptions(c.config(), email, ValidateOptions{IPAddress: ipAddress})
}

// redactedKey stands in for an API key in printed output.
func redactedKey(key string) string {
	if key == "" {
		return ""
	}
	return "***"
}

// formatRedacted prints a value through its String or GoString, so no verb
// (%v, %+v, %#v, %s, %q, %x) reaches the struct fields and the API key.
func formatRedacted(state fmt.State, verb rune, text, goText string) {
	if verb == 'v' && state.Flag('#') {
		io.WriteString(state, goText)
		return
	}
	io.WriteString(state, text)
}

// String names the type and its fields, with the API key redacted.
func (c Client) String() string {
	return fmt.Sprintf("Client{APIKey:%q, URI:%q, BulkURI:%q}", redactedKey(c.APIKey), c.URI, c.BulkURI)
}

// GoString is String in Go syntax (%#v), with the API key redacted.
func (c Client) GoString() string { return "zerobouncego." + c.String() }

// Format keeps the API key out of every fmt verb.
func (c Client) Format(state fmt.State, verb rune) {
	formatRedacted(state, verb, c.String(), c.GoString())
}

// String names the type and its fields, with the API key redacted.
func (c RequestConfig) String() string {
	return fmt.Sprintf("RequestConfig{APIKey:%q, URI:%q, BulkURI:%q}", redactedKey(c.APIKey), c.URI, c.BulkURI)
}

// GoString is String in Go syntax (%#v), with the API key redacted.
func (c RequestConfig) GoString() string { return "zerobouncego." + c.String() }

// Format keeps the API key out of every fmt verb.
func (c RequestConfig) Format(state fmt.State, verb rune) {
	formatRedacted(state, verb, c.String(), c.GoString())
}
