package zerobouncego

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jarcoal/httpmock"
)

// TESTING
type SingleTest struct {
	Email     string
	Status    string
	SubStatus string
	FreeEmail bool
}

var emailsToValidate = []SingleTest{
	{Email: "disposable@example.com", Status: "do_not_mail", SubStatus: "disposable"},
	{Email: "invalid@example.com", Status: "invalid", SubStatus: "mailbox_not_found"},
	{Email: "valid@example.com", Status: "valid", SubStatus: ""},
	{Email: "toxic@example.com", Status: "do_not_mail", SubStatus: "toxic"},
	{Email: "donotmail@example.com", Status: "do_not_mail", SubStatus: "role_based"},
	{Email: "spamtrap@example.com", Status: "spamtrap", SubStatus: ""},
	{Email: "abuse@example.com", Status: "abuse", SubStatus: ""},
	{Email: "catch_all@example.com", Status: "catch-all", SubStatus: ""},
	{Email: "antispam_system@example.com", Status: "unknown", SubStatus: "antispam_system"},
	{Email: "does_not_accept_mail@example.com", Status: "invalid", SubStatus: "does_not_accept_mail"},
	{Email: "exception_occurred@example.com", Status: "unknown", SubStatus: "exception_occurred"},
	{Email: "failed_smtp_connection@example.com", Status: "unknown", SubStatus: "failed_smtp_connection"},
	{Email: "failed_syntax_check@example.com", Status: "invalid", SubStatus: "failed_syntax_check"},
	{Email: "forcible_disconnect@example.com", Status: "unknown", SubStatus: "forcible_disconnect"},
	{Email: "global_suppression@example.com", Status: "do_not_mail", SubStatus: "global_suppression"},
	{Email: "greylisted@example.com", Status: "unknown", SubStatus: "greylisted"},
	{Email: "leading_period_removed@example.com", Status: "valid", SubStatus: "leading_period_removed"},
	{Email: "mail_server_did_not_respond@example.com", Status: "unknown", SubStatus: "mail_server_did_not_respond"},
	{Email: "mail_server_temporary_error@example.com", Status: "unknown", SubStatus: "mail_server_temporary_error"},
	{Email: "unknown@example.com", Status: "unknown", SubStatus: "mail_server_temporary_error"},
	{Email: "mailbox_quota_exceeded@example.com", Status: "invalid", SubStatus: "mailbox_quota_exceeded"},
	{Email: "mailbox_not_found@example.com", Status: "invalid", SubStatus: "mailbox_not_found"},
	{Email: "no_dns_entries@example.com", Status: "invalid", SubStatus: "no_dns_entries"},
	{Email: "possible_trap@example.com", Status: "do_not_mail", SubStatus: "possible_trap"},
	{Email: "possible_typo@example.com", Status: "invalid", SubStatus: "possible_typo"},
	{Email: "role_based@example.com", Status: "do_not_mail", SubStatus: "role_based"},
	{Email: "timeout_exceeded@example.com", Status: "unknown", SubStatus: "timeout_exceeded"},
	{Email: "unroutable_ip_address@example.com", Status: "invalid", SubStatus: "unroutable_ip_address"},
	{Email: "free_email@example.com", Status: "valid", SubStatus: "", FreeEmail: true},
	{Email: "gold@example.com", Status: "valid", SubStatus: "gold"},
	{Email: "role_based_accept_all@example.com", Status: "valid", SubStatus: "role_based_accept_all"},
	{Email: "role_based_catch_all@example.com", Status: "do_not_mail", SubStatus: "role_based_catch_all"},
}

// variables used for file-related unit tests
const (
	sample_date_time     = "2023-01-12T13:00:00Z"
	sample_file_contents = "valid@example.com\ninvalid@example.com\ntoxic@example.com\n"
	sample_error_message = "error message"
	sample_error_400     = `{
		"error": "` + sample_error_message + `"
	}`
	file_name_400          = "filename_400.csv"
	send_file_response_400 = `{
		"success": false,
		"error_message": "` + sample_error_message + `",
		"message": "` + sample_error_message + `"
	}`
	file_name_200          = "filename_200.csv"
	testing_file_id        = "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
	send_file_response_200 = `
	{
		"success": true,
		"message": "File Accepted",
		"file_name": "` + file_name_200 + `",
		"file_id": "` + testing_file_id + `"
	}`
)

// mockErrorResponse - mock http library to return error for given endpoint
func mockErrorResponse(method, endpoint string) {
	httpmock.RegisterResponder(
		method,
		`=~^(.*)`+endpoint+`(.*)\z`,
		func(r *http.Request) (*http.Response, error) { return nil, errors.New(sample_error_message) },
	)
}

// mockBadRequestResponse - mock http library to return 400 response for given endpoint
func mockBadRequestResponse(method, endpoint string) {
	httpmock.RegisterResponder(
		method,
		`=~^(.*)`+endpoint+`(.*)\z`,
		httpmock.NewStringResponder(400, sample_error_400),
	)
}

// mockBadRequestResponse - mock http library to return 200Ok response for given endpoint
// returning given endpoint
func mockOkResponse(method, endpoint, content string) {
	httpmock.RegisterResponder(
		method,
		`=~^(.*)`+endpoint+`(.*)\z`,
		httpmock.NewStringResponder(200, content),
	)
}

// testingCsvFileOk - sample csv file, used in testing
func testingCsvFileOk() CsvFile {
	return CsvFile{
		File:               strings.NewReader(sample_file_contents),
		FileName:           file_name_200,
		HasHeaderRow:       true,
		EmailAddressColumn: 1,
	}
}
