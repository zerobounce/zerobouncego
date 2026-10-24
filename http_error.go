package zerobouncego

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const maxJSONBodyBytes = 2 << 20

// ZBApiError is a failed API response. It carries the HTTP status so callers
// can tell a 403 (Cloudflare) from a 400/404 (API) or a 429 (Warmup rate
// limit). Both the validation client and the Warmup client return it.
type ZBApiError struct {
	StatusCode int
	Message    string
}

func (e *ZBApiError) Error() string { return e.Message }

// apiError builds a ZBApiError, or nil when there is no message.
func apiError(statusCode int, message string) error {
	if message == "" {
		return nil
	}
	return &ZBApiError{StatusCode: statusCode, Message: message}
}

// RequestConfig is the API key and base URLs for one caller.
// Package functions use the process-wide key. A Client keeps its own copy.
type RequestConfig struct {
	APIKey  string
	URI     string
	BulkURI string
}

func packageConfig() RequestConfig {
	return RequestConfig{APIKey: API_KEY, URI: URI, BulkURI: BULK_URI}
}

func (c RequestConfig) requireKey() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return errors.New("api_key is required: call Initialize or New with a non-empty api_key")
	}
	return nil
}

func readLimitedBody(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxJSONBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxJSONBodyBytes {
		return body[:4096], errors.New("ZeroBounce returned a response that is not valid JSON.")
	}
	return body, nil
}

func bodyLooksLikeHTML(body string) bool {
	sample := body
	if len(sample) > 2000 {
		sample = sample[:2000]
	}
	lower := strings.ToLower(sample)
	return strings.Contains(lower, "<!doctype html") ||
		strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<head") ||
		strings.Contains(lower, "<body")
}

func contentTypeIsHTML(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/html")
}

func stripErrorPrefix(message string) string {
	for {
		trimmed := strings.TrimSpace(message)
		switch {
		case strings.HasPrefix(trimmed, "Error:"):
			message = strings.TrimSpace(trimmed[len("Error:"):])
		case strings.HasPrefix(trimmed, "[Error]:"):
			message = strings.TrimSpace(trimmed[len("[Error]:"):])
		default:
			return trimmed
		}
	}
}

func htmlTitle(body string) string {
	lower := strings.ToLower(body)
	start := strings.Index(lower, "<title")
	if start < 0 {
		return ""
	}
	openEnd := strings.Index(lower[start:], ">")
	if openEnd < 0 {
		return ""
	}
	openEnd += start
	closeRel := strings.Index(lower[openEnd:], "</title>")
	if closeRel < 0 {
		return ""
	}
	return strings.TrimSpace(body[openEnd+1 : openEnd+closeRel])
}

func firstAPIErrorField(object map[string]interface{}) string {
	for _, key := range []string{"error", "message", "error_message", "Message"} {
		value, ok := object[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case []interface{}:
			if len(typed) > 0 {
				if text, ok := typed[0].(string); ok && strings.TrimSpace(text) != "" {
					return strings.TrimSpace(text)
				}
			}
		}
	}
	return ""
}

func messageFromBody(status int, contentType, body string) string {
	if bodyLooksLikeHTML(body) || contentTypeIsHTML(contentType) {
		title := htmlTitle(body)
		if title == "" {
			return "ZeroBounce returned an HTML error page instead of an API response."
		}
		return fmt.Sprintf("ZeroBounce returned an HTML error page (%q) instead of an API response.", title)
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return fmt.Sprintf("Empty response from ZeroBounce (HTTP %d)", status)
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		var parsed interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return "ZeroBounce returned a response that is not valid JSON."
		}
		if object, ok := parsed.(map[string]interface{}); ok {
			if title, ok := object["title"].(string); ok {
				accessDenied := strings.Contains(title, "Access denied") || strings.Contains(title, "1020")
				cloudflare, _ := object["cloudflare_error"].(bool)
				if accessDenied && cloudflare {
					return title
				}
			}
			if message := firstAPIErrorField(object); message != "" {
				return stripErrorPrefix(message)
			}
		}
		if len(trimmed) > 1000 {
			return "ZeroBounce returned a response that is not valid JSON."
		}
		return stripErrorPrefix(trimmed)
	}
	if len(trimmed) > 1000 {
		return "ZeroBounce returned a response that is not valid JSON."
	}
	message := stripErrorPrefix(trimmed)
	if message == "" {
		return fmt.Sprintf("Empty response from ZeroBounce (HTTP %d)", status)
	}
	return message
}

// ensureJSONAPIBody accepts a 2xx JSON document. Anything else becomes a short error.
func ensureJSONAPIBody(status int, contentType string, body []byte) error {
	success := status >= 200 && status < 300
	text := string(body)
	if !success || strings.TrimSpace(text) == "" || bodyLooksLikeHTML(text) || contentTypeIsHTML(contentType) {
		return errors.New(messageFromBody(status, contentType, text))
	}
	trimmed := strings.TrimSpace(text)
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return errors.New(messageFromBody(status, contentType, text))
	}
	// {"success": false, ...} on an HTTP 200 is a failure, not a result. The
	// live API answers an unknown file id that way, with a 200.
	if trimmed[0] == '{' {
		var probe map[string]interface{}
		if json.Unmarshal(body, &probe) == nil {
			if failed, found := probe["success"]; found && isFalsyAPISuccess(failed) {
				return apiError(status, messageFromBody(status, contentType, text))
			}
		}
	}
	return nil
}

// isFalsyAPISuccess reports whether a "success" field means failure. The API
// sends it as a JSON false, "false" or "False" depending on the endpoint.
func isFalsyAPISuccess(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return !typed
	case string:
		return strings.EqualFold(typed, "false")
	}
	return false
}

func decodeJSONBody(status int, contentType string, body []byte, dest interface{}) error {
	if err := ensureJSONAPIBody(status, contentType, body); err != nil {
		return err
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return errors.New("ZeroBounce returned a response that is not valid JSON.")
	}
	return nil
}

func getFileBodyError(status int, contentType, body string) error {
	if bodyLooksLikeHTML(body) || contentTypeIsHTML(contentType) || status != 200 {
		return errors.New(messageFromBody(status, contentType, body))
	}
	if shouldTreatGetFileBodyAsError(body, contentType) {
		message := stripErrorPrefix(strings.TrimSpace(FormatGetFileErrorMessage(strings.TrimSpace(body))))
		if message == "" || len(message) > 500 || bodyLooksLikeHTML(message) {
			return errors.New(messageFromBody(status, contentType, body))
		}
		return errors.New(message)
	}
	return nil
}
