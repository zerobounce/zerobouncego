package zerobouncego

import (
	"net/url"
)

// ScoreEmailResponse is the successful GET/POST /scoring payload.
type ScoreEmailResponse struct {
	Email string `json:"email"`
	Score int    `json:"score"`
}

// ScoreEmail returns real-time AI scoring for a single email (GET /scoring).
func ScoreEmail(emailAddress string) (*ScoreEmailResponse, error) {
	return scoreEmail(emailAddress, false)
}

// ScoreEmailPost returns real-time AI scoring using POST /scoring (form-encoded).
func ScoreEmailPost(emailAddress string) (*ScoreEmailResponse, error) {
	return scoreEmail(emailAddress, true)
}

func scoreEmail(emailAddress string, usePost bool) (*ScoreEmailResponse, error) {
	responseObject := &ScoreEmailResponse{}
	var err error

	if usePost {
		form := url.Values{}
		form.Set("email", emailAddress)
		err = DoPostFormRequest(ENDPOINT_SCORING, form, responseObject)
	} else {
		requestParameters := url.Values{}
		requestParameters.Set("email", emailAddress)
		requestParameters.Set("api_key", API_KEY)

		urlToAccess, prepErr := PrepareURL(ENDPOINT_SCORING, requestParameters)
		if prepErr != nil {
			return nil, prepErr
		}
		err = DoGetRequest(urlToAccess, responseObject)
	}

	if err != nil {
		return nil, err
	}
	return responseObject, nil
}
