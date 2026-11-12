package zerobouncego

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
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
//     StatusCode set to that 2xx status. That includes a JSON body from a
//     file download (getfile) and ValidateBatch's account-level error (an
//     error for address "all").
//
// A 2xx that is not an API response at all (an HTML page, an empty body,
// non-JSON text, a download with a JSON content type or an empty body) is a
// ZBApiError too, with StatusCode set to that 2xx status. The Warmup client
// returns a ZBApiError for every failed response, whatever its status.
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

// sentKeys holds every API key a request has been built with: requireKey
// records each one before it is sent. A message built from a response body
// is cleaned of all of them, so a server or proxy that echoes the key (an
// error quoting the request URL, {"error": "Invalid API key <key>"}) does not
// hand it to the caller. It grows by one entry per distinct key.
var sentKeys sync.Map

// rememberKey records a key that is about to be sent.
func rememberKey(key string) {
	if key != "" {
		sentKeys.Store(key, struct{}{})
	}
}

// apiKeyParameter matches an api_key query or form value, whichever key it
// holds.
var apiKeyParameter = regexp.MustCompile(`(?i)(api_key=)[^&\s"'<>]+`)

// maskSecrets replaces every form of each secret in text with ***: as sent,
// URL-encoded (spaces as + and as %20, and path escaping), JSON-escaped
// (plain, and HTML-safe the way encoding/json writes &, < and >, each with
// and without \/ for /), and the URL encoding of each JSON form, which is
// how a server echoes a URL-encoded JSON request body. Longer forms are
// replaced first, so a shorter one never leaves part of a longer one behind.
func maskSecrets(text string, secrets ...string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		for _, form := range secretForms(secret) {
			text = strings.ReplaceAll(text, form, "***")
		}
	}
	return text
}

// secretForms lists every form maskSecrets masks, longest first.
func secretForms(secret string) []string {
	queryForms := func(value string) []string {
		escaped := url.QueryEscape(value)
		return []string{escaped, strings.ReplaceAll(escaped, "+", "%20")}
	}
	htmlSafe, _ := json.Marshal(secret)
	var plain bytes.Buffer
	encoder := json.NewEncoder(&plain)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(secret)
	forms := []string{secret, url.PathEscape(secret)}
	forms = append(forms, queryForms(secret)...)
	for _, quoted := range []string{string(htmlSafe), strings.TrimSpace(plain.String())} {
		jsonForm := quoted[1 : len(quoted)-1]
		for _, escaped := range []string{jsonForm, strings.ReplaceAll(jsonForm, "/", `\/`)} {
			forms = append(forms, escaped)
			forms = append(forms, queryForms(escaped)...)
		}
	}
	sort.SliceStable(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	return forms
}

// redactKeys masks every API key this process has sent, and any api_key
// parameter value, in a message built from a response body.
func redactKeys(message string) string {
	keys := []string{API_KEY}
	sentKeys.Range(func(key, _ interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return apiKeyParameter.ReplaceAllString(maskSecrets(message, keys...), "${1}***")
}

// statusError is the error for a response that is not a usable API result: a
// *ZBApiError carrying the response's HTTP status, 2xx included (an HTML page
// or non-JSON text on a 200 is still the server's answer), with the API key
// masked (redactKeys).
func statusError(statusCode int, message string) error {
	message = redactKeys(message)
	if statusCode > 0 {
		return &ZBApiError{StatusCode: statusCode, Message: message}
	}
	return errors.New(message)
}

// apiError builds a ZBApiError, or nil when there is no message. The API key
// is masked in the message (redactKeys).
func apiError(statusCode int, message string) error {
	if message == "" {
		return nil
	}
	return &ZBApiError{StatusCode: statusCode, Message: redactKeys(message)}
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
	if !validKeyText(c.APIKey) {
		return errors.New("invalid parameter: api_key has surrounding whitespace or control characters")
	}
	rememberKey(c.APIKey)
	return nil
}

// validKeyText reports whether a key has no surrounding whitespace and no
// control characters. Such a key would be quoted, unredacted, in the HTTP
// library's own error.
func validKeyText(key string) bool {
	if key != strings.TrimSpace(key) {
		return false
	}
	return !strings.ContainsFunc(key, func(r rune) bool { return r < 32 || r == 127 })
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
	if !json.Valid(body) {
		return statusError(status, "ZeroBounce returned a response that is not valid JSON.")
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
	// An empty 200 is not a result file: there is nothing to save.
	if bodyLooksLikeHTML(body) || contentTypeIsHTML(contentType) || status != 200 || strings.TrimSpace(body) == "" {
		return statusError(status, messageFromBody(status, contentType, body))
	}
	if shouldTreatGetFileBodyAsError(body, contentType) {
		message := stripErrorPrefix(strings.TrimSpace(FormatGetFileErrorMessage(strings.TrimSpace(body))))
		if message == "" || len(message) > 500 || bodyLooksLikeHTML(message) {
			message = messageFromBody(status, contentType, body)
		}
		// A JSON error body, or a JSON content type on a download, is the API
		// reporting a failure, so it carries the status like any other.
		return statusError(status, message)
	}
	return nil
}
