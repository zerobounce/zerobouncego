package zerobouncego

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordedRequest struct {
	Method string
	Host   string
	Path   string
	APIKey string
	// The Inbox Placement Tester authenticates with a Bearer header rather than
	// an api_key parameter, so the key can arrive either way.
	Authorization string
	Fields        map[string]string
}

type fakeTransport struct {
	mu      sync.Mutex
	records []recordedRequest
}

func (f *fakeTransport) take() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]recordedRequest(nil), f.records...)
	return out
}

func (f *fakeTransport) clear() {
	f.mu.Lock()
	f.records = nil
	f.mu.Unlock()
}

func (f *fakeTransport) size() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records)
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorded := recordHTTPRequest(req)
	f.mu.Lock()
	f.records = append(f.records, recorded)
	f.mu.Unlock()

	time.Sleep(time.Duration(rand.Intn(21)) * time.Millisecond)
	return fakeResponse(recorded), nil
}

func recordHTTPRequest(req *http.Request) recordedRequest {
	fields := map[string]string{}
	for key, values := range req.URL.Query() {
		if len(values) > 0 {
			fields[key] = values[0]
		}
	}
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	mediaType, params, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))
	switch {
	case strings.HasPrefix(mediaType, "multipart/"):
		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			part, err := reader.NextPart()
			if err != nil {
				break
			}
			partBody, _ := io.ReadAll(part)
			if part.FileName() != "" {
				fields[part.FormName()] = "<file>"
			} else {
				fields[part.FormName()] = string(partBody)
			}
		}
	case mediaType == "application/json" || (len(body) > 0 && body[0] == '{'):
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err == nil {
			for key, value := range payload {
				encoded, _ := json.Marshal(value)
				fields[key] = jsonFieldString(value, encoded)
			}
		} else {
			fields["__raw"] = string(body)
		}
	case mediaType == "application/x-www-form-urlencoded" || bytes.Contains(body, []byte("=")):
		parsed, err := url.ParseQuery(string(body))
		if err == nil {
			for key, values := range parsed {
				if len(values) > 0 {
					fields[key] = values[0]
				}
			}
		}
	}
	apiKey := fields["api_key"]
	authorization := req.Header.Get("Authorization")
	if apiKey == "" && strings.HasPrefix(authorization, "Bearer ") {
		apiKey = strings.TrimPrefix(authorization, "Bearer ")
	}
	return recordedRequest{
		Method:        req.Method,
		Host:          req.URL.Host,
		Path:          req.URL.Path,
		APIKey:        apiKey,
		Authorization: authorization,
		Fields:        fields,
	}
}

func jsonFieldString(value interface{}, encoded []byte) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return string(encoded)
	}
}

func htmlAccessDeniedPage() string {
	var builder strings.Builder
	builder.WriteString(`<!DOCTYPE html><html lang="en-US"><head><title>Access denied | zerobounce.net | Zero Bounce</title></head><body>`)
	for i := 0; i < 20000; i++ {
		builder.WriteString("<p>blocked</p>")
	}
	builder.WriteString("</body></html>")
	return builder.String()
}

func fakeResponse(req recordedRequest) *http.Response {
	key := req.APIKey
	header := make(http.Header)
	status := 200
	var body string
	switch {
	case strings.HasPrefix(key, "html-403-"):
		status = 403
		body = htmlAccessDeniedPage()
		header.Set("Content-Type", "text/html; charset=UTF-8")
	case strings.HasPrefix(key, "html-200-"):
		status = 200
		body = htmlAccessDeniedPage()
		header.Set("Content-Type", "text/html; charset=UTF-8")
	case strings.HasPrefix(key, "empty-"):
		status = 200
		body = ""
		header.Set("Content-Type", "application/json")
	case strings.HasPrefix(key, "malformed-"):
		status = 200
		body = "<<<not-json>>>" + strings.Repeat("x", 5000)
		header.Set("Content-Type", "application/json")
	case strings.HasPrefix(key, "cf-json-"):
		status = 403
		body = `{"type":"https://developers.cloudflare.com/support/troubleshooting/http-status-codes/cloudflare-1xxx-errors/error-1020/","title":"Error 1020: Access denied","status":403,"detail":"The request was blocked by a Cloudflare firewall rule configured by the site owner.","error_code":1020,"cloudflare_error":true,"what_you_should_do":"Do not retry. This block is intentional."}`
		header.Set("Content-Type", "application/json")
	case strings.HasPrefix(key, "double-error-"):
		status = 400
		body = "Error: Error: api_key is invalid"
		header.Set("Content-Type", "text/plain")
	case strings.Contains(req.Path, "validatebatch") && strings.HasPrefix(key, "batch-account-"):
		status = 200
		body = `{"email_batch":[],"errors":[{"email_address":"all","error":"Invalid API Key or your account ran out of credits"}]}`
		header.Set("Content-Type", "application/json")
	case strings.Contains(req.Path, "validatebatch") && strings.HasPrefix(key, "batch-partial-"):
		status = 200
		body = `{"email_batch":[{"address":"good@example.com","status":"valid"}],"errors":[{"email_address":"bad@","error":"Invalid email address"}]}`
		header.Set("Content-Type", "application/json")
	default:
		body = successBody(req, key)
		if strings.Contains(req.Path, "getfile") {
			header.Set("Content-Type", "application/octet-stream")
		} else if strings.Contains(req.Path, "sendfile") || strings.Contains(req.Path, "listevaluator") || strings.Contains(req.Path, "send-evaluated-file") {
			status = 201
			header.Set("Content-Type", "application/json")
		} else {
			header.Set("Content-Type", "application/json")
		}
	}
	response := &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    nil,
	}
	return response
}

func successBody(req recordedRequest, key string) string {
	quoted, _ := json.Marshal(key)
	switch {
	case strings.Contains(req.Path, "getfile"):
		return "email\n" + key + "@example.com\n"
	case strings.Contains(req.Path, "listS3files"):
		return `[{"name":` + string(quoted) + `,"size":1,"last_modified":""}]`
	case strings.Contains(req.Path, "validatebatch"):
		return `{"email_batch":[{"address":` + string(quoted) + `,"status":"valid"}],"errors":[]}`
	case strings.HasSuffix(req.Path, "/validate"):
		return `{"address":` + string(quoted) + `,"status":"valid"}`
	case strings.HasSuffix(req.Path, "/scoring"):
		return `{"email":` + string(quoted) + `,"score":10}`
	case strings.Contains(req.Path, "getcredits"):
		return `{"Credits":"7"}`
	case strings.Contains(req.Path, "getapiusage"):
		return `{"total":1,"start_date":"1/1/2018","end_date":"1/31/2018"}`
	case strings.HasSuffix(req.Path, "/activity"):
		return `{"found":true,"active_in_days":"3"}`
	case strings.Contains(req.Path, "filestatus"):
		return `{"success":true,"file_id":` + string(quoted) + `,"file_name":"f.csv","upload_date":"2023-04-26T17:52:23Z","file_status":"Complete","complete_percentage":"100%","return_url":""}`
	case strings.Contains(req.Path, "deletefile"):
		return `{"success":true,"message":"File Deleted","file_name":"f.csv","file_id":` + string(quoted) + `}`
	case strings.Contains(req.Path, "guessformat"):
		return `{"email":` + string(quoted) + `,"domain":` + string(quoted) + `,"other_domain_formats":[]}`
	case strings.Contains(req.Path, "filters") || (req.Method == http.MethodGet && strings.Contains(req.Path, "listevaluator")):
		return `{"echo_api_key":` + string(quoted) + `}`
	default:
		return `{"success":true,"message":"ok","file_id":` + string(quoted) + `,"file_name":"f.csv","email":` + string(quoted) + `,"domain":` + string(quoted) + `}`
	}
}

func useFake(t *testing.T) *fakeTransport {
	t.Helper()
	fake := &fakeTransport{}
	previous := http.DefaultTransport
	previousKey := API_KEY
	previousURI := URI
	previousBulk := BULK_URI
	previousEvaluator := LIST_EVALUATOR_URI
	previousPlacement := INBOX_PLACEMENT_URI
	http.DefaultTransport = fake
	t.Cleanup(func() {
		http.DefaultTransport = previous
		API_KEY = previousKey
		URI = previousURI
		BULK_URI = previousBulk
		LIST_EVALUATOR_URI = previousEvaluator
		INBOX_PLACEMENT_URI = previousPlacement
	})
	return fake
}

func onlyRequest(t *testing.T, fake *fakeTransport) recordedRequest {
	t.Helper()
	records := fake.take()
	if len(records) != 1 {
		t.Fatalf("requests = %d, want 1", len(records))
	}
	return records[0]
}

func assertShortError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	message := err.Error()
	if len(message) >= 500 {
		t.Fatalf("error is %d bytes: %s", len(message), message[:120])
	}
	for _, forbidden := range []string{"<html", "<!DOCTYPE", "<p>blocked</p>", "Error: Error:", "invalid character", "json:", "unexpected end of JSON"} {
		if strings.Contains(message, forbidden) {
			t.Fatalf("error contains %q: %s", forbidden, message)
		}
	}
}

func boolPtr(value bool) *bool { return &value }
func intPtr(value int) *int    { return &value }
func strPtr(value string) *string {
	return &value
}

func TestOfflineVersionMatchesPackage(t *testing.T) {
	if Version != "2.1.4" {
		t.Fatalf("Version = %s, want 2.1.4", Version)
	}
}

func TestOfflineGetCreditsAndValidate(t *testing.T) {
	fake := useFake(t)
	Initialize("credits-key")
	credits, err := GetCredits()
	if err != nil {
		t.Fatal(err)
	}
	if credits.Credits() != 7 {
		t.Fatalf("credits = %d", credits.Credits())
	}
	request := onlyRequest(t, fake)
	if request.Method != http.MethodGet || request.Host != "api.zerobounce.net" || request.Path != "/v2/getcredits" || request.APIKey != "credits-key" {
		t.Fatalf("%+v", request)
	}

	fake.clear()
	activity := false
	verify := true
	response, err := ValidateWithOptions("person@example.com", ValidateOptions{
		ActivityData: &activity,
		VerifyPlus:   &verify,
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Address != "credits-key" {
		t.Fatalf("address = %s", response.Address)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/v2/validate" || request.Fields["email"] != "person@example.com" {
		t.Fatalf("%+v", request.Fields)
	}
	if request.Fields["activity_data"] != "false" || request.Fields["verify_plus"] != "true" {
		t.Fatalf("bools = %#v", request.Fields)
	}
	if _, ok := request.Fields["timeout"]; ok {
		t.Fatal("unset timeout was sent")
	}

	fake.clear()
	_, err = ValidateWithOptions("person@example.com", ValidateOptions{
		IPAddress: "203.0.113.5",
		Timeout:   "15",
		UsePost:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodPost || request.Path != "/v2/validate" {
		t.Fatalf("%+v", request)
	}
	if request.Fields["timeout"] != "15" || request.Fields["ip_address"] != "203.0.113.5" || request.APIKey != "credits-key" {
		t.Fatalf("%#v", request.Fields)
	}
	if _, ok := request.Fields["activity_data"]; ok {
		t.Fatal("unset activity_data was sent")
	}
}

func TestOfflineValidateBatch(t *testing.T) {
	fake := useFake(t)
	Initialize("batch-key")
	response, err := ValidateBatch([]EmailToValidate{
		{EmailAddress: "a@example.com"},
		{EmailAddress: "b@example.com", IPAddress: "203.0.113.9"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.EmailBatch) != 1 || response.EmailBatch[0].Address != "batch-key" {
		t.Fatalf("%+v", response)
	}
	request := onlyRequest(t, fake)
	if request.Method != http.MethodPost || request.Host != "api.zerobounce.net" || request.Path != "/v2/validatebatch" {
		t.Fatalf("%+v", request)
	}
	if !strings.Contains(request.Fields["email_batch"], `"email_address":"a@example.com"`) {
		t.Fatalf("batch = %s", request.Fields["email_batch"])
	}
	if !strings.Contains(request.Fields["email_batch"], `"ip_address":"203.0.113.9"`) {
		t.Fatalf("batch = %s", request.Fields["email_batch"])
	}
	for _, omitted := range []string{"timeout", "activity_data", "verify_plus"} {
		if _, ok := request.Fields[omitted]; ok {
			t.Fatalf("unset %s was sent", omitted)
		}
	}

	fake.clear()
	timeout := 30
	activity := false
	verify := true
	_, err = ValidateBatchWithOptions([]EmailToValidate{{EmailAddress: "a@example.com"}}, &ValidateBatchOptions{
		Timeout:      &timeout,
		ActivityData: &activity,
		VerifyPlus:   &verify,
	})
	if err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Fields["timeout"] != "30" || request.Fields["activity_data"] != "false" || request.Fields["verify_plus"] != "true" {
		t.Fatalf("%#v", request.Fields)
	}
}

func TestOfflineFileAndScoring(t *testing.T) {
	fake := useFake(t)
	Initialize("file-key")
	allow := false
	csvFile := CsvFile{
		File:               strings.NewReader("email\n"),
		FileName:           "emails.csv",
		HasHeaderRow:       true,
		EmailAddressColumn: 0,
		ReturnURL:          "https://example.com/hook",
		AllowPhase2:        &allow,
	}
	response, err := BulkValidationSubmit(csvFile, false)
	if err != nil {
		t.Fatal(err)
	}
	if response.FileId != "file-key" {
		t.Fatalf("file id = %s", response.FileId)
	}
	request := onlyRequest(t, fake)
	if request.Host != "bulkapi.zerobounce.net" || request.Path != "/v2/sendfile" {
		t.Fatalf("%+v", request)
	}
	if request.Fields["email_address_column"] != "0" || request.Fields["file"] != "<file>" {
		t.Fatalf("%#v", request.Fields)
	}
	if request.Fields["has_header_row"] != "true" || request.Fields["remove_duplicate"] != "false" || request.Fields["allow_phase_2"] != "false" {
		t.Fatalf("bools %#v", request.Fields)
	}
	for _, omitted := range []string{"first_name_column", "last_name_column", "gender_column", "ip_address_column"} {
		if _, ok := request.Fields[omitted]; ok {
			t.Fatalf("unset %s was sent", omitted)
		}
	}

	fake.clear()
	csvFile.EmailAddressColumn = 1
	csvFile.FirstNameColumn = 2
	csvFile.AllowPhase2 = boolPtr(true)
	_, err = AiScoringFileSubmit(csvFile, true)
	if err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/v2/scoring/sendfile" {
		t.Fatalf("path %s", request.Path)
	}
	if _, ok := request.Fields["allow_phase_2"]; ok {
		t.Fatal("scoring sendfile sent allow_phase_2")
	}
	if _, ok := request.Fields["first_name_column"]; ok {
		t.Fatal("scoring sendfile sent first_name_column")
	}
	if request.Fields["has_header_row"] != "true" || request.Fields["remove_duplicate"] != "true" {
		t.Fatalf("%#v", request.Fields)
	}

	fake.clear()
	status, err := BulkValidationFileStatus("file-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.FileId != "file-key" {
		t.Fatalf("status id %s", status.FileId)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/v2/filestatus" || request.Fields["file_id"] != "file-1" {
		t.Fatalf("%+v", request)
	}
	fake.clear()
	if _, err = AiScoringFileStatus("file-2"); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/v2/scoring/filestatus" {
		t.Fatal(onlyRequest(t, fake).Path)
	}
	fake.clear()
	if _, err = BulkValidationFileDelete("file-1"); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/v2/deletefile" {
		t.Fatal("delete path")
	}
	fake.clear()
	if _, err = AiScoringFileDelete("file-2"); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/v2/scoring/deletefile" {
		t.Fatal("scoring delete path")
	}

	fake.clear()
	var saved bytes.Buffer
	download := DownloadTypePhase2
	activity := true
	err = BulkValidationResultWithOptions("file-9", &saved, &GetFileOptions{DownloadType: &download, ActivityData: &activity})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saved.String(), "file-key@example.com") || strings.Contains(saved.String(), "<html") {
		t.Fatalf("saved %q", saved.String())
	}
	request = onlyRequest(t, fake)
	if request.Path != "/v2/getfile" || request.Fields["download_type"] != "phase_2" || request.Fields["activity_data"] != "true" {
		t.Fatalf("%#v", request.Fields)
	}
	fake.clear()
	saved.Reset()
	phase1 := DownloadTypePhase1
	err = AiScoringResultWithOptions("file-8", &saved, &GetFileOptions{DownloadType: &phase1, ActivityData: &activity})
	if err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/v2/scoring/getfile" || request.Fields["download_type"] != "phase_1" {
		t.Fatalf("%#v", request.Fields)
	}
	if _, ok := request.Fields["activity_data"]; ok {
		t.Fatal("scoring getfile sent activity_data")
	}
}

func TestOfflineUsageActivityScoreFinder(t *testing.T) {
	fake := useFake(t)
	Initialize("usage-key")
	start := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2018, 1, 31, 0, 0, 0, 0, time.UTC)
	usage, err := GetApiUsage(start, end)
	if err != nil || usage.Total != 1 {
		t.Fatal(err, usage)
	}
	request := onlyRequest(t, fake)
	if request.Path != "/v2/getapiusage" || request.Fields["start_date"] != "2018-01-01" || request.Fields["end_date"] != "2018-01-31" {
		t.Fatalf("%#v", request.Fields)
	}

	fake.clear()
	activity, err := GetActivityData("person@example.com")
	if err != nil || !activity.Found {
		t.Fatal(err, activity)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/v2/activity" || request.Fields["email"] != "person@example.com" {
		t.Fatalf("%#v", request.Fields)
	}

	fake.clear()
	score, err := ScoreEmail("person@example.com")
	if err != nil || score.Email != "usage-key" {
		t.Fatal(err, score)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodGet || request.Path != "/v2/scoring" || request.Host != "api.zerobounce.net" {
		t.Fatalf("%+v", request)
	}
	fake.clear()
	if _, err = ScoreEmailPost("person@example.com"); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodPost || request.Path != "/v2/scoring" || request.Fields["email"] != "person@example.com" {
		t.Fatalf("%+v %#v", request, request.Fields)
	}

	fake.clear()
	found, err := FindEmailByDomainFirstMiddleLastName("example.com", "Ada", "M", "Lovelace")
	if err != nil || found.Email != "usage-key" {
		t.Fatal(err, found)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/v2/guessformat" || request.Fields["domain"] != "example.com" || request.Fields["first_name"] != "Ada" || request.Fields["middle_name"] != "M" || request.Fields["last_name"] != "Lovelace" {
		t.Fatalf("%#v", request.Fields)
	}
	if _, ok := request.Fields["company_name"]; ok {
		t.Fatal("domain finder sent company_name")
	}
	fake.clear()
	if _, err = FindEmailByCompanyFirstName("ZeroBounce", "Ada"); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Fields["company_name"] != "ZeroBounce" || request.Fields["first_name"] != "Ada" {
		t.Fatalf("%#v", request.Fields)
	}
	if _, ok := request.Fields["domain"]; ok {
		t.Fatal("company finder sent domain")
	}
	if _, ok := request.Fields["last_name"]; ok {
		t.Fatal("unset last_name was sent")
	}

	fake.clear()
	domain, err := DomainSearchByDomain("example.com")
	if err != nil || domain.Domain != "usage-key" {
		t.Fatal(err, domain)
	}
	if _, ok := onlyRequest(t, fake).Fields["company_name"]; ok {
		t.Fatal("domain search sent company_name")
	}
	fake.clear()
	if _, err = DomainSearchByCompanyName("ZeroBounce"); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Fields["company_name"] != "ZeroBounce" {
		t.Fatalf("%#v", request.Fields)
	}
	if _, ok := request.Fields["domain"]; ok {
		t.Fatal("company domain search sent domain")
	}
}

func TestOfflinePriority3(t *testing.T) {
	fake := useFake(t)
	Initialize("p3-key")
	_, err := BulkEmailFinderSubmit(EmailFinderCsvFile{
		File:           strings.NewReader("example.com\n"),
		FileName:       "finder.csv",
		HasHeaderRow:   false,
		DomainColumn:   1,
		FullNameColumn: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := onlyRequest(t, fake)
	if request.Host != "bulkapi.zerobounce.net" || request.Path != "/email-finder/sendfile" {
		t.Fatalf("%+v", request)
	}
	if request.Fields["domain_column"] != "1" || request.Fields["has_header_row"] != "false" {
		t.Fatalf("%#v", request.Fields)
	}
	if request.Fields["full_name_column"] != "2" {
		t.Fatalf("%#v", request.Fields)
	}
	if _, ok := request.Fields["first_name_column"]; ok {
		t.Fatal("unset first_name_column was sent")
	}

	fake.clear()
	if _, err = BulkEmailFinderFileStatus("fid"); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/email-finder/filestatus" {
		t.Fatal("finder status path")
	}
	fake.clear()
	var saved bytes.Buffer
	if err = BulkEmailFinderResult("fid", &saved); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/email-finder/getfile" {
		t.Fatal("finder getfile path")
	}
	fake.clear()
	if _, err = BulkEmailFinderFileDelete("fid"); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/email-finder/deletefile" {
		t.Fatal("finder delete path")
	}

	fake.clear()
	if _, err = BulkDomainSearchSubmit(DomainSearchCsvFile{
		File:         strings.NewReader("example.com\n"),
		FileName:     "domains.csv",
		HasHeaderRow: false,
		DomainColumn: 2,
	}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/domain-search/sendfile" || request.Fields["domain_column"] != "2" || request.Fields["has_header_row"] != "false" {
		t.Fatalf("%+v %#v", request, request.Fields)
	}
	fake.clear()
	if _, err = BulkDomainSearchFileStatus("fid"); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/domain-search/filestatus" {
		t.Fatal("domain status")
	}
	fake.clear()
	saved.Reset()
	if err = BulkDomainSearchResult("fid", &saved); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/domain-search/getfile" {
		t.Fatal("domain getfile")
	}
	fake.clear()
	if _, err = BulkDomainSearchFileDelete("fid"); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/domain-search/deletefile" {
		t.Fatal("domain delete")
	}

	fake.clear()
	if _, err = ListEvaluatorSubmit(ListEvaluatorCsvFile{
		File:               strings.NewReader("a@example.com\n"),
		FileName:           "list.csv",
		EmailAddressColumn: 1,
	}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/v2/listevaluator/" || request.Fields["email_address_column"] != "1" {
		t.Fatalf("%+v %#v", request, request.Fields)
	}
	fake.clear()
	result, err := GetListEvaluatorResult("fid")
	if err != nil {
		t.Fatal(err)
	}
	if result["echo_api_key"] != "p3-key" {
		t.Fatalf("%#v", result)
	}
	if onlyRequest(t, fake).Path != "/v2/listevaluator/fid/" {
		t.Fatal("evaluator result path")
	}
	fake.clear()
	// LIVE: this takes the evaluator's file_id as a form field, not an upload.
	if _, err = SendEvaluatedFile("eval-file-1"); err != nil {
		t.Fatal(err)
	}
	evaluated := onlyRequest(t, fake)
	if evaluated.Path != "/v2/send-evaluated-file/" {
		t.Fatal("evaluated path")
	}
	if evaluated.Fields["file_id"] != "eval-file-1" {
		t.Fatalf("file_id not sent: %+v", evaluated.Fields)
	}

	fake.clear()
	if _, err = AddFilter("exclude", "domain", "example.com"); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodPost || request.Host != "api.zerobounce.net" || request.Path != "/v2/filters/add" {
		t.Fatalf("%+v", request)
	}
	if request.Fields["rule"] != "exclude" || request.Fields["target"] != "domain" || request.Fields["value"] != "example.com" {
		t.Fatalf("%#v", request.Fields)
	}
	fake.clear()
	raw, err := ListFilters(false)
	if err != nil || !strings.Contains(string(raw), "p3-key") {
		t.Fatal(err, string(raw))
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodGet || request.Path != "/v2/filters/list" {
		t.Fatalf("%+v", request)
	}
	fake.clear()
	if _, err = ListFilters(true); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Method != http.MethodPost {
		t.Fatal("list filters post")
	}
	fake.clear()
	if _, err = DeleteFilter("exclude", "domain", "example.com"); err != nil {
		t.Fatal(err)
	}
	if onlyRequest(t, fake).Path != "/v2/filters/delete" {
		t.Fatal("delete filter path")
	}

	fake.clear()
	header := false
	remove := true
	column := 0
	_, err = ValidateS3File(ValidateS3FileOptions{
		FileName:           "inbox.csv",
		EmailAddressColumn: &column,
		HasHeaderRow:       &header,
		RemoveDuplicate:    &remove,
		ReturnURL:          "https://example.com/done",
	})
	if err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Method != http.MethodPost || request.Path != "/validateS3file" {
		t.Fatalf("%+v", request)
	}
	if request.Fields["file_name"] != "inbox.csv" || request.Fields["email_address_column"] != "0" {
		t.Fatalf("%#v", request.Fields)
	}
	if request.Fields["has_header_row"] != "false" || request.Fields["return_url"] != "https://example.com/done" {
		t.Fatalf("%#v", request.Fields)
	}
	// The deprecated RemoveDuplicate now feeds the one documented field, and
	// the misspelled twin is no longer on the wire.
	if request.Fields["remove_duplicates"] != "true" {
		t.Fatalf("%#v", request.Fields)
	}
	if _, ok := request.Fields["remove_duplicate"]; ok {
		t.Fatalf("stray remove_duplicate sent: %#v", request.Fields)
	}
	if _, ok := request.Fields["first_name_column"]; ok {
		t.Fatal("unset first_name_column sent")
	}
	fake.clear()
	if _, err = ScoreS3File(ScoreS3FileOptions{
		FileName:  "inbox.csv",
		ReturnURL: "https://example.com/done",
	}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/scoreS3file" {
		t.Fatal(request.Path)
	}
	if request.Fields["return_url"] != "https://example.com/done" {
		t.Fatalf("%#v", request.Fields)
	}
	if _, ok := request.Fields["email_address_column"]; ok {
		t.Fatal("unset email column sent")
	}
	for _, absent := range []string{"remove_duplicate", "remove_duplicates", "has_header_row"} {
		if _, ok := request.Fields[absent]; ok {
			t.Fatalf("unset %s sent: %#v", absent, request.Fields)
		}
	}
	fake.clear()
	if _, err = ExportS3File("fid", "bucket-out"); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/exportS3file" || request.Fields["file_id"] != "fid" || request.Fields["export_bucket"] != "bucket-out" {
		t.Fatalf("%#v", request.Fields)
	}
	fake.clear()
	files, err := ListS3Files(ListS3FilesOptions{})
	if err != nil || len(files) != 1 || files[0].Name != "p3-key" {
		t.Fatal(err, files)
	}
	request = onlyRequest(t, fake)
	if request.Path != "/listS3files" {
		t.Fatal(request.Path)
	}
	if _, ok := request.Fields["bucket"]; ok {
		t.Fatal("unset bucket sent")
	}
	if _, ok := request.Fields["search"]; ok {
		t.Fatal("unset search sent")
	}
	fake.clear()
	if _, err = ListS3Files(ListS3FilesOptions{Bucket: "inbox", Search: "csv"}); err != nil {
		t.Fatal(err)
	}
	request = onlyRequest(t, fake)
	if request.Fields["bucket"] != "inbox" || request.Fields["search"] != "csv" {
		t.Fatalf("%#v", request.Fields)
	}
}

func TestOfflineRejectsInvalidArgumentsBeforeRequest(t *testing.T) {
	fake := useFake(t)
	Initialize("")
	if _, err := GetCredits(); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("credits: %v", err)
	}
	if _, err := FindEmailByDomainFirstName("example.com", "Ada"); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("finder: %v", err)
	}
	if _, err := DomainSearchByCompanyName("ZeroBounce"); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("domain: %v", err)
	}
	if _, err := ListFilters(false); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("filters: %v", err)
	}
	if _, err := BulkEmailFinderFileDelete("fid"); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("delete: %v", err)
	}
	if _, err := BulkDomainSearchSubmit(DomainSearchCsvFile{File: strings.NewReader("a\n"), FileName: "d.csv", DomainColumn: 1}); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("domain file: %v", err)
	}
	if fake.size() != 0 {
		t.Fatalf("empty key sent %d requests", fake.size())
	}

	Initialize("keyed")
	if _, err := ValidateWithOptions("a@example.com", ValidateOptions{Timeout: "2"}); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("validate timeout: %v", err)
	}
	badTimeout := 9
	if _, err := ValidateBatchWithOptions([]EmailToValidate{{EmailAddress: "a@example.com"}}, &ValidateBatchOptions{Timeout: &badTimeout}); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("batch timeout: %v", err)
	}
	if _, err := BulkEmailFinderSubmit(EmailFinderCsvFile{File: strings.NewReader("a\n"), FileName: "f.csv", DomainColumn: 0}); err == nil || !strings.Contains(err.Error(), "domain_column") {
		t.Fatalf("domain column: %v", err)
	}
	// The finder needs a name to work from: one of the two name columns.
	if _, err := BulkEmailFinderSubmit(EmailFinderCsvFile{File: strings.NewReader("a\n"), FileName: "f.csv", DomainColumn: 1}); err == nil ||
		!strings.Contains(err.Error(), "first_name_column") || !strings.Contains(err.Error(), "full_name_column") {
		t.Fatalf("name columns: %v", err)
	}
	if _, err := ValidateS3File(ValidateS3FileOptions{}); err == nil || !strings.Contains(err.Error(), "file_name") {
		t.Fatalf("s3: %v", err)
	}
	// LIVE: both S3 jobs are rejected without return_url.
	if _, err := ValidateS3File(ValidateS3FileOptions{FileName: "a.csv"}); err == nil || !strings.Contains(err.Error(), "return_url") {
		t.Fatalf("s3 return_url: %v", err)
	}
	if _, err := ScoreS3File(ScoreS3FileOptions{}); err == nil || !strings.Contains(err.Error(), "file_name") {
		t.Fatalf("score s3: %v", err)
	}
	if _, err := ScoreS3File(ScoreS3FileOptions{FileName: "a.csv"}); err == nil || !strings.Contains(err.Error(), "return_url") {
		t.Fatalf("score s3 return_url: %v", err)
	}
	if fake.size() != 0 {
		t.Fatalf("invalid args sent %d requests", fake.size())
	}
}

func TestOfflineErrorBodiesStayShort(t *testing.T) {
	fake := useFake(t)
	Initialize("html-403-user")
	_, err := GetCredits()
	assertShortError(t, err)
	if !strings.Contains(err.Error(), "Access denied") {
		t.Fatal(err)
	}
	if len(htmlAccessDeniedPage()) < 100000 {
		t.Fatal("fixture is not large")
	}

	Initialize("html-200-user")
	_, err = Validate("a@example.com", "")
	assertShortError(t, err)
	if !strings.Contains(err.Error(), "Access denied") {
		t.Fatal(err)
	}

	Initialize("html-200-file")
	var saved bytes.Buffer
	err = BulkValidationResult("fid", &saved)
	assertShortError(t, err)
	if !strings.Contains(err.Error(), "Access denied") || saved.Len() != 0 {
		t.Fatalf("err=%v saved=%d", err, saved.Len())
	}
	saved.Reset()
	err = BulkEmailFinderResult("fid", &saved)
	assertShortError(t, err)
	if saved.Len() != 0 {
		t.Fatal("finder HTML was saved")
	}

	Initialize("cf-json-user")
	_, err = GetCredits()
	assertShortError(t, err)
	if strings.Contains(err.Error(), "developers.cloudflare.com") || !strings.Contains(err.Error(), "Access denied") {
		t.Fatal(err)
	}

	Initialize("empty-user")
	_, err = Validate("a@example.com", "")
	assertShortError(t, err)
	if !strings.Contains(err.Error(), "Empty response") {
		t.Fatal(err)
	}

	Initialize("malformed-user")
	_, err = Validate("a@example.com", "")
	assertShortError(t, err)
	if !strings.Contains(err.Error(), "not valid JSON") || strings.Contains(err.Error(), "<<<not-json>>>") {
		t.Fatal(err)
	}

	Initialize("double-error-user")
	_, err = GetActivityData("a@example.com")
	assertShortError(t, err)
	if err.Error() != "api_key is invalid" {
		t.Fatalf("double error = %q", err.Error())
	}

	if fake.size() == 0 {
		t.Fatal("expected requests")
	}
}

func TestOfflineBatchAccountAndPartial(t *testing.T) {
	fake := useFake(t)
	Initialize("batch-account-user")
	_, err := ValidateBatch([]EmailToValidate{{EmailAddress: "a@example.com"}})
	assertShortError(t, err)
	if !strings.Contains(err.Error(), "Invalid API Key") || !strings.Contains(err.Error(), "ran out of credits") {
		t.Fatal(err)
	}

	Initialize("batch-partial-user")
	response, err := ValidateBatch([]EmailToValidate{{EmailAddress: "good@example.com"}, {EmailAddress: "bad@"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.EmailBatch) != 1 || response.EmailBatch[0].Address != "good@example.com" {
		t.Fatalf("%+v", response.EmailBatch)
	}
	if len(response.Errors) != 1 || response.Errors[0].EmailAddress != "bad@" || response.Errors[0].Error != "Invalid email address" {
		t.Fatalf("%+v", response.Errors)
	}
	_ = fake
}

func TestOfflineRedirectIsNotFollowed(t *testing.T) {
	transport := &redirectTransport{}
	previous := http.DefaultTransport
	previousKey := API_KEY
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = previous
		API_KEY = previousKey
	})
	Initialize("redirect-key")
	_, err := GetCredits()
	if err == nil {
		t.Fatal("redirect was treated as success")
	}
	if transport.calls != 1 {
		t.Fatalf("redirect was followed, calls=%d hosts=%v", transport.calls, transport.hosts)
	}
	if len(transport.hosts) != 1 || transport.hosts[0] != "api.zerobounce.net" {
		t.Fatalf("hosts=%v", transport.hosts)
	}
}

type redirectTransport struct {
	calls int
	hosts []string
}

func (r *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls++
	r.hosts = append(r.hosts, req.URL.Host)
	header := make(http.Header)
	header.Set("Location", "https://evil.example/collect")
	return &http.Response{
		StatusCode: http.StatusFound,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

func TestOfflineNilFileRejectedBeforeRequest(t *testing.T) {
	fake := useFake(t)
	Initialize("keyed")
	_, err := BulkValidationSubmit(CsvFile{FileName: "a.csv", EmailAddressColumn: 1}, false)
	if err == nil || !strings.Contains(err.Error(), "file") {
		t.Fatalf("nil file: %v", err)
	}
	if _, err = SendEvaluatedFile("  "); err == nil || !strings.Contains(err.Error(), "file_id") {
		t.Fatalf("blank evaluated file id: %v", err)
	}
	if fake.size() != 0 {
		t.Fatalf("nil file sent %d requests", fake.size())
	}
}

func TestOfflineConcurrentClientsDoNotShareKeys(t *testing.T) {
	fake := useFake(t)
	const calls = 12
	echoes := make([]string, calls)
	var group sync.WaitGroup
	for i := 0; i < calls; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			key := "conc-" + strings.Repeat("0", 0) + itoa(i)
			client := New(key)
			response, err := client.Validate("person@example.com", "")
			if err != nil {
				echoes[i] = "ERROR " + err.Error()
				return
			}
			echoes[i] = response.Address
		}(i)
	}
	group.Wait()
	records := fake.take()
	if len(records) != calls {
		t.Fatalf("requests = %d", len(records))
	}
	seen := map[string]int{}
	for _, request := range records {
		seen[request.APIKey]++
		if !strings.HasPrefix(request.APIKey, "conc-") || request.Fields["api_key"] != request.APIKey {
			t.Fatalf("leaked key %+v", request)
		}
	}
	for i := 0; i < calls; i++ {
		key := "conc-" + itoa(i)
		if seen[key] != 1 || echoes[i] != key {
			t.Fatalf("key %s seen %d echo %s", key, seen[key], echoes[i])
		}
	}
}

func itoa(value int) string {
	return strings.TrimPrefix(strings.ReplaceAll(jsonNumber(value), " ", ""), "+")
}

func jsonNumber(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
