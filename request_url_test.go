package zerobouncego

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// requestCall is one public function that sends a request, called with
// arguments that pass its own validation.
type requestCall struct {
	name string
	call func() error
}

func csvUpload() CsvFile {
	return CsvFile{File: strings.NewReader("a@example.com\n"), FileName: "a.csv", EmailAddressColumn: 1}
}

// sweepDownloads receives what the download calls in publicRequestCalls write,
// so a test can check that a failed download wrote nothing.
var sweepDownloads bytes.Buffer

// publicRequestCalls covers every exported function and method that sends a
// request. TestEveryPublicRequestFunctionIsSwept keeps the list complete.
func publicRequestCalls(client *Client, warmup *WarmupClient) []requestCall {
	email := "a@example.com"
	fileID := testing_file_id
	job := BulkFileJobOptions{FileName: "a.csv", ReturnURL: "https://example.com/done"}
	sink := &sweepDownloads
	return []requestCall{
		{"GetCredits", func() error { _, err := GetCredits(); return err }},
		{"GetApiUsage", func() error { _, err := GetApiUsage(time.Now(), time.Now()); return err }},
		{"GetActivityData", func() error { _, err := GetActivityData(email); return err }},
		{"Validate", func() error { _, err := Validate(email, ""); return err }},
		{"ValidateWithTimeout", func() error { _, err := ValidateWithTimeout(email, "", "10"); return err }},
		{"ValidateWithOptions", func() error { _, err := ValidateWithOptions(email, ValidateOptions{UsePost: true}); return err }},
		{"ValidateBatch", func() error { _, err := ValidateBatch([]EmailToValidate{{EmailAddress: email}}); return err }},
		{"ValidateBatchWithOptions", func() error {
			_, err := ValidateBatchWithOptions([]EmailToValidate{{EmailAddress: email}}, nil)
			return err
		}},
		{"ScoreEmail", func() error { _, err := ScoreEmail(email); return err }},
		{"ScoreEmailPost", func() error { _, err := ScoreEmailPost(email); return err }},
		{"FindEmail", func() error { _, err := FindEmail("example.com", "John", "", "Doe"); return err }},
		{"FindEmailByDomainFirstMiddleLastName", func() error {
			_, err := FindEmailByDomainFirstMiddleLastName("example.com", "John", "Q", "Doe")
			return err
		}},
		{"FindEmailByDomainFirstLastName", func() error {
			_, err := FindEmailByDomainFirstLastName("example.com", "John", "Doe")
			return err
		}},
		{"FindEmailByDomainFirstName", func() error { _, err := FindEmailByDomainFirstName("example.com", "John"); return err }},
		{"FindEmailByCompanyFirstMiddleLastName", func() error {
			_, err := FindEmailByCompanyFirstMiddleLastName("Example", "John", "Q", "Doe")
			return err
		}},
		{"FindEmailByCompanyFirstLastName", func() error {
			_, err := FindEmailByCompanyFirstLastName("Example", "John", "Doe")
			return err
		}},
		{"FindEmailByCompanyFirstName", func() error { _, err := FindEmailByCompanyFirstName("Example", "John"); return err }},
		{"DomainSearch", func() error { _, err := DomainSearch("example.com"); return err }},
		{"DomainSearchByDomain", func() error { _, err := DomainSearchByDomain("example.com"); return err }},
		{"DomainSearchByCompanyName", func() error { _, err := DomainSearchByCompanyName("Example"); return err }},
		{"AddFilter", func() error { _, err := AddFilter("allow", "email", email); return err }},
		{"ListFilters", func() error { _, err := ListFilters(false); return err }},
		{"ListFilters(post)", func() error { _, err := ListFilters(true); return err }},
		{"DeleteFilter", func() error { _, err := DeleteFilter("allow", "email", email); return err }},
		{"BulkValidationSubmit", func() error { _, err := BulkValidationSubmit(csvUpload(), false); return err }},
		{"BulkValidationFileStatus", func() error { _, err := BulkValidationFileStatus(fileID); return err }},
		{"BulkValidationResult", func() error { return BulkValidationResult(fileID, sink) }},
		{"BulkValidationResultWithOptions", func() error { return BulkValidationResultWithOptions(fileID, sink, nil) }},
		{"BulkValidationFileDelete", func() error { _, err := BulkValidationFileDelete(fileID); return err }},
		{"AiScoringFileSubmit", func() error { _, err := AiScoringFileSubmit(csvUpload(), false); return err }},
		{"AiScoringFileStatus", func() error { _, err := AiScoringFileStatus(fileID); return err }},
		{"AiScoringResult", func() error { return AiScoringResult(fileID, sink) }},
		{"AiScoringResultWithOptions", func() error { return AiScoringResultWithOptions(fileID, sink, nil) }},
		{"AiScoringFileDelete", func() error { _, err := AiScoringFileDelete(fileID); return err }},
		{"GenericFileSubmit", func() error { _, err := GenericFileSubmit(csvUpload(), false, ENDPOINT_FILE_SEND); return err }},
		{"GenericFileStatusCheck", func() error { _, err := GenericFileStatusCheck(fileID, ENDPOINT_FILE_STATUS); return err }},
		{"GenericResultFetch", func() error { return GenericResultFetch(fileID, ENDPOINT_FILE_RESULT, sink) }},
		{"GenericResultFetchWithOptions", func() error {
			return GenericResultFetchWithOptions(fileID, ENDPOINT_FILE_RESULT, sink, nil)
		}},
		{"GenericFileDelete", func() error { _, err := GenericFileDelete(fileID, ENDPOINT_FILE_DELETE); return err }},
		{"BulkEmailFinderSubmit", func() error {
			_, err := BulkEmailFinderSubmit(EmailFinderCsvFile{File: strings.NewReader("example.com,John\n"), FileName: "f.csv", DomainColumn: 1, FirstNameColumn: 2})
			return err
		}},
		{"BulkEmailFinderFileStatus", func() error { _, err := BulkEmailFinderFileStatus(fileID); return err }},
		{"BulkEmailFinderResult", func() error { return BulkEmailFinderResult(fileID, sink) }},
		{"BulkEmailFinderFileDelete", func() error { _, err := BulkEmailFinderFileDelete(fileID); return err }},
		{"BulkDomainSearchSubmit", func() error {
			_, err := BulkDomainSearchSubmit(DomainSearchCsvFile{File: strings.NewReader("example.com\n"), FileName: "d.csv", DomainColumn: 1})
			return err
		}},
		{"BulkDomainSearchFileStatus", func() error { _, err := BulkDomainSearchFileStatus(fileID); return err }},
		{"BulkDomainSearchResult", func() error { return BulkDomainSearchResult(fileID, sink) }},
		{"BulkDomainSearchFileDelete", func() error { _, err := BulkDomainSearchFileDelete(fileID); return err }},
		{"ListEvaluatorSubmit", func() error {
			_, err := ListEvaluatorSubmit(ListEvaluatorCsvFile{File: strings.NewReader("a@example.com\n"), FileName: "l.csv", EmailAddressColumn: 1})
			return err
		}},
		{"GetListEvaluatorResult", func() error { _, err := GetListEvaluatorResult(fileID); return err }},
		{"SendEvaluatedFile", func() error { _, err := SendEvaluatedFile(fileID); return err }},
		{"ValidateS3File", func() error {
			_, err := ValidateS3File(ValidateS3FileOptions{FileName: "a.csv", ReturnURL: "https://example.com/done"})
			return err
		}},
		{"ScoreS3File", func() error {
			_, err := ScoreS3File(ScoreS3FileOptions{FileName: "a.csv", ReturnURL: "https://example.com/done"})
			return err
		}},
		{"ListS3Files", func() error { _, err := ListS3Files(ListS3FilesOptions{Bucket: "b"}); return err }},
		{"ExportS3File", func() error { _, err := ExportS3File(fileID, ""); return err }},
		{"EmailFinderS3File", func() error { _, err := EmailFinderS3File(job); return err }},
		{"DomainSearchS3File", func() error { _, err := DomainSearchS3File(job); return err }},
		{"ValidateSFTPFile", func() error { _, err := ValidateSFTPFile(job); return err }},
		{"ScoreSFTPFile", func() error { _, err := ScoreSFTPFile(job); return err }},
		{"EmailFinderSFTPFile", func() error { _, err := EmailFinderSFTPFile(job); return err }},
		{"DomainSearchSFTPFile", func() error { _, err := DomainSearchSFTPFile(job); return err }},
		{"ExportSFTPFile", func() error { _, err := ExportSFTPFile(fileID, ""); return err }},
		{"GetInboxPlacementProviders", func() error { _, err := GetInboxPlacementProviders(); return err }},
		{"GetInboxPlacementTestTypes", func() error { _, err := GetInboxPlacementTestTypes(); return err }},
		{"CreateInboxPlacementTest", func() error { _, err := CreateInboxPlacementTest("t", nil); return err }},
		{"GetInboxPlacementTest", func() error { _, err := GetInboxPlacementTest("t1"); return err }},
		{"ListInboxPlacementTests", func() error { _, err := ListInboxPlacementTests(nil); return err }},
		{"DeleteInboxPlacementTest", func() error { return DeleteInboxPlacementTest("t1") }},
		{"GetInboxPlacementWebhook", func() error { _, err := GetInboxPlacementWebhook(); return err }},
		{"SetInboxPlacementWebhook", func() error { _, err := SetInboxPlacementWebhook("https://example.com/hook"); return err }},
		{"DoGetRequest", func() error {
			requestURL, err := PrepareURL(ENDPOINT_CREDITS, url.Values{})
			if err != nil {
				return err
			}
			return DoGetRequest(requestURL, &CreditsResponse{})
		}},
		{"DoPostFormRequest", func() error {
			return DoPostFormRequest(ENDPOINT_VALIDATE, url.Values{"email": {email}}, &ValidateResponse{})
		}},
		{"Client.Validate", func() error { _, err := client.Validate(email, ""); return err }},

		{"WarmupClient.ListSendingAccounts", func() error { _, err := warmup.ListSendingAccounts(); return err }},
		{"WarmupClient.GetSendingAccount", func() error { _, err := warmup.GetSendingAccount(email); return err }},
		{"WarmupClient.AddSendingAccount", func() error {
			_, err := warmup.AddSendingAccount(email, "self_managed", &WarmupSendingAccountOptions{EngagementRuleID: intPtr(1)})
			return err
		}},
		{"WarmupClient.UpdateSendingAccount", func() error { _, err := warmup.UpdateSendingAccount(email, 1, 4, false); return err }},
		{"WarmupClient.DeleteSendingAccount", func() error { return warmup.DeleteSendingAccount(email) }},
		{"WarmupClient.ChangeFromAddress", func() error { return warmup.ChangeFromAddress(email, "b@example.com") }},
		{"WarmupClient.ListFromAddresses", func() error { _, err := warmup.ListFromAddresses(nil); return err }},
		{"WarmupClient.GetEngagementRules", func() error { _, err := warmup.GetEngagementRules(); return err }},
		{"WarmupClient.SetEngagementRule", func() error { return warmup.SetEngagementRule(email, 1) }},
		{"WarmupClient.GetCustomEngagement", func() error { _, err := warmup.GetCustomEngagement(email); return err }},
		{"WarmupClient.CreateCustomEngagement", func() error { _, err := warmup.CreateCustomEngagement(email, 10, 10, 10); return err }},
		{"WarmupClient.UpdateCustomEngagement", func() error { _, err := warmup.UpdateCustomEngagement(email, 10, 10, 10); return err }},
		{"WarmupClient.DeleteCustomEngagement", func() error { _, err := warmup.DeleteCustomEngagement(email, 1); return err }},
		{"WarmupClient.GetSeeds", func() error { _, err := warmup.GetSeeds(); return err }},
		{"WarmupClient.DownloadSeeds", func() error { _, err := warmup.DownloadSeeds(); return err }},
		{"WarmupClient.GetSeedCount", func() error { _, err := warmup.GetSeedCount(); return err }},
		{"WarmupClient.GetProgressToday", func() error { _, err := warmup.GetProgressToday(); return err }},
		{"WarmupClient.GetSentToday", func() error { _, err := warmup.GetSentToday(); return err }},
		{"WarmupClient.GetEmailsPerDay", func() error { _, err := warmup.GetEmailsPerDay(); return err }},
		{"WarmupClient.GetStatistics", func() error { _, err := warmup.GetStatistics(email); return err }},
		{"WarmupClient.GetStatisticsPerDay", func() error {
			_, err := warmup.GetStatisticsPerDay(email, "2026-01-01", "2026-01-02")
			return err
		}},
		{"WarmupClient.GetStatisticsDetailed", func() error { _, err := warmup.GetStatisticsDetailed(email); return err }},
		{"WarmupClient.DownloadStatistics", func() error { _, err := warmup.DownloadStatistics(email); return err }},
		{"WarmupClient.GetFromAddressStatistics", func() error { _, err := warmup.GetFromAddressStatistics(email); return err }},
		{"WarmupClient.CheckDomainDmarc", func() error { _, err := warmup.CheckDomainDmarc(email, ""); return err }},
		{"WarmupClient.TestSMTPConfiguration", func() error {
			_, err := warmup.TestSMTPConfiguration(email, "user", "secret", "smtp.example.com", 587)
			return err
		}},
	}
}

// pointEverythingAt sets every base URL, package-wide and per client, to base.
func pointEverythingAt(t *testing.T, base string) (*Client, *WarmupClient) {
	t.Helper()
	Initialize("sweep-key")
	URI = base + "/v2/"
	BULK_URI = base + "/v2/"
	LIST_EVALUATOR_URI = base + "/v2/listevaluator"
	INBOX_PLACEMENT_URI = base + "/api/ipt"
	client := New("sweep-key")
	client.URI, client.BulkURI = base+"/v2/", base+"/v2/"
	warmup, err := NewWarmupClient("sweep-key", "")
	if err != nil {
		t.Fatal(err)
	}
	// NewWarmupClient refuses a plain-http base, so move an accepted client.
	warmup.root = base + warmupBasePath
	return client, warmup
}

// Only SetURI and the Warmup client used to insist on https. A base URL set
// any other way (the URI, BULK_URI, LIST_EVALUATOR_URI and INBOX_PLACEMENT_URI
// variables, their ZERO_BOUNCE_*_URI environment variables, Client.URI) sent
// the API key over plain http to any host.
func TestEveryRequestRefusesPlainHTTP(t *testing.T) {
	for _, base := range []string{"http://evil.example", "HTTP://evil.example", "ftp://evil.example", "http://127.0.0.2.evil.example"} {
		fake := useFake(t)
		client, warmup := pointEverythingAt(t, base)
		for _, call := range publicRequestCalls(client, warmup) {
			t.Run(base+"/"+call.name, func(t *testing.T) {
				fake.clear()
				err := call.call()
				if err == nil || !strings.Contains(err.Error(), "https://") {
					t.Fatalf("err = %v, want the https refusal", err)
				}
				if strings.Contains(err.Error(), "sweep-key") {
					t.Fatalf("error carries the key: %v", err)
				}
				if n := fake.size(); n != 0 {
					t.Fatalf("%d request(s) reached the transport", n)
				}
			})
		}
	}
}

// Loopback http stays allowed, so tests can run a local fake server.
func TestEveryRequestAllowsLoopbackHTTP(t *testing.T) {
	for _, base := range []string{"http://127.0.0.1:9", "http://localhost:9", "http://[::1]:9", "https://api.example.test"} {
		fake := useFake(t)
		client, warmup := pointEverythingAt(t, base)
		for _, call := range publicRequestCalls(client, warmup) {
			t.Run(base+"/"+call.name, func(t *testing.T) {
				fake.clear()
				_ = call.call()
				if n := fake.size(); n == 0 {
					t.Fatal("no request reached the transport")
				}
			})
		}
	}
}

// A key with surrounding whitespace or a control character is refused by every
// request function before anything is sent. The Warmup client checks its key
// once, in NewWarmupClient (its key field cannot be set any other way), so its
// methods are covered by that constructor check.
func TestEveryRequestRejectsPaddedOrControlCharacterKey(t *testing.T) {
	for _, key := range []string{" key", "key ", "\tkey", "key\n", "ke\x00y", "ke\x7fy", "ke\ry"} {
		fake := useFake(t)
		client, warmup := pointEverythingAt(t, "https://api.example.test")
		Initialize(key)
		client.APIKey = key
		if _, err := NewWarmupClient(key, ""); err == nil || !strings.Contains(err.Error(), "surrounding whitespace or control characters") {
			t.Fatalf("%q NewWarmupClient: err = %v", key, err)
		}
		swept := 0
		for _, call := range publicRequestCalls(client, warmup) {
			if strings.HasPrefix(call.name, "WarmupClient.") {
				continue
			}
			swept++
			t.Run(fmt.Sprintf("%q/%s", key, call.name), func(t *testing.T) {
				fake.clear()
				err := call.call()
				if err == nil || !strings.Contains(err.Error(), "surrounding whitespace or control characters") {
					t.Fatalf("err = %v, want the key refusal", err)
				}
				if n := fake.size(); n != 0 {
					t.Fatalf("%d request(s) reached the transport", n)
				}
			})
		}
		if swept == 0 {
			t.Fatal("no request functions swept")
		}
	}
}

// offlineExports are the exported functions and methods that send nothing.
var offlineExports = map[string]bool{
	"Getenv": true, "Initialize": true, "InitializeWithURI": true, "SetApiKey": true, "SetURI": true,
	"LoadEnvFromFile": true, "PrepareURL": true, "ErrorFromResponse": true, "ImportCsvFile": true,
	"GetFileJSONIndicatesError": true, "FormatGetFileErrorMessage": true, "New": true, "NewWarmupClient": true,
	"Client.String": true, "Client.GoString": true, "Client.Format": true,
	"WarmupClient.String": true, "WarmupClient.GoString": true, "WarmupClient.Format": true,
	"RequestConfig.String": true, "RequestConfig.GoString": true, "RequestConfig.Format": true,
}

// A new public request function has to join the sweep above.
func TestEveryPublicRequestFunctionIsSwept(t *testing.T) {
	swept := map[string]bool{}
	for _, call := range publicRequestCalls(nil, nil) {
		swept[strings.SplitN(call.name, "(", 2)[0]] = true
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(fset, file, source, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil {
				receiver := fn.Recv.List[0].Type
				if star, ok := receiver.(*ast.StarExpr); ok {
					receiver = star.X
				}
				ident, ok := receiver.(*ast.Ident)
				if !ok || (ident.Name != "Client" && ident.Name != "WarmupClient" && ident.Name != "RequestConfig") {
					continue
				}
				name = ident.Name + "." + name
			}
			if !swept[name] && !offlineExports[name] {
				missing = append(missing, name)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("exported functions missing from publicRequestCalls (or offlineExports): %v", missing)
	}
}
