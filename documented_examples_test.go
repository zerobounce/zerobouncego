package zerobouncego

import (
	"net/http"
	"testing"
	"time"
)

// The documented success answers must parse through the real calls (ACCEPT):
// a documented field the SDK cannot read is a finding, not a missing example.

// documentedBatchValidateSuccess is copied verbatim from zerobounce.net's API reference.
const documentedBatchValidateSuccess = `
{
      "email_batch": [
          {
              "address": "john.doe@zerobounce.net",
              "status": "valid",
              "sub_status": "",
              "free_email": false,
              "catchall_domain": true,
              "did_you_mean": null,
              "account": "johndoe",
              "domain": "zerobounce.net",
              "domain_age_days": "3431",
              "active_in_days": "30",
              "active_first_seen": "2026-02-25",
              "smtp_provider": "g-suite",
              "mx_found": "true",
              "mx_record": "aspmx.l.google.com",
              "firstname": "dan",
              "lastname": "cotet",
              "gender": "male",
              "country": null,
              "region": null,
              "city": null,
              "zipcode": null,
              "processed_at": "2026-06-18 07:05:00.033"
          },
          {
              "address": "invalid@example.com",
              "status": "invalid",
              "sub_status": "mailbox_not_found",
              "free_email": false,
              "did_you_mean": null,
              "account": null,
              "domain": null,
              "domain_age_days": "9692",
              "active_in_days": "365",
              "active_first_seen": "2026-02-25",
              "smtp_provider": "example",
              "mx_found": "true",
              "mx_record": "mx.example.com",
              "catchall_domain": false,
              "firstname": "zero",
              "lastname": "bounce",
              "gender": "male",
              "country": null,
              "region": null,
              "city": null,
              "zipcode": null,
              "processed_at": "2020-09-17 17:43:11.830"
          },
          {
              "address": "disposable@example.com",
              "status": "do_not_mail",
              "sub_status": "disposable",
              "free_email": false,
              "did_you_mean": null,
              "account": null,
              "domain": null,
              "domain_age_days": "9692",
              "active_in_days": "365",
              "active_first_seen": "2026-02-25",
              "smtp_provider": "example",
              "mx_found": "true",
              "mx_record": "mx.example.com",
              "catchall_domain": false,
              "firstname": "zero",
              "lastname": "bounce",
              "gender": "male",
              "country": null,
              "region": null,
              "city": null,
              "zipcode": null,
              "processed_at": "2020-09-17 17:43:11.830"
          }
      ],
      "errors": []
  }
`

// documentedApiUsageSuccess is copied verbatim from zerobounce.net's API reference.
const documentedApiUsageSuccess = `
{
      "total": 120,
      "status_valid": 17,
      "status_invalid": 69,
      "status_catch_all": 9,
      "status_do_not_mail": 16,
      "status_spamtrap": 0,
      "status_unknown": 6,
      "sub_status_toxic": 0,
      "sub_status_disposable": 1,
      "sub_status_role_based": 1,
      "sub_status_possible_trap": 0,
      "sub_status_global_suppression": 14,
      "sub_status_timeout_exceeded": 0,
      "sub_status_mail_server_temporary_error": 0,
      "sub_status_mail_server_did_not_respond": 0,
      "sub_status_greylisted": 6,
      "sub_status_antispam_system": 0,
      "sub_status_does_not_accept_mail": 27,
      "sub_status_exception_occurred": 0,
      "sub_status_failed_syntax_check": 20,
      "sub_status_mailbox_not_found": 12,
      "sub_status_unroutable_ip_address": 0,
      "sub_status_possible_typo": 0,
      "sub_status_no_dns_entries": 10,
      "sub_status_role_based_catch_all": 0,
      "sub_status_mailbox_quota_exceeded": 0,
      "sub_status_forcible_disconnect": 0,
      "sub_status_failed_smtp_connection": 0,
      "sub_status_leading_period_removed": 0,
      "sub_status_alias_address": 2,
      "sub_status_mx_forward": 0,
      "sub_status_alternate": 0,
      "sub_status_blocked": 0,
      "sub_status_allowed": 0,
      "start_date": "1/1/2023",
      "end_date": "1/30/2024"
    }
`

// documentedFileStatusPhase2NA is copied verbatim from zerobounce.net's API reference.
const documentedFileStatusPhase2NA = `
{
    "success": true,
    "file_id": "aaaaaaaa-zzzz-xxxx-yyyy-5003727fffff",
    "file_name": "Your file name.csv",
    "upload_date": "2023-04-28T15:25:41Z",
    "file_status": "Complete",
    "file_phase_2_status": "N/A",
    "complete_percentage": "100%",
    "return_url": "Your return URL if provided when calling sendfile API"
  }
`

func TestDocumentedBatchValidateSuccessExampleParses(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{body: documentedBatchValidateSuccess}
	http.DefaultTransport = fake
	pointEverythingAt(t, "https://api.example.test")

	batch, err := ValidateBatch([]EmailToValidate{{EmailAddress: "john.doe@zerobounce.net"}})
	if err != nil {
		t.Fatalf("the documented batch-validate answer: err = %v", err)
	}
	if len(batch.EmailBatch) != 3 || batch.EmailBatch[2].Status != "do_not_mail" {
		t.Fatalf("rows = %+v, want the three documented rows", batch.EmailBatch)
	}
	if len(batch.Errors) != 0 {
		t.Fatalf("errors = %+v, want none", batch.Errors)
	}
}

func TestDocumentedApiUsageSuccessExampleParses(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{body: documentedApiUsageSuccess}
	http.DefaultTransport = fake
	pointEverythingAt(t, "https://api.example.test")

	usage, err := GetApiUsage(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("the documented get-api-usage answer: err = %v", err)
	}
	if usage.Total != 120 || usage.SubStatusAliasAddress != 2 || usage.SubStatusLeadingPeriodRemoved != 0 {
		t.Fatalf("usage = %+v, want the documented counters", usage)
	}
}

func TestDocumentedFileStatusWithPhase2NAExampleParses(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{body: documentedFileStatusPhase2NA}
	http.DefaultTransport = fake
	pointEverythingAt(t, "https://api.example.test")

	status, err := BulkValidationFileStatus("aaaaaaaa-zzzz-xxxx-yyyy-5003727fffff")
	if err != nil {
		t.Fatalf("the documented file-status answer: err = %v", err)
	}
	if status.FilePhase2Status == nil || *status.FilePhase2Status != "N/A" {
		t.Fatalf("file_phase_2_status = %v, want N/A", status.FilePhase2Status)
	}
}
