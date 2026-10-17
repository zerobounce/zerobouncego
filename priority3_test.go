package zerobouncego

import (
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
		File:         strings.NewReader("domain,name\nacme.com,Jane"),
		FileName:     "finder.csv",
		DomainColumn: 1,
	}
	resp, err := BulkEmailFinderSubmit(file)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
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

func TestValidateS3FileURL(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		"https://bulkapi.zerobounce.net/validateS3file",
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(200, `{"success":true,"file_id":"s3-job"}`), nil
		},
	)

	hasHeader := true
	removeDup := true
	resp, err := ValidateS3File(ValidateS3FileOptions{
		FileName:         "emails/list.csv",
		HasHeaderRow:     &hasHeader,
		RemoveDuplicates: &removeDup,
	})
	assert.NoError(t, err)
	assert.Equal(t, "s3-job", resp.FileId)
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
