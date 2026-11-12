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
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf16"
)

const maxJSONBodyBytes = 2 << 20

// oversizeBodyMessage reports a response larger than maxJSONBodyBytes. It is
// a *ZBApiError with the response's status, a 2xx included.
const oversizeBodyMessage = "ZeroBounce returned a response larger than 2 MiB, which is not a valid API response."

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

// maskSecrets replaces every form of each secret in text with ***, composed
// from the encoders up to depth 2: {as sent, JSON-escaped (plain, and
// HTML-safe the way encoding/json writes &, < and >, each with and without
// \/ for /)} x {as is, URL-encoded with spaces as + and as %20}, the path
// escaping of the secret, and each JSON form escaped twice (a JSON error body
// quoting the JSON request body). Longer forms are replaced first, so a
// shorter one never leaves part of a longer one behind.
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
	forms := []string{secret, url.PathEscape(secret)}
	forms = append(forms, queryForms(secret)...)
	for _, escape := range jsonEscapers {
		escaped := escape(secret)
		forms = append(forms, escaped)
		forms = append(forms, queryForms(escaped)...)
		// A JSON error body quoting the JSON request body.
		forms = append(forms, escape(escaped))
	}
	sort.SliceStable(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	return forms
}

// jsonEscapers write text as it sits inside a JSON string: HTML-safe (&, <
// and > as \u0026, \u003c and \u003e, as json.Marshal writes them) or plain,
// each with / as is or as \/.
var jsonEscapers = []func(string) string{
	func(text string) string { return jsonEscaped(text, true) },
	func(text string) string { return jsonEscaped(text, false) },
	func(text string) string { return strings.ReplaceAll(jsonEscaped(text, true), "/", `\/`) },
	func(text string) string { return strings.ReplaceAll(jsonEscaped(text, false), "/", `\/`) },
}

func jsonEscaped(text string, htmlSafe bool) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(htmlSafe)
	_ = encoder.Encode(text)
	quoted := strings.TrimSpace(out.String())
	return quoted[1 : len(quoted)-1]
}

// redactKeys masks every API key this process has sent, and any api_key
// parameter value, in a message built from a response body. A message that
// still holds a key in a decoded view is replaced (omitIfSecretSurvives).
func redactKeys(message string) string {
	keys := []string{API_KEY}
	sentKeys.Range(func(key, _ interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	masked := apiKeyParameter.ReplaceAllString(maskSecrets(message, keys...), "${1}***")
	return omitIfSecretSurvives(masked, keys...)
}

// credentialOmittedMessage replaces a message that still held a secret after
// masking.
const credentialOmittedMessage = "ZeroBounce's response was omitted from this error because it contained a credential."

// omitIfSecretSurvives closes redaction under encoding: maskSecrets replaces
// the spellings this package can generate, and any other spelling (lowercase
// percent escapes, another JSON escaper) is caught here by decoding what is
// left. When any view of text, decoded up to maxSecretDecodeLayers layers
// deep in any order (percent-decoding, percent-decoding with + as a space,
// JSON-unescaping with \/ and \uXXXX), contains a secret, the whole text is
// dropped for credentialOmittedMessage.
func omitIfSecretSurvives(text string, secrets ...string) string {
	var present []string
	for _, secret := range secrets {
		if secret != "" {
			present = append(present, secret)
		}
	}
	if len(present) == 0 {
		return text
	}
	views := []string{text}
	seen := map[string]bool{text: true}
	for layer := 0; layer <= maxSecretDecodeLayers; layer++ {
		var next []string
		for _, view := range views {
			for _, secret := range present {
				if strings.Contains(view, secret) {
					return credentialOmittedMessage
				}
			}
			if layer == maxSecretDecodeLayers {
				continue
			}
			for _, decode := range secretDecoders {
				decoded := decode(view)
				if !seen[decoded] {
					seen[decoded] = true
					next = append(next, decoded)
				}
			}
		}
		views = next
	}
	return text
}

// maxSecretDecodeLayers is how deep omitIfSecretSurvives decodes: the SDK's
// own request encoding plus three layers a server or proxy may echo it in.
const maxSecretDecodeLayers = 4

// secretDecoders undo one layer of each encoding a server may echo a secret
// in. They are lenient: a sequence that does not decode is kept as it is.
var secretDecoders = []func(string) string{
	func(text string) string { return percentDecoded(text, false) },
	func(text string) string { return percentDecoded(text, true) },
	jsonUnescaped,
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func percentDecoded(text string, plusIsSpace bool) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '%' && i+2 < len(text) {
			high, okHigh := hexValue(text[i+1])
			low, okLow := hexValue(text[i+2])
			if okHigh && okLow {
				out.WriteByte(high<<4 | low)
				i += 2
				continue
			}
		}
		if c == '+' && plusIsSpace {
			out.WriteByte(' ')
			continue
		}
		out.WriteByte(c)
	}
	return out.String()
}

func jsonUnescaped(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '\\' || i+1 >= len(text) {
			out.WriteByte(c)
			continue
		}
		switch next := text[i+1]; next {
		case '"', '\\', '/':
			out.WriteByte(next)
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'u':
			if i+5 < len(text) {
				if code, err := strconv.ParseUint(text[i+2:i+6], 16, 16); err == nil {
					r := rune(code)
					// A surrogate pair (\ud83d\ude00) is one character.
					if utf16.IsSurrogate(r) && i+11 < len(text) && text[i+6] == '\\' && text[i+7] == 'u' {
						if low, err := strconv.ParseUint(text[i+8:i+12], 16, 16); err == nil {
							if pair := utf16.DecodeRune(r, rune(low)); pair != unicode.ReplacementChar {
								out.WriteRune(pair)
								i += 11
								continue
							}
						}
					}
					out.WriteRune(r)
					i += 5
					continue
				}
			}
			out.WriteByte(c)
			continue
		default:
			out.WriteByte(c)
			continue
		}
		i++
	}
	return out.String()
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

// readLimitedBody reads a JSON response body of at most maxJSONBodyBytes. A
// larger body is a *ZBApiError with the response's status (a 500 stays a 500,
// a 2xx is an API error with that status); a body that cannot be read is a
// transport failure, returned as it is.
func readLimitedBody(reader io.Reader, statusCode int) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxJSONBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxJSONBodyBytes {
		return nil, &ZBApiError{StatusCode: statusCode, Message: oversizeBodyMessage}
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
	// Strict RFC 8259 JSON on the body as received: json.Valid allows only
	// space, tab, CR and LF around the value, and no BOM, comment, trailing
	// comma or value, NaN or other extension (testdata/strict-json-corpus.tsv).
	if !json.Valid(body) {
		if trimmed[0] != '{' && trimmed[0] != '[' {
			return statusError(status, messageFromBody(status, contentType, text))
		}
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
	// The body is valid JSON (ensureJSONAPIBody), so this fails only when it
	// is not the shape the call returns, e.g. a bare string or number.
	// encoding/json takes null as "leave dest as it is", which would hand back
	// a result of zero values; only a raw-JSON result (*interface{}) takes it.
	if _, raw := dest.(*interface{}); !raw && string(bytes.Trim(body, " \t\r\n")) == "null" {
		return statusError(status, unexpectedJSONShapeMessage)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return statusError(status, unexpectedJSONShapeMessage)
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

// unexpectedJSONShapeMessage reports a 2xx body that is valid JSON but not
// the object or list the call returns (a bare string or number, say).
const unexpectedJSONShapeMessage = "ZeroBounce returned JSON that is not the expected response."
