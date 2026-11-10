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
// can tell a 403 (Cloudflare) from a 400/404 (API) or a 429 (rate limit).
// It may be wrapped, so match it with errors.As.
//
// The validation functions return it:
//
//   - for every response with an HTTP status above 299, a refused redirect
//     (3xx) included, whatever the body;
//   - for a 2xx whose JSON body reports a failure ({"success": false, ...}, or
//     an error-only body such as {"error": "Invalid API key ..."}), with
//     StatusCode set to that 2xx status.
//
// A 2xx that is not an API response at all (an HTML page, an empty body,
// non-JSON text) is a plain error there. The Warmup client returns a
// ZBApiError for every failed response, whatever its status.
//
// A transport failure such as a timeout, a body that cannot be read, and an
// argument rejected before any request is sent are plain errors.
type ZBApiError struct {
	StatusCode int
	Message    string
}

func (e *ZBApiError) Error() string { return e.Message }

// redirectRefusedMessage reports a 3xx. Redirects are never followed: the API
// key travels with the request, and a redirect could send it to another host.
const redirectRefusedMessage = "ZeroBounce returned a redirect, which this client does not follow."

// statusError is the error for a response that is not a usable API result. An
// HTTP error status (above 299) becomes a *ZBApiError carrying that status; a
// 2xx keeps a plain error. The message text is the same either way.
func statusError(statusCode int, message string) error {
	if statusCode > 299 {
		return &ZBApiError{StatusCode: statusCode, Message: message}
	}
	return errors.New(message)
}

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

// errorOnlyKeys only ever carry an error. "message" and "Message" are not
// among them: successful responses use those too (a bulk submit answers
// {"success": true, "message": "File Accepted"}, and a repeated greylist
// validate answers {"Message": "Greylist validation still in progress..."}).
var errorOnlyKeys = []string{"error", "error_message", "Error"}

// messageKeys are every key a message may sit under.
var messageKeys = []string{"error", "message", "error_message", "Message", "Error", "detail"}

func firstAPIErrorField(object map[string]interface{}) string {
	return apiErrorField(object, messageKeys)
}

// errorOnlyMessage is the API's own message when a 2xx body is an error with
// no "success" field to gate on. LIVE: an invalid or drained key answers
// validate, activity and getapiusage with {"error": "..."}, and the filters
// with {"Error": "..."}, all on an HTTP 200.
func errorOnlyMessage(object map[string]interface{}) string {
	if _, found := object["success"]; found {
		return ""
	}
	return apiErrorField(object, errorOnlyKeys)
}

// messageOnlyBody reports whether a JSON body is nothing but a message: an
// object with no "success" field whose keys are all message keys, at least
// one a non-empty string. Only for calls whose result is never a bare
// message. LIVE: guessformat answers an invalid or drained key with an HTTP
// 200 and {"Message": "Invalid API key or your account ran out of credits"}.
// validate is not one of them: a repeated greylist validate answers with a
// bare Message, and that is a result.
func messageOnlyBody(body []byte) bool {
	var object map[string]interface{}
	if json.Unmarshal(body, &object) != nil || len(object) == 0 {
		return false
	}
	if _, found := object["success"]; found {
		return false
	}
	for key := range object {
		known := false
		for _, candidate := range messageKeys {
			if key == candidate {
				known = true
				break
			}
		}
		if !known {
			return false
		}
	}
	return apiErrorField(object, messageKeys) != ""
}

func apiErrorField(object map[string]interface{}, keys []string) string {
	for _, key := range keys {
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
	if status >= 300 && status < 400 {
		return redirectRefusedMessage
	}
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
		return statusError(status, messageFromBody(status, contentType, text))
	}
	trimmed := strings.TrimSpace(text)
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return statusError(status, messageFromBody(status, contentType, text))
	}
	// {"success": false, ...} on an HTTP 200 is a failure, not a result. The
	// live API answers an unknown file id that way, with a 200.
	if trimmed[0] == '{' {
		var probe map[string]interface{}
		if json.Unmarshal(body, &probe) == nil {
			if failed, found := probe["success"]; found && isFalsyAPISuccess(failed) {
				return apiError(status, messageFromBody(status, contentType, text))
			}
			if errorOnlyMessage(probe) != "" {
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

// decodeNonMessageJSONBody is decodeJSONBody for a call whose result is
// never a bare message, so a message-only body is a failure.
func decodeNonMessageJSONBody(status int, contentType string, body []byte, dest interface{}) error {
	if err := ensureJSONAPIBody(status, contentType, body); err != nil {
		return err
	}
	if messageOnlyBody(body) {
		return apiError(status, messageFromBody(status, contentType, string(body)))
	}
	return decodeJSONBody(status, contentType, body, dest)
}

func getFileBodyError(status int, contentType, body string) error {
	if bodyLooksLikeHTML(body) || contentTypeIsHTML(contentType) || status != 200 {
		return statusError(status, messageFromBody(status, contentType, body))
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
