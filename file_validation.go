package zerobouncego

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BulkValidationResponse - response payload from a successful
type BulkValidationResponse struct {
	Success  bool        `json:"success"`
	Message  interface{} `json:"message"`
	FileName string      `json:"file_name"`
	FileId   string      `json:"file_id"`
}

// BulkValidationFileStatus - response payload after a file status check
type BulkValidationFileStatusResponse struct {
	Success				bool	`json:"success"`
	FileId				string	`json:"file_id"`
	FileName			string	`json:"file_name"`
	UploadDateRaw		string	`json:"upload_date"`
	FileStatus			string	`json:"file_status"`
	CompletePercentage	string	`json:"complete_percentage"`
	ReturnUrl			string	`json:"return_url"`
}

// Percentage - provide the percentage, from a response payload, as a float
func (b *BulkValidationFileStatusResponse)Percentage() (float64, error) {
	float_string := strings.ReplaceAll(b.CompletePercentage, "%", "")
	return strconv.ParseFloat(float_string, 64)
}


// UploadDate - provide the upload date, from a response payload, as a time.Time
func (b *BulkValidationFileStatusResponse)UploadDate() (time.Time, error) {
	return time.Parse(time.RFC3339, b.UploadDateRaw)
}


// handleErrorPayload - generate error based on an error payload with expected
// response payload: {"success": false, "message": ...}
func handleErrorPayload(response *http.Response) error {
	var error_ error
	var response_payload map[string]interface{}  // expected keys: success, message
	defer response.Body.Close()

	error_ = json.NewDecoder(response.Body).Decode(&response_payload)
	if error_ != nil {
		error_parsing := fmt.Errorf("error occurred while parsing a status %d response payload", response.StatusCode)
		return errors.Join(error_parsing, error_)
	}

	return fmt.Errorf("error message: %s", response_payload["message"])
}

// BulkValidate - submit a file with emails for validation
func BulkValidate(csv_file CsvFile, remove_duplicate bool) (*BulkValidationResponse, error) {
	var bytes_buffer bytes.Buffer
	var error_ error
	var form_writer io.Writer

	// MULTI-PART FORM PREPARATION 
	multipart_writer := multipart.NewWriter(&bytes_buffer)

	// add the fields FIRST
	multipart_writer.WriteField("api_key", API_KEY)
	multipart_writer.WriteField("has_header_row", fmt.Sprintf("%v", csv_file.HasHeaderRow))
	multipart_writer.WriteField("remove_duplicate", fmt.Sprintf("%v", remove_duplicate))

	// add column-related fields
	columns_mapping := csv_file.ColumnsMapping()
	for column_key := range columns_mapping {
		multipart_writer.WriteField(column_key, fmt.Sprintf("%d", columns_mapping[column_key]))
	}

	// add the file AFTERWARDS
	form_writer, error_ = multipart_writer.CreateFormFile("file", csv_file.FileName)
	if error_ != nil {
		return nil, error_
	}

	// add file in form-data and add terminating boundary
	io.Copy(form_writer, csv_file.File)
	error_ = multipart_writer.Close()
	if error_ != nil {
		return nil, error_
	}

	// THE ACTUAL REQUEST 
	endpoint, error_ := url.JoinPath(BULK_URI, ENDPOINT_FILE_SEND)
	if error_ != nil {
		return nil, error_
	}
	request, error_ := http.NewRequest("POST", endpoint, &bytes_buffer)
	if error_ != nil {
		return nil, error_
	}
	request.Header.Set("Content-Type", multipart_writer.FormDataContentType())
	response_http, error_ := http.DefaultClient.Do(request)
	if error_ != nil {
		return nil, error_
	}

	// INTERPRET RESPONSE
	defer response_http.Body.Close()
	if response_http.StatusCode != 201 {
		payload_error := handleErrorPayload(response_http)

		// expected error
		if response_http.StatusCode == 400 && remove_duplicate {
			error_duplicate := errors.New(`more than 50%% of the file has been modified due to duplicates`)
			return nil, errors.Join(error_duplicate, payload_error)
		}

		// unexpected error
		return nil, payload_error
	}

	// 201 OK
	response_object := &BulkValidationResponse{}
	error_ = json.NewDecoder(response_http.Body).Decode(response_object)
	if error_ != nil {
		return nil, error_
	}
	return response_object, nil
}


// BulkValidationFileStatus - check the percentage of completion of a file uploaded
// for bulk validation
func BulkValidationFileStatus(file_id string) (*BulkValidationFileStatusResponse, error) {
	var error_ error
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", file_id)

	response_object := &BulkValidationFileStatusResponse{}

	// Do the request
	url_to_request, error_ := url.JoinPath(BULK_URI, ENDPOINT_FILE_STATUS)
	if error_ != nil {
		return nil, error_
	}

	url_to_request = fmt.Sprintf("%s?%s", url_to_request, params.Encode())
	response_http, error_ := http.Get(url_to_request)
	if error_ != nil {
		return nil, error_
	}

	// Error response
	defer response_http.Body.Close()
	if response_http.StatusCode != 200 {
		return nil, handleErrorPayload(response_http)
	}

	// OK response
	error_ = json.NewDecoder(response_http.Body).Decode(response_object)
	if error_ != nil {
		return nil, error_
	}
	return response_object, nil
}


// BulkValidationResult - save a csv containing the results of the file with the given file ID
func BulkValidationResult(file_id string, file_writer io.WriteCloser) error {
	var error_ error

	// make request
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", file_id)
	url_to_request, error_ := url.JoinPath(BULK_URI, ENDPOINT_FILE_GET)
	if error_ != nil {
		return error_
	}

	url_to_request = fmt.Sprintf("%s?%s", url_to_request, params.Encode())
	response_http, error_ := http.Get(url_to_request)
	if error_ != nil {
		return error_
	}

	// handle errors
	defer response_http.Body.Close()
	if response_http.StatusCode != 200 {
		return handleErrorPayload(response_http)
	}
	content_type := response_http.Header.Get("Content-Type")
	if content_type != "application/octet-stream" {
		return fmt.Errorf(
			"unexpected content type; expected %s, got %s",
			"application/octet-stream",
			content_type,
		)
	}

	// save to file
	response_contents, error_ := io.ReadAll(response_http.Request.Body)
	if error_ != nil {
		return errors.Join(errors.New("could not read response body"), error_)
	}

	defer file_writer.Close()
	file_writer.Write(response_contents)
	_, error_ = file_writer.Write(response_contents)
	if error_ != nil {
		return errors.Join(errors.New("could not write into given file"), error_)
	}
	return nil
}


// BulkValidationDeleteFile - cancel the validation process for a given file ID
func BulkValidationDeleteFile(file_id string) error {
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", file_id)

	url_to_request, error_ := url.JoinPath(BULK_URI, ENDPOINT_FILE_DELETE)
	if error_ != nil {
		return error_
	}
	url_to_request = fmt.Sprintf("%s?%s", url_to_request, params.Encode())

	response_http, error_ := http.Get(url_to_request)
	if error_ != nil {
		return error_
	}
	if response_http.StatusCode != 200 {
		return handleErrorPayload(response_http)
	}
	return nil
}

