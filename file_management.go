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

const (
	CONTENT_TYPE_OCTET_STREAM = "application/octet-stream"
	CONTENT_TYPE_JSON         = "application/json"

	// DownloadTypePhase1 / Phase2 / Combined are v2 bulk getfile download_type query values.
	DownloadTypePhase1   = "phase_1"
	DownloadTypePhase2   = "phase_2"
	DownloadTypeCombined = "combined"
)

// GetFileOptions holds optional v2 query parameters for bulk getfile.
type GetFileOptions struct {
	DownloadType *string
	// ActivityData is sent only for validation getfile (not AI scoring).
	ActivityData *bool
}

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

	// ReturnURL is optional; sent as return_url when non-empty.
	ReturnURL string
	// AllowPhase2 is optional; sent as allow_phase_2 for bulk validation sendfile only (not AI scoring).
	AllowPhase2 *bool
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
// current `CsvFile` instance. validationSendfile is true for bulk validation /sendfile only.
func (csv_file *CsvFile) FillMultipartForm(multipart_writer *multipart.Writer, validationSendfile bool) error {
	var error_ error
	var file_form_writer io.Writer

	// add the fields FIRST
	multipart_writer.WriteField("api_key", API_KEY)
	multipart_writer.WriteField("has_header_row", fmt.Sprintf("%v", csv_file.HasHeaderRow))

	if csv_file.ReturnURL != "" {
		multipart_writer.WriteField("return_url", csv_file.ReturnURL)
	}

	// add column-related fields
	columns_mapping := csv_file.ColumnsMapping()
	for column_key := range columns_mapping {
		multipart_writer.WriteField(column_key, fmt.Sprintf("%d", columns_mapping[column_key]))
	}

	if validationSendfile && csv_file.AllowPhase2 != nil {
		multipart_writer.WriteField("allow_phase_2", fmt.Sprintf("%t", *csv_file.AllowPhase2))
	}

	// add the file AFTERWARDS
	file_form_writer, error_ = multipart_writer.CreateFormFile("file", csv_file.FileName)
	if error_ != nil {
		return errors.New("error creating multipart form: " + error_.Error())
	}

	// add file in form-data and add terminating boundary
	contents, error_ := io.ReadAll(csv_file.File)
	if error_ != nil {
		return errors.New("error reading from csv file: " + error_.Error())
	}

	file_form_writer.Write(contents)
	error_ = multipart_writer.Close()
	if error_ != nil {
		return errors.New("error populating multiform with file: " + error_.Error())
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
	FilePhase2Status   *string `json:"file_phase_2_status,omitempty"`
	ReturnUrl          string `json:"return_url"`
}

// Percentage - provide the percentage, from a response payload, as a float
func (b *FileStatusResponse) Percentage() float64 {
	// expected structures: "10%", "10% Completed."
	parts := strings.Split(b.CompletePercentage, "%")
	if len(parts) == 0 {
		return -1
	}
	parsed_float, error_ := strconv.ParseFloat(parts[0], 64)
	if error_ != nil {
		return -1
	}
	return parsed_float
}

// UploadDate - provide the upload date, from a response payload, as a time.Time
func (b *FileStatusResponse) UploadDate() (time.Time, error) {
	return time.Parse(time.RFC3339, b.UploadDateRaw)
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
	validationSendfile := endpoint == ENDPOINT_FILE_SEND
	error_ = csv_file.FillMultipartForm(multipart_writer, validationSendfile)
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

// isScoringBulkEndpoint is true for paths under /scoring/ (e.g. scoring getfile).
func isScoringBulkEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "/scoring/")
}

// genericResultFetch implements bulk getfile with optional v2 query params and JSON error handling.
func genericResultFetch(file_id, endpoint string, file_writer io.Writer, opts *GetFileOptions, scoring bool) error {
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", file_id)
	if opts != nil {
		if opts.DownloadType != nil && *opts.DownloadType != "" {
			params.Set("download_type", *opts.DownloadType)
		}
		if !scoring && opts.ActivityData != nil {
			params.Set("activity_data", fmt.Sprintf("%t", *opts.ActivityData))
		}
	}

	url_to_request, err := url.JoinPath(BULK_URI, endpoint)
	if err != nil {
		return err
	}
	url_to_request = fmt.Sprintf("%s?%s", url_to_request, params.Encode())
	response_http, err := http.Get(url_to_request)
	if err != nil {
		return err
	}
	defer response_http.Body.Close()

	body, err := io.ReadAll(response_http.Body)
	if err != nil {
		return errors.New("could not read response body: " + err.Error())
	}

	bodyStr := string(body)
	ct := response_http.Header.Get("Content-Type")

	if response_http.StatusCode != 200 {
		trim := strings.TrimSpace(bodyStr)
		if strings.HasPrefix(trim, "{") {
			return fmt.Errorf("%s", FormatGetFileErrorMessage(trim))
		}
		if trim == "" {
			return fmt.Errorf("HTTP %d", response_http.StatusCode)
		}
		return fmt.Errorf("%s", trim)
	}

	if shouldTreatGetFileBodyAsError(bodyStr, ct) {
		return fmt.Errorf("%s", FormatGetFileErrorMessage(strings.TrimSpace(bodyStr)))
	}

	_, err = file_writer.Write(body)
	if err != nil {
		return errors.New("could not write into given file: " + err.Error())
	}
	return nil
}

// GenericResultFetch - save a csv containing the results of the file with the given file ID
func GenericResultFetch(file_id, endpoint string, file_writer io.Writer) error {
	return genericResultFetch(file_id, endpoint, file_writer, nil, isScoringBulkEndpoint(endpoint))
}

// GenericResultFetchWithOptions - same as GenericResultFetch with optional v2 query parameters.
func GenericResultFetchWithOptions(file_id, endpoint string, file_writer io.Writer, opts *GetFileOptions) error {
	return genericResultFetch(file_id, endpoint, file_writer, opts, isScoringBulkEndpoint(endpoint))
}

// GenericFileDelete - delete the result file associated with a file ID
func GenericFileDelete(file_id, endpoint string) (*FileValidationResponse, error) {
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", file_id)

	url_to_request, error_ := url.JoinPath(BULK_URI, endpoint)
	if error_ != nil {
		return nil, error_
	}
	url_to_request = fmt.Sprintf("%s?%s", url_to_request, params.Encode())

	response_http, error_ := http.Get(url_to_request)
	if error_ != nil {
		return nil, error_
	}
	if response_http.StatusCode != 200 {
		return nil, handleErrorPayload(response_http)
	}

	defer response_http.Body.Close()
	// only `Success` and `Message` are of interest
	response_object := &FileValidationResponse{}
	error_ = json.NewDecoder(response_http.Body).Decode(response_object)
	if error_ != nil {
		return nil, error_
	}

	response_object.FileId = file_id
	return response_object, nil
}
