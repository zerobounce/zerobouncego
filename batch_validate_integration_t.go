package zerobouncego

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func EmailsToValidate() []EmailToValidate {
	var batchEmails []EmailToValidate
	for _, email_test := range emailsToValidate {
		batchEmails = append(batchEmails, EmailToValidate{email_test.Email, SANDBOX_IP})
	}
	return batchEmails
}

// TestInvalidApiKey test expecting one error, relevant to invalid API key
func TestInvalidApiKey(t *testing.T) {
	SetApiKey("some_invalid_value")
	response, error_ := ValidateBatch(EmailsToValidate())

	assert.Nil(t, error_)
	assert.Len(t, response.EmailBatch, 0, "no email batch response was expected due to lack of valid API key")
	assert.Len(t, response.Errors, 1, "errors were expected in response due to lack of valid API key")
	assert.Equal(t, response.Errors[0].EmailAddress, "all")
}

func TestBulkEmailValidation(t *testing.T) {
	ImportApiKeyFromEnvFile()
	response, error_ := ValidateBatch(EmailsToValidate())
	if error_ != nil {
		fmt.Println(error_)
		assert.Nil(t, error_)
	}

	emailToTest := make(map[string]SingleTest)
	for _, single_test := range emailsToValidate {
		emailToTest[single_test.Email] = single_test
	}
	for _, email_response := range response.EmailBatch {
		test_details := emailToTest[email_response.Address]
		// fmt.Println(email_response.Status, email_response.SubStatus, test_details)
		assert.Equalf(t, email_response.Status, test_details.Status, "failed for email %s", email_response.Address)
		assert.Equalf(t, email_response.SubStatus, test_details.SubStatus, "failed for email %s", email_response.Address)
	}
}
