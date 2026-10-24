package zerobouncego

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/joho/godotenv"
)

type ZbApiURL int

// CONSTANTS
const (
	ENDPOINT_CREDITS        = "/getcredits"
	ENDPOINT_ACTIVITY_DATA  = "/activity"
	ENDPOINT_SCORING        = "/scoring"
	ENDPOINT_VALIDATE       = "/validate"
	ENDPOINT_API_USAGE      = "/getapiusage"
	ENDPOINT_BATCH_VALIDATE = "/validatebatch"
	ENDPOINT_FILE_SEND      = "/sendfile"
	ENDPOINT_FILE_STATUS    = "/filestatus"
	ENDPOINT_FILE_RESULT    = "/getfile" // Content-type: application/octet-stream
	ENDPOINT_FILE_DELETE    = "/deletefile"
	ENDPOINT_SCORING_SEND   = "/scoring/sendfile"
	ENDPOINT_SCORING_STATUS = "/scoring/filestatus"
	ENDPOINT_SCORING_RESULT = "/scoring/getfile" // Content-type: application/octet-stream
	ENDPOINT_SCORING_DELETE = "/scoring/deletefile"
	ENDPOINT_EMAIL_FINDER   = "/guessformat"
	SANDBOX_IP              = "99.110.204.1"
)

const (
	DATE_TIME_FORMAT = "2006-01-02 15:04:05"
	DATE_ONLY_FORMAT = "2006-01-02"
	httpTimeout      = 120 * time.Second
)

// validation statuses
const (
	S_VALID       = "valid"
	S_INVALID     = "invalid"
	S_CATCH_ALL   = "catch-all"
	S_UNKNOWN     = "unknown"
	S_SPAMTRAP    = "spamtrap"
	S_ABUSE       = "abuse"
	S_DO_NOT_MAIL = "do_not_mail"
)

// validation sub statuses
const (
	SS_ANTISPAM_SYSTEM             = "antispam_system"
	SS_GREYLISTED                  = "greylisted"
	SS_MAIL_SERVER_TEMPORARY_ERROR = "mail_server_temporary_error"
	SS_FORCIBLE_DISCONNECT         = "forcible_disconnect"
	SS_MAIL_SERVER_DID_NOT_RESPOND = "mail_server_did_not_respond"
	SS_TIMEOUT_EXCEEDED            = "timeout_exceeded"
	SS_FAILED_SMTP_CONNECTION      = "failed_smtp_connection"
	SS_MAILBOX_QUOTA_EXCEEDED      = "mailbox_quota_exceeded"
	SS_EXCEPTION_OCCURRED          = "exception_occurred"
	SS_POSSIBLE_TRAP               = "possible_trap"
	SS_ROLE_BASED                  = "role_based"
	SS_GLOBAL_SUPPRESSION          = "global_suppression"
	SS_MAILBOX_NOT_FOUND           = "mailbox_not_found"
	SS_NO_DNS_ENTRIES              = "no_dns_entries"
	SS_FAILED_SYNTAX_CHECK         = "failed_syntax_check"
	SS_POSSIBLE_TYPO               = "possible_typo"
	SS_UNROUTABLE_IP_ADDRESS       = "unroutable_ip_address"
	SS_LEADING_PERIOD_REMOVED      = "leading_period_removed"
	SS_DOES_NOT_ACCEPT_MAIL        = "does_not_accept_mail"
	SS_ALIAS_ADDRESS               = "alias_address"
	SS_ROLE_BASED_CATCH_ALL        = "role_based_catch_all"
	SS_ROLE_BASED_ACCEPT_ALL       = "role_based_accept_all"
	SS_DISPOSABLE                  = "disposable"
	SS_TOXIC                       = "toxic"
	SS_ACCEPT_ALL                  = "accept_all"
	SS_ALTERNATE                   = "alternate"
	SS_MX_FORWARD                  = "mx_forward"
	SS_BLOCKED                     = "blocked"
	SS_ALLOWED                     = "allowed"
	SS_GOLD                        = "gold"
	SS_AI_AGENT_MAILBOX            = "ai_agent_mailbox"
)

const (
	ZB_API_URL_DEFAULT ZbApiURL = iota
	ZB_API_URL_USA
	ZB_API_URL_EU
)

// APIResponse basis for api responses
type APIResponse interface{}

func Getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}

// getAPIKeyFromEnv returns the API key from environment (ZEROBOUNCE_API_KEY or legacy ZERO_BOUNCE_API_KEY).
func getAPIKeyFromEnv() string {
	if v := os.Getenv("ZEROBOUNCE_API_KEY"); v != "" {
		return v
	}
	return os.Getenv("ZERO_BOUNCE_API_KEY")
}

// VAR
var (
	// URI used to make requests to the ZeroBounce API
	URI      = Getenv(`ZERO_BOUNCE_URI`, `https://api.zerobounce.net/v2/`)
	BULK_URI = Getenv(`ZERO_BOUNCE_BULK_URI`, `https://bulkapi.zerobounce.net/v2/`)
	// LIST_EVALUATOR_URI and INBOX_PLACEMENT_URI are separate hosts, not paths
	// built from BULK_URI. SetURI does not touch them, so a test that only
	// overrides URI and BULK_URI would still reach the live API.
	LIST_EVALUATOR_URI  = Getenv(`ZERO_BOUNCE_LIST_EVALUATOR_URI`, `https://bulkapi.zerobounce.net/v2/listevaluator`)
	INBOX_PLACEMENT_URI = Getenv(`ZERO_BOUNCE_INBOX_PLACEMENT_URI`, `https://bulkapi.zerobounce.net/api/ipt`)

	// API_KEY the API key used in order to make the requests (from ZEROBOUNCE_API_KEY or ZERO_BOUNCE_API_KEY)
	API_KEY string = getAPIKeyFromEnv()

	zbApiURLValue = map[ZbApiURL]string{
		ZB_API_URL_DEFAULT: "https://api.zerobounce.net/v2/",
		ZB_API_URL_USA:     "https://api-us.zerobounce.net/v2/",
		ZB_API_URL_EU:      "https://api-eu.zerobounce.net/v2/",
	}
)

// FUNCTIONS

// Initialize the API key explicitly
func Initialize(new_api_key_value string) {
	API_KEY = new_api_key_value
	URI = zbApiURLValue[ZB_API_URL_DEFAULT]
}

// Initialize the API key and the URI explicitly
func InitializeWithURI(new_api_key_value string, zbApiURL ZbApiURL) {
	API_KEY = new_api_key_value
	URI = zbApiURLValue[zbApiURL]
}

// SetApiKey set the API key explicitly
//
// Deprecated: Use Initialize methods
func SetApiKey(new_api_key_value string) {
	API_KEY = new_api_key_value
}

func requireHTTPS(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("URI must be an https:// URL")
	}
	return raw, nil
}

// Update URI, BULK_URI or both (will not be updated if empty strings are passed)
func SetURI(new_uri string, new_bulk_uri string) {
	if new_uri != "" {
		if validated, err := requireHTTPS(new_uri); err == nil {
			URI = validated
		}
	}
	if new_bulk_uri != "" {
		if validated, err := requireHTTPS(new_bulk_uri); err == nil {
			BULK_URI = validated
		}
	}
}

// LoadEnvFromFile provided that a .env file can be found where the
// program is running, load it, extract the API key and set it.
// Supports standard ZEROBOUNCE_API_KEY and legacy ZERO_BOUNCE_API_KEY.
func LoadEnvFromFile() bool {
	error_ := godotenv.Load(".env")
	if error_ != nil {
		fmt.Printf("The '.env' file was not found (%s). Continuing without it\n", error_.Error())
		return false
	}
	Initialize(getAPIKeyFromEnv())
	SetURI(os.Getenv("ZERO_BOUNCE_URI"), os.Getenv("ZERO_BOUNCE_BULK_URI"))
	return true
}

// PrepareURL prepares the URL for a get request by attaching both the API
// key and the given params
func PrepareURL(endpoint string, params url.Values) (string, error) {
	return prepareURL(packageConfig(), endpoint, params)
}

func prepareURL(cfg RequestConfig, endpoint string, params url.Values) (string, error) {
	if err := cfg.requireKey(); err != nil {
		return "", err
	}

	params.Set("api_key", cfg.APIKey)

	finalURL, err := url.JoinPath(cfg.URI, endpoint)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s?%s", finalURL, params.Encode()), nil
}

// handleErrorPayload turns a non-success HTTP payload into a short error.
func handleErrorPayload(response *http.Response) error {
	defer response.Body.Close()
	return ErrorFromResponse(response)
}

// ErrorFromResponse turns an HTTP error body into a short error.
// The caller owns closing the body when it does not use handleErrorPayload.
func ErrorFromResponse(response *http.Response) error {
	responseBody, err := readLimitedBody(response.Body)
	if err != nil {
		if len(responseBody) > 0 && strings.Contains(err.Error(), "not valid JSON") {
			return err
		}
		return errors.New("server error: " + err.Error())
	}
	return errors.New(messageFromBody(response.StatusCode, response.Header.Get("Content-Type"), string(responseBody)))
}

func timedHTTPClient() *http.Client {
	c := *http.DefaultClient
	if c.Timeout == 0 {
		c.Timeout = httpTimeout
	}
	// A 3xx must not be followed. The API key is in the query string, and a
	// redirect could send it to another host.
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &c
}

// redactAPIKey masks the api_key query value of a request URL.
func redactAPIKey(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "(url redacted)"
	}
	query := parsed.Query()
	if _, found := query["api_key"]; !found {
		return rawURL
	}
	query.Set("api_key", "REDACTED")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func httpGet(requestURL string) (*http.Response, error) {
	response, err := timedHTTPClient().Get(requestURL)
	var urlErr *url.Error
	if err != nil && errors.As(err, &urlErr) {
		urlErr.URL = redactAPIKey(urlErr.URL)
	}
	return response, err
}

// DoGetRequest does a GET request to the API
func DoGetRequest(requestURL string, object APIResponse) error {
	response, err := httpGet(requestURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := readLimitedBody(response.Body)
	if err != nil {
		if len(body) > 0 && strings.Contains(err.Error(), "not valid JSON") {
			return err
		}
		return err
	}
	return decodeJSONBody(response.StatusCode, response.Header.Get("Content-Type"), body, object)
}

// DoPostFormRequest POSTs application/x-www-form-urlencoded data to the main API.
func DoPostFormRequest(endpoint string, form url.Values, object APIResponse) error {
	return doPostFormRequest(packageConfig(), endpoint, form, object)
}

func doPostFormRequest(cfg RequestConfig, endpoint string, form url.Values, object APIResponse) error {
	if err := cfg.requireKey(); err != nil {
		return err
	}
	if form == nil {
		form = url.Values{}
	}
	form.Set("api_key", cfg.APIKey)

	urlToAccess, err := url.JoinPath(cfg.URI, endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL (%s) or endpoint (%s) value", cfg.URI, endpoint)
	}

	response, err := timedHTTPClient().PostForm(urlToAccess, form)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := readLimitedBody(response.Body)
	if err != nil {
		if len(body) > 0 && strings.Contains(err.Error(), "not valid JSON") {
			return err
		}
		return err
	}
	return decodeJSONBody(response.StatusCode, response.Header.Get("Content-Type"), body, object)
}

// TESTING
type SingleTest struct {
	Email     string
	Status    string
	SubStatus string
	FreeEmail bool
}

var emailsToValidate = []SingleTest{
	{Email: "disposable@example.com", Status: "do_not_mail", SubStatus: "disposable"},
	{Email: "invalid@example.com", Status: "invalid", SubStatus: "mailbox_not_found"},
	{Email: "valid@example.com", Status: "valid", SubStatus: ""},
	{Email: "toxic@example.com", Status: "do_not_mail", SubStatus: "toxic"},
	{Email: "donotmail@example.com", Status: "do_not_mail", SubStatus: "role_based"},
	{Email: "spamtrap@example.com", Status: "spamtrap", SubStatus: ""},
	{Email: "abuse@example.com", Status: "abuse", SubStatus: ""},
	{Email: "catch_all@example.com", Status: "catch-all", SubStatus: ""},
	{Email: "antispam_system@example.com", Status: "unknown", SubStatus: "antispam_system"},
	{Email: "does_not_accept_mail@example.com", Status: "invalid", SubStatus: "does_not_accept_mail"},
	{Email: "exception_occurred@example.com", Status: "unknown", SubStatus: "exception_occurred"},
	{Email: "failed_smtp_connection@example.com", Status: "unknown", SubStatus: "failed_smtp_connection"},
	{Email: "failed_syntax_check@example.com", Status: "invalid", SubStatus: "failed_syntax_check"},
	{Email: "forcible_disconnect@example.com", Status: "unknown", SubStatus: "forcible_disconnect"},
	{Email: "global_suppression@example.com", Status: "do_not_mail", SubStatus: "global_suppression"},
	{Email: "greylisted@example.com", Status: "unknown", SubStatus: "greylisted"},
	{Email: "leading_period_removed@example.com", Status: "valid", SubStatus: "leading_period_removed"},
	{Email: "mail_server_did_not_respond@example.com", Status: "unknown", SubStatus: "mail_server_did_not_respond"},
	{Email: "mail_server_temporary_error@example.com", Status: "unknown", SubStatus: "mail_server_temporary_error"},
	{Email: "unknown@example.com", Status: "unknown", SubStatus: "mail_server_temporary_error"},
	{Email: "mailbox_quota_exceeded@example.com", Status: "invalid", SubStatus: "mailbox_quota_exceeded"},
	{Email: "mailbox_not_found@example.com", Status: "invalid", SubStatus: "mailbox_not_found"},
	{Email: "no_dns_entries@example.com", Status: "invalid", SubStatus: "no_dns_entries"},
	{Email: "possible_trap@example.com", Status: "do_not_mail", SubStatus: "possible_trap"},
	{Email: "possible_typo@example.com", Status: "invalid", SubStatus: "possible_typo"},
	{Email: "role_based@example.com", Status: "do_not_mail", SubStatus: "role_based"},
	{Email: "timeout_exceeded@example.com", Status: "unknown", SubStatus: "timeout_exceeded"},
	{Email: "unroutable_ip_address@example.com", Status: "invalid", SubStatus: "unroutable_ip_address"},
	{Email: "free_email@example.com", Status: "valid", SubStatus: "", FreeEmail: true},
	{Email: "gold@example.com", Status: "valid", SubStatus: "gold"},
	{Email: "role_based_accept_all@example.com", Status: "valid", SubStatus: "role_based_accept_all"},
	{Email: "role_based_catch_all@example.com", Status: "do_not_mail", SubStatus: "role_based_catch_all"},
}

// variables used for file-related unit tests
const (
	sample_date_time     = "2023-01-12T13:00:00Z"
	sample_file_contents = "valid@example.com\ninvalid@example.com\ntoxic@example.com\n"
	sample_error_message = "error message"
	sample_error_400     = `{
		"error": "` + sample_error_message + `"
	}`
	file_name_400          = "filename_400.csv"
	send_file_response_400 = `{
		"success": false,
		"error_message": "` + sample_error_message + `",
		"message": "` + sample_error_message + `"
	}`
	file_name_200          = "filename_200.csv"
	testing_file_id        = "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
	send_file_response_200 = `
	{
		"success": true,
		"message": "File Accepted",
		"file_name": "` + file_name_200 + `",
		"file_id": "` + testing_file_id + `"
	}`
)

// mockErrorResponse - mock http library to return error for given endpoint
func mockErrorResponse(method, endpoint string) {
	httpmock.RegisterResponder(
		method,
		`=~^(.*)`+endpoint+`(.*)\z`,
		func(r *http.Request) (*http.Response, error) { return nil, errors.New(sample_error_message) },
	)
}

// mockBadRequestResponse - mock http library to return 400 response for given endpoint
func mockBadRequestResponse(method, endpoint string) {
	httpmock.RegisterResponder(
		method,
		`=~^(.*)`+endpoint+`(.*)\z`,
		httpmock.NewStringResponder(400, sample_error_400),
	)
}

// mockBadRequestResponse - mock http library to return 200Ok response for given endpoint
// returning given endpoint
func mockOkResponse(method, endpoint, content string) {
	httpmock.RegisterResponder(
		method,
		`=~^(.*)`+endpoint+`(.*)\z`,
		httpmock.NewStringResponder(200, content),
	)
}

// testingCsvFileOk - sample csv file, used in testing
func testingCsvFileOk() CsvFile {
	return CsvFile{
		File:               strings.NewReader(sample_file_contents),
		FileName:           file_name_200,
		HasHeaderRow:       true,
		EmailAddressColumn: 1,
	}
}
