package zerobouncego

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// REQUEST related structures

type EmailToValidate struct {
	EmailAddress string `json:"email_address" default:""`
	IPAddress    string `json:"ip_address"`
}

// RESPONSE related structures

type EmailBatchError struct {
	Error        string `json:"error"`
	EmailAddress string `json:"email_address"`
}

type ValidateBatchResponse struct {
	EmailBatch []ValidateResponse	`json:"email_batch"`
	Errors     []EmailBatchError	`json:"errors"`
}

func ValidateBatch(emails_list []EmailToValidate) (ValidateBatchResponse, error) {
	response_object := &ValidateBatchResponse{}
	var error_ error

	// request preparation
	payload_data := map[string]interface{}{
		"api_key": API_KEY,
		"email_batch": emails_list,
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
	response, error_ := http.DefaultClient.Post(url_to_access, "application/json", request_payload)

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
