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

	"github.com/joho/godotenv"
)

// CONSTANTS

const (
	URI                     = `https://api.zerobounce.net/v2/`
	BULK_URI				= `https://bulkapi.zerobounce.net/v2/`
	ENDPOINT_CREDITS        = "getcredits"
	ENDPOINT_VALIDATE       = "validate"
	ENDPOINT_API_USAGE      = "getapiusage"
	ENDPOINT_BATCH_VALIDATE = "validatebatch"
	ENDPOINT_FILE_SEND      = "sendfile"
	ENDPOINT_FILE_STATUS    = "filestatus"
	ENDPOINT_FILE_GET       = "getfile" // Content-type: application/octet-stream
	ENDPOINT_FILE_DELETE    = "deletefile"
	SANDBOX_IP              = "99.110.204.1"
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

// CsvFile - used for bulk validations that include csv files
type CsvFile struct {
	File				io.Reader	`json:"file"`
	FileName			string		`json:"file_name"`
	HasHeaderRow		bool		`json:"has_header_row"`
	
	// column index starts from 1
	// if either of the following will be 0, will be excluded from the request
	EmailAddressColumn 	int			`json:"email_address_column"`
	FirstNameColumn		int			`json:"first_name_column"`
	LastNameColumn		int			`json:"last_name_column"`
	GenderColumn		int			`json:"gender_column"`
	IpAddressColumn		int			`json:"ip_address_column"`
}

// ColumnsMapping - function generating how columns-index mapping of the instance
func (c *CsvFile)ColumnsMapping() map[string]int {
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

// API_KEY the API key used in order to make the requests
var API_KEY string = os.Getenv("ZERO_BOUNCE_API_KEY")

// FUNCTIONS

// SetApiKey set the API key explicitly
func SetApiKey(new_api_key_value string) {
	API_KEY = new_api_key_value
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
	return true
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


// ErrorFromResponse given a response who is expected to have a json structure,
// generate a joined response of all values within that json
// This function was done because error messages have inconsistent keys
// eg: error, message, Message etc
func ErrorFromResponse(response *http.Response) error {
	// ERROR handling: expect a json payload containing details about the error
	var error_response map[string]string

	response_body, error_ := io.ReadAll(response.Body)
	if error_ != nil {
		return errors.New("server error")
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
	sample_file_contents   = "valid@example.com\ninvalid@example.com\ntoxic@example.com\n"
	sample_error_message   = "error message"
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
