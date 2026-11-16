package zerobouncego

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"
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

// redactedURL prints a base URL without the password of its userinfo
// (https://user:secret@host/ prints as https://***@host/). A URL that does not
// parse and holds an @ is not printed at all.
func redactedURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		if strings.Contains(raw, "@") {
			return "(url redacted)"
		}
		return raw
	}
	if parsed.User == nil {
		return raw
	}
	parsed.User = url.User("***")
	return parsed.String()
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
	return fmt.Sprintf("Client{APIKey:%q, URI:%q, BulkURI:%q}", redactedKey(c.APIKey), redactedURL(c.URI), redactedURL(c.BulkURI))
}

// GoString is String in Go syntax (%#v), with the API key redacted.
func (c Client) GoString() string { return "zerobouncego." + c.String() }

// Format keeps the API key out of every fmt verb.
// fmt does not call Format when a Client is an unexported field of the
// caller's struct, and %+v then prints APIKey. Unexporting APIKey would
// not hide it: fmt still prints unexported fields.
func (c Client) Format(state fmt.State, verb rune) {
	formatRedacted(state, verb, c.String(), c.GoString())
}

// String names the type and its fields, with the API key redacted.
func (c RequestConfig) String() string {
	return fmt.Sprintf("RequestConfig{APIKey:%q, URI:%q, BulkURI:%q}", redactedKey(c.APIKey), redactedURL(c.URI), redactedURL(c.BulkURI))
}

// GoString is String in Go syntax (%#v), with the API key redacted.
func (c RequestConfig) GoString() string { return "zerobouncego." + c.String() }

// Format keeps the API key out of every fmt verb.
func (c RequestConfig) Format(state fmt.State, verb rune) {
	formatRedacted(state, verb, c.String(), c.GoString())
}

// redactedConfig is the JSON and gob shape of a client. The key is masked
// because encoding/json and encoding/gob otherwise write the exported field.
type redactedConfig struct {
	APIKey  string
	URI     string
	BulkURI string
}

func redactedConfigOf(key, uri, bulk string) redactedConfig {
	return redactedConfig{APIKey: redactedKey(key), URI: uri, BulkURI: bulk}
}

// MarshalJSON keeps the API key out of json.Marshal, including when a Client
// sits in a slice or a map.
func (c Client) MarshalJSON() ([]byte, error) {
	return json.Marshal(redactedConfigOf(c.APIKey, c.URI, c.BulkURI))
}

// MarshalXML keeps the API key out of xml.Marshal, including when a Client
// sits in a slice or another struct.
func (c Client) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	return encoder.EncodeElement(redactedConfigOf(c.APIKey, c.URI, c.BulkURI), start)
}

// GobEncode keeps the API key out of gob. Encoding/gob calls it for this
// value and for slices and maps of it.
func (c Client) GobEncode() ([]byte, error) {
	return json.Marshal(redactedConfigOf(c.APIKey, c.URI, c.BulkURI))
}

// GobDecode restores a client from GobEncode. The key is the masked value.
func (c *Client) GobDecode(data []byte) error {
	var decoded redactedConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = Client{APIKey: decoded.APIKey, URI: decoded.URI, BulkURI: decoded.BulkURI}
	return nil
}

// MarshalJSON keeps the API key out of json.Marshal of a RequestConfig.
func (c RequestConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(redactedConfigOf(c.APIKey, c.URI, c.BulkURI))
}

// MarshalXML keeps the API key out of xml.Marshal of a RequestConfig.
func (c RequestConfig) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	return encoder.EncodeElement(redactedConfigOf(c.APIKey, c.URI, c.BulkURI), start)
}

// GobEncode keeps the API key out of gob of a RequestConfig.
func (c RequestConfig) GobEncode() ([]byte, error) {
	return json.Marshal(redactedConfigOf(c.APIKey, c.URI, c.BulkURI))
}

// GobDecode restores a RequestConfig from GobEncode.
func (c *RequestConfig) GobDecode(data []byte) error {
	var decoded redactedConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = RequestConfig{APIKey: decoded.APIKey, URI: decoded.URI, BulkURI: decoded.BulkURI}
	return nil
}
