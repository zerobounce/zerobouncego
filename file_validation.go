package zerobouncego

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
)

type BulkValidationRequest struct {
	File         *os.File
	HasHeaderRow bool

	// fields below represent column indexes in the csv file, starting with 1
	EmailAddressColumn int
}

type BulkValidationResponse struct {
	Success  bool        `json:"success"`
	Message  interface{} `json:"message"`
	FileName string      `json:"file_name"`
	FileId   string      `json:"file_id"`
}

// ImportFileForBulkValidation - import a file to be uploaded for validation
func ImportFileForBulkValidation(path_to_file string, has_header bool, email_column int) (*BulkValidationRequest, error) {
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
	bulk_request := &BulkValidationRequest{
		File: file, HasHeaderRow: has_header, EmailAddressColumn: email_column,
	}
	return bulk_request, nil
}

// BulkValidateViaFile - submit a file with emails for validation
// TODO: expose has_header and email_column params somehow
func BulkValidateViaFile(path_to_file string) (*BulkValidationResponse, error) {
	var bytes_buffer bytes.Buffer
	var error_ error
	var form_writer io.Writer

	// MULTI-PART FORM PREPARATION 
	request_payload, error_ := ImportFileForBulkValidation(path_to_file, true, 1)
	if error_ != nil {
		return nil, error_
	}

	defer request_payload.File.Close()
	multipart_writer := multipart.NewWriter(&bytes_buffer)

	form_writer, error_ = multipart_writer.CreateFormFile("file", path.Base(path_to_file))
	if error_ != nil {
		return nil, error_
	}

	// add file in form-data and add terminating boundary
	io.Copy(form_writer, request_payload.File)
	error_ = multipart_writer.Close()
	if error_ != nil {
		return nil, error_
	}

	// add the other fields
	multipart_writer.WriteField("api_key", API_KEY)
	multipart_writer.WriteField("has_header_row", fmt.Sprintf("%v", request_payload.HasHeaderRow))
	multipart_writer.WriteField("email_address_column", fmt.Sprintf("%d", request_payload.EmailAddressColumn))

	// THE ACTUAL REQUEST 
	endpoint, error_ := url.JoinPath(URI, ENDPOINT_FILE_SEND)
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
	response_object := &BulkValidationResponse{} 
	error_ = json.NewDecoder(response_http.Body).Decode(response_object)
	if error_ != nil {
		return nil, error_
	}
	return response_object, nil
}
