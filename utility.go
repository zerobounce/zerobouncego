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
	ENDPOINT_CREDITS        = "getcredits"
	ENDPOINT_VALIDATE       = "validate"
	ENDPOINT_API_USAGE      = "getapiusage"
	ENDPOINT_BATCH_VALIDATE = "validatebatch"
	SANDBOX_IP				= "99.110.204.1"
)


// validation statuses
const (
	S_VALID			= "valid"
	S_INVALID		= "invalid"
	S_CATCH_ALL		= "catch-all"
	S_UNKNOWN		= "unknown"
	S_SPAMTRAP		= "spamtrap"
	S_ABUSE			= "abuse"
	S_DO_NOT_MAIL	= "do_not_mail"
)

// validation sub statuses
const (
	SS_ANTISPAM_SYSTEM				= "antispam_system"
	SS_GREYLISTED					= "greylisted"
	SS_MAIL_SERVER_TEMPORARY_ERROR	= "mail_server_temporary_error"
	SS_FORCIBLE_DISCONNECT			= "forcible_disconnect"
	SS_MAIL_SERVER_DID_NOT_RESPOND	= "mail_server_did_not_respond"
	SS_TIMEOUT_EXCEEDED				= "timeout_exceeded"
	SS_FAILED_SMTP_CONNECTION		= "failed_smtp_connection"
	SS_MAILBOX_QUOTA_EXCEEDED		= "mailbox_quota_exceeded"
	SS_EXCEPTION_OCCURRED			= "exception_occurred"
	SS_POSSIBLE_TRAP				= "possible_trap"
	SS_ROLE_BASED					= "role_based"
	SS_GLOBAL_SUPPRESSION			= "global_suppression"
	SS_MAILBOX_NOT_FOUND			= "mailbox_not_found"
	SS_NO_DNS_ENTRIES				= "no_dns_entries"
	SS_FAILED_SYNTAX_CHECK			= "failed_syntax_check"
	SS_POSSIBLE_TYPO				= "possible_typo"
	SS_UNROUTABLE_IP_ADDRESS		= "unroutable_ip_address"
	SS_LEADING_PERIOD_REMOVED		= "leading_period_removed"
	SS_DOES_NOT_ACCEPT_MAIL			= "does_not_accept_mail"
	SS_ALIAS_ADDRESS				= "alias_address"
	SS_ROLE_BASED_CATCH_ALL			= "role_based_catch_all"
	SS_DISPOSABLE					= "disposable"
	SS_TOXIC						= "toxic"
)

// APIResponse basis for api responses
type APIResponse interface{}

// FUNCTIONS

var API_KEY string = os.Getenv("ZERO_BOUNCE_API_KEY")

func SetApiKey(new_api_key_value string) {
	API_KEY = new_api_key_value
}

func ImportApiKeyFromEnvFile() {
	error_ := godotenv.Load(".env") 
	if error_ != nil {
		fmt.Printf("The '.env' file was not found (%s). Continuing without it\n", error_.Error())
		return
	}
	SetApiKey(os.Getenv("ZERO_BOUNCE_API_KEY"))
}


// PrepareURL prepares the URL
func PrepareURL(endpoint string, params url.Values) string {

	// Set API KEY
	params.Set("api_key", API_KEY)

	// Create a return the final URL
	return fmt.Sprintf("%s/%s?%s", URI, endpoint, params.Encode())
}

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
	return errors.New("error: " + strings.Join(error_strings, ", "))}


// DoGetRequest does the request to the API
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

// add test for unknown@example.com also

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