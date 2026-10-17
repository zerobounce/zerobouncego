package zerobouncego

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// EmailFinderCsvFile input for bulk email finder sendfile.
type EmailFinderCsvFile struct {
	File             io.Reader
	FileName         string
	HasHeaderRow     bool
	DomainColumn     int
	FirstNameColumn  int
	LastNameColumn   int
	MiddleNameColumn int
	FullNameColumn   int
}

func (f EmailFinderCsvFile) multipartFields() map[string]string {
	fields := map[string]string{
		"domain_column": fmt.Sprintf("%d", f.DomainColumn),
	}
	if f.HasHeaderRow {
		fields["has_header_row"] = formatBoolField(true)
	}
	if v, ok := optionalIntField(f.FirstNameColumn); ok {
		fields["first_name_column"] = v
	}
	if v, ok := optionalIntField(f.LastNameColumn); ok {
		fields["last_name_column"] = v
	}
	if v, ok := optionalIntField(f.MiddleNameColumn); ok {
		fields["middle_name_column"] = v
	}
	if v, ok := optionalIntField(f.FullNameColumn); ok {
		fields["full_name_column"] = v
	}
	return fields
}

// DomainSearchCsvFile input for bulk domain search sendfile.
type DomainSearchCsvFile struct {
	File         io.Reader
	FileName     string
	HasHeaderRow bool
	DomainColumn int
}

// BulkEmailFinderSubmit uploads a CSV for bulk email finder.
func BulkEmailFinderSubmit(file EmailFinderCsvFile) (*FileValidationResponse, error) {
	if file.DomainColumn == 0 {
		return nil, fmt.Errorf("empty parameter: domain_column")
	}
	if file.File == nil {
		return nil, fmt.Errorf("empty parameter: file")
	}
	requestURL, err := url.JoinPath(emailFinderBaseURL(), "sendfile")
	if err != nil {
		return nil, err
	}
	return postBulkRootMultipart(requestURL, file.File, file.FileName, file.multipartFields())
}

func BulkEmailFinderFileStatus(fileID string) (*FileStatusResponse, error) {
	return bulkRootFileStatus(emailFinderBaseURL(), fileID)
}

func BulkEmailFinderResult(fileID string, fileWriter io.Writer) error {
	return bulkRootResultFetch(emailFinderBaseURL(), fileID, fileWriter)
}

func BulkEmailFinderFileDelete(fileID string) (*FileValidationResponse, error) {
	return bulkRootFileDelete(emailFinderBaseURL(), fileID)
}

// BulkDomainSearchSubmit uploads a CSV for bulk domain search.
func BulkDomainSearchSubmit(file DomainSearchCsvFile) (*FileValidationResponse, error) {
	if file.DomainColumn == 0 {
		return nil, fmt.Errorf("empty parameter: domain_column")
	}
	if file.File == nil {
		return nil, fmt.Errorf("empty parameter: file")
	}
	fields := map[string]string{
		"domain_column": fmt.Sprintf("%d", file.DomainColumn),
	}
	if file.HasHeaderRow {
		fields["has_header_row"] = formatBoolField(true)
	}
	requestURL, err := url.JoinPath(domainSearchBaseURL(), "sendfile")
	if err != nil {
		return nil, err
	}
	return postBulkRootMultipart(requestURL, file.File, file.FileName, fields)
}

func BulkDomainSearchFileStatus(fileID string) (*FileStatusResponse, error) {
	return bulkRootFileStatus(domainSearchBaseURL(), fileID)
}

func BulkDomainSearchResult(fileID string, fileWriter io.Writer) error {
	return bulkRootResultFetch(domainSearchBaseURL(), fileID, fileWriter)
}

func BulkDomainSearchFileDelete(fileID string) (*FileValidationResponse, error) {
	return bulkRootFileDelete(domainSearchBaseURL(), fileID)
}

// ListEvaluatorCsvFile input for list evaluator sendfile.
type ListEvaluatorCsvFile struct {
	File                io.Reader
	FileName            string
	EmailAddressColumn  int
}

// ListEvaluatorSubmit uploads a CSV for list evaluation.
func ListEvaluatorSubmit(file ListEvaluatorCsvFile) (*FileValidationResponse, error) {
	if file.EmailAddressColumn == 0 {
		return nil, fmt.Errorf("empty parameter: email_address_column")
	}
	if file.File == nil {
		return nil, fmt.Errorf("empty parameter: file")
	}
	fields := map[string]string{
		"email_address_column": fmt.Sprintf("%d", file.EmailAddressColumn),
	}
	requestURL, err := url.JoinPath(bulkV2Base(), "listevaluator")
	if err != nil {
		return nil, err
	}
	requestURL += "/"
	return postBulkRootMultipart(requestURL, file.File, file.FileName, fields)
}

// GetListEvaluatorResult fetches list evaluator job status/result metadata.
func GetListEvaluatorResult(fileID string) (map[string]interface{}, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("empty parameter: file_id")
	}
	params := url.Values{}
	params.Set("api_key", API_KEY)

	requestURL, err := url.JoinPath(bulkV2Base(), "listevaluator", fileID)
	if err != nil {
		return nil, err
	}
	requestURL += "/"
	requestURL = fmt.Sprintf("%s?%s", requestURL, params.Encode())

	responseHTTP, err := httpGet(requestURL)
	if err != nil {
		return nil, err
	}
	defer responseHTTP.Body.Close()
	if responseHTTP.StatusCode != 200 {
		return nil, handleErrorPayload(responseHTTP)
	}

	result := map[string]interface{}{}
	if err = json.NewDecoder(responseHTTP.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SendEvaluatedFile uploads a pre-evaluated list file.
func SendEvaluatedFile(file io.Reader, fileName string) (*FileValidationResponse, error) {
	if file == nil {
		return nil, fmt.Errorf("empty parameter: file")
	}
	requestURL, err := url.JoinPath(bulkV2Base(), "send-evaluated-file")
	if err != nil {
		return nil, err
	}
	requestURL += "/"
	return postBulkRootMultipart(requestURL, file, fileName, map[string]string{})
}

// ValidateS3FileOptions optional fields for validateS3file.
type ValidateS3FileOptions struct {
	FileName           string
	HasHeaderRow       *bool
	RemoveDuplicates   *bool
	RemoveDuplicate    *bool
	EmailAddressColumn *int
	ReturnURL          string
	FirstNameColumn    int
	LastNameColumn     int
	GenderColumn       int
	IpAddressColumn    int
	AllowPhase2        *bool
}

// ValidateS3File submits an S3 object for bulk validation.
func ValidateS3File(opts ValidateS3FileOptions) (*FileValidationResponse, error) {
	if strings.TrimSpace(opts.FileName) == "" {
		return nil, fmt.Errorf("empty parameter: file_name")
	}
	fields := url.Values{}
	fields.Set("file_name", opts.FileName)
	appendOptionalBoolField(fields, "has_header_row", opts.HasHeaderRow)
	appendOptionalBoolField(fields, "remove_duplicates", opts.RemoveDuplicates)
	appendOptionalBoolField(fields, "remove_duplicate", opts.RemoveDuplicate)
	if opts.EmailAddressColumn != nil {
		fields.Set("email_address_column", fmt.Sprintf("%d", *opts.EmailAddressColumn))
	}
	if opts.ReturnURL != "" {
		fields.Set("return_url", opts.ReturnURL)
	}
	if v, ok := optionalIntField(opts.FirstNameColumn); ok {
		fields.Set("first_name_column", v)
	}
	if v, ok := optionalIntField(opts.LastNameColumn); ok {
		fields.Set("last_name_column", v)
	}
	if v, ok := optionalIntField(opts.GenderColumn); ok {
		fields.Set("gender_column", v)
	}
	if v, ok := optionalIntField(opts.IpAddressColumn); ok {
		fields.Set("ip_address_column", v)
	}
	if opts.AllowPhase2 != nil {
		fields.Set("allow_phase_2", fmt.Sprintf("%t", *opts.AllowPhase2))
	}
	return postBulkRootForm("/validateS3file", fields)
}

// ScoreS3FileOptions optional fields for scoreS3file.
type ScoreS3FileOptions struct {
	FileName           string
	HasHeaderRow       *bool
	RemoveDuplicates   *bool
	RemoveDuplicate    *bool
	EmailAddressColumn *int
}

// ScoreS3File submits an S3 object for AI scoring.
func ScoreS3File(opts ScoreS3FileOptions) (*FileValidationResponse, error) {
	if strings.TrimSpace(opts.FileName) == "" {
		return nil, fmt.Errorf("empty parameter: file_name")
	}
	fields := url.Values{}
	fields.Set("file_name", opts.FileName)
	appendOptionalBoolField(fields, "has_header_row", opts.HasHeaderRow)
	appendOptionalBoolField(fields, "remove_duplicates", opts.RemoveDuplicates)
	appendOptionalBoolField(fields, "remove_duplicate", opts.RemoveDuplicate)
	if opts.EmailAddressColumn != nil {
		fields.Set("email_address_column", fmt.Sprintf("%d", *opts.EmailAddressColumn))
	}
	return postBulkRootForm("/scoreS3file", fields)
}

// ExportS3File exports a completed bulk job to an S3 bucket.
func ExportS3File(fileID, exportBucket string) (*FileValidationResponse, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("empty parameter: file_id")
	}
	if strings.TrimSpace(exportBucket) == "" {
		return nil, fmt.Errorf("empty parameter: export_bucket")
	}
	fields := url.Values{}
	fields.Set("file_id", fileID)
	fields.Set("export_bucket", exportBucket)
	return postBulkRootForm("/exportS3file", fields)
}

func appendOptionalBoolField(fields url.Values, key string, value *bool) {
	if value != nil {
		fields.Set(key, formatBoolField(*value))
	}
}
