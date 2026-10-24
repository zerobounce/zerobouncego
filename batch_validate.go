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
	return validateBatchWithOptions(packageConfig(), emails_list, opts)
}

func validateBatchWithOptions(cfg RequestConfig, emails_list []EmailToValidate, opts *ValidateBatchOptions) (ValidateBatchResponse, error) {
	response_object := &ValidateBatchResponse{}
	var error_ error

	if opts != nil && opts.Timeout != nil {
		timeout := *opts.Timeout
		if timeout < 10 || timeout > 120 {
			return *response_object, fmt.Errorf("Invalid parameter: timeout (must be between 10 and 120 seconds)")
		}
	}
	if error_ = cfg.requireKey(); error_ != nil {
		return *response_object, error_
	}

	payload_data := map[string]interface{}{
		"api_key":     cfg.APIKey,
		"email_batch": emails_list,
	}
	if opts != nil {
		if opts.Timeout != nil {
			// LIVE: the API requires timeout as a JSON string ("30"). As a
			// number it rejects the whole batch with the account-level
			// "Invalid API Key or your account ran out of credits" error.
			// activity_data and verify_plus stay JSON booleans.
			payload_data["timeout"] = fmt.Sprintf("%d", *opts.Timeout)
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

	urlToAccess, error_ := url.JoinPath(cfg.URI, ENDPOINT_BATCH_VALIDATE)
	if error_ != nil {
		return *response_object, fmt.Errorf("invalid URL (%s) or endpoint (%s) value", cfg.URI, ENDPOINT_BATCH_VALIDATE)
	}
	response, error_ := timedHTTPClient().Post(urlToAccess, "application/json", request_payload)
	if error_ != nil {
		return *response_object, error_
	}
	defer response.Body.Close()

	body, error_ := readLimitedBody(response.Body)
	if error_ != nil {
		if len(body) > 0 && strings.Contains(error_.Error(), "not valid JSON") {
			return *response_object, error_
		}
		return *response_object, error_
	}
	if error_ = decodeJSONBody(response.StatusCode, response.Header.Get("Content-Type"), body, response_object); error_ != nil {
		return ValidateBatchResponse{}, error_
	}
	for _, item := range response_object.Errors {
		if strings.EqualFold(strings.TrimSpace(item.EmailAddress), "all") {
			message := stripErrorPrefix(strings.TrimSpace(item.Error))
			if message == "" {
				message = "Invalid API Key or your account ran out of credits"
			}
			return ValidateBatchResponse{}, fmt.Errorf("%s", message)
		}
	}
	return *response_object, nil
}
