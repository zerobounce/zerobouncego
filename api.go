package zerobouncego

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/guregu/null.v4"
)

// CreditsResponse response of the credits balance
type CreditsResponse struct {
	CreditsRaw string `json:"Credits"`
}

func (c *CreditsResponse) Credits() int {
	amount, error_ := strconv.Atoi(c.CreditsRaw)
	if error_ != nil {
		return -1
	}
	return amount
}

// ValidateResponse response structure for single email validation
type ValidateResponse struct {
	Address        string      `json:"address"`
	Status         string      `json:"status"`
	SubStatus      string      `json:"sub_status"`
	FreeEmail      bool        `json:"free_email"`
	DidYouMean     null.String `json:"did_you_mean"`
	Account        string      `json:"account"`
	Domain         string      `json:"domain"`
	DomainAgeDays  null.String `json:"domain_age_days"`
	SMTPProvider   null.String `json:"smtp_provider"`
	MxRecord       string      `json:"mx_record"`
	MxFound        string      `json:"mx_found"`
	Firstname      null.String `json:"firstname"`
	Lastname       null.String `json:"lastname"`
	Gender         null.String `json:"gender"`
	Country        null.String `json:"country"`
	Region         null.String `json:"region"`
	City           null.String `json:"city"`
	Zipcode        null.String `json:"zipcode"`
	RawProcessedAt string      `json:"processed_at"`
}

func (v ValidateResponse) ProcessedAt() (time.Time, error) {
	return time.Parse(DATE_TIME_FORMAT, strings.Trim(v.RawProcessedAt, `"`))
}

// IsValid checks if an email is valid
func (v *ValidateResponse) IsValid() bool {
	return v.Status == "valid"
}

// ApiUsageResponse response structure for the API usage functionality
type ApiUsageResponse struct {
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
	// Start date of query.
	RawStartDate string `json:"start_date"`
	// End date of query.
	RawEndDate string `json:"end_date"`
}

// StartDate provide the parsed start date of an API usage response
func (v ApiUsageResponse) StartDate() (time.Time, error) {
	return time.Parse("2/1/2006", strings.Trim(v.RawStartDate, `"`))
}

// StartDate provide the parsed end date of an API usage response
func (v ApiUsageResponse) EndDate() (time.Time, error) {
	return time.Parse("2/1/2006", strings.Trim(v.RawEndDate, `"`))
}

// Validate validates a single email
func Validate(email string, IPAddress string) (*ValidateResponse, error) {

	// Prepare the parameters
	params := url.Values{}
	params.Set("email", email)
	params.Set("ip_address", IPAddress)

	response := &ValidateResponse{}

	// Do the request
	url_to_request, error_ := PrepareURL(ENDPOINT_VALIDATE, params)
	if error_ != nil {
		return response, error_
	}
	error_ = DoGetRequest(url_to_request, response)
	return response, error_
}

// GetCredits gets credits balance
func GetCredits() (*CreditsResponse, error) {
	var error_ error
	response := &CreditsResponse{}

	url_to_request, error_ := PrepareURL(ENDPOINT_CREDITS, url.Values{})
	if error_ != nil {
		return response, error_
	}
	error_ = DoGetRequest(url_to_request, response)
	return response, error_
}

// GetApiUsage the usage of the API within a date interval
func GetApiUsage(start_date, end_date time.Time) (*ApiUsageResponse, error) {
	var error_ error
	response := &ApiUsageResponse{}
	request_parameters := url.Values{}
	request_parameters.Set("start_date", start_date.Format(DATE_ONLY_FORMAT))
	request_parameters.Set("end_date", end_date.Format(DATE_ONLY_FORMAT))
	url_to_request, error_ := PrepareURL(ENDPOINT_API_USAGE, request_parameters)
	if error_ != nil {
		return response, error_
	}
	error_ = DoGetRequest(url_to_request, response)
	return response, error_
}
