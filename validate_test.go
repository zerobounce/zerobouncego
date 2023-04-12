package zerobouncego

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

// mockValidateRequest mock responses of GET/validate
func mockValidateRequest() {
	httpmock.RegisterResponder("GET", `=~^(.*)`+ENDPOINT_VALIDATE+`(.*)\z`,
		func(req *http.Request) (*http.Response, error) {
			request_query := req.URL.Query()
			if request_query.Get("api_key") == "" {
				return nil, errors.New("'api_key' missing from request arguments")
			}
			email_address := request_query.Get("email")

			mock_response := MOCK_VALIDATE_RESPONSE[email_address]
			if mock_response == "" {
				return nil, fmt.Errorf("no mock for email address %s", email_address)
			}
			return httpmock.NewStringResponse(200, mock_response), nil
		},
	)
}

// mockBatchValidateRequest mock responses of GET/validatebatch
func mockBatchValidateRequest() {
	type BatchValidateRequestPayload struct {
		ApiKey string             `json:"api_key"`
		Emails *[]EmailToValidate `json:"email_batch"`
	}

	httpmock.RegisterResponder("POST", `=~^(.*)`+ENDPOINT_BATCH_VALIDATE+`(.*)\z`,
		func(req *http.Request) (*http.Response, error) {
			var error_ error
			var email_responses []string
			var error_responses []string
			const missing_email_address_value = `{
				"error": "Missing email_batch key: email_address.",
				"email_address": "unknown"
			}`
			const missing_email_batch_param = `{
				"Message": "Missing parameter: email_batch."
			}`
			const invalid_api_key = `{
				"email_address": "all",
				"error": "Invalid API Key or your account ran out of credits"
			}`

			request_body := &BatchValidateRequestPayload{}

			defer req.Body.Close()
			request_body_raw, error_ := io.ReadAll(req.Body)
			if error_ != nil {
				return nil, error_
			}
			error_ = json.NewDecoder(strings.NewReader(string(request_body_raw))).Decode(request_body)
			if error_ != nil {
				return nil, error_
			}

			// normally, errors request errors should be returned when this cases occur
			if request_body.Emails == nil {
				return httpmock.NewStringResponse(400, missing_email_batch_param), nil
			}
			if request_body.ApiKey == "" {
				return httpmock.NewStringResponse(200, `{"email_batch": [], "errors": [`+invalid_api_key+`]}`), nil
			}
			if len(*request_body.Emails) == 0 {
				return httpmock.NewStringResponse(400, missing_email_address_value), nil
			}

			// map request emails to expected responses
			for _, validation_email := range *request_body.Emails {
				email_address := validation_email.EmailAddress
				if email_address == "" {
					error_responses = append(error_responses, missing_email_address_value)
				} else if MOCK_VALIDATE_RESPONSE[email_address] == "" {
					return nil, fmt.Errorf("an email address that was not mocked was used: %s", email_address)
				} else {
					email_responses = append(email_responses, MOCK_VALIDATE_RESPONSE[email_address])
				}
			}

			// return response
			final_response := `{"email_batch": [` + strings.Join(email_responses, ",") + `], "errors": [` + strings.Join(error_responses, ",") + `]}` + "\n"
			return httpmock.NewStringResponse(200, final_response), nil
		},
	)
}

func TestMockValidationNoApiKeySet(t *testing.T) {
	SetApiKey("")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockValidateRequest()

	_, error_ := Validate("valid@example.com", SANDBOX_IP)
	assert.NotNil(t, error_)
	assert.Contains(t, error_.Error(), "api_key")
}

// TestMockValidation test the `Validate` function on each example email
func TestMockValidationOk(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockValidateRequest()

	for _, test_case := range emailsToValidate {
		email_response, error_ := Validate(test_case.Email, SANDBOX_IP)
		assert.Nil(t, error_)
		assert.Equalf(t, test_case.Status, email_response.Status, "failed for email %s", email_response.Address)
		assert.Equalf(t, test_case.SubStatus, email_response.SubStatus, "failed for email %s", email_response.Address)
	}
}

func TestMockBulkValidationNoApiKey(t *testing.T) {
	SetApiKey("")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockBatchValidateRequest()

	bulk_response, error_ := ValidateBatch(EmailsToValidate())
	if error_ != nil {
		t.Error(error_.Error())
	}
	if !assert.Len(t, bulk_response.EmailBatch, 0) {
		t.FailNow()
	}
	if !assert.Len(t, bulk_response.Errors, 1) {
		t.FailNow()
	}
	response_error := bulk_response.Errors[0]
	if !assert.Equal(t, response_error.EmailAddress, "all") {
		t.FailNow()
	}
	if !assert.Contains(t, response_error.Error, "Invalid API Key") {
		t.FailNow()
	}
}

func TestMockBulkValidationNoEmails(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockBatchValidateRequest()

	_, error_ := ValidateBatch([]EmailToValidate{})
	assert.NotNil(t, error_)
}

func TestMockBulkValidationValidAndErroneousMail(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockBatchValidateRequest()

	response, error_ := ValidateBatch([]EmailToValidate{
		{EmailAddress: "valid@example.com"}, {},
	})
	if !assert.Nil(t, error_) {
		t.FailNow()
	}
	assert.Len(t, response.EmailBatch, 1)
	assert.Len(t, response.Errors, 1)
}

// TestMockValidation test the `ValidateBatch` function on all example emails
func TestMockBulkValidationOk(t *testing.T) {
	SetApiKey("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockBatchValidateRequest()

	emails_to_validate := EmailsToValidate()
	response, error_ := ValidateBatch(emails_to_validate)
	if error_ != nil {
		t.Errorf(error_.Error())
	}
	for error_response := range response.Errors {
		t.Error(error_response)
	}
	assert.Len(t, response.EmailBatch, len(emails_to_validate))

	emailToTest := make(map[string]SingleTest)
	for _, single_test := range emailsToValidate {
		emailToTest[single_test.Email] = single_test
	}
	for _, email_response := range response.EmailBatch {
		test_details := emailToTest[email_response.Address]
		assert.Equalf(t, email_response.Status, test_details.Status, "failed for email %s", email_response.Address)
		assert.Equalf(t, email_response.SubStatus, test_details.SubStatus, "failed for email %s", email_response.Address)
	}
}
