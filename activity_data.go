package zerobouncego

import (
	"net/url"
	"strconv"

	"gopkg.in/guregu/null.v4"
)

type ActivityDataResponse struct {
	Found           bool        `json:"found"`
	ActiveInDaysRaw null.String `json:"active_in_days"`
}

func (a ActivityDataResponse) ActiveInDays() int {
	if !a.ActiveInDaysRaw.Valid {
		return -1
	}
	conversion, error_ := strconv.Atoi(a.ActiveInDaysRaw.String)
	if error_ != nil {
		return -1
	}
	return conversion
}

// GetActivityData check the activity of an email address
func GetActivityData(email_address string) (*ActivityDataResponse, error) {
	var error_ error
	request_parameters := url.Values{}
	request_parameters.Set("email", email_address)
	request_parameters.Set("api_key", API_KEY)

	url_to_access, error_ := PrepareURL(ENDPOINT_ACTIVITY_DATA, request_parameters)
	if error_ != nil {
		return nil, error_
	}

	response_object := &ActivityDataResponse{}
	error_ = DoGetRequest(url_to_access, response_object)
	if error_ != nil {
		return nil, error_
	}

	return response_object, nil
}
