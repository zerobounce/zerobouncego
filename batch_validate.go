package zerobouncego

import (
	"encoding/json"
	"errors"
	"io"
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

type EmailBatchErrorResponse struct {
	Error        string `json:"error"`
	EmailAddress string `json:"email_address"`
}

type ValidateBatchResponse struct {
	EmailBatch []ValidateResponse        `json:"email_batch"`
	Errors     []EmailBatchErrorResponse `json:"errors"`
}

func ValidateBatch(emails_list []EmailToValidate) (ValidateBatchResponse, error) {
	response_object := &ValidateBatchResponse{}
	var error_ error

	// request preparation
	payload_data := make(map[string]interface{})
	payload_data["api_key"] = API_KEY
	payload_data["email_batch"] = emails_list
	request_payload := &strings.Builder{}
	encode_error := json.NewEncoder(request_payload).Encode(payload_data)
	if encode_error != nil {
		return *response_object, encode_error
	}
	body_payload := strings.NewReader(request_payload.String())

	// actual request
	response, error_ := http.DefaultClient.Post(PrepareURL(ENDPOINT_BATCH_VALIDATE, url.Values{}), "application/json", body_payload)

	// handle errors
	if error_ != nil {
		return *response_object, error_
	}

	// queue body closing
	defer response.Body.Close()

	if response.StatusCode != 200 {
		response_body, error_ := io.ReadAll(response.Body)
		if error_ != nil {
			return *response_object, error_
		} else {
			return *response_object, errors.New(string(response_body))
		}
	}
	// 200OK response
	json.NewDecoder(response.Body).Decode(response_object)
	return *response_object, nil
}
