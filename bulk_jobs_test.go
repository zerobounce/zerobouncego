package zerobouncego

import (
	"net/http"
	"strings"
	"testing"
)

// Offline tests for the endpoints ported from the Python SDK (407f0a8 and
// b0444f8): the new validate options, the S3 email finder / domain search and
// SFTP/FTP bulk jobs, and the Inbox Placement Tester.
//
// They reuse useFake from offline_api_test.go, which swaps http.DefaultTransport
// for a recorder. Nothing here reaches the live API.

func TestOfflineValidateSendsTheNewOptionsOnlyWhenSet(t *testing.T) {
	fake := useFake(t)
	Initialize("validate-key")

	if _, err := ValidateWithOptions("a@example.com", ValidateOptions{}); err != nil {
		t.Fatal(err)
	}
	request := onlyRequest(t, fake)
	for _, absent := range []string{"domain_info", "greylist_processing", "callback_url"} {
		if _, found := request.Fields[absent]; found {
			t.Fatalf("%s should not be sent: %+v", absent, request.Fields)
		}
	}

	fake.clear()
	yes, no := true, false
	if _, err := ValidateWithOptions("a@example.com", ValidateOptions{
		DomainInfo:         &yes,
		GreylistProcessing: &yes,
		CallbackURL:        "https://example.com/greylist",
	}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	// Booleans go on the wire as the strings "true"/"false".
	if request.Fields["domain_info"] != "true" ||
		request.Fields["greylist_processing"] != "true" ||
		request.Fields["callback_url"] != "https://example.com/greylist" {
		t.Fatalf("%+v", request.Fields)
	}

	fake.clear()
	if _, err := ValidateWithOptions("a@example.com", ValidateOptions{
		UsePost:            true,
		DomainInfo:         &no,
		GreylistProcessing: &yes,
		CallbackURL:        "https://example.com/greylist",
	}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodPost || request.Fields["domain_info"] != "false" {
		t.Fatalf("%+v", request)
	}
}

func TestOfflineCallbackURLRequiresGreylistProcessing(t *testing.T) {
	fake := useFake(t)
	Initialize("validate-key")
	no, yes := false, true

	for _, options := range []ValidateOptions{
		{CallbackURL: "https://example.com/cb"},
		{GreylistProcessing: &no, CallbackURL: "https://example.com/cb"},
	} {
		_, err := ValidateWithOptions("a@example.com", options)
		if err == nil || !strings.Contains(err.Error(), "greylist_processing") {
			t.Fatalf("err = %v", err)
		}
	}

	_, err := ValidateWithOptions("a@example.com", ValidateOptions{
		GreylistProcessing: &yes,
		CallbackURL:        "   ",
	})
	if err == nil || !strings.Contains(err.Error(), "callback_url") {
		t.Fatalf("err = %v", err)
	}

	if fake.size() != 0 {
		t.Fatalf("requests = %d, want 0", fake.size())
	}
}

func TestOfflineS3AndSftpFileJobsSendTheDocumentedFieldsOnly(t *testing.T) {
	fake := useFake(t)
	Initialize("bulk-key")

	if _, err := EmailFinderS3File(BulkFileJobOptions{
		FileName:     "lists/leads.csv",
		HasHeaderRow: true,
		ReturnURL:    "https://example.com/done",
	}); err != nil {
		t.Fatal(err)
	}
	request := onlyRequest(t, fake)
	if request.Method != http.MethodPost || request.Host != "bulkapi.zerobounce.net" {
		t.Fatalf("%+v", request)
	}
	// The S3 and SFTP jobs live on the bulkapi root, WITHOUT /v2.
	if request.Path != "/emailFinderS3file" {
		t.Fatalf("path = %s", request.Path)
	}
	// LIVE: file_name, has_header_row and return_url are all required.
	if request.Fields["file_name"] != "lists/leads.csv" ||
		request.Fields["has_header_row"] != "true" ||
		request.Fields["return_url"] != "https://example.com/done" {
		t.Fatalf("%+v", request.Fields)
	}
	// remove_duplicates is the only one of the four that stays optional.
	if _, found := request.Fields["remove_duplicates"]; found {
		t.Fatalf("remove_duplicates should not be sent: %+v", request.Fields)
	}

	fake.clear()
	remove := true
	if _, err := DomainSearchS3File(BulkFileJobOptions{
		FileName:         "lists/leads.csv",
		HasHeaderRow:     false,
		RemoveDuplicates: &remove,
		ReturnURL:        "https://example.com/done",
	}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/domainSearchS3file" ||
		request.Fields["has_header_row"] != "false" ||
		request.Fields["remove_duplicates"] != "true" ||
		request.Fields["return_url"] != "https://example.com/done" {
		t.Fatalf("%+v", request)
	}

	// LIVE: validateSFTPfile needs file_name too, although the docs mark it
	// optional ("file_name: This field is required").
	fake.clear()
	if _, err := ValidateSFTPFile(BulkFileJobOptions{FileName: "my_file.txt", HasHeaderRow: true, ReturnURL: "https://example.com/done"}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/validateSFTPfile" {
		t.Fatalf("path = %s", request.Path)
	}
	if request.Fields["file_name"] != "my_file.txt" || request.Fields["has_header_row"] != "true" {
		t.Fatalf("%+v", request.Fields)
	}
	// A job without file_name or return_url never reaches the API.
	if _, err := ValidateSFTPFile(BulkFileJobOptions{HasHeaderRow: true, ReturnURL: "https://example.com/done"}); err == nil {
		t.Fatal("missing file_name should be rejected")
	}
	if _, err := ValidateSFTPFile(BulkFileJobOptions{FileName: "f.csv", HasHeaderRow: true}); err == nil {
		t.Fatal("missing return_url should be rejected")
	}

	named := BulkFileJobOptions{FileName: "my_file.txt", HasHeaderRow: true, ReturnURL: "https://example.com/done"}
	for path, call := range map[string]func(BulkFileJobOptions) (*FileValidationResponse, error){
		"/validateSFTPfile":     ValidateSFTPFile,
		"/scoreSFTPfile":        ScoreSFTPFile,
		"/emailFinderSFTPfile":  EmailFinderSFTPFile,
		"/domainSearchSFTPfile": DomainSearchSFTPFile,
	} {
		fake.clear()
		if _, err := call(named); err != nil {
			t.Fatal(err)
		}
		request = onlyRequest(t, fake)
		if request.Path != path || request.Fields["file_name"] != "my_file.txt" {
			t.Fatalf("%s: %+v", path, request)
		}
	}

	// exportSFTPfile takes export_path, not the export_bucket exportS3file uses.
	fake.clear()
	if _, err := ExportSFTPFile("fid", "/exports"); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/exportSFTPfile" ||
		request.Fields["file_id"] != "fid" ||
		request.Fields["export_path"] != "/exports" {
		t.Fatalf("%+v", request)
	}
	if _, found := request.Fields["export_bucket"]; found {
		t.Fatalf("export_bucket should not be sent: %+v", request.Fields)
	}

	fake.clear()
	if _, err := ExportSFTPFile("fid", ""); err != nil {
		t.Fatal(err)
	}
	if _, found := onlyRequest(t, fake).Fields["export_path"]; found {
		t.Fatal("export_path should not be sent when empty")
	}
}

func TestOfflineBulkJobsRejectInvalidArgumentsBeforeRequest(t *testing.T) {
	fake := useFake(t)
	Initialize("bulk-key")
	blank := BulkFileJobOptions{FileName: "   ", HasHeaderRow: true, ReturnURL: "https://example.com/done"}

	for _, call := range []func(BulkFileJobOptions) (*FileValidationResponse, error){
		EmailFinderS3File, DomainSearchS3File,
		ScoreSFTPFile, EmailFinderSFTPFile, DomainSearchSFTPFile,
	} {
		if _, err := call(blank); err == nil || !strings.Contains(err.Error(), "file_name") {
			t.Fatalf("err = %v", err)
		}
	}

	if _, err := ExportSFTPFile("  ", ""); err == nil || !strings.Contains(err.Error(), "file_id") {
		t.Fatalf("err = %v", err)
	}

	if fake.size() != 0 {
		t.Fatalf("requests = %d, want 0", fake.size())
	}
}

func TestOfflineListEvaluatorUsesItsOwnBaseURL(t *testing.T) {
	fake := useFake(t)
	Initialize("eval-key")

	// The file id is one path segment, so a slash in it must stay encoded.
	if _, err := GetListEvaluatorResult("a b/c"); err != nil {
		t.Fatal(err)
	}
	request := onlyRequest(t, fake)
	if request.Host != "bulkapi.zerobounce.net" || request.Path != "/v2/listevaluator/a b/c/" {
		t.Fatalf("%+v", request)
	}

	fake.clear()
	LIST_EVALUATOR_URI = "https://bulk.example.test/v2/listevaluator"
	if _, err := GetListEvaluatorResult("fid"); err != nil {
		t.Fatal(err)
	}
	if host := onlyRequest(t, fake).Host; host != "bulk.example.test" {
		t.Fatalf("host = %s", host)
	}
}

func TestOfflineInboxPlacementUsesItsOwnHostAndCamelCaseJSON(t *testing.T) {
	fake := useFake(t)
	Initialize("ipt-key")

	if _, err := GetInboxPlacementProviders(); err != nil {
		t.Fatal(err)
	}
	request := onlyRequest(t, fake)
	if request.Path != "/api/ipt/providers" {
		t.Fatalf("path = %s", request.Path)
	}
	// The documented auth for this API is a Bearer header, not a query key.
	if request.Authorization != "Bearer ipt-key" {
		t.Fatalf("authorization = %s", request.Authorization)
	}

	fake.clear()
	if _, err := GetInboxPlacementTestTypes(); err != nil {
		t.Fatal(err)
	}
	if path := onlyRequest(t, fake).Path; path != "/api/ipt/test-types" {
		t.Fatalf("path = %s", path)
	}

	fake.clear()
	testType := 1
	if _, err := CreateInboxPlacementTest("Launch", &CreateInboxPlacementTestOptions{
		TestType:    &testType,
		SenderEmail: "news@example.com",
		WebhookURL:  "https://example.com/ipt",
	}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodPost || request.Path != "/api/ipt/test" {
		t.Fatalf("%+v", request)
	}
	// Bodies are camelCase JSON, and unset fields are left out.
	if request.Fields["testType"] != "1" ||
		request.Fields["senderEmail"] != "news@example.com" ||
		request.Fields["webhookUrl"] != "https://example.com/ipt" {
		t.Fatalf("%+v", request.Fields)
	}
	for _, absent := range []string{"providerIds", "seedIds"} {
		if _, found := request.Fields[absent]; found {
			t.Fatalf("%s should not be sent: %+v", absent, request.Fields)
		}
	}

	fake.clear()
	page, pageSize := 1, 25
	if _, err := ListInboxPlacementTests(&ListInboxPlacementTestsOptions{
		Status:   "finished",
		Page:     &page,
		PageSize: &pageSize,
	}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Fields["status"] != "finished" ||
		request.Fields["page"] != "1" ||
		request.Fields["page_size"] != "25" {
		t.Fatalf("%+v", request.Fields)
	}

	fake.clear()
	if _, err := ListInboxPlacementTests(nil); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	for _, absent := range []string{"status", "page", "page_size"} {
		if _, found := request.Fields[absent]; found {
			t.Fatalf("%s should not be sent: %+v", absent, request.Fields)
		}
	}

	fake.clear()
	if err := DeleteInboxPlacementTest("123"); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodDelete || request.Path != "/api/ipt/test/123" {
		t.Fatalf("%+v", request)
	}

	fake.clear()
	if _, err := SetInboxPlacementWebhook("https://example.com/ipt"); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodPut || request.Path != "/api/ipt/webhook" ||
		request.Fields["webhookUrl"] != "https://example.com/ipt" {
		t.Fatalf("%+v", request)
	}

	fake.clear()
	if _, err := GetInboxPlacementWebhook(); err != nil {
		t.Fatal(err)
	}
	if path := onlyRequest(t, fake).Path; path != "/api/ipt/webhook" {
		t.Fatalf("path = %s", path)
	}

	fake.clear()
	if _, err := GetInboxPlacementTest("   "); err == nil || !strings.Contains(err.Error(), "test_id") {
		t.Fatalf("err = %v", err)
	}
	if _, err := CreateInboxPlacementTest(" ", nil); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("err = %v", err)
	}
	if _, err := SetInboxPlacementWebhook(""); err == nil || !strings.Contains(err.Error(), "webhook_url") {
		t.Fatalf("err = %v", err)
	}
	if fake.size() != 0 {
		t.Fatalf("requests = %d, want 0", fake.size())
	}
}
