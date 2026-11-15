package zerobouncego

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// A finder or domain-search answer whose keys are only message keys is an error
// with status 200, whatever the message's value: a string, a blank, a null or a
// list (standard.md §3). Validate's greylist {"Message": ...} is a result.
func TestMessageOnlyFinderAndDomainSearchAnswersAreErrorsWhateverTheMessage(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	var finderCalls []requestCall
	for _, call := range publicRequestCalls(client, warmup) {
		name := strings.TrimPrefix(call.name, "Client.")
		if (strings.HasPrefix(name, "FindEmail") || strings.HasPrefix(name, "DomainSearch")) &&
			!strings.Contains(name, "S3") && !strings.Contains(name, "SFTP") {
			finderCalls = append(finderCalls, call)
		}
	}
	if len(finderCalls) < 6 {
		t.Fatalf("sweep found %d finder and domain-search calls, want at least 6", len(finderCalls))
	}
	bodies := []string{
		`{"Message":"Invalid API key or your account ran out of credits"}`,
		`{"Message":""}`,
		`{"Message":"   "}`,
		`{"Message":null}`,
		`{"Message":["Invalid API key"]}`,
		`{"Message":[]}`,
		`{"Message":5}`,
		`{"error":""}`,
	}
	for _, call := range finderCalls {
		for _, body := range bodies {
			expectStatus200Error(t, call, fake, body)
		}
		expectStatus200Error(t, call, fake, `{}`)
	}
}

// A file job's message may be a list of strings on a delete and on a file
// status, and both are results, not errors.
func TestAFileJobListMessageIsAcceptedOnDeleteAndFileStatus(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	body := `{"success":true,"file_id":"x","file_name":"f.csv","file_status":"Complete",` +
		`"complete_percentage":"100%","return_url":null,"upload_date":"2023-04-26T17:52:23Z",` +
		`"message":["File is complete","Second line"]}`
	checked := 0
	for _, call := range publicRequestCalls(client, warmup) {
		name := strings.TrimPrefix(call.name, "Client.")
		if !strings.HasSuffix(name, "FileDelete") && !strings.HasSuffix(name, "FileStatus") &&
			!strings.HasSuffix(name, "FileStatusCheck") {
			continue
		}
		checked++
		expectAccepted(t, call, fake, body)
	}
	if checked < 4 {
		t.Fatalf("sweep found %d delete and file-status calls, want at least 4", checked)
	}
}

// Message-only answers, and an empty object, are errors on every typed call
// except validate's greylist, the filter calls, Inbox Placement, the
// list-evaluator result and Warmup.
func TestMessageOnlyBodiesAreErrorsOnEveryTypedCall(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	bodies := []string{`{"message":"x"}`, `{"Message":""}`, `{"detail":"x"}`, `{}`}
	checked := 0
	for _, call := range publicRequestCalls(client, warmup) {
		if messageOnlyExemptCall(call.name) {
			continue
		}
		checked++
		for _, body := range bodies {
			fake.body = body
			err := call.call()
			var apiErr *ZBApiError
			// File submits answer 201. The error carries that status.
			if !errors.As(err, &apiErr) || apiErr.StatusCode != fake.status {
				t.Errorf("%s %s: err = %#v, want a *ZBApiError with status %d", call.name, body, err, fake.status)
			}
		}
	}
	if checked < 30 {
		t.Fatalf("message-only sweep checked %d calls, want at least 30", checked)
	}
}

func messageOnlyExemptCall(name string) bool {
	switch name {
	case "Validate", "ValidateWithTimeout", "ValidateWithOptions", "Client.Validate",
		"AddFilter", "ListFilters", "ListFilters(post)", "DeleteFilter",
		"GetListEvaluatorResult", "DoPostFormRequest":
		return true
	}
	return strings.HasPrefix(name, "WarmupClient.") || strings.Contains(name, "InboxPlacement")
}
