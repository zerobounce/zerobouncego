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
	"os"
	"strconv"
	"strings"
	"time"
)

// CsvFile - used for bulk validations and AI scoring
type CsvFile struct {
	File         io.Reader `json:"file"`
	FileName     string    `json:"file_name"`
	HasHeaderRow bool      `json:"has_header_row"`

	// column index starts from 1
	// if either of the following will be 0, will be excluded from the request
	EmailAddressColumn int `json:"email_address_column"`
	FirstNameColumn    int `json:"first_name_column"`
	LastNameColumn     int `json:"last_name_column"`
	GenderColumn       int `json:"gender_column"`
	IpAddressColumn    int `json:"ip_address_column"`
}

// ColumnsMapping - function generating how columns-index mapping of the instance
func (c *CsvFile) ColumnsMapping() map[string]int {
	column_to_value := make(map[string]int)

	// include this field regardless, as it's required
	column_to_value["email_address_column"] = c.EmailAddressColumn

	// populate optional values
	if c.FirstNameColumn != 0 {
		column_to_value["first_name_column"] = c.FirstNameColumn
	}
	if c.LastNameColumn != 0 {
		column_to_value["last_name_column"] = c.LastNameColumn
	}
	if c.GenderColumn != 0 {
		column_to_value["gender_column"] = c.GenderColumn
	}
	if c.IpAddressColumn != 0 {
		column_to_value["ip_address_column"] = c.IpAddressColumn
	}

	return column_to_value
}

// FillMultipartForm - populate a multi-part form with the data contained within
// current `CsvFile` instance
func (csv_file *CsvFile) FillMultipartForm(multipart_writer *multipart.Writer) error {
	var error_ error
	var form_writer io.Writer

	// add the fields FIRST
	multipart_writer.WriteField("api_key", API_KEY)
	multipart_writer.WriteField("has_header_row", fmt.Sprintf("%v", csv_file.HasHeaderRow))

	// add column-related fields
	columns_mapping := csv_file.ColumnsMapping()
	for column_key := range columns_mapping {
		multipart_writer.WriteField(column_key, fmt.Sprintf("%d", columns_mapping[column_key]))
	}

	// add the file AFTERWARDS
	form_writer, error_ = multipart_writer.CreateFormFile("file", csv_file.FileName)
	if error_ != nil {
		return error_
	}

	// add file in form-data and add terminating boundary
	io.Copy(form_writer, csv_file.File)
	error_ = multipart_writer.Close()
	if error_ != nil {
		return error_
	}
	return nil
}

// FileValidationResponse - response payload from a successful
type FileValidationResponse struct {
	Success  bool        `json:"success"`
	Message  interface{} `json:"message"`
	FileName string      `json:"file_name"`
	FileId   string      `json:"file_id"`
}

// BulkValidationFileStatus - response payload after a file status check
type FileStatusResponse struct {
	Success            bool   `json:"success"`
	FileId             string `json:"file_id"`
	FileName           string `json:"file_name"`
	UploadDateRaw      string `json:"upload_date"`
	FileStatus         string `json:"file_status"`
	CompletePercentage string `json:"complete_percentage"`
	ReturnUrl          string `json:"return_url"`
}

// Percentage - provide the percentage, from a response payload, as a float
func (b *FileStatusResponse) Percentage() (float64, error) {
	float_string := strings.ReplaceAll(b.CompletePercentage, "%", "")
	return strconv.ParseFloat(float_string, 64)
}

// UploadDate - provide the upload date, from a response payload, as a time.Time
func (b *FileStatusResponse) UploadDate() (time.Time, error) {
	return time.Parse(time.RFC3339, b.UploadDateRaw)
}

// handleErrorPayload - generate error based on an error payload with expected
// response payload: {"success": false, "message": ...}
func handleErrorPayload(response *http.Response) error {
	var error_ error
	var response_payload map[string]interface{} // expected keys: success, message
	defer response.Body.Close()

	error_ = json.NewDecoder(response.Body).Decode(&response_payload)
	if error_ != nil {
		error_parsing := fmt.Errorf("error occurred while parsing a status %d response payload", response.StatusCode)
		return errors.Join(error_parsing, error_)
	}

	return fmt.Errorf("error message: %s", response_payload["message"])
}

// ImportCsvFile - import a file to be uploaded for validation
func ImportCsvFile(path_to_file string, has_header bool, email_column int) (*CsvFile, error) {
	var error_ error
	_, error_ = os.Stat(path_to_file)
	if error_ != nil {
		return nil, error_
	}
	file, error_ := os.Open(path_to_file)
	if error_ != nil {
		return nil, error_
	}

	// server interprets columns indexing from 1
	if email_column == 0 {
		email_column = 1
	}

	csv_file := &CsvFile{
		File: file, FileName: file.Name(), HasHeaderRow: has_header, EmailAddressColumn: email_column,
	}
	return csv_file, nil
}


// GenericFileSubmit - submits a csv file to an operation represented by the given endpoint
func GenericFileSubmit(
	csv_file CsvFile,
	remove_duplicate bool,
	endpoint string,
) (*FileValidationResponse, error) {
	var multipart_buffer *bytes.Buffer = &bytes.Buffer{}
	var error_ error

	// MULTI-PART FORM PREPARATION
	multipart_writer := multipart.NewWriter(multipart_buffer)
	multipart_writer.WriteField("remove_duplicate", fmt.Sprintf("%v", remove_duplicate))
	error_ = csv_file.FillMultipartForm(multipart_writer)
	if error_ != nil {
		return nil, error_
	}

	// THE ACTUAL REQUEST
	url_to_access, error_ := url.JoinPath(BULK_URI, endpoint)
	if error_ != nil {
		return nil, error_
	}
	response_http, error_ := http.DefaultClient.Post(
		url_to_access,
		multipart_writer.FormDataContentType(),
		multipart_buffer,
	)
	if error_ != nil {
		return nil, error_
	}

	// INTERPRET RESPONSE
	defer response_http.Body.Close()
	if response_http.StatusCode != 201 {
		return nil, handleErrorPayload(response_http)
	}

	// 201 OK
	response_object := &FileValidationResponse{}
	error_ = json.NewDecoder(response_http.Body).Decode(response_object)
	if error_ != nil {
		return nil, error_
	}
	return response_object, nil
}

// GenericFileStatusCheck - check the percentage of completion of a file uploaded
// for the operation represented by the given endpoint
func GenericFileStatusCheck(file_id, endpoint string) (*FileStatusResponse, error) {
	var error_ error
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", file_id)

	// Do the request
	url_to_request, error_ := url.JoinPath(BULK_URI, endpoint)
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
	response_object := &FileStatusResponse{}
	error_ = json.NewDecoder(response_http.Body).Decode(response_object)
	if error_ != nil {
		return nil, error_
	}
	return response_object, nil
}

// GenericResultFetch - save a csv containing the results of the file with the given file ID
func GenericResultFetch(file_id, endpoint string, file_writer io.WriteCloser) error {
	var error_ error

	// make request
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", file_id)
	url_to_request, error_ := url.JoinPath(BULK_URI, endpoint)
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

// GenericFileDelete - cancel the process started for a given file ID
func GenericFileDelete(file_id, endpoint string) error {
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", file_id)

	url_to_request, error_ := url.JoinPath(BULK_URI, endpoint)
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