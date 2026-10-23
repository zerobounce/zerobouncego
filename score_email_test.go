package zerobouncego

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

const (
	scoreContents200 = `{
		"email": "mark@gmail.com",
		"score": 10
	}`
	scoreContentsError = `{
		"error": "Please provide a valid email address"
	}`
	scoreEmailValid   = "mark@gmail.com"
	scoreEmailInvalid = "not-an-email"
)

func mockScoreEmailResponses() {
	httpmock.RegisterResponder(
		"GET", `=~^(.*)`+ENDPOINT_SCORING+`(.*)\z`,
		func(req *http.Request) (*http.Response, error) {
			queryParams := req.URL.Query()
			if !queryParams.Has("api_key") {
				return nil, errors.New("missing parameter api_key")
			}
			if !queryParams.Has("email") {
				return nil, errors.New("missing parameter email")
			}
			email := queryParams.Get("email")
			if email == scoreEmailInvalid {
				return httpmock.NewStringResponse(400, scoreContentsError), nil
			}
			if email == scoreEmailValid {
				return httpmock.NewStringResponse(200, scoreContents200), nil
			}
			return httpmock.NewStringResponse(400, scoreContentsError), nil
		},
	)
	httpmock.RegisterResponder(
		"POST", `=~^(.*)`+ENDPOINT_SCORING+`\z`,
		func(req *http.Request) (*http.Response, error) {
			if err := req.ParseForm(); err != nil {
				return nil, err
			}
			if req.Form.Get("api_key") == "" {
				return nil, errors.New("missing parameter api_key")
			}
			email := req.Form.Get("email")
			if email == scoreEmailValid {
				return httpmock.NewStringResponse(200, scoreContents200), nil
			}
			return httpmock.NewStringResponse(200, scoreContentsError), nil
		},
	)
}

func TestScoreEmailGet(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockScoreEmailResponses()

	result, err := ScoreEmail(scoreEmailValid)
	assert.Nil(t, err)
	if assert.NotNil(t, result) {
		assert.Equal(t, scoreEmailValid, result.Email)
		assert.Equal(t, 10, result.Score)
	}
}

func TestScoreEmailPost(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockScoreEmailResponses()

	result, err := ScoreEmailPost(scoreEmailValid)
	assert.Nil(t, err)
	if assert.NotNil(t, result) {
		assert.Equal(t, scoreEmailValid, result.Email)
		assert.Equal(t, 10, result.Score)
	}
}

func TestScoreEmailErrorPayload(t *testing.T) {
	Initialize("mock_key")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	mockScoreEmailResponses()

	result, err := ScoreEmail(scoreEmailInvalid)
	assert.Nil(t, result)
	assert.NotNil(t, err)
}
