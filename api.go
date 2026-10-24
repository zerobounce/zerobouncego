package zerobouncego

import (
	"fmt"
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
	Address         string      `json:"address"`
	Status          string      `json:"status"`
	SubStatus       string      `json:"sub_status"`
	FreeEmail       bool        `json:"free_email"`
	DidYouMean      null.String `json:"did_you_mean"`
	Account         string      `json:"account"`
	Domain          string      `json:"domain"`
	DomainAgeDays   null.String `json:"domain_age_days"`
	ActiveInDays    null.String `json:"active_in_days"`
	ActiveFirstSeen null.String `json:"active_first_seen"`

	// DomainWebsiteExists and DomainRegistrantCompanyName come back with domain_info.
	DomainWebsiteExists         null.String `json:"domain_website_exists"`
	DomainRegistrantCompanyName null.String `json:"domain_registrant_company_name"`

	// Message is the API's "Message" key, returned instead of a result while a
	// greylist retry is still running.
	Message        string      `json:"Message"`
	SMTPProvider   null.String `json:"smtp_provider"`
	CatchallDomain null.Bool   `json:"catchall_domain"`
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
	return v.Status == ValidateStatusValid
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
	// Total number of times the API has a sub status of "accept_all"
	SubStatusAcceptAll int `json:"sub_status_accept_all"`
	// Total number times the API has a sub status "mx_forward"
	SubStatusMxForward int `json:"sub_status_mx_forward"`
	// Total number of times the API has a sub status of "alternate"
	SubStatusAlternate int `json:"sub_status_alternate"`
	// Total number of times the API has a sub status of "blocked"
	SubStatusBlocked int `json:"sub_status_blocked"`
	// Total number of times the API has a sub status of "allowed"
	SubStatusAllowed int `json:"sub_status_allowed"`
	// Total number of times the API has a sub status of "gold"
	SubStatusGold int `json:"sub_status_gold"`
	// Total number of times the API has a sub status of "role_based_accept_all"
	SubStatusRoleBasedAcceptAll int `json:"sub_status_role_based_accept_all"`
	// Total emails with a leading period that was removed
	SubStatusLeadingPeriodRemoved int `json:"sub_status_leading_period_removed"`
	// Total number of times the API has a sub status of "alias_address"
	SubStatusAliasAddress int `json:"sub_status_alias_address"`
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

// ValidateOptions optional query parameters for GET /validate.
type ValidateOptions struct {
	IPAddress    string
	Timeout      string
	ActivityData *bool
	VerifyPlus   *bool
	// DomainInfo fills DomainWebsiteExists and DomainRegistrantCompanyName.
	DomainInfo *bool
	// GreylistProcessing retries a greylisted address in the background.
	GreylistProcessing *bool
	// CallbackURL is where the greylist retry posts its result. It needs
	// GreylistProcessing set to true and is rejected without it, and when
	// blank, before any request is sent.
	CallbackURL string
	UsePost     bool
}

// Validate validates a single email
func Validate(email string, IPAddress string) (*ValidateResponse, error) {
	return ValidateWithOptions(email, ValidateOptions{IPAddress: IPAddress})
}

func ValidateWithTimeout(email string, IPAddress string, timeout string) (*ValidateResponse, error) {
	return ValidateWithOptions(email, ValidateOptions{IPAddress: IPAddress, Timeout: timeout})
}

func ValidateWithOptions(email string, opts ValidateOptions) (*ValidateResponse, error) {
	return validateWithOptions(packageConfig(), email, opts)
}

func validateWithOptions(cfg RequestConfig, email string, opts ValidateOptions) (*ValidateResponse, error) {
	if opts.Timeout != "" {
		timeout, err := strconv.Atoi(opts.Timeout)
		if err != nil || timeout < 3 || timeout > 60 {
			return nil, fmt.Errorf("Invalid parameter: timeout (must be between 3 and 60 seconds)")
		}
	}
	if opts.CallbackURL != "" {
		if strings.TrimSpace(opts.CallbackURL) == "" {
			return nil, fmt.Errorf("empty parameter: callback_url")
		}
		// The callback is where the greylist retry posts its result, so it is
		// meaningless - and rejected by the API - without the retry itself.
		if opts.GreylistProcessing == nil || !*opts.GreylistProcessing {
			return nil, fmt.Errorf("callback_url requires greylist_processing to be true")
		}
	}

	params := url.Values{}
	params.Set("email", email)
	params.Set("ip_address", opts.IPAddress)
	if opts.Timeout != "" {
		params.Set("timeout", opts.Timeout)
	}
	if opts.ActivityData != nil {
		params.Set("activity_data", fmt.Sprintf("%t", *opts.ActivityData))
	}
	if opts.VerifyPlus != nil {
		params.Set("verify_plus", fmt.Sprintf("%t", *opts.VerifyPlus))
	}
	if opts.DomainInfo != nil {
		params.Set("domain_info", fmt.Sprintf("%t", *opts.DomainInfo))
	}
	if opts.GreylistProcessing != nil {
		params.Set("greylist_processing", fmt.Sprintf("%t", *opts.GreylistProcessing))
	}
	if opts.CallbackURL != "" {
		params.Set("callback_url", opts.CallbackURL)
	}

	response := &ValidateResponse{}
	if opts.UsePost {
		form := url.Values{}
		for key, values := range params {
			if len(values) > 0 {
				form.Set(key, values[0])
			}
		}
		err := doPostFormRequest(cfg, ENDPOINT_VALIDATE, form, response)
		return response, err
	}

	urlToRequest, err := prepareURL(cfg, ENDPOINT_VALIDATE, params)
	if err != nil {
		return response, err
	}
	err = DoGetRequest(urlToRequest, response)
	return response, err
}

// GetCredits gets credits balance
func GetCredits() (*CreditsResponse, error) {
	return getCredits(packageConfig())
}

func getCredits(cfg RequestConfig) (*CreditsResponse, error) {
	response := &CreditsResponse{}
	urlToRequest, err := prepareURL(cfg, ENDPOINT_CREDITS, url.Values{})
	if err != nil {
		return response, err
	}
	err = DoGetRequest(urlToRequest, response)
	return response, err
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
