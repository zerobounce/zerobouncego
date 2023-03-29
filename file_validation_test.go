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

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
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
		LastNameColumn:		3,
		GenderColumn:		4,
		IpAddressColumn:	5,
	}

	// mock the function also does the validation
	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)` + ENDPOINT_FILE_SEND + `(.*)\z`,
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
	response_object, error_ := BulkValidate(csv_file, false)
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
		`=~^(.*)` + ENDPOINT_FILE_SEND + `(.*)\z`,
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
	response_object, error_ := BulkValidate(csv_file, false)
	assert.Nil(t, error_)
	assert.NotNil(t, response_object)
}

func TestBulkValidate400Error(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)` + ENDPOINT_FILE_SEND + `(.*)\z`,
		handleMockedBulkValidate,
	)

	// setup file
	csv_file := CsvFile{
		File:               strings.NewReader(""),
		FileName:           file_name_400,
		HasHeaderRow:       true,
		EmailAddressColumn: 1,
	}
	response, error_ := BulkValidate(csv_file, false)
	assert.Nil(t, response)
	if !assert.NotNil(t, error_) { t.FailNow() }
	assert.Contains(t, error_.Error(), sample_error_message)
}

func TestBulkValidate200OK(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		`=~^(.*)` + ENDPOINT_FILE_SEND + `(.*)\z`,
		handleMockedBulkValidate,
	)

	csv_file := CsvFile{
		File:               strings.NewReader(""),
		FileName:           file_name_200,
		HasHeaderRow:       true,
		EmailAddressColumn: 1,
	}
	validate_object, error_ := BulkValidate(csv_file, false)
	if !assert.Nil(t, error_) { t.FailNow() }
	if !assert.NotNil(t, validate_object) { t.FailNow() }

	assert.Equal(t, validate_object.Success, true)
	assert.Equal(t, validate_object.FileName, file_name_200)
	assert.Equal(t, validate_object.FileId, testing_file_id)
}
