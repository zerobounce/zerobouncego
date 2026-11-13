package zerobouncego

import (
	"net/http"
	"strings"
	"testing"
)

// `null` in an optional response field reads as the field's empty value, never
// as an API error (standard.md §3). The calls whose answers carry these fields
// must accept one body holding every such null.
func TestNullOptionalFieldsReadAsEmpty(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")

	answersWithNullFields := map[string]bool{
		"Validate": true, "ValidateWithTimeout": true, "ValidateWithOptions": true,
		"ValidateBatch": true, "ValidateBatchWithOptions": true,
		"FindEmail": true, "FindEmailByDomainFirstMiddleLastName": true,
		"FindEmailByDomainFirstLastName": true, "FindEmailByDomainFirstName": true,
		"FindEmailByCompanyFirstMiddleLastName": true, "FindEmailByCompanyFirstLastName": true,
		"FindEmailByCompanyFirstName": true, "DomainSearch": true,
		"DomainSearchByDomain": true, "DomainSearchByCompanyName": true,
		"BulkValidationFileStatus": true, "BulkValidationSubmit": true,
	}
	body := `{"success":true,"file_id":null,"file_name":"f.csv","file_status":"Complete",` +
		`"complete_percentage":"100%","return_url":null,"upload_date":null,"error_reason":null,` +
		`"email_batch":null,"errors":null,"other_domain_formats":null,"processed_at":null,` +
		`"address":"a@b.c","status":"valid","sub_status":"","email":"a@b.c","domain":"b.c",` +
		`"format":"first","confidence":"high","did_you_mean":"","failure_reason":"","message":null}`

	covered := 0
	for _, call := range publicRequestCalls(client, warmup) {
		name := strings.TrimPrefix(call.name, "Client.")
		if !answersWithNullFields[name] {
			continue
		}
		covered++
		expectAccepted(t, call, fake, body)
	}
	if covered < len(answersWithNullFields) {
		t.Fatalf("only %d of %d calls in the sweep", covered, len(answersWithNullFields))
	}
}
