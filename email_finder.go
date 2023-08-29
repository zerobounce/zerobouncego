package zerobouncego

import (
	"fmt"
	"io"
	"net/url"
)

// DomainFormats part of the `FindEmailResponse` describing other domain formats
type DomainFormats struct {
	Format		string	`json:"format"`
	Confidence	string	`json:"confidence"`
}

// FindEmailResponse response structure for Find Email API
// `Confidence` field possible values: low, medium, high, unknown, undetermined
// (it is inconsistent as it can be either lowercase or uppercase)
type FindEmailResponse struct {
	Email				string			`json:"email"`
	Domain				string			`json:"domain"`
	Format				string			`json:"format"`
	Status				string			`json:"status"`
	SubStatus			string			`json:"sub_status"`
	Confidence			string			`json:"confidence"`
	DidYouMean			string			`json:"did_you_mean"`
	FailureReason		string			`json:"failure_reason"`
	OtherDomainFormats	[]DomainFormats	`json:"other_domain_formats"`
}

// FindEmail uses parameters to provide valid business email
func FindEmail(first_name, middle_name, last_name, domain string) (*FindEmailResponse, error) {
	var error_ error
	response := &FindEmailResponse{}

	request_parameters := url.Values{}
	request_parameters.Set("first_name", first_name)
	request_parameters.Set("middle_name", middle_name)
	request_parameters.Set("last_name", last_name)
	request_parameters.Set("domain", domain)
	url_to_request, error_ := PrepareURL(ENDPOINT_EMAIL_FINDER, request_parameters)
	if error_ != nil {
		return response, error_
	}

	error_ = DoGetRequest(url_to_request, response)
	return response, error_
}

// FindEmailSubmit - submit a file with emails for email finding
// Required columns: DomainColumn, FirstNameColumn/FullNameColumn (either)
// Optional columns: FirstNameColumn, MiddleName, LastNameColumn, FullNameColumn
func FindEmailFileSubmit(csv_file CsvFile, remove_duplicate bool) (*FileValidationResponse, error) {
	if csv_file.ColumnsNotSet() {
		// there are multiple required columns for this operation so we can't
		// fallback; if no column is set, return error
		var error_message = "bulk 'FindEmail' action requires `csv_file` parameter " +
			"to have values set for `DomainColumn` and either of `FirstNameColumn` " +
			"and `FullNameColumn`"
		return nil, fmt.Errorf(error_message)
	}
	return GenericFileSubmit(csv_file, remove_duplicate, ENDPOINT_EMAIL_FINDER_SEND)
}

// BulkValidationFileStatus - check the percentage of completion of a file uploaded
// for email finding
func FindEmailFileStatus(file_id string) (*FileStatusResponse, error) {
	return GenericFileStatusCheck(file_id, ENDPOINT_EMAIL_FINDER_STATUS)
}

// FindEmailResult - save a csv containing the results of the file previously
// submitted for email finding that corresponds to the given file ID parameter
func FindEmailResult(file_id string, file_writer io.Writer) error {
	return GenericResultFetch(file_id, ENDPOINT_EMAIL_FINDER_RESULT, file_writer)
}

// FindEmailFileDelete - delete the result file associated with a file ID
func FindEmailFileDelete(file_id string) (*FileValidationResponse, error) {
	return GenericFileDelete(file_id, ENDPOINT_EMAIL_FINDER_DELETE)
}

// DomainSearch - attempts to detect possible patterns a specific company uses
func DomainSearch(domain string) (*FindEmailResponse, error) {
	return FindEmail("", "", "", domain)
}

// DomainSearchSubmit - submit a file with emails for domain searching
// Required columns: DomainColumn
// Optional columns: FirstNameColumn, LastNameColumn, GenderColumn, IpAddressColumn
func DomainSearchFileSubmit(csv_file CsvFile, remove_duplicate bool) (*FileValidationResponse, error) {
	if csv_file.ColumnsNotSet() {
		// if no column is set, fallback the required column to index 1
		csv_file.DomainColumn = 1
	}
	return GenericFileSubmit(csv_file, remove_duplicate, ENDPOINT_DOMAIN_SEARCH_SEND)
}

// BulkValidationFileStatus - check the percentage of completion of a file uploaded
// for domain searching
func DomainSearchFileStatus(file_id string) (*FileStatusResponse, error) {
	return GenericFileStatusCheck(file_id, ENDPOINT_DOMAIN_SEARCH_STATUS)
}

// DomainSearchResult - save a csv containing the results of the file previously
// submitted for domain searching that corresponds to the given file ID parameter
func DomainSearchResult(file_id string, file_writer io.Writer) error {
	return GenericResultFetch(file_id, ENDPOINT_DOMAIN_SEARCH_RESULT, file_writer)
}

// DomainSearchFileDelete - delete the result file associated with a file ID of
// a domain search result
func DomainSearchFileDelete(file_id string) (*FileValidationResponse, error) {
	return GenericFileDelete(file_id, ENDPOINT_DOMAIN_SEARCH_DELETE)
}
