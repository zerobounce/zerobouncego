package zerobouncego

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
