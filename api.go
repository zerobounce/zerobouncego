package zerobounce

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// URI URL for the requests
const URI = `https://api.zerobounce.net/v2/`

// APIKey Key to be used in requests
var APIKey string

// APIResponse basis for api responses
type APIResponse interface{}

// CreditsResponse response of the credits balance
type CreditsResponse struct {
	Credits string `json:"Credits"`
}

// ValidateResponse Response from API
type ValidateResponse struct {
	Address       string      `json:"address"`
	Status        string      `json:"status"`
	SubStatus     string      `json:"sub_status"`
	FreeEmail     bool        `json:"free_email"`
	DidYouMean    interface{} `json:"did_you_mean"`
	Account       string      `json:"account"`
	Domain        string      `json:"domain"`
	DomainAgeDays string      `json:"domain_age_days"`
	SMTPProvider  string      `json:"smtp_provider"`
	MxRecord      string      `json:"mx_record"`
	MxFound       string      `json:"mx_found"`
	Firstname     string      `json:"firstname"`
	Lastname      string      `json:"lastname"`
	Gender        string      `json:"gender"`
	Country       string      `json:"country"`
	Region        string      `json:"region"`
	City          string      `json:"city"`
	Zipcode       string      `json:"zipcode"`
	ProcessedAt   string      `json:"processed_at"`
}

// IsValid checks if an email is valid
func (v *ValidateResponse) IsValid() bool {
	return v.Status == "valid"
}

// PrepareURL prepares the URL
func PrepareURL(endpoint string, params url.Values) string {

	// Set API KEY
	params.Set("api_key", APIKey)

	// Create a return the final URL
	return fmt.Sprintf("%s/%s?%s", URI, endpoint, params.Encode())
}

// DoRequest does the request to the API
func DoRequest(url string, object APIResponse) error {

	// Do the request
	response, err := http.Get(url)
	if err != nil {
		return err
	}

	// Check if server response is not HTTP 200
	if response.StatusCode != 200 {
		return errors.New("server error")
	}

	// Close the request
	defer response.Body.Close()

	// Decode JSON Request
	err = json.NewDecoder(response.Body).Decode(&object)
	return err
}

// Validate validates a single email
func Validate(email string, IPAddress string) (*ValidateResponse, error) {

	// Prepare the parameters
	params := url.Values{}
	params.Set("email", email)
	params.Set("ip_address", IPAddress)

	response := &ValidateResponse{}

	// Do the request
	err := DoRequest(PrepareURL("validate", params), response)
	return response, err
}

// GetCredits gets credits balance
func GetCredits() (*CreditsResponse, error) {

	response := &CreditsResponse{}
	err := DoRequest(PrepareURL("getcredits", url.Values{}), response)
	return response, err
}
