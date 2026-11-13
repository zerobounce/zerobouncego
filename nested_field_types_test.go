package zerobouncego

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
)

// Documented response examples, copied verbatim from zerobounce.net/docs
// (the "Response" samples of each endpoint page).
const (
	docsValidateExample = `{
      "address": "john.doe@zerobounce.net",
      "status": "valid",
      "sub_status": "",
      "free_email": false,
      "catchall_domain": true,
      "did_you_mean": null,
      "account": "johndoe",
      "domain": "zerobounce.net",
      "domain_age_days": "3431",
      "active_in_days": "30",
      "active_first_seen": "2026-02-25",
      "smtp_provider": "g-suite",
      "mx_found": "true",
      "mx_record": "aspmx.l.google.com",
      "firstname": "dan",
      "lastname": "cotet",
      "gender": "male",
      "country": null,
      "region": null,
      "city": null,
      "zipcode": null,
      "processed_at": "2026-06-18 07:05:00.033"
  }`
	docsFileStatusExample = `{
    "success": true,
    "file_id": "aaaaaaaa-zzzz-xxxx-yyyy-5003727fffff",
    "file_name": "Your file name.csv",
    "upload_date": "2023-04-28T15:25:41Z",
    "file_status": "Complete",
    "file_phase_2_status": "Complete",
    "complete_percentage": "100%",
    "return_url": "Your return URL if provided when calling sendfile API"
  }`
	docsScoringFileStatusExample = `{
          "success": true,
          "file_id": "aaaaaaaa-zzzz-xxxx-yyyy-5003727fffff",
          "file_name": "Your file name.csv",
          "upload_date": "2023-04-28T15:25:41Z",
          "file_status": "Complete",
          "complete_percentage": "100%",
          "return_url": "Your return URL if provided when calling sendfile API"
      }`
	docsSendFileExample = `{
      "success": true,
      "message": "File Accepted",
      "file_name": "Your file name.csv",
      "file_id": "aaaaaaaa-zzzz-xxxx-yyyy-5003727fffff"
    }`
	docsDeleteFileExample = `{
    "success": true,
    "message": "File Deleted",
    "file_name": "test2",
    "file_id": "b222a0fd-90d5-416c-8f1a-9cc3851fc823"
  }`
)

// jsonPath is one place in a decoded JSON document: object keys and array
// indexes from the root.
type jsonPath []interface{}

func (p jsonPath) String() string {
	parts := make([]string, len(p))
	for i, step := range p {
		parts[i] = fmt.Sprint(step)
	}
	return "/" + strings.Join(parts, "/")
}

// everyNestedPath lists every value below the root of doc: object members,
// array items, and everything inside them.
func everyNestedPath(doc interface{}, prefix jsonPath) []jsonPath {
	var paths []jsonPath
	switch typed := doc.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			path := append(append(jsonPath{}, prefix...), key)
			paths = append(paths, path)
			paths = append(paths, everyNestedPath(typed[key], path)...)
		}
	case []interface{}:
		for index, item := range typed {
			path := append(append(jsonPath{}, prefix...), index)
			paths = append(paths, path)
			paths = append(paths, everyNestedPath(item, path)...)
		}
	}
	return paths
}

func decodeExample(t *testing.T, example string) interface{} {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(example))
	decoder.UseNumber()
	var doc interface{}
	if err := decoder.Decode(&doc); err != nil {
		t.Fatalf("example does not parse: %v", err)
	}
	return doc
}

// withValueAt returns example with the value at path replaced.
func withValueAt(t *testing.T, example string, path jsonPath, value interface{}) string {
	t.Helper()
	doc := decodeExample(t, example)
	var parent interface{} = doc
	for _, step := range path[:len(path)-1] {
		switch typed := parent.(type) {
		case map[string]interface{}:
			parent = typed[step.(string)]
		case []interface{}:
			parent = typed[step.(int)]
		}
	}
	switch typed := parent.(type) {
	case map[string]interface{}:
		typed[path[len(path)-1].(string)] = value
	case []interface{}:
		typed[path[len(path)-1].(int)] = value
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func valueAt(doc interface{}, path jsonPath) interface{} {
	for _, step := range path {
		switch typed := doc.(type) {
		case map[string]interface{}:
			doc = typed[step.(string)]
		case []interface{}:
			doc = typed[step.(int)]
		}
	}
	return doc
}

// wrongTypesFor is one value of every JSON type other than original's
// (null is left out: it is accepted for optional fields). A null in an
// example stands for a string that is not known.
func wrongTypesFor(original interface{}) []interface{} {
	candidates := map[string]interface{}{
		"string": "x", "number": json.Number("5"), "bool": true,
		"object": map[string]interface{}{}, "array": []interface{}{},
	}
	kind := ""
	switch original.(type) {
	case string, nil: // every null in the examples is a nullable string
		kind = "string"
	case json.Number:
		kind = "number"
	case bool:
		kind = "bool"
	case map[string]interface{}:
		kind = "object"
	case []interface{}:
		kind = "array"
	}
	var wrong []interface{}
	for _, name := range []string{"string", "number", "bool", "object", "array"} {
		if name != kind {
			wrong = append(wrong, candidates[name])
		}
	}
	return wrong
}

// Field types are part of the shape at every depth (standard.md §3): every
// documented example parses, and putting a value of any other JSON type at
// any nested path of it (a field, a batch row, a field inside a row, an
// array item) is a *ZBApiError with the 2xx status on every call that
// returns that example. The calls come from the complete public sweep.
func TestAWrongJSONTypeAtAnyNestedPathOfADocumentedExampleIsAnError(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")

	batchExample := `{"email_batch":[` + docsValidateExample + `],"errors":[{"error":"Invalid email","email_address":"bad@"}]}`
	examples := []struct {
		matches func(name string) bool
		example string
	}{
		{func(n string) bool { return strings.Contains(n, "Credits") }, docsCreditsResponse},
		{func(n string) bool { return strings.HasPrefix(n, "ValidateBatch") }, batchExample},
		{func(n string) bool {
			return strings.HasPrefix(n, "Validate") && !strings.HasPrefix(n, "ValidateBatch") &&
				!strings.Contains(n, "S3") && !strings.Contains(n, "SFTP")
		}, docsValidateExample},
		{func(n string) bool {
			return n == "BulkValidationFileStatus" || n == "GenericFileStatusCheck"
		}, docsFileStatusExample},
		{func(n string) bool { return n == "AiScoringFileStatus" }, docsScoringFileStatusExample},
		{func(n string) bool {
			return strings.HasSuffix(n, "FileDelete") && !strings.Contains(n, "Finder") && !strings.Contains(n, "DomainSearch")
		}, docsDeleteFileExample},
	}

	swept := map[int]int{}
	for _, call := range publicRequestCalls(client, warmup) {
		name := strings.TrimPrefix(call.name, "Client.")
		for index, entry := range examples {
			if !entry.matches(name) {
				continue
			}
			swept[index]++
			expectAccepted(t, call, fake, entry.example)
			doc := decodeExample(t, entry.example)
			for _, path := range everyNestedPath(doc, nil) {
				for _, wrong := range wrongTypesFor(valueAt(doc, path)) {
					if _, list := wrong.([]interface{}); list && path[len(path)-1] == "message" {
						continue // a list of strings is a documented "message" too
					}
					body := withValueAt(t, entry.example, path, wrong)
					fake.body = body
					err := call.call()
					var apiErr *ZBApiError
					if !errors.As(err, &apiErr) || apiErr.StatusCode != fake.status {
						t.Errorf("%s %s = %s: err = %v, want a *ZBApiError with status %d", call.name, path, compactJSON(wrong), err, fake.status)
					}
				}
			}
		}
	}
	for index := range examples {
		if swept[index] == 0 {
			t.Errorf("example %d matched no call in the sweep", index)
		}
	}
}

func compactJSON(value interface{}) string {
	encoded, _ := json.Marshal(value)
	return string(bytes.TrimSpace(encoded))
}

// fileValidationResponseCall reports whether a call returns a
// *FileValidationResponse (sendfile, deletefile and the bulk-root file jobs).
func fileValidationResponseCall(name string) bool {
	return !strings.Contains(name, "Finder") && !strings.Contains(name, "DomainSearch") &&
		(strings.Contains(name, "Submit") || strings.HasSuffix(name, "FileDelete"))
}

// "message" on a sendfile or deletefile answer is a string, or a list of
// strings (the sendfile error example); any other JSON type, or a list item
// that is not a string, is a *ZBApiError with the 2xx status.
func TestFileResponseMessageIsAStringOrAListOfStrings(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	calls := 0
	for _, call := range publicRequestCalls(client, warmup) {
		name := strings.TrimPrefix(call.name, "Client.")
		if !fileValidationResponseCall(name) {
			continue
		}
		calls++
		for _, message := range []string{`5`, `true`, `{}`, `[5]`, `["a",5]`, `[null]`, `[["a"]]`, `[{}]`} {
			fake.body = `{"success":true,"message":` + message + `,"file_id":"f"}`
			err := call.call()
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != fake.status {
				t.Errorf("%s %s: err = %v, want a *ZBApiError with status %d", call.name, fake.body, err, fake.status)
			}
		}
		for _, message := range []string{`"File Accepted"`, `["File Accepted"]`, `[]`, `null`} {
			expectAccepted(t, call, fake, `{"success":true,"message":`+message+`,"file_id":"f"}`)
		}
	}
	if calls < 9 {
		t.Fatalf("only %d sendfile and deletefile calls", calls)
	}
}
