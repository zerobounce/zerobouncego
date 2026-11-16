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

// GetFileJSONIndicatesError reports whether a getfile response body looks like a JSON error payload (including HTTP 200).
func GetFileJSONIndicatesError(body string) bool {
	s := trimBOMSpace(body)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return false
	}
	var o map[string]interface{}
	if err := json.Unmarshal([]byte(s), &o); err != nil {
		// Not an object: a list is still JSON, not a result file.
		return s[0] == '[' && json.Valid([]byte(s))
	}
	if sv, ok := o["success"]; ok {
		if b, ok := sv.(bool); ok && !b {
			return true
		}
		if str, ok := sv.(string); ok {
			if strings.EqualFold(str, "false") {
				return true
			}
		}
	}
	for _, k := range []string{"message", "error", "error_message"} {
		if v, ok := o[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return true
				}
			case []interface{}:
				if len(t) > 0 {
					return true
				}
			}
		}
	}
	// Any other JSON object is not a result file either.
	return true
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
	for _, k := range []string{"message", "error", "error_message"} {
		if v, ok := o[k]; ok && v != nil {
			if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
				return str
			}
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				if str, ok := arr[0].(string); ok && strings.TrimSpace(str) != "" {
					return str
				}
			}
		}
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
