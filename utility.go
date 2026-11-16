package zerobouncego

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

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
	// built from BULK_URI. SetURI rebases them onto the bulk host it is given,
	// so a test that overrides URI and BULK_URI no longer reaches the live API
	// here. Set them through the environment to point them somewhere else;
	// SetURI then leaves them alone.
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

// Initialize the API key explicitly.
//
// It also resets URI to the default. ZERO_BOUNCE_URI is that default when it
// is set: Initialize used to overwrite URI with the hardcoded production host,
// which silently discarded an environment override and sent later calls to
// production.
func Initialize(new_api_key_value string) {
	API_KEY = new_api_key_value
	URI = Getenv(`ZERO_BOUNCE_URI`, zbApiURLValue[ZB_API_URL_DEFAULT])
}

// Initialize the API key and the URI explicitly
func InitializeWithURI(new_api_key_value string, zbApiURL ZbApiURL) error {
	uri, ok := zbApiURLValue[zbApiURL]
	if !ok || uri == "" {
		return fmt.Errorf("invalid parameter: api url")
	}
	API_KEY = new_api_key_value
	URI = uri
	return nil
}

// SetApiKey set the API key explicitly
//
// Deprecated: Use Initialize methods
func SetApiKey(new_api_key_value string) {
	API_KEY = new_api_key_value
}

// errPlainHTTP refuses a request URL that is not https:// (or http:// to a
// loopback host, for tests).
var errPlainHTTP = errors.New("refusing to send the API key to a URL that is not https:// (http:// is allowed only for 127.0.0.1, localhost and [::1])")

// errBadURL refuses a request URL that names the wrong host, no host, or a
// host that cannot be reached, so the key would go elsewhere or nowhere.
var errBadURL = errors.New("refusing to send the API key to a URL that is not a valid https:// URL (a host, no userinfo, no query or fragment, no whitespace or control characters, a valid port)")

var hostLabels = regexp.MustCompile(`^([A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\.)*[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\.?$`)

// unsafeURLRune reports whitespace, control, format characters and backslash.
func unsafeURLRune(r rune) bool {
	return r == '\\' || unicode.IsSpace(r) || unicode.IsControl(r) ||
		unicode.In(r, unicode.Cf, unicode.Zs, unicode.Zl, unicode.Zp) || r == 0x180e
}

// validURLHost reports whether host (with its optional port) is a DNS name or
// an IP address, with a valid port.
func validURLHost(parsed *url.URL) bool {
	host := parsed.Host
	if host == "" || strings.HasSuffix(host, ":") || strings.ContainsFunc(host, unsafeURLRune) || strings.Contains(host, "@") {
		return false
	}
	if port := parsed.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
		for _, c := range port {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	name := parsed.Hostname()
	if name == "" {
		return false
	}
	if strings.HasPrefix(host, "[") {
		return strings.Contains(name, ":") && !strings.Contains(name, "%") && net.ParseIP(name) != nil
	}
	return len(name) <= 253 && hostLabels.MatchString(name)
}

// checkRequestURL accepts https:// and, for tests, http:// to 127.0.0.1,
// localhost or [::1], when the URL also has a valid host, a valid port and no
// userinfo (https://api.zerobounce.net@evil.example sends the key to
// evil.example).
func checkRequestURL(parsed *url.URL) error {
	if parsed == nil {
		return errPlainHTTP
	}
	secure := strings.EqualFold(parsed.Scheme, "https")
	if !secure {
		if !strings.EqualFold(parsed.Scheme, "http") {
			return errPlainHTTP
		}
		switch strings.ToLower(parsed.Hostname()) {
		case "127.0.0.1", "localhost", "::1":
		default:
			return errPlainHTTP
		}
	}
	if parsed.User != nil || parsed.Opaque != "" || !validURLHost(parsed) {
		return errBadURL
	}
	// A request URL carries no fragment, and its query must parse: a second
	// '?' or a malformed pair is what a base URL with a query leaves behind
	// ("…/getcredits?x=1?api_key=KEY").
	if parsed.Fragment != "" || parsed.RawFragment != "" {
		return errBadURL
	}
	if parsed.RawQuery != "" {
		if strings.Contains(parsed.RawQuery, "?") {
			return errBadURL
		}
		if _, err := url.ParseQuery(parsed.RawQuery); err != nil {
			return errBadURL
		}
	}
	return nil
}

// checkBaseURLText parses a configured base URL and applies checkRequestURL
// to it, after refusing whitespace, control characters and backslashes in the
// text itself (net/url would escape or ignore some of them).
func checkBaseURLText(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return errBadURL
	}
	if err := checkRequestURL(parsed); err != nil {
		return err
	}
	if strings.ContainsFunc(raw, unsafeURLRune) {
		return errBadURL
	}
	// A base URL is a scheme, a host and a path. The SDK appends its own
	// query, so a base with a query, a bare '?' or a fragment would send the
	// request somewhere else or put the API key in the wrong place.
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawFragment != "" ||
		strings.ContainsAny(raw, "?#") {
		return errBadURL
	}
	return nil
}

// requireHTTPSBase is checkBaseURLText for a base that must be https://
// (no loopback http exception), as the Warmup client's.
func requireHTTPSBase(raw string) error {
	if !strings.HasPrefix(strings.ToLower(raw), "https://") {
		return errPlainHTTP
	}
	return checkBaseURLText(raw)
}

// joinBaseURL is url.JoinPath for a base URL that is checked first: a base of
// "https://" would otherwise join into "https://getcredits", a different host.
// A base that fails the check yields a URL the transport refuses (the joined
// one when its scheme is refused, else one without a host), so
// the caller's error handling is unchanged and nothing is sent.
func joinBaseURL(base string, elem ...string) (string, error) {
	joined, err := url.JoinPath(base, elem...)
	if checkBaseURLText(base) == nil {
		return joined, err
	}
	if err == nil {
		if parsed, perr := url.Parse(joined); perr == nil && checkRequestURL(parsed) == errPlainHTTP {
			return joined, nil
		}
	}
	return "https:///", nil
}

func requireHTTPS(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if err := checkBaseURLText(raw); err != nil {
		return "", err
	}
	return raw, nil
}

// errHTTP2Debug is the error while GODEBUG turns on net/http's HTTP/2
// frame logging, which writes the :path (with api_key) and the API-Key header.
var errHTTP2Debug = errors.New("HTTP/2 debug logging (GODEBUG http2debug) is on and would write your API key to the log; remove http2debug from GODEBUG")

// http2DebugEnabledAtInit is GODEBUG as this process started. net/http reads
// that once, so clearing GODEBUG later does not stop the frame log.
var http2DebugEnabledAtInit = godebugEnablesHTTP2FrameLog(os.Getenv("GODEBUG"))

// http2DebugLogsRequests reports whether GODEBUG, as the process started or
// as it is now, turns on the net/http HTTP/2 frame log. That log writes the
// :path, and the API key is in it.
func http2DebugLogsRequests() bool {
	return http2DebugEnabledAtInit || godebugEnablesHTTP2FrameLog(os.Getenv("GODEBUG"))
}

// godebugEnablesHTTP2FrameLog reports whether a GODEBUG value turns on
// net/http's HTTP/2 frame log. net/http matches the substrings http2debug=1
// and http2debug=2 anywhere in the value, so a later http2debug=0 does not
// turn them off. Any other non-zero http2debug setting is refused too.
func godebugEnablesHTTP2FrameLog(value string) bool {
	if strings.Contains(value, "http2debug=1") || strings.Contains(value, "http2debug=2") {
		return true
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "http2debug=") {
			continue
		}
		setting := strings.TrimSpace(strings.TrimPrefix(part, "http2debug="))
		if setting != "" && setting != "0" {
			return true
		}
	}
	return false
}

// SetURI updates URI, BULK_URI or both. An empty string keeps the current
// value.
//
// Both must be https:// URLs. A malformed or non-https value is returned as an
// error and leaves every base URL untouched. The error used to be swallowed,
// so a caller who redirected to a test host and mistyped it kept hitting the
// old host without being told.
//
// A new bulk URI also rebases LIST_EVALUATOR_URI and INBOX_PLACEMENT_URI onto
// that host. They are separate base URLs rather than paths built from
// BULK_URI, so the list evaluator and the Inbox Placement Tester used to keep
// reaching production after a redirect. A value set through
// ZERO_BOUNCE_LIST_EVALUATOR_URI or ZERO_BOUNCE_INBOX_PLACEMENT_URI is an
// explicit choice of host and is left alone.
func SetURI(new_uri string, new_bulk_uri string) error {
	validatedURI, err := requireHTTPS(new_uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	validatedBulkURI, err := requireHTTPS(new_bulk_uri)
	if err != nil {
		return fmt.Errorf("invalid bulk URI: %w", err)
	}

	if validatedURI != "" {
		URI = validatedURI
	}
	if validatedBulkURI != "" {
		BULK_URI = validatedBulkURI
		root := bulkAPIRoot()
		if os.Getenv(`ZERO_BOUNCE_LIST_EVALUATOR_URI`) == "" {
			LIST_EVALUATOR_URI = root + "/v2/listevaluator"
		}
		if os.Getenv(`ZERO_BOUNCE_INBOX_PLACEMENT_URI`) == "" {
			INBOX_PLACEMENT_URI = root + "/api/ipt"
		}
	}
	return nil
}

// LoadEnvFromFile provided that a .env file can be found where the
// program is running, load it, extract the API key and set it.
// Supports standard ZEROBOUNCE_API_KEY and legacy ZERO_BOUNCE_API_KEY.
func LoadEnvFromFile() bool {
	error_ := godotenv.Load(".env")
	if error_ != nil {
		// godotenv quotes the offending line, API key included. The message
		// stays fixed, and a missing file is not the same as a bad line.
		if os.IsNotExist(error_) {
			fmt.Printf("The '.env' file was not found. Continuing without it\n")
		} else {
			fmt.Printf("The '.env' file could not be read. Continuing without it\n")
		}
		return false
	}
	// Initialize copies ZERO_BOUNCE_URI with no check. If that URL is
	// rejected, put the previous one back. The parse error quotes the whole
	// URL, and that URL can contain the API key, so the message stays fixed.
	previousURI := URI
	Initialize(getAPIKeyFromEnv())
	if err := SetURI(os.Getenv("ZERO_BOUNCE_URI"), os.Getenv("ZERO_BOUNCE_BULK_URI")); err != nil {
		URI = previousURI
		fmt.Printf("The '.env' base URL was not applied. Continuing with the current one\n")
	}
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

	finalURL, err := joinBaseURL(cfg.URI, endpoint)
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

// ErrorFromResponse turns an HTTP error response into a short error. For a
// status above 299 it is a *ZBApiError carrying that status.
// The caller owns closing the body when it does not use handleErrorPayload.
func ErrorFromResponse(response *http.Response) error {
	responseBody, err := readLimitedBody(response.Body, response.StatusCode)
	if err != nil {
		var apiErr *ZBApiError
		if errors.As(err, &apiErr) {
			return err
		}
		// A body that could not be read is a transport failure: on a 2xx it
		// carries no status.
		if response.StatusCode < 300 {
			return errors.New(redactKeys("server error: " + err.Error()))
		}
		return statusError(response.StatusCode, "server error: "+err.Error())
	}
	return statusError(response.StatusCode, messageFromBody(response.StatusCode, response.Header.Get("Content-Type"), string(responseBody)))
}

// httpsOnlyTransport checks every request URL before it is sent, whichever
// base URL it was built from: URI, BULK_URI, LIST_EVALUATOR_URI,
// INBOX_PLACEMENT_URI (set directly or through ZERO_BOUNCE_*_URI), a
// Client's URI and BulkURI, or the Warmup client's base.
type httpsOnlyTransport struct {
	base http.RoundTripper
}

func (t httpsOnlyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	err := checkRequestURL(request.URL)
	if err == nil && http2DebugLogsRequests() {
		err = errHTTP2Debug
	}
	if err != nil {
		if request.Body != nil {
			request.Body.Close()
		}
		return nil, err
	}
	base := t.base
	if base == nil {
		base = sdkTransport()
	}
	return base.RoundTrip(request)
}

var (
	// builtinDefaultTransport is http.DefaultTransport as the program started.
	builtinDefaultTransport = http.DefaultTransport
	sdkTransportOnce        sync.Once
	sdkTransportValue       http.RoundTripper
)

// sdkTransport is the transport every request goes through: the SDK's own
// *http.Transport, cloned from the standard one (proxy from the environment,
// TLS verification on, the standard dial and TLS timeouts) so a program
// changing http.DefaultTransport's settings does not change the SDK's. If a
// program or test replaced http.DefaultTransport with its own round tripper
// (a fake, httpmock), that one is used, looked up per request.
func sdkTransport() http.RoundTripper {
	if current := http.DefaultTransport; current != builtinDefaultTransport {
		return current
	}
	sdkTransportOnce.Do(func() {
		if standard, ok := builtinDefaultTransport.(*http.Transport); ok {
			sdkTransportValue = standard.Clone()
			return
		}
		sdkTransportValue = &http.Transport{Proxy: http.ProxyFromEnvironment}
	})
	return sdkTransportValue
}

// errorTransport fails every request with err, sending nothing.
type errorTransport struct{ err error }

func (t errorTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Body != nil {
		request.Body.Close()
	}
	return nil, t.err
}

// timedHTTPClient returns a fresh client: no cookie jar (a global
// http.DefaultClient.Jar must not receive or send cookies), the SDK's own
// transport and the request timeout.
func timedHTTPClient() *http.Client {
	// A non-zero http.DefaultClient.Timeout replaces the 120 s default and a
	// transport set on http.DefaultClient is used under the https check
	// (both documented). Nothing else is copied: its cookie Jar in particular
	// must not receive or send cookies.
	c := &http.Client{Timeout: http.DefaultClient.Timeout}
	if c.Timeout < 0 {
		// net/http treats a negative Timeout as no deadline at all.
		c.Timeout = 0
		c.Transport = errorTransport{err: errors.New("invalid parameter: timeout must not be negative")}
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		return c
	}
	if c.Timeout == 0 {
		c.Timeout = httpTimeout
	}
	c.Transport = httpsOnlyTransport{base: http.DefaultClient.Transport}
	// A 3xx must not be followed. The API key is in the query string, and a
	// redirect could send it to another host.
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return c
}

// apiKeyInText matches an api_key=<value> pair anywhere in a string, whatever
// surrounds it: the value runs to the next &, # or white space.
var apiKeyInText = regexp.MustCompile(`(?i)(api(?:_|%5f)key=)[^&#\s]*`)

// redactAPIKey masks the api_key value of a request URL. It masks the parsed
// query value, and then, as a backstop for a URL the parser reads differently
// from the sender (a second '?', a fragment, a malformed query), any
// api_key=<value> in the text.
func redactAPIKey(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "(url redacted)"
	}
	if query := parsed.Query(); query.Has("api_key") {
		query.Set("api_key", "REDACTED")
		parsed.RawQuery = query.Encode()
		rawURL = parsed.String()
	}
	return apiKeyInText.ReplaceAllString(rawURL, "${1}REDACTED")
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
	return doGetRequestWith(requestURL, object, decodeJSONBody)
}

// doGetValidateRequest is DoGetRequest for validate, whose greylist answer is a
// bare {"Message": "<text>"}.
func doGetValidateRequest(requestURL string, object APIResponse) error {
	return doGetRequestWith(requestURL, object, decodeValidateJSONBody)
}

func doGetRequestWith(requestURL string, object APIResponse, decode func(int, string, []byte, interface{}) error) error {
	response, err := httpGet(requestURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := readLimitedBody(response.Body, response.StatusCode)
	if err != nil {
		return err
	}
	return decode(response.StatusCode, response.Header.Get("Content-Type"), body, object)
}

// DoPostFormRequest POSTs application/x-www-form-urlencoded data to the main API.
func DoPostFormRequest(endpoint string, form url.Values, object APIResponse) error {
	return doPostFormRequest(packageConfig(), endpoint, form, object)
}

func doPostFormRequest(cfg RequestConfig, endpoint string, form url.Values, object APIResponse) error {
	return doPostFormRequestWith(cfg, endpoint, form, object, decodeJSONBody)
}

func doPostFormRequestWith(cfg RequestConfig, endpoint string, form url.Values, object APIResponse, decode func(int, string, []byte, interface{}) error) error {
	if err := cfg.requireKey(); err != nil {
		return err
	}
	if form == nil {
		form = url.Values{}
	}
	form.Set("api_key", cfg.APIKey)

	urlToAccess, err := joinBaseURL(cfg.URI, endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL (%s) or endpoint (%s) value", cfg.URI, endpoint)
	}

	response, err := timedHTTPClient().PostForm(urlToAccess, form)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := readLimitedBody(response.Body, response.StatusCode)
	if err != nil {
		return err
	}
	return decode(response.StatusCode, response.Header.Get("Content-Type"), body, object)
}
