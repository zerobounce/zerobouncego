package zerobouncego

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestBulkEmailFinderSubmitURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/email-finder/sendfile",
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(201, send_file_response_200), nil
		},
	)

	file := EmailFinderCsvFile{
		File:            strings.NewReader("domain,name\nacme.com,Jane"),
		FileName:        "finder.csv",
		DomainColumn:    1,
		FirstNameColumn: 2,
	}
	resp, err := BulkEmailFinderSubmit(file)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

// The finder is rejected without one of first_name_column / full_name_column,
// and the file never leaves the process.
func TestBulkEmailFinderSubmitRequiresANameColumn(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/email-finder/sendfile",
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(201, send_file_response_200), nil
		},
	)

	_, err := BulkEmailFinderSubmit(EmailFinderCsvFile{
		File:         strings.NewReader("domain\nacme.com"),
		FileName:     "finder.csv",
		DomainColumn: 1,
	})
	assert.ErrorContains(t, err, "first_name_column")
	assert.ErrorContains(t, err, "full_name_column")
	assert.Equal(t, 0, httpmock.GetTotalCallCount())

	// full_name_column alone is enough.
	_, err = BulkEmailFinderSubmit(EmailFinderCsvFile{
		File:           strings.NewReader("domain,name\nacme.com,Jane Doe"),
		FileName:       "finder.csv",
		DomainColumn:   1,
		FullNameColumn: 2,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, httpmock.GetTotalCallCount())
}

func TestBulkDomainSearchFileStatusURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		`=~^https://bulkapi\.zerobounce\.net/domain-search/filestatus`,
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "file-123", req.URL.Query().Get("file_id"))
			return httpmock.NewStringResponse(200, sample_file_validation_status_200_ok), nil
		},
	)

	resp, err := BulkDomainSearchFileStatus("file-123")
	assert.NoError(t, err)
	assert.Equal(t, testing_file_id, resp.FileId)
}

func TestListEvaluatorSubmitURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/v2/listevaluator/",
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(201, send_file_response_200), nil
		},
	)

	file := ListEvaluatorCsvFile{
		File:               strings.NewReader("email\na@b.com"),
		FileName:           "list.csv",
		EmailAddressColumn: 2,
	}
	_, err := ListEvaluatorSubmit(file)
	assert.NoError(t, err)
}

func TestGetListEvaluatorResultURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		`=~^https://bulkapi\.zerobounce\.net/v2/listevaluator/abc-def/`,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(200, `{"file_status":"Complete"}`), nil
		},
	)

	result, err := GetListEvaluatorResult("abc-def")
	assert.NoError(t, err)
	assert.Equal(t, "Complete", result["file_status"])
}

func TestAddFilterPostsToMainAPI(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)/filters/add\z`,
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "allow", req.FormValue("rule"))
			return httpmock.NewStringResponse(200, `{"Message":"Filter successfully added"}`), nil
		},
	)

	resp, err := AddFilter("allow", "domain", "example.com")
	assert.NoError(t, err)
	assert.Equal(t, "Filter successfully added", resp["Message"])
}

func TestListFiltersPostMethod(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)/filters/list\z`,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(200, `[]`), nil
		},
	)

	raw, err := ListFilters(true)
	assert.NoError(t, err)
	assert.Equal(t, "[]", string(raw))
}

func TestListS3FilesURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		`=~^https://bulkapi\.zerobounce\.net/listS3files`,
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "mock_key", req.URL.Query().Get("api_key"))
			assert.Equal(t, "my-bucket", req.URL.Query().Get("bucket"))
			return httpmock.NewStringResponse(200, `[{"name":"a.csv","size":1,"last_modified":"2024-01-01T00:00:00Z"}]`), nil
		},
	)

	entries, err := ListS3Files(ListS3FilesOptions{Bucket: "my-bucket"})
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "a.csv", entries[0].Name)
}

func TestValidateS3FileURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/validateS3file",
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "mock_key", req.FormValue("api_key"))
			assert.Equal(t, "emails/list.csv", req.FormValue("file_name"))
			assert.Equal(t, "true", req.FormValue("has_header_row"))
			assert.Equal(t, "true", req.FormValue("remove_duplicates"))
			assert.Equal(t, "https://example.com/done", req.FormValue("return_url"))
			// Only remove_duplicates is documented.
			assert.Empty(t, req.FormValue("remove_duplicate"))
			return httpmock.NewStringResponse(200, `{"success":true,"file_id":"s3-job"}`), nil
		},
	)

	hasHeader := true
	removeDup := true
	resp, err := ValidateS3File(ValidateS3FileOptions{
		FileName:         "emails/list.csv",
		HasHeaderRow:     &hasHeader,
		RemoveDuplicates: &removeDup,
		ReturnURL:        "https://example.com/done",
	})
	assert.NoError(t, err)
	assert.Equal(t, "s3-job", resp.FileId)
}

// LIVE: POST /validateS3file without return_url answers 400
// {"error_message":"Provided return URL is not permitted: None"}, so the SDK
// stops the job before it goes out.
func TestValidateS3FileRequiresReturnURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/validateS3file",
		httpmock.NewStringResponder(400, `{"error_message":"Provided return URL is not permitted: None"}`),
	)

	_, err := ValidateS3File(ValidateS3FileOptions{FileName: "emails/list.csv"})
	assert.ErrorContains(t, err, "return_url")
	assert.Equal(t, 0, httpmock.GetTotalCallCount())
}

func TestScoreS3FileFields(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/scoreS3file",
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "mock_key", req.FormValue("api_key"))
			assert.Equal(t, "emails/list.csv", req.FormValue("file_name"))
			assert.Equal(t, "https://example.com/done", req.FormValue("return_url"))
			assert.Equal(t, "false", req.FormValue("remove_duplicates"))
			assert.Empty(t, req.FormValue("remove_duplicate"))
			assert.Equal(t, "3", req.FormValue("email_address_column"))
			return httpmock.NewStringResponse(200, `{"success":true,"file_id":"score-job"}`), nil
		},
	)

	removeDup := false
	column := 3
	resp, err := ScoreS3File(ScoreS3FileOptions{
		FileName:           "emails/list.csv",
		RemoveDuplicates:   &removeDup,
		EmailAddressColumn: &column,
		ReturnURL:          "https://example.com/done",
	})
	assert.NoError(t, err)
	assert.Equal(t, "score-job", resp.FileId)
}

// Same gate on the scoring job, which did not send return_url at all.
func TestScoreS3FileRequiresReturnURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/scoreS3file",
		httpmock.NewStringResponder(400, `{"error_message":"Provided return URL is not permitted: None"}`),
	)

	_, err := ScoreS3File(ScoreS3FileOptions{FileName: "emails/list.csv"})
	assert.ErrorContains(t, err, "return_url")

	_, err = ScoreS3File(ScoreS3FileOptions{ReturnURL: "https://example.com/done"})
	assert.ErrorContains(t, err, "file_name")
	assert.Equal(t, 0, httpmock.GetTotalCallCount())
}

// The 400 body the API returns for a job it rejects comes back as a short error.
func TestScoreS3FileErrorBody(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/scoreS3file",
		httpmock.NewStringResponder(400, `{"error_message":"Provided return URL is not permitted: None"}`),
	)

	_, err := ScoreS3File(ScoreS3FileOptions{
		FileName:  "emails/list.csv",
		ReturnURL: "https://example.com/done",
	})
	assert.ErrorContains(t, err, "Provided return URL is not permitted")
}

func TestExportS3FileFields(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/exportS3file",
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "job-1", req.FormValue("file_id"))
			assert.Equal(t, "my-bucket", req.FormValue("export_bucket"))
			return httpmock.NewStringResponse(200, `{"success":true}`), nil
		},
	)

	resp, err := ExportS3File("job-1", "my-bucket")
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

// export_bucket is optional: an empty one falls back to the bucket configured
// in the account's settings, so it must not be sent and must not be rejected.
func TestExportS3FileBucketIsOptional(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/exportS3file",
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "job-1", req.FormValue("file_id"))
			assert.Empty(t, req.FormValue("export_bucket"))
			return httpmock.NewStringResponse(200, `{"success":true}`), nil
		},
	)

	resp, err := ExportS3File("job-1", "")
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 1, httpmock.GetTotalCallCount())

	_, err = ExportS3File("  ", "my-bucket")
	assert.ErrorContains(t, err, "file_id")
}

// ListS3Files must go through httpGet: a transport failure otherwise surfaces
// as a *url.Error carrying the api_key from the query string.
func TestListS3FilesRedactsAPIKeyOnTransportError(t *testing.T) {
	Initialize("mock_key_to_redact")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		`=~^https://bulkapi\.zerobounce\.net/listS3files`,
		func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp: connection refused")
		},
	)

	_, err := ListS3Files(ListS3FilesOptions{})
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "mock_key_to_redact")
	assert.Contains(t, err.Error(), "REDACTED")
}
