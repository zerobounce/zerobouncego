// The test cases in this file

package zerobouncego

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

// reusing constants from file_validation_test.go

const (
	invalid_file_id  = "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX"
	sample_scoring_submit_ok = `{
		"success": true,
		"message": "File Accepted",
		"file_name": "` + file_name_200 + `",
		"file_id": "` + testing_file_id + `"
	}`
	sample_scoring_file_status_200_invalid = `{
		"success": false,
		"message": "file_id is invalid."
	}`
	sample_scoring_file_status_200_ok = `{
		"success": true,
		"file_id": "` + testing_file_id + `",
		"file_name": "` + file_name_200 + `",
		"upload_date": "` + sample_date_time + `",
		"file_status": "Complete",
		"complete_percentage": "100% Complete.",
		"return_url": null
	}`
	sample_scoring_delete_200_invalid = `{
		"success": false,
		"message": "file_id is invalid."
	}`
	sample_scoring_delete_200_success = `{
		"success": true,
		"message": "File Deleted",
		"file_name": "` + file_name_200 + `",
		"file_id": "` + testing_file_id + `"
	}`
	sample_get_results_200_not_success = `{
		"success": false,
		"message": "File not processed."
	}`
	sample_get_results_200_content = `"Email Address","ZeroBounceQualityScore"` + "\n" +
		`"valid@example.com","10"` + "\n" +
		`"invalid@example.com","10"` + "\n" +
		`"toxic@example.com","2"`
)


// TestScoringSubmitEnsureParametersSubmit - ensure that a configured `CsvFile`
// instance has all its parameters passed to the request
func TestScoringSubmitEnsureParametersSubmit(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	csv_file := testingCsvFileOk()

	// mock the function also does the validation
	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)`+ENDPOINT_SCORING_SEND+`(.*)\z`,
		func(request *http.Request) (*http.Response, error) {
			var error_ error
			error_ = request.ParseMultipartForm(100_000_000)
			if error_ != nil {
				return nil, error_
			}

			// following conversion is valid as both url.Values and multipart.Form.Value
			// are type aliases for `map[string][]string`
			var form_values url.Values = request.MultipartForm.Value

			// ensure expected fields exist
			expected_fields := []string{
				"has_header_row",
				"email_address_column",
			}
			for _, field := range expected_fields {
				assert.Truef(t, form_values.Has(field), field)
			}

			// ensure fields have expected values
			assert.Equal(t, API_KEY, form_values.Get("api_key"))
			assert.Equal(t, fmt.Sprintf("%v", csv_file.HasHeaderRow), form_values.Get("has_header_row"))
			assert.Equal(t, fmt.Sprintf("%d", csv_file.EmailAddressColumn), form_values.Get("email_address_column"))

			// ensure file was properly sent
			file_header_list := request.MultipartForm.File["file"]
			if len(file_header_list) == 0 {
				return nil, errors.New("file contents not found")
			}
			file_header := file_header_list[0]
			assert.Equal(t, csv_file.FileName, file_header.Filename)
			file_descriptor, error_ := file_header.Open()
			if error_ != nil {
				return nil, error_
			}

			defer file_descriptor.Close()
			file_contents, error_ := io.ReadAll(file_descriptor)
			if error_ != nil {
				return nil, error_
			}
			assert.Equal(t, sample_file_contents, string(file_contents))

			return httpmock.NewStringResponse(201, send_file_response_200), nil
		},
	)

	// making the request
	response_object, error_ := AiScoringFileSubmit(csv_file, false)
	assert.Nil(t, error_)
	assert.NotNil(t, response_object)
}

// TestScoringSubmitLibraryError - error returned by http library
func TestScoringSubmitLibraryError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock responder
	mockErrorResponse("POST", ENDPOINT_SCORING_SEND)

	// make request
	response_object, error_ := AiScoringFileSubmit(testingCsvFileOk(), false)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
}

// TestScoringSubmit400Error - bad request returned as response
func TestScoringSubmit400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock request to return 400 response
	mockBadRequestResponse("POST", ENDPOINT_SCORING_SEND)

	// make request
	response_object, error_ := AiScoringFileSubmit(testingCsvFileOk(), false)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
}

// TestScoringSubmit200NotSuccess - error encountered but not a bad request
func TestScoringSubmit200NotSuccess(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// 200OK will be interpret as error as the file was not created
	mockOkResponse(
		"GET",
		ENDPOINT_SCORING_SEND,
		sample_scoring_file_status_200_invalid,
	)
	response_object, error_ := AiScoringFileSubmit(testingCsvFileOk(), false)
	assert.NotNil(t, error_)
	assert.Nil(t, response_object)
}

// TestScoringSubmit200Success - process expected to go accordingly
func TestScoringSubmit200Success(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)`+ENDPOINT_FILE_SEND+`(.*)\z`,
		httpmock.NewStringResponder(201, sample_scoring_submit_ok),
	)

	response_object, error_ := AiScoringFileSubmit(testingCsvFileOk(), false)
	if !assert.Nil(t, error_) { t.FailNow() }
	if !assert.NotNil(t, response_object) { t.FailNow() }
	assert.Equal(t, true, response_object.Success)
	assert.Equal(t, file_name_200, response_object.FileName)
	assert.Equal(t, testing_file_id, response_object.FileId)
}

// TestScoringStatusLibraryError - error returned by http library
func TestScoringStatusLibraryError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock responder
	mockErrorResponse("GET", ENDPOINT_SCORING_STATUS)

	// make request
	response_object, error_ := AiScoringFileStatus(testing_file_id)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)

}

// TestScoringStatus400Error - bad request returned as response
func TestScoringStatus400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock request to return 400 response
	mockBadRequestResponse("GET", ENDPOINT_SCORING_STATUS)

	// make request
	response_object, error_ := AiScoringFileStatus(testing_file_id)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
}

// TestScoringStatus200NotSuccess - error encountered but not a bad request (invalid)
func TestScoringStatus200NotSuccess(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse(
		"GET",
		ENDPOINT_SCORING_STATUS,
		sample_scoring_file_status_200_invalid,
	)

	response_object, error_ := AiScoringFileStatus(testing_file_id)
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	if !assert.NotNil(t, response_object) {
		t.FailNow()
	}
	assert.Equal(t, false, response_object.Success)
}

// TestScoringStatus200Success - process expected to go accordingly
func TestScoringStatus200Success(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse(
		"GET",
		ENDPOINT_SCORING_STATUS,
		sample_scoring_file_status_200_ok,
	)

	response_object, error_ := AiScoringFileStatus(testing_file_id)
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	if !assert.NotNil(t, response_object) {
		t.FailNow()
	}
	assert.Equal(t, true, response_object.Success)
	assert.Equal(t, file_name_200, response_object.FileName)
	assert.Equal(t, testing_file_id, response_object.FileId)
	assert.Equal(t, 100., response_object.Percentage())  // float

	// date parsing
	expected_parsed_date, error_ := time.Parse(time.RFC3339, sample_date_time)
	if !assert.Nil(t, error_) {t.FailNow()}
	response_date, error_ := response_object.UploadDate()
	if !assert.Nil(t, error_) {t.FailNow()}

	assert.Equal(t, expected_parsed_date, response_date)
}


// TestScoringDeleteLibraryError - error returned by http library
func TestScoringDeleteLibraryError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock responder
	mockErrorResponse("GET", ENDPOINT_SCORING_DELETE)

	// make request
	response_object, error_ := AiScoringFileDelete(testing_file_id)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)

}

// TestScoringDelete400Error - bad request returned as response
func TestScoringDelete400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock request to return 400 response
	mockBadRequestResponse("GET", ENDPOINT_SCORING_DELETE)

	// make request
	response_object, error_ := AiScoringFileDelete(testing_file_id)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)

}

// TestScoringDelete200NotSuccess - error encountered but not a bad request (invalid)
func TestScoringDelete200NotSuccess(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse("GET", ENDPOINT_SCORING_DELETE, sample_scoring_delete_200_invalid)
	response_object, error_ := AiScoringFileDelete(testing_file_id)
	assert.Nil(t, error_)
	if !assert.NotNil(t, response_object) { t.FailNow() }
	assert.Equal(t, false, response_object.Success)
}

// TestScoringDelete200Success - process expected to go accordingly
func TestScoringDelete200Success(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse("GET", ENDPOINT_SCORING_DELETE, sample_scoring_delete_200_success)
	response_object, error_ := AiScoringFileDelete(testing_file_id)
	assert.Nil(t, error_)
	if !assert.NotNil(t, response_object) { t.FailNow() }
	assert.Equal(t, true, response_object.Success)
}

// TestScoringResultLibraryError - error returned by http library
func TestScoringResultLibraryError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	string_builder := &strings.Builder{}

	// mock responder
	mockErrorResponse("GET", ENDPOINT_SCORING_RESULT)

	// make request
	error_ := AiScoringResult(testing_file_id, string_builder)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
	assert.Equal(t, "", string_builder.String())
}

// TestScoringResult400Error - bad request returned as response
func TestScoringResult400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	string_builder := &strings.Builder{}

	// mock request to return 400 response
	mockBadRequestResponse("GET", ENDPOINT_SCORING_RESULT)

	// make request
	error_ := AiScoringResult(testing_file_id, string_builder)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
	assert.Equal(t, "", string_builder.String())
}

// TestScoringResult200NotSuccess - error encountered but not a bad request (invalid)
func TestScoringResult200NotSuccess(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	string_builder := &strings.Builder{}

	mockOkResponse("GET", ENDPOINT_SCORING_RESULT, sample_get_results_200_not_success)
	error_ := AiScoringResult(testing_file_id, string_builder)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Equal(t, "", string_builder.String())
}

// TestScoringResult200Success - process expected to go accordingly
func TestScoringResult200Success(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	string_builder := &strings.Builder{}

	httpmock.RegisterResponder(
		"GET",
		`=~^(.*)`+ENDPOINT_SCORING_RESULT+`(.*)\z`,
		func(r *http.Request) (*http.Response, error) {
			response := httpmock.NewBytesResponse(200, []byte(sample_get_results_200_content))
			response.Header.Add("Content-Type", CONTENT_TYPE_OCTET_STREAM)
			return response, nil
		},
	)

	error_ := AiScoringResult(testing_file_id, string_builder)
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	assert.Equal(t, sample_get_results_200_content, string_builder.String())
}
