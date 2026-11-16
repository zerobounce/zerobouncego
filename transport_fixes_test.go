package zerobouncego

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

type funcTransport func(*http.Request) (*http.Response, error)

func (f funcTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A getfile body that is JSON is an error whatever its content type, BOM or
// shape; nothing is written.
func TestGetFileRefusesEveryJSONBody(t *testing.T) {
	bom := "\xef\xbb\xbf"
	bodies := []string{
		`{"success":false,"message":"nope"}`,
		bom + `{"success":false,"message":"nope"}`,
		bom + "\n " + `{"foo":"bar"}`,
		`[1,2]`,
		bom + `["a"]`,
		`{"foo":{"bar":1}}`,
	}
	types := []string{"text/csv", "application/octet-stream", "text/plain", "application/problem+json",
		"application/vnd.api+json; charset=utf-8", "text/x-thing+json", "APPLICATION/JSON"}
	for _, body := range bodies {
		for _, ct := range types {
			if err := getFileBodyError(200, ct, body); err == nil {
				t.Errorf("getfile accepted %q under %q", body, ct)
			}
		}
	}
	// end to end: nothing is written
	for _, body := range bodies {
		fake := &fakeTransport{}
		_ = fake
		previous := http.DefaultTransport
		http.DefaultTransport = funcTransport(func(r *http.Request) (*http.Response, error) {
			return textResponse(r, 200, "text/csv", body), nil
		})
		prevKey := API_KEY
		API_KEY = "test-key-0123"
		var out bytes.Buffer
		err := BulkValidationResult("id", &out)
		http.DefaultTransport = previous
		API_KEY = prevKey
		if err == nil || out.Len() != 0 {
			t.Errorf("BulkValidationResult(%q) err=%v wrote %d bytes", body, err, out.Len())
		}
	}
	// a real CSV (also with a BOM) still passes
	for _, csv := range []string{"email,status\na@example.com,valid\n", bom + "email,status\na@example.com,valid\n"} {
		if err := getFileBodyError(200, "text/csv", csv); err != nil {
			t.Errorf("csv refused: %v", err)
		}
	}
	for _, ct := range []string{"text/csv", "application/jsonl", "text/json-seq", "application/x-json"} {
		if ct != "text/csv" && contentTypeIncludesApplicationJSON(ct) {
			t.Errorf("%q wrongly counted as JSON", ct)
		}
	}
}

func TestWarmupCSVRefusesBOMAndJSON(t *testing.T) {
	bom := "\xef\xbb\xbf"
	for _, body := range []string{bom + `{"success":false,"message":"bad"}`, bom + "[1]", `{"a":1}`} {
		for _, ct := range []string{"text/csv", "application/problem+json"} {
			_, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, _ string) (*http.Response, error) {
				return textResponse(req, 200, ct, body), nil
			})
			text, err := warmup.DownloadSeeds()
			if err == nil || text != "" {
				t.Errorf("warmup csv accepted %q under %q", body, ct)
			}
			if strings.Contains(err.Error(), "not valid JSON") {
				t.Errorf("BOM body not parsed: %v", err)
			}
		}
	}
	_, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, _ string) (*http.Response, error) {
		return textResponse(req, 200, "text/csv", bom+"seed,provider\n"), nil
	})
	if text, err := warmup.DownloadSeeds(); err != nil || text == "" {
		t.Errorf("csv refused: %v", err)
	}
}

// CR and LF in a file name cannot start a new header inside the part.
func TestMultipartFileNameCannotInjectHeaders(t *testing.T) {
	name := "a\r\nX-Evil: 1\r\n\r\nb\"c.csv"
	var bodies []string
	useFake(t)
	API_KEY = "test-key-0123"
	http.DefaultTransport = funcTransport(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		return textResponse(r, 201, CONTENT_TYPE_JSON, `{"success":true,"message":"File Accepted","file_id":"AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA","file_name":"f.csv"}`), nil
	})
	csv := CsvFile{File: strings.NewReader("a@example.com\n"), FileName: name, HasHeaderRow: false, EmailAddressColumn: 1}
	if _, err := BulkValidationSubmit(csv, true); err != nil {
		t.Fatal(err)
	}
	if _, err := postBulkRootMultipart("https://bulkapi.zerobounce.net/v2/x", strings.NewReader("a"), name, nil); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 {
		t.Fatalf("%d bodies", len(bodies))
	}
	want := "Content-Disposition: form-data; name=\"file\"; filename=\"aX-Evil: 1b\\\"c.csv\"\r\n"
	for _, b := range bodies {
		if !strings.Contains(b, want) || strings.Contains(b, "\r\nX-Evil") {
			t.Errorf("part header not byte-exact:\n%q", b)
		}
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestFillMultipartFormReturnsWriteErrors(t *testing.T) {
	// the multipart writer streams to an io.Writer that always fails
	for _, validation := range []bool{true, false} {
		w := newFailingMultipart()
		csv := CsvFile{File: strings.NewReader("a"), FileName: "f.csv", EmailAddressColumn: 1, ReturnURL: "https://r.example/x"}
		if err := csv.FillMultipartForm(w, validation); err == nil {
			t.Errorf("write error ignored (validation=%v)", validation)
		}
	}
}

// The SDK has its own transport and a client without a cookie jar.
func TestSDKClientIsIndependentOfDefaults(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	previousJar := http.DefaultClient.Jar
	http.DefaultClient.Jar = jar
	defer func() { http.DefaultClient.Jar = previousJar }()

	c := timedHTTPClient()
	if c.Jar != nil {
		t.Fatal("client inherited the global cookie jar")
	}
	if c.Timeout != httpTimeout || c.CheckRedirect == nil {
		t.Fatal("timeout or redirect policy lost")
	}
	tr, ok := sdkTransport().(*http.Transport)
	if !ok || tr == http.DefaultTransport || tr.Proxy == nil {
		t.Fatalf("sdk transport = %#v", sdkTransport())
	}
	if tr.TLSClientConfig != nil && tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS verification off")
	}

	// cookies are neither stored nor sent
	var cookies []string
	calls := 0
	useFake(t)
	API_KEY = "test-key-0123"
	http.DefaultTransport = funcTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		cookies = append(cookies, r.Header.Get("Cookie"))
		resp := textResponse(r, 200, CONTENT_TYPE_JSON, `{"Credits":"5"}`)
		resp.Header.Add("Set-Cookie", "sid=1; Path=/")
		return resp, nil
	})
	for i := 0; i < 2; i++ {
		if _, err := GetCredits(); err != nil {
			t.Fatal(err)
		}
	}
	u, _ := url.Parse(URI)
	if calls != 2 || cookies[1] != "" || len(jar.Cookies(u)) != 0 {
		t.Fatalf("cookies leaked: calls=%d %q jar=%v", calls, cookies, jar.Cookies(u))
	}
}

func newFailingMultipart() *multipart.Writer { return multipart.NewWriter(failWriter{}) }
