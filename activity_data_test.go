package zerobouncego

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

const (
	// not the only error response that can occur; just an example
	contents_400 = `{
		"error": "email must be specified"
	}`
	contents_200_found = `{
		"found": true,
		"active_in_days": "180"
	}`
	contents_200_not_found = `{
		"found": false,
		"active_in_days": null
	}`
	email_address_found     = "valid@example.com"
	email_address_not_found = "unknown@example.com"
)

// mockActivityDataResponses mock expected responses from GET/activity
func mockActivityDataResponses() {
	httpmock.RegisterResponder(
		"GET", `=~^(.*)`+ENDPOINT_ACTIVITY_DATA+`(.*)\z`,
		func(req *http.Request) (*http.Response, error) {
			query_params := req.URL.Query()

			// should not happen in current implementation following checks
			// were included for completeness and robustness
			if !query_params.Has("api_key") {
				return nil, errors.New("missing parameter api_key")
			} else if !query_params.Has("email") {
				return nil, errors.New("missing parameter email")
			}

			email_address := query_params.Get("email")
			if email_address == "" {
				return httpmock.NewStringResponse(400, contents_400), nil
			} else if email_address == email_address_found {
				return httpmock.NewStringResponse(200, contents_200_found), nil
			} else {
				return httpmock.NewStringResponse(200, contents_200_not_found), nil
			}
		},
	)
}

func TestActivityDataError(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockActivityDataResponses()

	activity_data, error_ := GetActivityData("")
	assert.Nil(t, activity_data)
	if !assert.NotNil(t, error_) {
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), "email must be specified")
}

func TestActivityDataNotFound(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockActivityDataResponses()

	activity_data, error_ := GetActivityData(email_address_not_found)
	assert.Nil(t, error_)
	if !assert.NotNil(t, activity_data) {
		t.FailNow()
	}
	assert.Equal(t, false, activity_data.Found)
	assert.Equal(t, false, activity_data.ActiveInDaysRaw.Valid)
	assert.Equal(t, -1, activity_data.ActiveInDays())
}

func TestActivityDataFound(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockActivityDataResponses()

	activity_data, error_ := GetActivityData(email_address_found)
	assert.Nil(t, error_)
	if !assert.NotNil(t, activity_data) {
		t.FailNow()
	}
	assert.Equal(t, true, activity_data.Found)
	assert.Equal(t, true, activity_data.ActiveInDaysRaw.Valid)
	assert.Equal(t, 180, activity_data.ActiveInDays())
}
