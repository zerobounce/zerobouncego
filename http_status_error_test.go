package zerobouncego

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
)

const (
	cloudflareBlockPage = `<!DOCTYPE html><html><head><title>Access denied | api.zerobounce.net used Cloudflare to restrict access</title></head><body><p>blocked</p></body></html>`
	redirectRefused     = "ZeroBounce returned a redirect, which this client does not follow."
)

// redirectTarget is where every 3xx points. A request that reaches it means
// the redirect was followed.
const redirectTarget = "evil.example"

// errRedirectFollowed stops a client that follows redirects at the first hop,
// so the test fails at once instead of looping on the same 3xx.
var errRedirectFollowed = errors.New("the redirect was followed to " + redirectTarget)

// statusResponder answers every request with one status, content type and body.
func statusResponder(status int, contentType, body string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() == redirectTarget {
			return nil, errRedirectFollowed
		}
		response := httpmock.NewStringResponse(status, body)
		if contentType != "" {
			response.Header.Set("Content-Type", contentType)
		}
		if status >= 300 && status < 400 {
			response.Header.Set("Location", "https://"+redirectTarget+"/collect")
		}
		response.Request = req
		return response, nil
	}
}

// statusErrorCalls are validation-client calls that go through each of the
// error paths: decodeJSONBody, decodeNonMessageJSONBody, getFileBodyError,
// ErrorFromResponse (Inbox Placement) and the bulk-root helpers.
var statusErrorCalls = []struct {
	name string
	call func() error
}{
	{"GetCredits", func() error { _, err := GetCredits(); return err }},
	{"Validate", func() error { _, err := Validate("a@example.com", ""); return err }},
	{"ValidateBatch", func() error {
		_, err := ValidateBatch([]EmailToValidate{{EmailAddress: "a@example.com"}})
		return err
	}},
	{"FindEmail", func() error { _, err := FindEmail("example.com", "John", "", "Doe"); return err }},
	{"BulkValidationFileStatus", func() error { _, err := BulkValidationFileStatus(testing_file_id); return err }},
	{"BulkValidationResult", func() error {
		var saved bytes.Buffer
		err := BulkValidationResult(testing_file_id, &saved)
		if saved.Len() != 0 {
			return errors.New("an error body was saved as the result file")
		}
		return err
	}},
	{"ListFilters", func() error { _, err := ListFilters(false); return err }},
	{"ListS3Files", func() error { _, err := ListS3Files(ListS3FilesOptions{Bucket: "b"}); return err }},
	{"GetInboxPlacementProviders", func() error { _, err := GetInboxPlacementProviders(); return err }},
}

func withStatusResponder(t *testing.T, status int, contentType, body string) {
	t.Helper()
	previousKey, previousURI, previousBulk := API_KEY, URI, BULK_URI
	previousEvaluator, previousPlacement := LIST_EVALUATOR_URI, INBOX_PLACEMENT_URI
	httpmock.Activate()
	t.Cleanup(func() {
		httpmock.DeactivateAndReset()
		API_KEY, URI, BULK_URI = previousKey, previousURI, previousBulk
		LIST_EVALUATOR_URI, INBOX_PLACEMENT_URI = previousEvaluator, previousPlacement
	})
	Initialize("status-key")
	httpmock.RegisterNoResponder(statusResponder(status, contentType, body))
}

// An HTTP error status is a *ZBApiError carrying that status, so a caller can
// tell a Cloudflare 403 from an API 400/404 or a 429. The message text is the
// same short text messageFromBody produces.
func TestHTTPErrorStatusIsZBApiError(t *testing.T) {
	cases := []struct {
		status      int
		contentType string
		body        string
		message     string
	}{
		{403, "text/html", cloudflareBlockPage, `ZeroBounce returned an HTML error page ("Access denied | api.zerobounce.net used Cloudflare to restrict access") instead of an API response.`},
		{400, "application/json", `{"error": "Missing parameter: email."}`, "Missing parameter: email."},
		{404, "application/json", `{"success": false, "message": "File not found."}`, "File not found."},
		{429, "text/plain", "Too many requests", "Too many requests"},
		{500, "", "", "Empty response from ZeroBounce (HTTP 500)"},
	}
	for _, tc := range cases {
		for _, call := range statusErrorCalls {
			t.Run(call.name+"/"+http.StatusText(tc.status), func(t *testing.T) {
				withStatusResponder(t, tc.status, tc.contentType, tc.body)
				err := call.call()
				assertShortError(t, err)
				var apiErr *ZBApiError
				if !errors.As(err, &apiErr) {
					t.Fatalf("%T is not a *ZBApiError: %v", err, err)
				}
				if apiErr.StatusCode != tc.status {
					t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
				}
				if apiErr.Error() != tc.message {
					t.Fatalf("message = %q, want %q", apiErr.Error(), tc.message)
				}
			})
		}
	}
}

// ErrorFromResponse is exported; it returns the same *ZBApiError.
func TestErrorFromResponseCarriesStatus(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{"Content-Type": {"text/html"}},
		Body:       io.NopCloser(strings.NewReader(cloudflareBlockPage)),
	}
	err := ErrorFromResponse(response)
	var apiErr *ZBApiError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("got %T %v", err, err)
	}
	if strings.Contains(err.Error(), "<html") {
		t.Fatal(err)
	}
}

// A 2xx HTML page is still an error, but it carries no HTTP error status, so
// it stays a plain error: the godoc promises *ZBApiError only for statuses
// above 299 and for a 2xx JSON body that reports a failure.
func TestHTMLOn200IsAPlainError(t *testing.T) {
	withStatusResponder(t, 200, "text/html", cloudflareBlockPage)
	_, err := GetCredits()
	assertShortError(t, err)
	var apiErr *ZBApiError
	if errors.As(err, &apiErr) {
		t.Fatalf("unexpected *ZBApiError: %+v", apiErr)
	}
}

// A redirect is refused, not followed, and reported as such rather than as
// "Empty response from ZeroBounce (HTTP 302)" or an HTML page.
func TestRedirectIsReportedAsRefused(t *testing.T) {
	bodies := []struct {
		contentType string
		body        string
	}{
		{"", ""},
		{"text/html", `<html><head><title>Moved</title></head><body><a href="https://evil.example/collect">here</a></body></html>`},
	}
	for _, status := range []int{301, 302, 307, 308} {
		for _, body := range bodies {
			for _, call := range statusErrorCalls {
				t.Run(call.name+"/"+http.StatusText(status), func(t *testing.T) {
					withStatusResponder(t, status, body.contentType, body.body)
					err := call.call()
					if err == nil {
						t.Fatal("redirect was treated as success")
					}
					if err.Error() != redirectRefused {
						t.Fatalf("message = %q, want %q", err.Error(), redirectRefused)
					}
					var apiErr *ZBApiError
					if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
						t.Fatalf("got %T %+v", err, err)
					}
					if calls := httpmock.GetTotalCallCount(); calls != 1 {
						t.Fatalf("redirect was followed: %d calls", calls)
					}
				})
			}
		}
	}
}

// A getfile JSON error body on an HTTP 200 (a file that is not ready, an
// unknown file id) is an API failure, so it carries the status like any other.
func TestGetFileJSONErrorOn200IsZBApiError(t *testing.T) {
	calls := map[string]func(io.Writer) error{
		"BulkValidationResult":   func(w io.Writer) error { return BulkValidationResult(testing_file_id, w) },
		"AiScoringResult":        func(w io.Writer) error { return AiScoringResult(testing_file_id, w) },
		"BulkEmailFinderResult":  func(w io.Writer) error { return BulkEmailFinderResult(testing_file_id, w) },
		"BulkDomainSearchResult": func(w io.Writer) error { return BulkDomainSearchResult(testing_file_id, w) },
	}
	bodies := []struct{ contentType, body, message string }{
		{"application/json", `{"success": false, "message": "File is not ready."}`, "File is not ready."},
		{"application/octet-stream", `{"success": "False", "message": "File not found."}`, "File not found."},
		{"application/json", `{"error": "Invalid file id"}`, "Invalid file id"},
	}
	for name, call := range calls {
		for _, body := range bodies {
			t.Run(name, func(t *testing.T) {
				withStatusResponder(t, 200, body.contentType, body.body)
				var saved bytes.Buffer
				err := call(&saved)
				var apiErr *ZBApiError
				if !errors.As(err, &apiErr) {
					t.Fatalf("%T is not a *ZBApiError: %v", err, err)
				}
				if apiErr.StatusCode != 200 || apiErr.Message != body.message {
					t.Fatalf("got %+v", apiErr)
				}
				if saved.Len() != 0 {
					t.Fatal("the error body was saved as the result file")
				}
			})
		}
	}
}

// The account-level batch failure (HTTP 200, an error for address "all") is
// an API failure as well, not a plain error.
func TestBatchAccountLevelErrorIsZBApiError(t *testing.T) {
	withStatusResponder(t, 200, "application/json",
		`{"email_batch": [], "errors": [{"email_address": "all", "error": "Invalid API Key or your account ran out of credits"}]}`)
	_, err := ValidateBatch([]EmailToValidate{{EmailAddress: "a@example.com"}})
	var apiErr *ZBApiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("%T is not a *ZBApiError: %v", err, err)
	}
	if apiErr.StatusCode != 200 || apiErr.Message != "Invalid API Key or your account ran out of credits" {
		t.Fatalf("got %+v", apiErr)
	}
}
