package zerobouncego

import (
	"strings"
	"testing"
)

// return_url is sent only when the caller set one (file_management.go:87), on
// every sendfile call that takes it.
func TestSendFileOmitsReturnURLUnlessSet(t *testing.T) {
	fake := useFake(t)
	Initialize("return-url-key")
	submits := map[string]func(CsvFile) (*FileValidationResponse, error){
		"BulkValidationSubmit": func(f CsvFile) (*FileValidationResponse, error) { return BulkValidationSubmit(f, false) },
		"AiScoringFileSubmit":  func(f CsvFile) (*FileValidationResponse, error) { return AiScoringFileSubmit(f, false) },
	}
	for name, submit := range submits {
		newFile := func() CsvFile {
			return CsvFile{File: strings.NewReader("email\n"), FileName: "emails.csv", HasHeaderRow: true}
		}
		fake.clear()
		if _, err := submit(newFile()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, sent := onlyRequest(t, fake).Fields["return_url"]; sent {
			t.Fatalf("%s sent return_url although none was set", name)
		}

		withURL := newFile()
		withURL.ReturnURL = "https://example.com/hook"
		fake.clear()
		if _, err := submit(withURL); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := onlyRequest(t, fake).Fields["return_url"]; got != "https://example.com/hook" {
			t.Fatalf("%s return_url = %q", name, got)
		}
	}
}
