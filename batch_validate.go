package zerobouncego

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// EmailToValidate represents one unit send to the batch validate endpoint
type EmailToValidate struct {
	EmailAddress string `json:"email_address"`
	IPAddress    string `json:"ip_address"`
}

// EmailBatchError an error unit received in the response, that can be associated
// with an email sent to the batch validate endpoint
type EmailBatchError struct {
	Error        string `json:"error"`
	EmailAddress string `json:"email_address"`
}

// ValidateBatchResponse represents the structure of a 200OK batch validate
// response
type ValidateBatchResponse struct {
	EmailBatch []ValidateResponse `json:"email_batch"`
	Errors     []EmailBatchError  `json:"errors"`
}

// ValidateBatchOptions optional body fields for POST /validatebatch.
type ValidateBatchOptions struct {
	Timeout      *int
	ActivityData *bool
	VerifyPlus   *bool
}

// ValidateBatch given a list of emails (and, optionally, their IPs), validate
// them and return both validation details and errors about the emails sent
func ValidateBatch(emails_list []EmailToValidate) (ValidateBatchResponse, error) {
	return ValidateBatchWithOptions(emails_list, nil)
}

func ValidateBatchWithOptions(emails_list []EmailToValidate, opts *ValidateBatchOptions) (ValidateBatchResponse, error) {
	response_object := &ValidateBatchResponse{}
	var error_ error

	payload_data := map[string]interface{}{
		"api_key":     API_KEY,
		"email_batch": emails_list,
	}
	if opts != nil {
		if opts.Timeout != nil {
			payload_data["timeout"] = *opts.Timeout
		}
		if opts.ActivityData != nil {
			payload_data["activity_data"] = *opts.ActivityData
		}
		if opts.VerifyPlus != nil {
			payload_data["verify_plus"] = *opts.VerifyPlus
		}
	}
	request_payload_builder := &strings.Builder{}
	encode_error := json.NewEncoder(request_payload_builder).Encode(payload_data)
	if encode_error != nil {
		return *response_object, encode_error
	}
	request_payload := strings.NewReader(request_payload_builder.String())

	// actual request
	url_to_access, error_ := url.JoinPath(URI, ENDPOINT_BATCH_VALIDATE)
	if error_ != nil {
		return *response_object, fmt.Errorf("invalid URL (%s) or endpoint (%s) value", URI, ENDPOINT_BATCH_VALIDATE)
	}
	response, error_ := timedHTTPClient().Post(url_to_access, "application/json", request_payload)

	// handle errors
	if error_ != nil {
		return *response_object, error_
	}

	// queue body closing before accessing it
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return *response_object, ErrorFromResponse(response)
	}
	json.NewDecoder(response.Body).Decode(response_object)
	return *response_object, nil
}
