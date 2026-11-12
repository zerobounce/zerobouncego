package zerobouncego

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// sweepWith answers every request through respond and returns the swept
// calls, every base URL pointed at an https test host.
func sweepWith(t *testing.T, respond func(req *http.Request) *http.Response) []requestCall {
	t.Helper()
	useFake(t)
	http.DefaultTransport = &keyEchoTransport{respond: func(req *http.Request, _ string) *http.Response {
		return respond(req)
	}}
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	return publicRequestCalls(client, warmup)
}

// csvSuccess answers like the API does when everything works, with a CSV
// for every download.
func csvSuccess(req *http.Request) *http.Response {
	if strings.Contains(req.URL.Path, "getfile") || strings.Contains(req.URL.Path, "/download") {
		return textResponse(req, 200, "application/octet-stream", "email,status\na@example.com,valid\n")
	}
	return fakeResponse(recordHTTPRequest(req))
}

// downloadCalls are the swept calls that write a file: the ones that put
// bytes into sweepDownloads when the server answers with a CSV. Found by
// running every call, so a new download cannot be missed.
func downloadCalls(t *testing.T) map[string]bool {
	t.Helper()
	downloads := map[string]bool{}
	for _, call := range sweepWith(t, csvSuccess) {
		sweepDownloads.Reset()
		if err := call.call(); err == nil && sweepDownloads.Len() > 0 {
			downloads[call.name] = true
		}
	}
	sweepDownloads.Reset()
	// BulkValidation, AiScoring, Generic (each with and without options),
	// BulkEmailFinder and BulkDomainSearch.
	if len(downloads) < 8 {
		t.Fatalf("found %d downloads, want at least 8: %v", len(downloads), downloads)
	}
	return downloads
}

func assertStatus200(t *testing.T, err error) {
	t.Helper()
	var apiErr *ZBApiError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 200 {
		t.Fatalf("err = %T %v, want a *ZBApiError with status 200", err, err)
	}
}

// An empty 200 is not a result file. Every bulk download wrote nothing and
// returned no error, so a caller got an empty file and was told it worked.
func TestEveryDownloadRejectsAnEmpty200(t *testing.T) {
	downloads := downloadCalls(t)
	for _, contentType := range []string{"application/octet-stream", "text/csv", ""} {
		for _, body := range []string{"", " \r\n"} {
			calls := sweepWith(t, func(req *http.Request) *http.Response {
				response := textResponse(req, 200, contentType, body)
				if contentType == "" {
					response.Header.Del("Content-Type")
				}
				return response
			})
			for _, call := range calls {
				if !downloads[call.name] {
					continue
				}
				t.Run(contentType+"/"+call.name, func(t *testing.T) {
					sweepDownloads.Reset()
					err := call.call()
					assertStatus200(t, err)
					if sweepDownloads.Len() != 0 {
						t.Fatalf("wrote %d bytes", sweepDownloads.Len())
					}
				})
			}
		}
	}
}

// A JSON content type on a download is an error even when the body does not
// look like a JSON error: the API sends file bytes as a file type.
func TestEveryDownloadRejectsAJSONContentType(t *testing.T) {
	downloads := downloadCalls(t)
	calls := sweepWith(t, func(req *http.Request) *http.Response {
		return textResponse(req, 200, "application/json; charset=utf-8", "email,status\na@example.com,valid\n")
	})
	for _, call := range calls {
		if !downloads[call.name] {
			continue
		}
		t.Run(call.name, func(t *testing.T) {
			sweepDownloads.Reset()
			err := call.call()
			assertStatus200(t, err)
			if sweepDownloads.Len() != 0 {
				t.Fatalf("wrote %d bytes", sweepDownloads.Len())
			}
		})
	}
}

// A 2xx that is not an API response (an HTML page, by content type or by
// body, or text that is not JSON) is an error carrying the status, like
// every other failure the API reports. Only the Warmup client carried it; the
// other calls returned a plain error with no status.
func TestEveryCallReportsANonJSON2xxWithItsStatus(t *testing.T) {
	page := `<!DOCTYPE html><html><head><title>Just a moment</title></head><body></body></html>`
	cases := []struct {
		name, contentType, body string
		downloadsToo            bool
		message                 string
	}{
		{"html content type", "text/html; charset=UTF-8", page, true, "HTML"},
		{"html body as json", "application/json", page, true, "HTML"},
		{"text", "text/plain", "Service temporarily unavailable", false, "Service temporarily unavailable"},
		{"broken json", "application/json", `{"success": tru`, false, "not valid JSON"},
	}
	csv := map[string]bool{}
	for _, method := range warmupCSVMethods(t) {
		csv["WarmupClient."+method] = true
	}
	downloads := downloadCalls(t)
	for _, tc := range cases {
		calls := sweepWith(t, func(req *http.Request) *http.Response {
			return textResponse(req, 200, tc.contentType, tc.body)
		})
		for _, call := range calls {
			// Text is what a download returns.
			if !tc.downloadsToo && (downloads[call.name] || csv[call.name]) {
				continue
			}
			t.Run(tc.name+"/"+call.name, func(t *testing.T) {
				sweepDownloads.Reset()
				err := call.call()
				assertStatus200(t, err)
				if !strings.Contains(err.Error(), tc.message) {
					t.Fatalf("message %q does not say %q", err.Error(), tc.message)
				}
				if sweepDownloads.Len() != 0 {
					t.Fatalf("wrote %d bytes", sweepDownloads.Len())
				}
			})
		}
	}
}

// {"success": "false"} (a string, as some endpoints send it) is a failure on
// every call, like {"success": false}.
func TestEveryCallTreatsSuccessFalseAsAStringAsAnError(t *testing.T) {
	for _, value := range []string{`"false"`, `"False"`} {
		calls := sweepWith(t, func(req *http.Request) *http.Response {
			return textResponse(req, 200, CONTENT_TYPE_JSON, `{"success":`+value+`,"message":"File not found."}`)
		})
		for _, call := range calls {
			if strings.HasPrefix(call.name, "WarmupClient.") {
				continue // the Warmup API has no success field
			}
			t.Run(value+"/"+call.name, func(t *testing.T) {
				sweepDownloads.Reset()
				err := call.call()
				assertStatus200(t, err)
				if sweepDownloads.Len() != 0 {
					t.Fatalf("wrote %d bytes", sweepDownloads.Len())
				}
			})
		}
	}
}

// A bulk URI with extra trailing slashes reaches the same paths as the plain
// one. bulkAPIRoot stripped one "/" before "/v2", so ".../v2//" sent the
// bulkapi-root calls to /v2/<path> and the list evaluator (rebased by
// SetURI) to /v2/v2/listevaluator.
func TestBulkURIWithExtraTrailingSlashesReachesTheSamePaths(t *testing.T) {
	paths := func(bulk string) map[string]string {
		var last string
		calls := sweepWith(t, func(req *http.Request) *http.Response {
			last = req.URL.Path
			return csvSuccess(req)
		})
		t.Setenv("ZERO_BOUNCE_LIST_EVALUATOR_URI", "")
		t.Setenv("ZERO_BOUNCE_INBOX_PLACEMENT_URI", "")
		if err := SetURI("https://api.example.test/v2/", bulk); err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, call := range calls {
			last = ""
			_ = call.call()
			got[call.name] = last
		}
		return got
	}
	want := paths("https://api.example.test/v2")
	for _, bulk := range []string{"https://api.example.test/v2/", "https://api.example.test/v2//", "https://api.example.test/v2///"} {
		got := paths(bulk)
		for name, path := range want {
			if got[name] != path {
				t.Errorf("%s with %q: path %q, want %q", name, bulk, got[name], path)
			}
		}
	}
	if want["BulkEmailFinderFileStatus"] != "/email-finder/filestatus" || !strings.HasPrefix(want["GetListEvaluatorResult"], "/v2/listevaluator/"+testing_file_id) {
		t.Fatalf("control paths are wrong: %v", want)
	}
}

// The Warmup client refuses a base URL that is not https before it stores
// anything, and strips trailing slashes from one that is.
func TestNewWarmupClientRefusesHTTPAndTrimsTrailingSlashes(t *testing.T) {
	for _, base := range []string{"http://warmup.example.test", "HTTP://warmup.example.test/", "ftp://warmup.example.test"} {
		if client, err := NewWarmupClient("warmup-key", base); err == nil || client != nil {
			t.Fatalf("%q accepted: %v", base, client)
		}
	}
	var path string
	responder, _ := useWarmupResponder(t, "warmup-key", func(req *http.Request, _ string) (*http.Response, error) {
		path = req.URL.Path
		return textResponse(req, 200, CONTENT_TYPE_JSON, `{"success":true}`), nil
	})
	for _, base := range []string{"https://warmup.example.test/", "https://warmup.example.test///"} {
		warmup, err := NewWarmupClient("warmup-key", base)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := warmup.GetSeedCount(); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(path, warmupBasePath+"/") || strings.Contains(path, "//") {
			t.Fatalf("%q: path %q", base, path)
		}
	}
	if len(responder.take()) != 2 {
		t.Fatal("want two requests")
	}
}

// The content type alone decides when it contradicts the body: a text/html
// response is an HTML page even when its body looks like JSON (every call),
// or like a CSV (every download, Warmup CSV downloads included), and a JSON
// content type is not a result file even when the body looks like a CSV.
// Each case is caught only by the content-type half of its guard.
func TestEveryCallRejectsAContentTypeThatContradictsTheBody(t *testing.T) {
	const csvBody = "email,status\na@example.com,valid\n"
	csv := map[string]bool{}
	for _, method := range warmupCSVMethods(t) {
		csv["WarmupClient."+method] = true
	}
	downloads := downloadCalls(t)
	cases := []struct {
		name, contentType, body string
		downloadsOnly           bool
	}{
		{"html content type, json body", "text/html; charset=UTF-8", `{"a":1}`, false},
		{"html content type, csv body", "Text/HTML", csvBody, true},
		{"json content type, csv body", "application/json; charset=utf-8", csvBody, true},
	}
	for _, tc := range cases {
		calls := sweepWith(t, func(req *http.Request) *http.Response {
			return textResponse(req, 200, tc.contentType, tc.body)
		})
		for _, call := range calls {
			if tc.downloadsOnly && !downloads[call.name] && !csv[call.name] {
				continue
			}
			t.Run(tc.name+"/"+call.name, func(t *testing.T) {
				sweepDownloads.Reset()
				err := call.call()
				assertStatus200(t, err)
				if !tc.downloadsOnly && !csv[call.name] && !strings.Contains(err.Error(), "HTML") {
					t.Fatalf("message %q does not say HTML", err.Error())
				}
				if sweepDownloads.Len() != 0 {
					t.Fatalf("wrote %d bytes", sweepDownloads.Len())
				}
			})
		}
	}
}
