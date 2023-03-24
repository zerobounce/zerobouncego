package zerobouncego

import (
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

// MOCK_API_USAGE contains a response with all fields expected, such that
// resulted parsed object will not fallback to 0
const MOCK_API_USAGE = `{
    "total": 10,
    "status_valid": 10,
    "status_invalid": 10,
    "status_catch_all": 10,
    "status_do_not_mail": 10,
    "status_spamtrap": 10,
    "status_unknown": 10,
    "sub_status_toxic": 10,
    "sub_status_disposable": 10,
    "sub_status_role_based": 10,
    "sub_status_possible_trap": 10,
    "sub_status_global_suppression": 10,
    "sub_status_timeout_exceeded": 10,
    "sub_status_mail_server_temporary_error": 10,
    "sub_status_mail_server_did_not_respond": 10,
    "sub_status_greylisted": 10,
    "sub_status_antispam_system": 10,
    "sub_status_does_not_accept_mail": 10,
    "sub_status_exception_occurred": 10,
    "sub_status_failed_syntax_check": 10,
    "sub_status_mailbox_not_found": 10,
    "sub_status_unroutable_ip_address": 10,
    "sub_status_possible_typo": 10,
    "sub_status_no_dns_entries": 10,
    "sub_status_role_based_catch_all": 10,
    "sub_status_mailbox_quota_exceeded": 10,
    "sub_status_forcible_disconnect": 10,
    "sub_status_failed_smtp_connection": 10,
    "sub_status_mx_forward": 10,
    "sub_status_alternate": 10,
    "sub_status_blocked": 10,
    "sub_status_allowed": 10,
    "start_date": "1/1/2023",
    "end_date": "12/12/2023"
}`

func mockCreditsRequest() {
	httpmock.RegisterResponder("GET", `=~^(.*)`+ENDPOINT_CREDITS+`(.*)\z`,
		func(r *http.Request) (*http.Response, error) {
			args := r.URL.Query()
			if args.Get("api_key") == "" {
				return httpmock.NewStringResponse(400, `{"error": "Missing parameter: api_key."}`), nil
			}
			return httpmock.NewStringResponse(200, `{"Credits": 50}`), nil
		},
	)
}

func mockApiUsageRequest() {
	httpmock.RegisterResponder("GET", `=~^(.*)`+ENDPOINT_API_USAGE+`(.*)\z`,
		func(r *http.Request) (*http.Response, error) {
			args := r.URL.Query()
			if args.Get("api_key") == "" {
				return httpmock.NewStringResponse(400, `{"error": "Missing parameter: api_key."}`), nil
			}
			return httpmock.NewStringResponse(200, MOCK_API_USAGE), nil
		},
	)
}

func TestCreditsNoKey(t *testing.T) {
	SetApiKey("")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockCreditsRequest()

	_, error_ := GetCredits()
	if !assert.NotNil(t, error_) { t.FailNow() }
	assert.Contains(t, error_.Error(), "api_key")
}

func TestCreditsOk(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockCreditsRequest()

	credits_result, error_ := GetCredits()
	if !assert.Nil(t, error_) { t.FailNow() }
	assert.Equal(t, credits_result.Credits, 50)
}

func TestApiUsageNoKey(t *testing.T) {
	SetApiKey("")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockApiUsageRequest()

	_, error_ := GetApiUsage(time.Now(), time.Now())
	if !assert.NotNil(t, error_) { t.FailNow() }
	assert.Contains(t, error_.Error(), "api_key")
}

func TestApiUsageOk(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockApiUsageRequest()

	api_usage, error_ := GetApiUsage(time.Now(), time.Now())
	if !assert.Nil(t, error_) { t.FailNow() }

	// assert that all following fields do not fallback to zero
	assert.NotEqual(t, api_usage.Total, 0)
	assert.NotEqual(t, api_usage.StatusValid, 0)
	assert.NotEqual(t, api_usage.StatusInvalid, 0)
	assert.NotEqual(t, api_usage.StatusCatchAll, 0)
	assert.NotEqual(t, api_usage.StatusDoNotMail, 0)
	assert.NotEqual(t, api_usage.StatusSpamtrap, 0)
	assert.NotEqual(t, api_usage.StatusUnknown, 0)
	assert.NotEqual(t, api_usage.SubStatusToxic, 0)
	assert.NotEqual(t, api_usage.SubStatusDisposable, 0)
	assert.NotEqual(t, api_usage.SubStatusRoleBased, 0)
	assert.NotEqual(t, api_usage.SubStatusPossibleTrap, 0)
	assert.NotEqual(t, api_usage.SubStatusGlobalSuppression, 0)
	assert.NotEqual(t, api_usage.SubStatusTimeoutExceeded, 0)
	assert.NotEqual(t, api_usage.SubStatusMailServerTemporaryError, 0)
	assert.NotEqual(t, api_usage.SubStatusMailServerDidNotRespond, 0)
	assert.NotEqual(t, api_usage.SubStatusGreylisted, 0)
	assert.NotEqual(t, api_usage.SubStatusAntispamSystem, 0)
	assert.NotEqual(t, api_usage.SubStatusDoesNotAcceptMail, 0)
	assert.NotEqual(t, api_usage.SubStatusExceptionOccurred, 0)
	assert.NotEqual(t, api_usage.SubStatusFailedSyntaxCheck, 0)
	assert.NotEqual(t, api_usage.SubStatusMailboxNotFound, 0)
	assert.NotEqual(t, api_usage.SubStatusUnroutableIpAddress, 0)
	assert.NotEqual(t, api_usage.SubStatusPossibleTypo, 0)
	assert.NotEqual(t, api_usage.SubStatusNoDnsEntries, 0)
	assert.NotEqual(t, api_usage.SubStatusRoleBasedCatchAll, 0)
	assert.NotEqual(t, api_usage.SubStatusMailboxQuotaExceeded, 0)
	assert.NotEqual(t, api_usage.SubStatusForcibleDisconnect, 0)
	assert.NotEqual(t, api_usage.SubStatusFailedSmtpConnection, 0)
	assert.NotEqual(t, api_usage.SubStatusMxForward, 0)

	expected_start := time.Date(2023,1,1,0,0,0,0,time.Local)
	expected_end := time.Date(2023,12,12,0,0,0,0,time.Local)

	start_date, error_start := api_usage.StartDate()
	end_date, error_end := api_usage.EndDate()
	if error_start != nil || error_end != nil {
		t.Error(error_start, error_end)
		t.FailNow()
	}

	assert.Equal(t, start_date.Year(), expected_start.Year())
	assert.Equal(t, start_date.Month(), expected_start.Month())
	assert.Equal(t, start_date.Day(), expected_start.Day())

	assert.Equal(t, end_date.Year(), expected_end.Year())
	assert.Equal(t, end_date.Month(), expected_end.Month())
	assert.Equal(t, end_date.Day(), expected_end.Day())
}