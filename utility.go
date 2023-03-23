package zerobouncego

import (
	"errors"
	"os"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const URI = `https://api.zerobounce.net/v2/`
var API_KEY string = os.Getenv("ZERO_BOUNCE_API_KEY")

func SetApiKey(new_api_key_value string) {
	API_KEY = new_api_key_value
}

// IsValid checks if an email is valid
func (v *ValidateResponse) IsValid() bool {
	return v.Status == "valid"
}

// PrepareURL prepares the URL
func PrepareURL(endpoint string, params url.Values) string {

	// Set API KEY
	params.Set("api_key", API_KEY)

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
