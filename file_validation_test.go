// This file handles tests for the bulk validation functionality.
// In order to mock this endpoint, responses will be provided based on
// the file names (skipping their contents) thus simulating file upload.

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

const (
	sample_file_validation_status_200_invalid = `{
		"success": false,
		"message": "file_id is invalid."
	}`
	sample_file_validation_status_200_ok = `{
		"success": true,
		"file_id": "` + testing_file_id + `",
		"file_name": "` + file_name_200 + `",
		"upload_date": "` + sample_date_time + `",
		"file_status": "Complete",
		"complete_percentage": "100%",
		"error_reason": null,
		"return_url": null
	}`
	sample_validation_delete_200_not_success = `{
		"success": false,
		"message": "File cannot be found."
	}`
	sample_validation_delete_200_success = `{
		"success": true,
		"message": "File Deleted",
		"file_name": "` + file_name_200 + `",
		"file_id": "` + testing_file_id + `"
	}`
	sample_validation_result_200_not_success = `{
		"success": false,
		"message": "File cannot be found."
	}`
	sample_validation_result_200_content = `inaccurate_content_used_for_mocking`
)

// handleMockedBulkValidate - simple file-to-response mock
func handleMockedBulkValidate(request *http.Request) (*http.Response, error) {
	_, file_header, error_ := request.FormFile("file")
	if error_ != nil {
		return nil, error_
	}
	switch file_header.Filename {
	case file_name_400:
		return httpmock.NewStringResponse(400, send_file_response_400), nil

	case file_name_200:
		return httpmock.NewStringResponse(201, send_file_response_200), nil
	}
	return nil, errors.New("case not covered")
}

// TestEnsureParametersArePassedToRequest - ensure that a configured `CsvFile`
// instance has all its parameters passed to the request
func TestAllParametersArePassedToRequest(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	csv_file := CsvFile{
		File:               strings.NewReader(sample_file_contents),
		FileName:           file_name_200,
		HasHeaderRow:       true,
		EmailAddressColumn: 1,
		FirstNameColumn:    2,
		LastNameColumn:     3,
		GenderColumn:       4,
		IpAddressColumn:    5,
	}

	// mock the function also does the validation
	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)`+ENDPOINT_FILE_SEND+`(.*)\z`,
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
				"first_name_column",
				"last_name_column",
				"gender_column",
				"ip_address_column",
			}
			for _, field := range expected_fields {
				assert.Truef(t, form_values.Has(field), field)
			}

			// ensure fields have expected values
			assert.Equal(t, API_KEY, form_values.Get("api_key"))
			assert.Equal(t, fmt.Sprintf("%v", csv_file.HasHeaderRow), form_values.Get("has_header_row"))
			assert.Equal(t, fmt.Sprintf("%d", csv_file.EmailAddressColumn), form_values.Get("email_address_column"))
			assert.Equal(t, fmt.Sprintf("%d", csv_file.FirstNameColumn), form_values.Get("first_name_column"))
			assert.Equal(t, fmt.Sprintf("%d", csv_file.LastNameColumn), form_values.Get("last_name_column"))
			assert.Equal(t, fmt.Sprintf("%d", csv_file.GenderColumn), form_values.Get("gender_column"))
			assert.Equal(t, fmt.Sprintf("%d", csv_file.IpAddressColumn), form_values.Get("ip_address_column"))

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
	response_object, error_ := BulkValidationSubmit(csv_file, false)
	assert.Nil(t, error_)
	assert.NotNil(t, response_object)
}

func TestSomeParametersArePassedToRequest(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	csv_file := CsvFile{
		File:               strings.NewReader(""),
		FileName:           file_name_200,
		HasHeaderRow:       true,
		EmailAddressColumn: 1,
	}

	// mock the function also does the validation
	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)`+ENDPOINT_FILE_SEND+`(.*)\z`,
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
				assert.Truef(t, form_values.Has(field), "missing field %s", field)
			}
			excluded_fields := []string{
				"first_name_column",
				"last_name_column",
				"gender_column",
				"ip_address_column",
			}
			for _, field := range excluded_fields {
				assert.Truef(t, !form_values.Has(field), "unexpected field %s", field)
			}

			// ensure fields have expected values
			assert.Equal(t, API_KEY, form_values.Get("api_key"))
			assert.Equal(t, fmt.Sprintf("%v", csv_file.HasHeaderRow), form_values.Get("has_header_row"))
			assert.Equal(t, fmt.Sprintf("%d", csv_file.EmailAddressColumn), form_values.Get("email_address_column"))

			// check file contents
			file_header_list := request.MultipartForm.File["file"]
			if len(file_header_list) == 0 {
				return nil, errors.New("file contents not found")
			}
			file_header := file_header_list[0]
			file_descriptor, error_ := file_header.Open()
			if error_ != nil {
				return nil, error_
			}

			defer file_descriptor.Close()
			file_contents, error_ := io.ReadAll(file_descriptor)
			if error_ != nil {
				return nil, error_
			}
			assert.Equal(t, "", string(file_contents))

			return httpmock.NewStringResponse(201, send_file_response_200), nil
		},
	)

	// making the request
	response_object, error_ := BulkValidationSubmit(csv_file, false)
	assert.Nil(t, error_)
	assert.NotNil(t, response_object)
}

// TestBulkValidateSubmit400Error - bad request returned as response
func TestBulkValidateSubmit400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)`+ENDPOINT_FILE_SEND+`(.*)\z`,
		handleMockedBulkValidate,
	)

	// setup file
	csv_file := CsvFile{
		File:               strings.NewReader(""),
		FileName:           file_name_400,
		HasHeaderRow:       true,
		EmailAddressColumn: 1,
	}
	response, error_ := BulkValidationSubmit(csv_file, false)
	assert.Nil(t, response)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
}

// TestBulkValidateSubmit200Success - process expected to go accordingly
func TestBulkValidateSubmit200Success(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)`+ENDPOINT_FILE_SEND+`(.*)\z`,
		handleMockedBulkValidate,
	)

	csv_file := CsvFile{
		File:               strings.NewReader(""),
		FileName:           file_name_200,
		HasHeaderRow:       true,
		EmailAddressColumn: 1,
	}
	validate_object, error_ := BulkValidationSubmit(csv_file, false)
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	if !assert.NotNil(t, validate_object) {
		t.FailNow()
	}

	assert.Equal(t, validate_object.Success, true)
	assert.Equal(t, validate_object.FileName, file_name_200)
	assert.Equal(t, validate_object.FileId, testing_file_id)
}

// BORDER

// TestBulkValidateSubmitLibraryError - error returned by http library
func TestBulkValidateSubmitLibraryError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock responder
	mockErrorResponse("POST", ENDPOINT_FILE_SEND)

	// make request
	response_object, error_ := BulkValidationSubmit(testingCsvFileOk(), false)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
}

// TestBulkValidateSubmit200NotSuccess - error encountered but not a bad request
func TestBulkValidateSubmit200NotSuccess(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// 200OK will be interpret as error as the file was not created
	mockOkResponse(
		"GET",
		ENDPOINT_FILE_SEND,
		sample_file_validation_status_200_invalid,
	)
	response_object, error_ := BulkValidationSubmit(testingCsvFileOk(), false)
	assert.NotNil(t, error_)
	assert.Nil(t, response_object)
}

// TestBulkValidateStatusLibraryError - error returned by http library
func TestBulkValidateStatusLibraryError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock responder
	mockErrorResponse("GET", ENDPOINT_FILE_STATUS)

	// make request
	response_object, error_ := BulkValidationFileStatus(testing_file_id)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)

}

// TestBulkValidateStatus400Error - bad request returned as response
func TestBulkValidateStatus400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock request to return 400 response
	mockBadRequestResponse("GET", ENDPOINT_FILE_STATUS)

	// make request
	response_object, error_ := BulkValidationFileStatus(testing_file_id)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
}

// TestBulkValidateStatus200NotSuccess - error encountered but not a bad request (invalid)
func TestBulkValidateStatus200NotSuccess(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse(
		"GET",
		ENDPOINT_FILE_STATUS,
		sample_file_validation_status_200_invalid,
	)

	response_object, error_ := BulkValidationFileStatus(testing_file_id)
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	if !assert.NotNil(t, response_object) {
		t.FailNow()
	}
	assert.Equal(t, false, response_object.Success)
}

// TestBulkValidateStatus200Success - process expected to go accordingly
func TestBulkValidateStatus200Success(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse(
		"GET",
		ENDPOINT_FILE_STATUS,
		sample_file_validation_status_200_ok,
	)

	response_object, error_ := BulkValidationFileStatus(testing_file_id)
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	if !assert.NotNil(t, response_object) {
		t.FailNow()
	}
	assert.Equal(t, true, response_object.Success)
	assert.Equal(t, file_name_200, response_object.FileName)
	assert.Equal(t, testing_file_id, response_object.FileId)
	assert.Equal(t, 100., response_object.Percentage()) // float

	// date parsing
	expected_parsed_date, error_ := time.Parse(time.RFC3339, sample_date_time)
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	response_date, error_ := response_object.UploadDate()
	if !assert.Nil(t, error_) {
		t.FailNow()
	}

	assert.Equal(t, expected_parsed_date, response_date)
}

// TestBulkValidateDeleteLibraryError - error returned by http library
func TestBulkValidateDeleteLibraryError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock responder
	mockErrorResponse("GET", ENDPOINT_FILE_DELETE)

	// make request
	response_object, error_ := BulkValidationFileDelete(testing_file_id)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)

}

// TestBulkValidateDelete400Error - bad request returned as response
func TestBulkValidateDelete400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// mock request to return 400 response
	mockBadRequestResponse("GET", ENDPOINT_FILE_DELETE)

	// make request
	response_object, error_ := BulkValidationFileDelete(testing_file_id)
	assert.Nil(t, response_object)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)

}

// TestBulkValidateDelete200NotSuccess - error encountered but not a bad request (invalid)
func TestBulkValidateDelete200NotSuccess(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse("GET", ENDPOINT_FILE_DELETE, sample_validation_delete_200_not_success)
	response_object, error_ := BulkValidationFileDelete(testing_file_id)
	assert.Nil(t, error_)
	if !assert.NotNil(t, response_object) {
		t.FailNow()
	}
	assert.Equal(t, false, response_object.Success)
}

// TestBulkValidateDelete200Success - process expected to go accordingly
func TestBulkValidateDelete200Success(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse("GET", ENDPOINT_FILE_DELETE, sample_validation_delete_200_success)
	response_object, error_ := BulkValidationFileDelete(testing_file_id)
	assert.Nil(t, error_)
	if !assert.NotNil(t, response_object) {
		t.FailNow()
	}
	assert.Equal(t, true, response_object.Success)
}

// TestBulkValidateResultLibraryError - error returned by http library
func TestBulkValidateResultLibraryError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	string_builder := &strings.Builder{}

	// mock responder
	mockErrorResponse("GET", ENDPOINT_FILE_RESULT)

	// make request
	error_ := BulkValidationResult(testing_file_id, string_builder)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
	assert.Equal(t, "", string_builder.String())
}

// TestBulkValidateResult400Error - bad request returned as response
func TestBulkValidateResult400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	string_builder := &strings.Builder{}

	// mock request to return 400 response
	mockBadRequestResponse("GET", ENDPOINT_FILE_RESULT)

	// make request
	error_ := BulkValidationResult(testing_file_id, string_builder)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
	assert.Equal(t, "", string_builder.String())
}

// TestBulkValidateResult200NotSuccess - error encountered but not a bad request (invalid)
func TestBulkValidateResult200NotSuccess(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	string_builder := &strings.Builder{}

	mockOkResponse("GET", ENDPOINT_FILE_RESULT, sample_validation_result_200_not_success)
	error_ := BulkValidationResult(testing_file_id, string_builder)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Equal(t, "", string_builder.String())
}

// TestBulkValidateResult200Success - process expected to go accordingly
func TestBulkValidateResult200Success(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	string_builder := &strings.Builder{}

	httpmock.RegisterResponder(
		"GET",
		`=~^(.*)`+ENDPOINT_FILE_RESULT+`(.*)\z`,
		func(r *http.Request) (*http.Response, error) {
			response := httpmock.NewBytesResponse(200, []byte(sample_validation_result_200_content))
			response.Header.Add("Content-Type", "application/octet-stream")
			return response, nil
		},
	)

	error_ := BulkValidationResult(testing_file_id, string_builder)
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	assert.Equal(t, sample_validation_result_200_content, string_builder.String())
}
