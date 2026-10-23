package zerobouncego

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AddFilter creates a custom allow/block list filter on the main API.
func AddFilter(rule, target, value string) (map[string]interface{}, error) {
	if strings.TrimSpace(rule) == "" || strings.TrimSpace(target) == "" || strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("empty parameter: rule, target, and value are required")
	}
	form := url.Values{}
	form.Set("rule", rule)
	form.Set("target", target)
	form.Set("value", value)
	return postFilterForm("/filters/add", form)
}

// ListFilters returns custom filters (GET by default; POST when usePost is true).
func ListFilters(usePost bool) (json.RawMessage, error) {
	if usePost {
		form := url.Values{}
		responseHTTP, err := postFilterRequest("/filters/list", form)
		if err != nil {
			return nil, err
		}
		return readFilterResponseBody(responseHTTP)
	}
	requestURL, err := PrepareURL("/filters/list", url.Values{})
	if err != nil {
		return nil, err
	}
	responseHTTP, err := httpGet(requestURL)
	if err != nil {
		return nil, err
	}
	return readFilterResponseBody(responseHTTP)
}

// DeleteFilter removes a custom filter.
func DeleteFilter(rule, target, value string) (map[string]interface{}, error) {
	if strings.TrimSpace(rule) == "" || strings.TrimSpace(target) == "" || strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("empty parameter: rule, target, and value are required")
	}
	form := url.Values{}
	form.Set("rule", rule)
	form.Set("target", target)
	form.Set("value", value)
	return postFilterForm("/filters/delete", form)
}

func postFilterForm(endpoint string, form url.Values) (map[string]interface{}, error) {
	responseHTTP, err := postFilterRequest(endpoint, form)
	if err != nil {
		return nil, err
	}
	defer responseHTTP.Body.Close()
	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{}
	if err = decodeJSONBody(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func postFilterRequest(endpoint string, form url.Values) (*http.Response, error) {
	if err := packageConfig().requireKey(); err != nil {
		return nil, err
	}
	if form == nil {
		form = url.Values{}
	}
	form.Set("api_key", API_KEY)

	urlToAccess, err := url.JoinPath(URI, strings.TrimPrefix(endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid URL (%s) or endpoint (%s) value", URI, endpoint)
	}

	return timedHTTPClient().PostForm(urlToAccess, form)
}

func readFilterResponseBody(responseHTTP *http.Response) (json.RawMessage, error) {
	defer responseHTTP.Body.Close()
	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return nil, err
	}
	if err = ensureJSONAPIBody(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), body); err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}
