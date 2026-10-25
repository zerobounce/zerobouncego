package zerobouncego

import (
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
		"domain_column":  fmt.Sprintf("%d", f.DomainColumn),
		"has_header_row": formatBoolField(f.HasHeaderRow),
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
//
// domain_column is required, and so is one of first_name_column or
// full_name_column: the finder has no name to work from without one.
func BulkEmailFinderSubmit(file EmailFinderCsvFile) (*FileValidationResponse, error) {
	if file.DomainColumn == 0 {
		return nil, fmt.Errorf("empty parameter: domain_column")
	}
	if file.FirstNameColumn == 0 && file.FullNameColumn == 0 {
		return nil, fmt.Errorf("empty parameter: first_name_column or full_name_column (one of them is required)")
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
		"domain_column":  fmt.Sprintf("%d", file.DomainColumn),
		"has_header_row": formatBoolField(file.HasHeaderRow),
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
	File               io.Reader
	FileName           string
	EmailAddressColumn int
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
	requestURL, err := url.JoinPath(LIST_EVALUATOR_URI, "")
	if err != nil {
		return nil, err
	}
	requestURL += "/"
	return postBulkRootMultipart(requestURL, file.File, file.FileName, fields)
}

// GetListEvaluatorResult fetches list evaluator job status/result metadata.
func GetListEvaluatorResult(fileID string) (map[string]interface{}, error) {
	if err := packageConfig().requireKey(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("empty parameter: file_id")
	}
	params := url.Values{}
	params.Set("api_key", API_KEY)

	// The list evaluator has its own base URL, not a path built from BULK_URI,
	// and the file id is one path segment, so a slash in it must stay encoded.
	requestURL, err := url.JoinPath(LIST_EVALUATOR_URI, url.PathEscape(strings.TrimSpace(fileID)))
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
	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{}
	if err = decodeJSONBody(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SendEvaluatedFile submits a finished evaluation for validation.
//
// LIVE: this takes the list evaluator's file_id as a form field, not a file
// upload as the docs describe — an upload is rejected with
// "file_id: This field is required". It returns a new validation job's file_id,
// which answers "File cannot be found" for a short while after it is accepted.
func SendEvaluatedFile(fileID string) (*FileValidationResponse, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("empty parameter: file_id")
	}
	requestURL, err := url.JoinPath(bulkV2Base(), "send-evaluated-file")
	if err != nil {
		return nil, err
	}
	requestURL += "/"

	fields := url.Values{}
	fields.Set("file_id", fileID)
	return postBulkRootFormAt(requestURL, fields)
}

// ValidateS3FileOptions fields for validateS3file.
//
// FileName and ReturnURL are required: the live API rejects a job without
// either. The remaining fields are omitted until they are set.
type ValidateS3FileOptions struct {
	FileName         string
	HasHeaderRow     *bool
	RemoveDuplicates *bool
	// RemoveDuplicate is the misspelled twin of RemoveDuplicates. Only
	// remove_duplicates is documented, and it is now the only one sent.
	//
	// Deprecated: set RemoveDuplicates instead. This field is used only when
	// RemoveDuplicates is nil, and it will be removed in the next major version.
	RemoveDuplicate    *bool
	EmailAddressColumn *int
	ReturnURL          string
	FirstNameColumn    int
	LastNameColumn     int
	GenderColumn       int
	IpAddressColumn    int
	AllowPhase2        *bool
}

// removeDuplicates folds the deprecated RemoveDuplicate into RemoveDuplicates.
func (opts ValidateS3FileOptions) removeDuplicates() *bool {
	if opts.RemoveDuplicates != nil {
		return opts.RemoveDuplicates
	}
	return opts.RemoveDuplicate
}

// ValidateS3File submits an S3 object for bulk validation
// (POST /validateS3file). FileName and ReturnURL are required.
func ValidateS3File(opts ValidateS3FileOptions) (*FileValidationResponse, error) {
	fields, err := bulkRootJobFields(opts.FileName, opts.HasHeaderRow, opts.removeDuplicates(), opts.ReturnURL)
	if err != nil {
		return nil, err
	}
	if opts.EmailAddressColumn != nil {
		fields.Set("email_address_column", fmt.Sprintf("%d", *opts.EmailAddressColumn))
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

// ScoreS3FileOptions fields for scoreS3file.
//
// FileName and ReturnURL are required: the live API rejects a job without
// either. The remaining fields are omitted until they are set.
type ScoreS3FileOptions struct {
	FileName         string
	HasHeaderRow     *bool
	RemoveDuplicates *bool
	// RemoveDuplicate is the misspelled twin of RemoveDuplicates. Only
	// remove_duplicates is documented, and it is now the only one sent.
	//
	// Deprecated: set RemoveDuplicates instead. This field is used only when
	// RemoveDuplicates is nil, and it will be removed in the next major version.
	RemoveDuplicate    *bool
	EmailAddressColumn *int
	ReturnURL          string
}

// removeDuplicates folds the deprecated RemoveDuplicate into RemoveDuplicates.
func (opts ScoreS3FileOptions) removeDuplicates() *bool {
	if opts.RemoveDuplicates != nil {
		return opts.RemoveDuplicates
	}
	return opts.RemoveDuplicate
}

// ScoreS3File submits an S3 object for AI scoring (POST /scoreS3file).
// FileName and ReturnURL are required.
func ScoreS3File(opts ScoreS3FileOptions) (*FileValidationResponse, error) {
	fields, err := bulkRootJobFields(opts.FileName, opts.HasHeaderRow, opts.removeDuplicates(), opts.ReturnURL)
	if err != nil {
		return nil, err
	}
	if opts.EmailAddressColumn != nil {
		fields.Set("email_address_column", fmt.Sprintf("%d", *opts.EmailAddressColumn))
	}
	return postBulkRootForm("/scoreS3file", fields)
}

// S3ListFileEntry is one file returned by GET /listS3files.
type S3ListFileEntry struct {
	Name         string `json:"name"`
	Size         int    `json:"size"`
	LastModified string `json:"last_modified"`
}

// ListS3FilesOptions optional query parameters for GET /listS3files.
type ListS3FilesOptions struct {
	Bucket string
	Search string
}

// ListS3Files lists S3 objects available for validation or scoring.
func ListS3Files(opts ListS3FilesOptions) ([]S3ListFileEntry, error) {
	if err := packageConfig().requireKey(); err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("api_key", API_KEY)
	if strings.TrimSpace(opts.Bucket) != "" {
		params.Set("bucket", opts.Bucket)
	}
	if strings.TrimSpace(opts.Search) != "" {
		params.Set("search", opts.Search)
	}
	requestURL, err := url.JoinPath(bulkAPIRoot(), "listS3files")
	if err != nil {
		return nil, err
	}
	fullURL := requestURL + "?" + params.Encode()
	// httpGet, not the bare client: a transport failure surfaces as a
	// *url.Error carrying the full URL, and the api_key is in that query.
	responseHTTP, err := httpGet(fullURL)
	if err != nil {
		return nil, err
	}
	defer responseHTTP.Body.Close()
	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return nil, err
	}
	var entries []S3ListFileEntry
	if err = decodeJSONBody(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), body, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ExportS3File exports a completed bulk job to an S3 bucket
// (POST /exportS3file).
//
// export_bucket is optional, like ExportSFTPFile's export_path: pass "" to use
// the export bucket configured in your ZeroBounce settings.
func ExportS3File(fileID, exportBucket string) (*FileValidationResponse, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("empty parameter: file_id")
	}
	fields := url.Values{}
	fields.Set("file_id", fileID)
	if strings.TrimSpace(exportBucket) != "" {
		fields.Set("export_bucket", exportBucket)
	}
	return postBulkRootForm("/exportS3file", fields)
}

func appendOptionalBoolField(fields url.Values, key string, value *bool) {
	if value != nil {
		fields.Set(key, formatBoolField(*value))
	}
}
