package zerobouncego

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every JSON call reads at most maxJSONBodyBytes, the bulk, filter, priority 3
// and Warmup calls included. A body over the cap is the size-cap error with
// the response's status. The getfile downloads are not capped.
func TestEveryJSONCallIsCapped(t *testing.T) {
	downloads := downloadCalls(t)
	big := `{"a":"` + strings.Repeat("x", 2<<20) + `"}`
	for _, status := range []int{200, 500} {
		useFake(t)
		fake := &oversizeTransport{status: status, body: big}
		http.DefaultTransport = fake
		client, warmup := pointEverythingAt(t, "https://api.example.test")
		for _, call := range publicRequestCalls(client, warmup) {
			if downloads[call.name] {
				continue
			}
			fake.last = 0
			err := call.call()
			if fake.last == 0 {
				continue // refused before sending (argument checks)
			}
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) || apiErr.Message != oversizeBodyMessage || apiErr.StatusCode != fake.last {
				t.Errorf("%d %s: err = %#v, want the size-cap error with status %d", status, call.name, err, fake.last)
			}
		}
	}
}

// A nil writer is refused before anything is sent, on every download entry
// point; it used to send the request and then panic on the first Write.
func TestNilWriterRefusedBeforeSending(t *testing.T) {
	var nilBuffer *bytes.Buffer
	opts := &GetFileOptions{}
	entries := map[string]func(w io.Writer) error{
		"BulkValidationResult":            func(w io.Writer) error { return BulkValidationResult("id", w) },
		"BulkValidationResultWithOptions": func(w io.Writer) error { return BulkValidationResultWithOptions("id", w, opts) },
		"AiScoringResult":                 func(w io.Writer) error { return AiScoringResult("id", w) },
		"AiScoringResultWithOptions":      func(w io.Writer) error { return AiScoringResultWithOptions("id", w, opts) },
		"BulkEmailFinderResult":           func(w io.Writer) error { return BulkEmailFinderResult("id", w) },
		"BulkDomainSearchResult":          func(w io.Writer) error { return BulkDomainSearchResult("id", w) },
		"GenericResultFetch":              func(w io.Writer) error { return GenericResultFetch("id", ENDPOINT_FILE_RESULT, w) },
		"GenericResultFetchWithOptions": func(w io.Writer) error {
			return GenericResultFetchWithOptions("id", ENDPOINT_FILE_RESULT, w, opts)
		},
	}
	for name, call := range entries {
		for label, writer := range map[string]io.Writer{"nil": nil, "typed nil": nilBuffer} {
			fake := useFake(t)
			API_KEY = "test-key-0123"
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s (%s) panicked: %v", name, label, r)
					}
				}()
				err = call(writer)
			}()
			if err == nil || !strings.Contains(err.Error(), "file_writer") {
				t.Errorf("%s (%s): err = %v, want an empty file_writer error", name, label, err)
			}
			if fake.size() != 0 {
				t.Errorf("%s (%s): sent %d requests", name, label, fake.size())
			}
		}
	}
}

// A list of messages is joined one per line (like the main calls), never cut
// to its first item.
func TestErrorMessageListsAreJoined(t *testing.T) {
	body := `{"error":["a","","b"]}`
	if got := FormatGetFileErrorMessage(body); got != "a\nb" {
		t.Errorf("FormatGetFileErrorMessage = %q", got)
	}
	err := getFileBodyError(400, "application/json", body)
	if err == nil || err.Error() != "a\nb" {
		t.Errorf("getFileBodyError = %v", err)
	}
	for _, in := range []string{body, `{"message":["a","b"]}`, `["a","b"]`, `{"detail":["a","b"]}`} {
		if got := warmupMessageFromBody([]byte(in), 400); got != "a\nb" {
			t.Errorf("warmupMessageFromBody(%s) = %q", in, got)
		}
	}
	if got := warmupMessageFromBody([]byte(`{"detail":[{"loc":["body","x"],"msg":"bad"}]}`), 422); got != "x: bad" {
		t.Errorf("validation list = %q", got)
	}
}

// ImportCsvFile sends the file's base name, not the local path.
func TestImportCsvFileUsesBaseName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "emails.csv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("a@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	csv, err := ImportCsvFile(path, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer csv.File.(*os.File).Close()
	if csv.FileName != "emails.csv" {
		t.Errorf("FileName = %q, want emails.csv", csv.FileName)
	}
}

// GetFileJSONIndicatesError: every valid JSON object or list is true, anything
// else (a result file, or text that only starts like JSON) is false.
func TestGetFileJSONIndicatesErrorMeaning(t *testing.T) {
	for body, want := range map[string]bool{
		`{"success":true}`: true, `{"x":1}`: true, `[1,2]`: true, "\xef\xbb\xbf {\"a\":1}": true,
		`{"error":"x"} trailing`: false, "a@b.c,valid\n": false, "": false, `{`: false,
	} {
		if got := GetFileJSONIndicatesError(body); got != want {
			t.Errorf("GetFileJSONIndicatesError(%q) = %v, want %v", body, got, want)
		}
	}
}

// failAfter fails only its n-th Write (0-based); the others succeed, so an
// ignored error is not caught by a later one.
type failAfter struct{ n, calls int }

func (f *failAfter) Write(p []byte) (int, error) {
	f.calls++
	if f.calls == f.n+1 {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

// Each WriteField / Write error path of FillMultipartForm is reached by a
// writer that fails only at one later write; every failure point is an error.
func TestFillMultipartFormErrorAtEveryWrite(t *testing.T) {
	for _, validation := range []bool{true, false} {
		for n := 0; ; n++ {
			w := &failAfter{n: n}
			csv := CsvFile{File: strings.NewReader("a"), FileName: "f.csv", EmailAddressColumn: 2, FirstNameColumn: 1,
				ReturnURL: "https://r.example/x", AllowPhase2: new(bool)}
			err := csv.FillMultipartForm(multipart.NewWriter(w), validation)
			if w.calls <= n { // the failing write was never reached: all writes done
				if err != nil {
					t.Fatalf("validation=%v n=%d: error without a failing write: %v", validation, n, err)
				}
				if n < 5 {
					t.Fatalf("validation=%v: only %d writes", validation, n)
				}
				break
			}
			if err == nil {
				t.Errorf("validation=%v: write %d failed, error ignored", validation, n)
			}
		}
	}
}
