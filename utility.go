package zerobouncego

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jarcoal/httpmock"
	"github.com/joho/godotenv"
)

// CONSTANTS
const (
	ENDPOINT_CREDITS        = "/getcredits"
	ENDPOINT_ACTIVITY_DATA  = "/activity"
	ENDPOINT_VALIDATE       = "/validate"
	ENDPOINT_API_USAGE      = "/getapiusage"
	ENDPOINT_BATCH_VALIDATE = "/validatebatch"
	ENDPOINT_FILE_SEND      = "/sendfile"
	ENDPOINT_FILE_STATUS    = "/filestatus"
	ENDPOINT_FILE_RESULT    = "/getfile" // Content-type: application/octet-stream
	ENDPOINT_FILE_DELETE    = "/deletefile"
	ENDPOINT_SCORING_SEND   = "/scoring/sendfile"
	ENDPOINT_SCORING_STATUS = "/scoring/filestatus"
	ENDPOINT_SCORING_RESULT = "/scoring/getfile" // Content-type: application/octet-stream
	ENDPOINT_SCORING_DELETE = "/scoring/deletefile"
	ENDPOINT_EMAIL_FINDER   = "/guessformat"
	SANDBOX_IP              = "99.110.204.1"
)

const (
	DATE_TIME_FORMAT = "2006-01-02 15:04:05"
	DATE_ONLY_FORMAT = "2006-01-02"
)

// validation statuses
const (
	S_VALID       = "valid"
	S_INVALID     = "invalid"
	S_CATCH_ALL   = "catch-all"
	S_UNKNOWN     = "unknown"
	S_SPAMTRAP    = "spamtrap"
	S_ABUSE       = "abuse"
	S_DO_NOT_MAIL = "do_not_mail"
)

// validation sub statuses
const (
	SS_ANTISPAM_SYSTEM             = "antispam_system"
	SS_GREYLISTED                  = "greylisted"
	SS_MAIL_SERVER_TEMPORARY_ERROR = "mail_server_temporary_error"
	SS_FORCIBLE_DISCONNECT         = "forcible_disconnect"
	SS_MAIL_SERVER_DID_NOT_RESPOND = "mail_server_did_not_respond"
	SS_TIMEOUT_EXCEEDED            = "timeout_exceeded"
	SS_FAILED_SMTP_CONNECTION      = "failed_smtp_connection"
	SS_MAILBOX_QUOTA_EXCEEDED      = "mailbox_quota_exceeded"
	SS_EXCEPTION_OCCURRED          = "exception_occurred"
	SS_POSSIBLE_TRAP               = "possible_trap"
	SS_ROLE_BASED                  = "role_based"
	SS_GLOBAL_SUPPRESSION          = "global_suppression"
	SS_MAILBOX_NOT_FOUND           = "mailbox_not_found"
	SS_NO_DNS_ENTRIES              = "no_dns_entries"
	SS_FAILED_SYNTAX_CHECK         = "failed_syntax_check"
	SS_POSSIBLE_TYPO               = "possible_typo"
	SS_UNROUTABLE_IP_ADDRESS       = "unroutable_ip_address"
	SS_LEADING_PERIOD_REMOVED      = "leading_period_removed"
	SS_DOES_NOT_ACCEPT_MAIL        = "does_not_accept_mail"
	SS_ALIAS_ADDRESS               = "alias_address"
	SS_ROLE_BASED_CATCH_ALL        = "role_based_catch_all"
	SS_DISPOSABLE                  = "disposable"
	SS_TOXIC                       = "toxic"
)

// APIResponse basis for api responses
type APIResponse interface{}

func Getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}

// VAR
var (
	// URI used to make requests to the ZeroBounce API
	URI      = Getenv(`ZERO_BOUNCE_URI`, `https://api.zerobounce.net/v2/`)
	BULK_URI = Getenv(`ZERO_BOUNCE_BULK_URI`, `https://bulkapi.zerobounce.net/v2/`)

	// API_KEY the API key used in order to make the requests
	API_KEY string = os.Getenv("ZERO_BOUNCE_API_KEY")
)

// FUNCTIONS

// SetApiKey set the API key explicitly
func SetApiKey(new_api_key_value string) {
	API_KEY = new_api_key_value
}

// Update URI, BULK_URI or both (will not be updated if empty strings are passed)
func SetURI(new_uri string, new_bulk_uri string) {
	if new_uri != "" {
		URI = new_uri
	}
	if new_bulk_uri != "" {
		BULK_URI = new_bulk_uri
	}
}

// ImportApiKeyFromEnvFile provided that a .env file can be found where the
// program is running, load it, extract the API key and set it
func ImportApiKeyFromEnvFile() bool {
	error_ := godotenv.Load(".env")
	if error_ != nil {
		fmt.Printf("The '.env' file was not found (%s). Continuing without it\n", error_.Error())
		return false
	}
	SetApiKey(os.Getenv("ZERO_BOUNCE_API_KEY"))
	SetURI(os.Getenv("ZERO_BOUNCE_URI"), os.Getenv("ZERO_BOUNCE_BULK_URI"))
	return true
}

// PrepareURL prepares the URL for a get request by attaching both the API
// key and the given params
func PrepareURL(endpoint string, params url.Values) (string, error) {

	// Set API KEY
	params.Set("api_key", API_KEY)

	// Create a return the final URL
	final_url, error_ := url.JoinPath(URI, endpoint)
	if error_ != nil {
		return "", error_
	}
	return fmt.Sprintf("%s?%s", final_url, params.Encode()), nil
}

// handleErrorPayload - generate error based on an error payload with expected
// response payload: {"success": false, "message": ...}
func handleErrorPayload(response *http.Response) error {
	var error_ error
	var response_payload map[string]interface{} // expected keys: success, message
	defer response.Body.Close()

	error_ = json.NewDecoder(response.Body).Decode(&response_payload)
	if error_ != nil {
		return fmt.Errorf(
			"error occurred while parsing a status %d response payload: %s",
			response.StatusCode,
			error_.Error(),
		)
	}

	return fmt.Errorf("error message: %s", response_payload["message"])
}

// ErrorFromResponse given a response who is expected to have a json structure,
// generate a joined response of all values within that json
// This function was done because error messages have inconsistent keys
// eg: error, message, Message etc
func ErrorFromResponse(response *http.Response) error {
	// ERROR handling: expect a json payload containing details about the error
	var error_response map[string]string

	response_body, error_ := io.ReadAll(response.Body)
	if error_ != nil {
		return errors.New("server error: " + error_.Error())
	}
	error_ = json.NewDecoder(strings.NewReader(string(response_body))).Decode(&error_response)
	if error_ != nil {
		// unexpected non-json payload
		return errors.New(string(response_body))
	}

	// return all possible details about the error
	var error_strings []string
	for _, value := range error_response {
		error_strings = append(error_strings, value)
	}
	return errors.New("error: " + strings.Join(error_strings, ", "))
}

// DoGetRequest does a GET request to the API
func DoGetRequest(url string, object APIResponse) error {

	// Do the request
	response, err := http.Get(url)
	if err != nil {
		return err
	}

	// Close the request
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return ErrorFromResponse(response)
	}

	// Decode JSON Request
	err = json.NewDecoder(response.Body).Decode(&object)
	return err
}

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
