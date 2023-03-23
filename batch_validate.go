package zerobouncego

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gopkg.in/guregu/null.v4"
)

// REQUEST related structures

type EmailForBatchCheck struct {
	EmailAddress	string	`json:"email_address" default:""`
	IPAddress		string	`json:"ip_address"`	
}

// RESPONSE related structures


type EmailBatchIndividualEmailResponse struct {
	// The email address you are validating.
	Address			string				`json:"address"`
	Status			BatchEmailStatus	`json:"status"`
	SubStatus		BatchEmailSubStatus	`json:"sub_status"`
	// The portion of the email address before the "@" symbol.
	Account			string				`json:"account"`
	// The portion of the email address after the "@" symbol.
	Domain			string				`json:"domain"`
	// Suggestive Fix for an email typo
	DidYouMean		null.String			`json:"did_you_mean"`
	// Age of the email domain in days or [null].
	DomainAgeDays	null.String			`json:"domain_age_days"`
	// [true/false] If the email comes from a free provider.
	FreeEmail		string				`json:"free_email"`
	// [true/false] Does the domain have an MX record.
	MxFound			string				`json:"mx_found"`
	// The preferred MX record of the domain
	MxRecord		string				`json:"mx_record"`
	// The SMTP Provider of the email or [null] [BETA].
	SmtpProvider	null.String			`json:"smtp_provider"`
	// The first name of the owner of the email when available or [null].
	Firstname		null.String			`json:"firstname"`
	// The last name of the owner of the email when available or [null].
	Lastname		null.String			`json:"lastname"`
	// The gender of the owner of the email when available or [null].
	Gender			null.String			`json:"gender"`
	// The city of the IP passed in or [null]
	City			null.String			`json:"city"`
	// The region/state of the IP passed in or [null]
	Region			null.String			`json:"region"`
	// The zipcode of the IP passed in or [null]
	Zipcode			null.String			`json:"zipcode"`
	// The country of the IP passed in or [null]
	Country			null.String			`json:"country"`
	// The UTC time the email was validated.
	ProcessedAt		string				`json:"processed_at"`
}


type EmailBatchErrorResponse struct {
	Error			string	`json:"error"`
	EmailAddress	string	`json:"email_address"`
}

type ValidateBatchResponse struct {
	EmailBatch	[]EmailBatchIndividualEmailResponse	`json:"email_batch"`
	Errors		[]EmailBatchErrorResponse			`json:"errors"`
}


func ValidateBatch(emails_list []EmailForBatchCheck) (ValidateBatchResponse, error) {
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