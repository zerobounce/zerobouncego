package zerobouncego

import (
	"encoding/json"
	"strings"
)

// GetFileJSONIndicatesError reports whether a getfile response body looks like a JSON error payload (including HTTP 200).
// withoutBOM drops a leading UTF-8 BOM. TrimSpace leaves U+FEFF in place,
// so a JSON error that starts with one would otherwise be saved as a file.
func withoutBOM(body string) string {
	s := strings.TrimSpace(body)
	for strings.HasPrefix(s, "\uFEFF") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "\uFEFF"))
	}
	return s
}

func GetFileJSONIndicatesError(body string) bool {
	s := withoutBOM(body)
	if len(s) == 0 || s[0] != '{' {
		return false
	}
	var o map[string]interface{}
	if err := json.Unmarshal([]byte(s), &o); err != nil {
		return false
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
	_, has := o["success"]
	return has
}

// FormatGetFileErrorMessage returns a short message from a getfile JSON error body.
func FormatGetFileErrorMessage(body string) string {
	s := withoutBOM(body)
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

func contentTypeIncludesApplicationJSON(ct string) bool {
	ct = strings.ToLower(ct)
	// +json covers application/problem+json and application/vnd.api+json.
	return strings.Contains(ct, "application/json") || strings.Contains(ct, "+json")
}

func shouldTreatGetFileBodyAsError(body, contentType string) bool {
	if contentTypeIncludesApplicationJSON(contentType) {
		return true
	}
	// Any body that starts like JSON is not a result file, including one that
	// is not shaped like an error, one that follows one or more BOMs, and one
	// that is not valid JSON (`{"error":"x"} trailing`). A result file does
	// not start with `{` or `[`.
	s := withoutBOM(body)
	return len(s) > 0 && (s[0] == '{' || s[0] == '[')
}
