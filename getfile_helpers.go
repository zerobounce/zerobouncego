package zerobouncego

import (
	"encoding/json"
	"strings"
)

// GetFileJSONIndicatesError reports whether a getfile response body looks like a JSON error payload (including HTTP 200).
func GetFileJSONIndicatesError(body string) bool {
	s := strings.TrimSpace(body)
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
	s := strings.TrimSpace(body)
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
	return strings.Contains(strings.ToLower(ct), "application/json")
}

func shouldTreatGetFileBodyAsError(body, contentType string) bool {
	if contentTypeIncludesApplicationJSON(contentType) {
		return true
	}
	return GetFileJSONIndicatesError(body)
}
