package zerobouncego

import (
	"fmt"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

const (
	MOCK_FIND_MAIL_INVALID = `{
		"email": "",
		"email_confidence": "",
		"domain": "example.com",
		"company_name": "",
		"did_you_mean": "",
		"failure_reason": ""
	}`

	MOCK_FIND_MAIL_VALID = `{
		"email": "john.doe@example.com",
		"email_confidence": "high",
		"domain": "example.com",
		"company_name": "",
		"did_you_mean": "",
		"failure_reason": ""
	}`
)

func TestFindEmail400Error(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockBadRequestResponse("GET", ENDPOINT_EMAIL_FINDER)
	_, error_ := FindEmailByDomainFirstMiddleLastName("example.com", "John", "", "Doe")
	if !assert.NotNil(t, error_) {
		// expected not nil; fail otherwise
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
}

func TestFindEmail200Invalid(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse("GET", ENDPOINT_EMAIL_FINDER, MOCK_FIND_MAIL_INVALID)
	response_object, error_ := FindEmailByDomainFirstMiddleLastName("example.com", "John", "", "Doe")
	if !assert.Nil(t, error_) {
		// expected nil; fail otherwise
		t.FailNow()
	}
	assert.Equal(t, "", response_object.Email)
	assert.Equal(t, "", response_object.EmailConfidence)
	assert.Equal(t, "", response_object.DidYouMean)
	assert.Equal(t, "", response_object.FailureReason)
	assert.Equal(t, "example.com", response_object.Domain)
	assert.Equal(t, "", response_object.CompanyName)
}

func TestFindEmail200Valid(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse("GET", ENDPOINT_EMAIL_FINDER, MOCK_FIND_MAIL_VALID)
	response_object, error_ := FindEmailByDomainFirstMiddleLastName("example.com", "John", "", "Doe")
	if !assert.Nil(t, error_) {
		// expected nil; fail otherwise
		fmt.Print(error_.Error())
		t.FailNow()
	}

	assert.Equal(t, "john.doe@example.com", response_object.Email)
	assert.Equal(t, "high", response_object.EmailConfidence)
	assert.Equal(t, "", response_object.DidYouMean)
	assert.Equal(t, "", response_object.FailureReason)
	assert.Equal(t, "example.com", response_object.Domain)
	assert.Equal(t, "", response_object.CompanyName)

	_ = `
	"domain": "example.com",
	"did_you_mean": "",
	"failure_reason": "",
	`
}