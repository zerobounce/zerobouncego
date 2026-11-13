package zerobouncego

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Field types are part of the shape (standard.md §3): on a typed call a field
// of the wrong JSON type is a *ZBApiError with the 2xx status, never a result
// with a zero or sentinel value. getcredits must carry Credits as an integer,
// either a JSON string holding one or a JSON integer; validate (and each validatebatch row) must carry
// address and status as strings, except the greylist {"Message": ...} answer.

// fieldTypeCalls picks the getcredits and validate calls out of the complete
// public sweep, so a new credits or validate entry point is covered too.
func fieldTypeCalls(t *testing.T, client *Client, warmup *WarmupClient) (credits, validate, batch []requestCall) {
	t.Helper()
	for _, call := range publicRequestCalls(client, warmup) {
		name := strings.TrimPrefix(call.name, "Client.")
		switch {
		case strings.Contains(name, "Credits"):
			credits = append(credits, call)
		case strings.HasPrefix(name, "ValidateBatch"):
			batch = append(batch, call)
		case strings.HasPrefix(name, "Validate") && !strings.Contains(name, "S3") && !strings.Contains(name, "SFTP"):
			validate = append(validate, call)
		}
	}
	if len(credits) < 1 || len(validate) < 4 || len(batch) < 2 {
		t.Fatalf("sweep found %d credits, %d validate and %d batch calls", len(credits), len(validate), len(batch))
	}
	return credits, validate, batch
}

func expectStatus200Error(t *testing.T, call requestCall, fake *fixedBodyTransport, body string) {
	t.Helper()
	fake.body = body
	err := call.call()
	var apiErr *ZBApiError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 200 {
		t.Errorf("%s %s: err = %#v, want a *ZBApiError with status 200", call.name, body, err)
	}
}

func expectAccepted(t *testing.T, call requestCall, fake *fixedBodyTransport, body string) {
	t.Helper()
	fake.body = body
	if err := call.call(); err != nil {
		t.Errorf("%s %s: err = %v, want the documented response to parse", call.name, body, err)
	}
}

const docsValidateResponse = `{
	"address": "flowerjill@aol.com",
	"status": "valid",
	"sub_status": "",
	"free_email": true,
	"did_you_mean": null,
	"account": "flowerjill",
	"domain": "aol.com",
	"domain_age_days": "8426",
	"active_in_days": "100",
	"smtp_provider": "yahoo",
	"mx_record": "mx-aol.mail.gm0.yahoodns.net",
	"mx_found": "true",
	"firstname": "Jill",
	"lastname": "Stein",
	"gender": "female",
	"country": "United States",
	"region": "Florida",
	"city": "West Palm Beach",
	"zipcode": "33401",
	"processed_at": "2017-04-01 02:48:02.592"
}`

// docsCreditsResponse is zerobounce.net's own getcredits example, verbatim
// (v2-credit-balance, "Response"): Credits as a JSON integer.
const docsCreditsResponse = `{"Credits":2375323}`

func TestCreditsMustBeAnIntegerAsAStringOrANumber(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	credits, _, _ := fieldTypeCalls(t, client, warmup)
	for _, call := range credits {
		for _, body := range []string{
			`{}`, `{"Credits":null}`, `{"Credits":true}`, `{"Credits":"abc"}`,
			`{"Credits":"5.0"}`, `{"Credits":""}`, `{"Credits":" 5"}`, `{"Credits":{}}`, `{"other":"5"}`,
			`{"Credits":5.5}`, `{"Credits":5.0}`, `{"Credits":1e3}`, `{"Credits":-0.5}`, `{"Credits":[]}`, `{"Credits":[5]}`,
			`{"Credits":"99999999999999999999"}`, `{"Credits":99999999999999999999}`,
		} {
			expectStatus200Error(t, call, fake, body)
		}
		for _, body := range []string{
			docsCreditsResponse, `{"Credits":"2375323"}`, `{"Credits":"-1"}`, `{"Credits":"0"}`,
			`{"Credits":-1}`, `{"Credits":0}`, `{"Credits": 12345 }`,
		} {
			expectAccepted(t, call, fake, body)
		}
	}

	for body, want := range map[string]int{
		`{"Credits":"-1"}`: -1, `{"Credits":"12345"}`: 12345, docsCreditsResponse: 2375323, `{"Credits":-1}`: -1,
	} {
		fake.body = body
		response, err := GetCredits()
		if err != nil || response.Credits() != want {
			t.Fatalf("GetCredits %s: %+v, %v", body, response, err)
		}
	}
	fake.body = `{"Credits":"-1"}`
	if response, err := GetCredits(); err != nil || response.CreditsRaw != "-1" {
		t.Fatalf("GetCredits -1: %+v, %v", response, err)
	}
	fake.body = docsCreditsResponse
	if response, err := GetCredits(); err != nil || response.CreditsRaw != "2375323" {
		t.Fatalf("GetCredits docs example: %+v, %v", response, err)
	}
}

func TestValidateAddressAndStatusMustBeStrings(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	_, validate, batch := fieldTypeCalls(t, client, warmup)
	refused := []string{
		`{}`,
		`{"address":true,"status":"valid"}`,
		`{"address":"a@b.c","status":5}`,
		`{"address":null,"status":"valid"}`,
		`{"address":"a@b.c","status":null}`,
		`{"address":"a@b.c"}`,
		`{"status":"valid"}`,
		`{"address":"a@b.c","status":"valid","free_email":"yes"}`,
		`{"address":"a@b.c","status":"valid","did_you_mean":7}`,
		`{"Message":5}`,
	}
	for _, call := range validate {
		for _, body := range refused {
			expectStatus200Error(t, call, fake, body)
		}
		for _, body := range []string{
			docsValidateResponse,
			MOCK_VALIDATE_RESPONSE["disposable@example.com"],
			`{"address":"a@b.c","status":"valid","did_you_mean":null,"account":null}`,
			`{"Message":"Greylisted. Please validate again later."}`,
		} {
			expectAccepted(t, call, fake, body)
		}
	}
	for _, call := range batch {
		// Every row needs its address and status, and a row that is not an
		// object at all is refused too.
		rows := append(append([]string{}, refused[:7]...), `5`, `null`, `"x"`, `[]`, `true`)
		for _, row := range rows {
			expectStatus200Error(t, call, fake, `{"email_batch":[`+row+`],"errors":[]}`)
			expectStatus200Error(t, call, fake, `{"email_batch":[`+docsValidateResponse+`,`+row+`],"errors":[]}`)
		}
		expectAccepted(t, call, fake, `{"email_batch":[`+docsValidateResponse+`],"errors":[]}`)
	}
}

// status200Transport answers every request with one JSON body and HTTP 200.
type status200Transport struct{ body string }

func (f *status200Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	header := make(http.Header)
	header.Set("Content-Type", CONTENT_TYPE_JSON)
	return &http.Response{
		StatusCode:    200,
		Header:        header,
		Body:          io.NopCloser(strings.NewReader(f.body)),
		ContentLength: int64(len(f.body)),
		Request:       req,
	}, nil
}

// A dead or invalid key answered with HTTP 200 (E2) is a *ZBApiError with
// status 200 on every call, the multipart sendfile uploads included. Warmup
// is left out: its dead-key handling is an open owner decision.
func TestADeadKeyOn200IsAnErrorOnEveryCall(t *testing.T) {
	useFake(t)
	fake := &status200Transport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	uploads := 0
	for _, call := range publicRequestCalls(client, warmup) {
		if strings.HasPrefix(call.name, "WarmupClient.") {
			continue
		}
		if strings.Contains(call.name, "Submit") {
			uploads++
		}
		for _, body := range []string{
			`{"error":"Invalid API key or your account ran out of credits"}`,
			`{"error_message":"Invalid API key or your account ran out of credits"}`,
			`{"Error":"Invalid API key or your account ran out of credits"}`,
		} {
			fake.body = body
			err := call.call()
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != 200 {
				t.Errorf("%s %s: err = %#v, want a *ZBApiError with status 200", call.name, body, err)
			}
		}
	}
	if uploads < 8 {
		t.Fatalf("only %d sendfile uploads in the sweep", uploads)
	}
}
