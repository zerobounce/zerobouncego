package zerobouncego

import (
	"bytes"
	"encoding/json"
	"mime"
	"strings"
)

// utf8BOM is the byte order mark some servers and spreadsheet tools put in
// front of a body.
const utf8BOM = "\xef\xbb\xbf"

// trimBOMSpace drops leading BOMs and surrounding white space.
func trimBOMSpace(body string) string {
	s := strings.TrimSpace(body)
	for strings.HasPrefix(s, utf8BOM) {
		s = strings.TrimSpace(strings.TrimPrefix(s, utf8BOM))
	}
	return s
}

// bodyIsJSONObjectOrList reports whether the body, after a BOM and white
// space, is a JSON object or list: never a result file.
func bodyIsJSONObjectOrList(body string) bool {
	s := trimBOMSpace(body)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return false
	}
	return json.Valid([]byte(s))
}

// bodyIsJSONObjectOrListBytes is bodyIsJSONObjectOrList for a byte body.
func bodyIsJSONObjectOrListBytes(body []byte) bool {
	return bodyIsJSONObjectOrList(string(bytes.TrimSpace(body)))
}

// GetFileJSONIndicatesError reports whether a getfile response body is a JSON
// object or list (after any BOM and white space), which is never a result file:
// the API reports a failure that way, including on HTTP 200. Every JSON object
// counts, whether or not it has a success, message or error field; a body that
// only starts like JSON but is not valid JSON does not.
func GetFileJSONIndicatesError(body string) bool {
	return bodyIsJSONObjectOrList(body)
}

// FormatGetFileErrorMessage returns a short message from a getfile JSON error body.
func FormatGetFileErrorMessage(body string) string {
	s := trimBOMSpace(body)
	var o map[string]interface{}
	if err := json.Unmarshal([]byte(s), &o); err != nil {
		if s == "" {
			return "Invalid getfile response"
		}
		return s
	}
	// A list of messages is joined one per line, like messageFromBody.
	if message := apiErrorField(o, []string{"message", "error", "error_message"}); message != "" {
		return message
	}
	return s
}

// contentTypeIncludesApplicationJSON matches application/json and every
// structured-syntax JSON type (application/problem+json, text/x+json, ...).
func contentTypeIncludesApplicationJSON(ct string) bool {
	lower := strings.ToLower(ct)
	media, _, err := mime.ParseMediaType(lower)
	if err != nil {
		media = strings.TrimSpace(strings.SplitN(lower, ";", 2)[0])
	}
	return media == "application/json" || (strings.Contains(media, "/") && strings.HasSuffix(media, "+json"))
}

func shouldTreatGetFileBodyAsError(body, contentType string) bool {
	if contentTypeIncludesApplicationJSON(contentType) {
		return true
	}
	// Any body that starts like JSON is not a result file, including one that
	// is not shaped like an error, one that follows one or more BOMs, and one
	// that is not valid JSON (`{"error":"x"} trailing`). A result file does
	// not start with `{` or `[`.
	s := trimBOMSpace(body)
	return len(s) > 0 && (s[0] == '{' || s[0] == '[')
}
