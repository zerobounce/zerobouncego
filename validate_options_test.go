package zerobouncego

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestValidateWithOptionsSendsActivityAndVerifyPlus(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", `=~^(.*)`+ENDPOINT_VALIDATE+`(.*)\z`,
		func(req *http.Request) (*http.Response, error) {
			q := req.URL.Query()
			assert.Equal(t, "true", q.Get("activity_data"))
			assert.Equal(t, "false", q.Get("verify_plus"))
			return httpmock.NewStringResponse(200, `{"address":"a@b.com","status":"valid","sub_status":""}`), nil
		},
	)

	ad, vp := true, false
	_, err := ValidateWithOptions("a@b.com", ValidateOptions{
		IPAddress:    SANDBOX_IP,
		ActivityData: &ad,
		VerifyPlus:   &vp,
	})
	assert.Nil(t, err)
}

func TestValidateWithOptionsPost(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", `=~^(.*)`+ENDPOINT_VALIDATE+`\z`,
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "POST", req.Method)
			assert.NoError(t, req.ParseForm())
			assert.Equal(t, "a@b.com", req.FormValue("email"))
			assert.Equal(t, "mock_key", req.FormValue("api_key"))
			return httpmock.NewStringResponse(200, `{"address":"a@b.com","status":"valid","sub_status":""}`), nil
		},
	)

	_, err := ValidateWithOptions("a@b.com", ValidateOptions{
		IPAddress: SANDBOX_IP,
		UsePost:   true,
	})
	assert.Nil(t, err)
}

func TestValidateBatchWithOptionsSendsBodyFields(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", `=~^(.*)`+ENDPOINT_BATCH_VALIDATE+`(.*)\z`,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(200, `{"email_batch":[],"errors":[]}`), nil
		},
	)

	timeout := 60
	ad, vp := true, true
	_, err := ValidateBatchWithOptions(
		[]EmailToValidate{{EmailAddress: "a@b.com", IPAddress: SANDBOX_IP}},
		&ValidateBatchOptions{Timeout: &timeout, ActivityData: &ad, VerifyPlus: &vp},
	)
	assert.Nil(t, err)
}
