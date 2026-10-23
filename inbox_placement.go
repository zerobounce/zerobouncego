package zerobouncego

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Inbox Placement Tester (https://bulkapi.zerobounce.net/api/ipt).
//
// Unlike the rest of the SDK these endpoints take the API key on the query
// string, send and receive camelCase JSON, and report errors under a "detail"
// key (Django REST framework). They spend the account's inbox-test credits.

// iptTestPath URL-encodes a test id as a single path segment.
func iptTestPath(test_id string) (string, error) {
	trimmed := strings.TrimSpace(test_id)
	if trimmed == "" {
		return "", fmt.Errorf("empty parameter: test_id")
	}
	return url.PathEscape(trimmed), nil
}

// iptRequest performs one Inbox Placement Tester call. body may be nil.
func iptRequest(method, path string, query url.Values, body interface{}, out interface{}) error {
	if err := packageConfig().requireKey(); err != nil {
		return err
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("api_key", API_KEY)

	url_to_request, error_ := url.JoinPath(INBOX_PLACEMENT_URI, path)
	if error_ != nil {
		return error_
	}
	url_to_request = fmt.Sprintf("%s?%s", url_to_request, query.Encode())

	var request *http.Request
	if body == nil {
		request, error_ = http.NewRequest(method, url_to_request, nil)
	} else {
		encoded, marshal_error := json.Marshal(body)
		if marshal_error != nil {
			return marshal_error
		}
		request, error_ = http.NewRequest(method, url_to_request, bytes.NewReader(encoded))
		if error_ == nil {
			request.Header.Set("Content-Type", CONTENT_TYPE_JSON)
		}
	}
	if error_ != nil {
		return error_
	}
	request.Header.Set("Accept", CONTENT_TYPE_JSON)

	response_http, error_ := timedHTTPClient().Do(request)
	if error_ != nil {
		return error_
	}
	defer response_http.Body.Close()

	if response_http.StatusCode < 200 || response_http.StatusCode > 299 {
		return ErrorFromResponse(response_http)
	}
	if out == nil {
		return nil
	}
	payload, error_ := io.ReadAll(response_http.Body)
	if error_ != nil {
		return error_
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return nil
	}
	return json.Unmarshal(payload, out)
}

// GetInboxPlacementProviders - seed providers available for new tests
// (GET /api/ipt/providers).
func GetInboxPlacementProviders() (map[string]interface{}, error) {
	payload := map[string]interface{}{}
	return payload, iptRequest("GET", "/providers", nil, nil, &payload)
}

// GetInboxPlacementTestTypes - supported test types and the default one
// (GET /api/ipt/test-types).
func GetInboxPlacementTestTypes() (map[string]interface{}, error) {
	payload := map[string]interface{}{}
	return payload, iptRequest("GET", "/test-types", nil, nil, &payload)
}

// CreateInboxPlacementTestOptions - the optional fields of a new test.
// Only the fields that are set are sent.
type CreateInboxPlacementTestOptions struct {
	TestType    *int
	SenderEmail string
	ProviderIDs []int
	SeedIDs     []int
	Providers   []map[string]interface{}
	WebhookURL  string
}

// CreateInboxPlacementTest - start a test and get back its seed list
// (POST /api/ipt/test). Name is required.
func CreateInboxPlacementTest(name string, options *CreateInboxPlacementTestOptions) (map[string]interface{}, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("empty parameter: name")
	}
	body := map[string]interface{}{"name": name}
	if options != nil {
		if options.TestType != nil {
			body["testType"] = *options.TestType
		}
		if options.SenderEmail != "" {
			body["senderEmail"] = options.SenderEmail
		}
		if options.ProviderIDs != nil {
			body["providerIds"] = options.ProviderIDs
		}
		if options.SeedIDs != nil {
			body["seedIds"] = options.SeedIDs
		}
		if options.Providers != nil {
			body["providers"] = options.Providers
		}
		if options.WebhookURL != "" {
			body["webhookUrl"] = options.WebhookURL
		}
	}
	payload := map[string]interface{}{}
	return payload, iptRequest("POST", "/test", nil, body, &payload)
}

// GetInboxPlacementTest - the results of one test (GET /api/ipt/test/{id}).
func GetInboxPlacementTest(test_id string) (map[string]interface{}, error) {
	segment, error_ := iptTestPath(test_id)
	if error_ != nil {
		return nil, error_
	}
	payload := map[string]interface{}{}
	return payload, iptRequest("GET", "/test/"+segment, nil, nil, &payload)
}

// ListInboxPlacementTestsOptions - the documented filters of the test list.
type ListInboxPlacementTestsOptions struct {
	Status   string
	Page     *int
	PageSize *int
}

// ListInboxPlacementTests - the account's tests (GET /api/ipt/test).
func ListInboxPlacementTests(options *ListInboxPlacementTestsOptions) (map[string]interface{}, error) {
	query := url.Values{}
	if options != nil {
		if options.Status != "" {
			query.Set("status", options.Status)
		}
		if options.Page != nil {
			query.Set("page", fmt.Sprintf("%d", *options.Page))
		}
		if options.PageSize != nil {
			query.Set("pageSize", fmt.Sprintf("%d", *options.PageSize))
		}
	}
	payload := map[string]interface{}{}
	return payload, iptRequest("GET", "/test", query, nil, &payload)
}

// DeleteInboxPlacementTest - delete one test (DELETE /api/ipt/test/{id}).
func DeleteInboxPlacementTest(test_id string) error {
	segment, error_ := iptTestPath(test_id)
	if error_ != nil {
		return error_
	}
	return iptRequest("DELETE", "/test/"+segment, nil, nil, nil)
}

// GetInboxPlacementWebhook - the configured webhook (GET /api/ipt/webhook).
func GetInboxPlacementWebhook() (map[string]interface{}, error) {
	payload := map[string]interface{}{}
	return payload, iptRequest("GET", "/webhook", nil, nil, &payload)
}

// SetInboxPlacementWebhook - set the webhook that test results post to
// (PUT /api/ipt/webhook).
func SetInboxPlacementWebhook(webhook_url string) (map[string]interface{}, error) {
	if strings.TrimSpace(webhook_url) == "" {
		return nil, fmt.Errorf("empty parameter: webhook_url")
	}
	payload := map[string]interface{}{}
	return payload, iptRequest("PUT", "/webhook", nil, map[string]interface{}{"webhookUrl": webhook_url}, &payload)
}
