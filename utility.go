package zerobouncego

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
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


type BatchEmailStatus string

const (
	S_VALID			BatchEmailStatus = "valid"
	S_INVALID		BatchEmailStatus = "invalid"
	S_CATCH_ALL		BatchEmailStatus = "catch-all"
	S_UNKNOWN		BatchEmailStatus = "unknown"
	S_SPAMTRAP		BatchEmailStatus = "spamtrap"
	S_ABUSE			BatchEmailStatus = "abuse"
	S_DO_NOT_MAIL	BatchEmailStatus = "do_not_mail"
)

type BatchEmailSubStatus string

const (
	SS_ANTISPAM_SYSTEM				BatchEmailSubStatus = "antispam_system"
	SS_GREYLISTED					BatchEmailSubStatus = "greylisted"
	SS_MAIL_SERVER_TEMPORARY_ERROR	BatchEmailSubStatus = "mail_server_temporary_error"
	SS_FORCIBLE_DISCONNECT			BatchEmailSubStatus = "forcible_disconnect"
	SS_MAIL_SERVER_DID_NOT_RESPOND	BatchEmailSubStatus = "mail_server_did_not_respond"
	SS_TIMEOUT_EXCEEDED				BatchEmailSubStatus = "timeout_exceeded"
	SS_FAILED_SMTP_CONNECTION		BatchEmailSubStatus = "failed_smtp_connection"
	SS_MAILBOX_QUOTA_EXCEEDED		BatchEmailSubStatus = "mailbox_quota_exceeded"
	SS_EXCEPTION_OCCURRED			BatchEmailSubStatus = "exception_occurred"
	SS_POSSIBLE_TRAP				BatchEmailSubStatus = "possible_trap"
	SS_ROLE_BASED					BatchEmailSubStatus = "role_based"
	SS_GLOBAL_SUPPRESSION			BatchEmailSubStatus = "global_suppression"
	SS_MAILBOX_NOT_FOUND			BatchEmailSubStatus = "mailbox_not_found"
	SS_NO_DNS_ENTRIES				BatchEmailSubStatus = "no_dns_entries"
	SS_FAILED_SYNTAX_CHECK			BatchEmailSubStatus = "failed_syntax_check"
	SS_POSSIBLE_TYPO				BatchEmailSubStatus = "possible_typo"
	SS_UNROUTABLE_IP_ADDRESS		BatchEmailSubStatus = "unroutable_ip_address"
	SS_LEADING_PERIOD_REMOVED		BatchEmailSubStatus = "leading_period_removed"
	SS_DOES_NOT_ACCEPT_MAIL			BatchEmailSubStatus = "does_not_accept_mail"
	SS_ALIAS_ADDRESS				BatchEmailSubStatus = "alias_address"
	SS_ROLE_BASED_CATCH_ALL			BatchEmailSubStatus = "role_based_catch_all"
	SS_DISPOSABLE					BatchEmailSubStatus = "disposable"
	SS_TOXIC						BatchEmailSubStatus = "toxic"
)

// APIResponse basis for api responses
type APIResponse interface{}

// FUNCTIONS

var API_KEY string = os.Getenv("ZERO_BOUNCE_API_KEY")

func SetApiKey(new_api_key_value string) {
	API_KEY = new_api_key_value
}


// PrepareURL prepares the URL
func PrepareURL(endpoint string, params url.Values) string {

	// Set API KEY
	params.Set("api_key", API_KEY)

	// Create a return the final URL
	return fmt.Sprintf("%s/%s?%s", URI, endpoint, params.Encode())
}

// DoGetRequest does the request to the API
func DoGetRequest(url string, object APIResponse) error {

	// Do the request
	response, err := http.Get(url)
	if err != nil {
		return err
	}

	// Check if server response is not HTTP 200
	if response.StatusCode != 200 {
		return errors.New("server error")
	}

	// Close the request
	defer response.Body.Close()

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