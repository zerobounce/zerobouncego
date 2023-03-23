package zerobouncego

import (
	"net/url"
	"time"
)

// URI URL for the requests

// APIResponse basis for api responses
type APIResponse interface{}

// CreditsResponse response of the credits balance
type CreditsResponse struct {
	Credits string `json:"Credits"`
}

// ValidateResponse Response from API
type ValidateResponse struct {
	Address       string      `json:"address"`
	Status        string      `json:"status"`
	SubStatus     string      `json:"sub_status"`
	FreeEmail     bool        `json:"free_email"`
	DidYouMean    interface{} `json:"did_you_mean"`
	Account       string      `json:"account"`
	Domain        string      `json:"domain"`
	DomainAgeDays string      `json:"domain_age_days"`
	SMTPProvider  string      `json:"smtp_provider"`
	MxRecord      string      `json:"mx_record"`
	MxFound       string      `json:"mx_found"`
	Firstname     string      `json:"firstname"`
	Lastname      string      `json:"lastname"`
	Gender        string      `json:"gender"`
	Country       string      `json:"country"`
	Region        string      `json:"region"`
	City          string      `json:"city"`
	Zipcode       string      `json:"zipcode"`
	ProcessedAt   string      `json:"processed_at"`
}

type GetApiUsageResponse struct {
	// Total number of times the API has been called
	Total int `json:"total"`
	// Total valid email addresses returned by the API
	StatusValid int `json:"status_valid"`
	// Total invalid email addresses returned by the API
	StatusInvalid int `json:"status_invalid"`
	// Total catch-all email addresses returned by the API
	StatusCatchAll int `json:"status_catch_all"`
	// Total do not mail email addresses returned by the API
	StatusDoNotMail int `json:"status_do_not_mail"`
	// Total spamtrap email addresses returned by the API
	StatusSpamtrap int `json:"status_spamtrap"`
	// Total unknown email addresses returned by the API
	StatusUnknown int `json:"status_unknown"`
	// Total number of times the API has a sub status of "toxic"
	SubStatusToxic int `json:"sub_status_toxic"`
	// Total number of times the API has a sub status of "disposable"
	SubStatusDisposable int `json:"sub_status_disposable"`
	// Total number of times the API has a sub status of "role_based"
	SubStatusRoleBased int `json:"sub_status_role_based"`
	// Total number of times the API has a sub status of "possible_trap"
	SubStatusPossibleTrap int `json:"sub_status_possible_trap"`
	// Total number of times the API has a sub status of "global_suppression"
	SubStatusGlobalSuppression int `json:"sub_status_global_suppression"`
	// Total number of times the API has a sub status of "timeout_exceeded"
	SubStatusTimeoutExceeded int `json:"sub_status_timeout_exceeded"`
	// Total number of times the API has a sub status of "mail_server_temporary_error"
	SubStatusMailServerTemporaryError int `json:"sub_status_mail_server_temporary_error"`
	// Total number of times the API has a sub status of "mail_server_did_not_respond"
	SubStatusMailServerDidNotRespond int `json:"sub_status_mail_server_did_not_respond"`
	// Total number of times the API has a sub status of "greylisted"
	SubStatusGreylisted int `json:"sub_status_greylisted"`
	// Total number of times the API has a sub status of "antispam_system"
	SubStatusAntispamSystem int `json:"sub_status_antispam_system"`
	// Total number of times the API has a sub status of "does_not_accept_mail"
	SubStatusDoesNotAcceptMail int `json:"sub_status_does_not_accept_mail"`
	// Total number of times the API has a sub status of "exception_occurred"
	SubStatusExceptionOccurred int `json:"sub_status_exception_occurred"`
	// Total number of times the API has a sub status of "failed_syntax_check"
	SubStatusFailedSyntaxCheck int `json:"sub_status_failed_syntax_check"`
	// Total number of times the API has a sub status of "mailbox_not_found"
	SubStatusMailboxNotFound int `json:"sub_status_mailbox_not_found"`
	// Total number of times the API has a sub status of "unroutable_ip_address"
	SubStatusUnroutableIpAddress int `json:"sub_status_unroutable_ip_address"`
	// Total number of times the API has a sub status of "possible_typo"
	SubStatusPossibleTypo int `json:"sub_status_possible_typo"`
	// Total number of times the API has a sub status of "no_dns_entries"
	SubStatusNoDnsEntries int `json:"sub_status_no_dns_entries"`
	// Total role based catch alls the API has a sub status of "role_based_catch_all"
	SubStatusRoleBasedCatchAll int `json:"sub_status_role_based_catch_all"`
	// Total number of times the API has a sub status of "mailbox_quota_exceeded"
	SubStatusMailboxQuotaExceeded int `json:"sub_status_mailbox_quota_exceeded"`
	// Total forcible disconnects the API has a sub status of "forcible_disconnect"
	SubStatusForcibleDisconnect int `json:"sub_status_forcible_disconnect"`
	// Total failed SMTP connections the API has a sub status of "failed_smtp_connection"
	SubStatusFailedSmtpConnection int `json:"sub_status_failed_smtp_connection"`
	// Total number times the API has a sub status "mx_forward"
	SubStatusMxForward int `json:"sub_status_mx_forward"`
	// Start date of query. (format: yyyy/mm/dd)
	StartDate string `json:"start_date"`
	// End date of query. (format: yyyy/mm/dd)
	EndDate string `json:"end_date"`
}


// Validate validates a single email
func Validate(email string, IPAddress string) (*ValidateResponse, error) {

	// Prepare the parameters
	params := url.Values{}
	params.Set("email", email)
	params.Set("ip_address", IPAddress)

	response := &ValidateResponse{}

	// Do the request
	err := DoRequest(PrepareURL("validate", params), response)
	return response, err
}

// GetCredits gets credits balance
func GetCredits() (*CreditsResponse, error) {

	response := &CreditsResponse{}
	err := DoRequest(PrepareURL("getcredits", url.Values{}), response)
	return response, err
}

// GetApiUsage the usage of the API within a date interval
func GetApiUsage(start_date, end_date time.Time) (*GetApiUsageResponse, error) {
	response := &GetApiUsageResponse{}
	request_parameters := url.Values{}
	request_parameters.Set("start_date", start_date.Format(time.DateOnly))
	request_parameters.Set("end_date", end_date.Format(time.DateOnly))
	err := DoRequest(PrepareURL("getapiusage", request_parameters), response)
	return response, err
}