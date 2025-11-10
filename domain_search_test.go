package zerobouncego

import (
	"fmt"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

const (
	MOCK_DOMAIN_SEARCH_INVALID = `{
		"domain": "example.com",
		"company_name": "",
		"format": "unknown",
		"confidence": "undetermined",
		"did_you_mean": "",
		"failure_reason": "",
		"other_domain_formats": []
	}`

	MOCK_DOMAIN_SEARCH_VALID = `{
		"domain": "example.com",
		"company_name": "",
		"format": "first.last",
		"confidence": "high",
		"did_you_mean": "",
		"failure_reason": "",
		"other_domain_formats": [
			{
				"format": "first_last",
				"confidence": "high"
			},
			{
				"format": "first",
				"confidence": "medium"
			}
		]
	}`
)

func TestDomainSearch400Error(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockBadRequestResponse("GET", ENDPOINT_EMAIL_FINDER)
	_, error_ := DomainSearchByDomain("example.com")
	if !assert.NotNil(t, error_) {
		// expected not nil; fail otherwise
		t.FailNow()
	}
	assert.Contains(t, error_.Error(), sample_error_message)
}

func TestDomainSearch200Invalid(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse("GET", ENDPOINT_EMAIL_FINDER, MOCK_DOMAIN_SEARCH_INVALID)
	response_object, error_ := DomainSearchByDomain("example.com")
	if !assert.Nil(t, error_) {
		// expected nil; fail otherwise
		t.FailNow()
	}
	assert.Equal(t, "", response_object.CompanyName)
	assert.Equal(t, "", response_object.DidYouMean)
	assert.Equal(t, "", response_object.FailureReason)
	assert.Equal(t, "example.com", response_object.Domain)
	assert.Equal(t, "unknown", response_object.Format)
	assert.Equal(t, "undetermined", response_object.Confidence)
	assert.Equal(t, 0, len(response_object.OtherDomainFormats))
}

func TestDomainSearch200Valid(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockOkResponse("GET", ENDPOINT_EMAIL_FINDER, MOCK_DOMAIN_SEARCH_VALID)
	response_object, error_ := DomainSearchByDomain("example.com")
	if !assert.Nil(t, error_) {
		// expected nil; fail otherwise
		fmt.Print(error_.Error())
		t.FailNow()
	}
	assert.Equal(t, "", response_object.CompanyName)
	assert.Equal(t, "", response_object.DidYouMean)
	assert.Equal(t, "", response_object.FailureReason)
	assert.Equal(t, "example.com", response_object.Domain)
	assert.Equal(t, "first.last", response_object.Format)
	assert.Equal(t, "high", response_object.Confidence)
	assert.Equal(t, 2, len(response_object.OtherDomainFormats))

	assert.Equal(t, "first_last", response_object.OtherDomainFormats[0].Format)
	assert.Equal(t, "high", response_object.OtherDomainFormats[0].Confidence)
	assert.Equal(t, "first", response_object.OtherDomainFormats[1].Format)
	assert.Equal(t, "medium", response_object.OtherDomainFormats[1].Confidence)
}
