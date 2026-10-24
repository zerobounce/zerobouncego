package zerobouncego

// Client for the ZeroBounce Warmup API (https://www.zerobounce.net/docs/warmup-api).
//
// The Warmup API is separate from the email validation API: its own host, its
// own team-scoped key, and an "API-Key" header instead of an api_key
// parameter. Use WarmupClient with a Warmup API key; the package-level
// validation functions keep using API_KEY.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	// WarmupDefaultBaseURL is the documented host. Override it on the client.
	WarmupDefaultBaseURL = "https://warmup-api.zerobounce.net"
	warmupBasePath       = "/api/v1/warmup"
)

var (
	warmupSearchFrom = regexp.MustCompile(`^[A-Za-z0-9.\-_@+]*$`)
	warmupISODate    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	// WarmupOnboardingModels are the documented onboarding models.
	WarmupOnboardingModels = []string{"self_managed", "zb_managed"}
	// WarmupEmailTypes are the documented types_of_emails values.
	WarmupEmailTypes = []string{"transactional", "newsletter", "marketing"}
)

// Required by the schema but always replaced with the key's own team.
const warmupAnyTeamID = 1

// WarmupClient is one Warmup API key. The key is bound to one team.
type WarmupClient struct {
	apiKey string
	root   string
	client *http.Client
}

// NewWarmupClient builds a client for a Warmup API key. An empty baseURL uses
// WarmupDefaultBaseURL.
func NewWarmupClient(apiKey, baseURL string) (*WarmupClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("empty parameter: api_key")
	}
	// A key with surrounding whitespace or control characters would be quoted,
	// unredacted, in the HTTP library's own error.
	if apiKey != strings.TrimSpace(apiKey) || strings.ContainsFunc(apiKey, func(r rune) bool {
		return r < 32 || r == 127
	}) {
		return nil, fmt.Errorf("invalid parameter: api_key has surrounding whitespace or control characters")
	}
	base := baseURL
	if strings.TrimSpace(base) == "" {
		base = WarmupDefaultBaseURL
	}
	if !strings.HasPrefix(strings.ToLower(base), "https://") {
		return nil, fmt.Errorf("invalid parameter: base_url must be an https:// URL")
	}
	return &WarmupClient{
		apiKey: apiKey,
		root:   strings.TrimRight(base, "/") + warmupBasePath,
		client: timedHTTPClient(),
	}, nil
}

// redact removes every form of a secret from a message.
func (c *WarmupClient) redact(text string, secrets ...string) string {
	for _, secret := range append([]string{c.apiKey}, secrets...) {
		if secret == "" {
			continue
		}
		for _, form := range []string{secret, url.QueryEscape(secret)} {
			text = strings.ReplaceAll(text, form, "***")
		}
	}
	return text
}

// send performs one Warmup request and returns the status and body.
func (c *WarmupClient) send(method, path string, params url.Values, body interface{}, secrets ...string) (int, []byte, http.Header, error) {
	requestURL := c.root + path
	if params != nil && len(params) > 0 {
		requestURL = fmt.Sprintf("%s?%s", requestURL, params.Encode())
	}
	if !strings.HasPrefix(strings.ToLower(requestURL), "https://") {
		return 0, nil, nil, fmt.Errorf("refusing to send the API key to a URL that is not https://")
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, nil, nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, requestURL, reader)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("%s", c.redact(err.Error(), secrets...))
	}
	request.Header.Set("API-Key", c.apiKey)
	request.Header.Set("Accept", CONTENT_TYPE_JSON)
	if body != nil {
		request.Header.Set("Content-Type", CONTENT_TYPE_JSON)
	}

	response, err := c.client.Do(request)
	if err != nil {
		// Rebuilt rather than wrapped: the original holds the URL and headers.
		return 0, nil, nil, fmt.Errorf("%s", c.redact(err.Error(), secrets...))
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return response.StatusCode, nil, response.Header, &ZBApiError{
			StatusCode: response.StatusCode,
			Message:    "ZeroBounce returned a redirect, which this client does not follow.",
		}
	}
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, response.Header, fmt.Errorf("%s", c.redact(err.Error(), secrets...))
	}
	if response.StatusCode > 299 {
		return response.StatusCode, payload, response.Header, &ZBApiError{
			StatusCode: response.StatusCode,
			Message:    c.redact(warmupMessageFromBody(payload, response.StatusCode), secrets...),
		}
	}
	return response.StatusCode, payload, response.Header, nil
}

// json decodes a successful response, or returns nil for the documented empty
// successes (204, and an empty 200).
func (c *WarmupClient) json(method, path string, params url.Values, body interface{}, secrets ...string) (map[string]interface{}, error) {
	status, payload, _, err := c.send(method, path, params, body, secrets...)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent || len(bytes.TrimSpace(payload)) == 0 {
		return nil, nil
	}
	var out interface{}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, &ZBApiError{
			StatusCode: status,
			Message:    c.redact(warmupMessageFromBody(payload, status), secrets...),
		}
	}
	if mapped, ok := out.(map[string]interface{}); ok {
		return mapped, nil
	}
	// A list payload is wrapped so one return type covers every endpoint.
	return map[string]interface{}{"data": out}, nil
}

// csv returns a download body as text.
func (c *WarmupClient) csv(path string, params url.Values) (string, error) {
	status, payload, header, err := c.send("GET", path, params, nil)
	if err != nil {
		return "", err
	}
	contentType := strings.ToLower(header.Get("Content-Type"))
	if len(bytes.TrimSpace(payload)) == 0 || strings.Contains(contentType, CONTENT_TYPE_JSON) {
		return "", &ZBApiError{
			StatusCode: status,
			Message:    c.redact(warmupMessageFromBody(payload, status)),
		}
	}
	return string(payload), nil
}

// -- argument checks ---------------------------------------------------------

func warmupRequired(value, name string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("empty parameter: %s", name)
	}
	return nil
}

func warmupInRange(value, low, high int, name string) error {
	if value < low || value > high {
		return fmt.Errorf("invalid parameter: %s (must be %d to %d)", name, low, high)
	}
	return nil
}

// WarmupDate is a date for the API: a time.Time or a YYYY-MM-DD string.
func warmupISO(value interface{}, name string) (string, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02"), nil
	case string:
		if warmupISODate.MatchString(typed) {
			return typed, nil
		}
	}
	return "", fmt.Errorf("invalid parameter: %s (expected YYYY-MM-DD)", name)
}

func warmupEngagementBody(cFrom string, replyPct, importantPct, clickPct int) (map[string]interface{}, error) {
	if err := warmupRequired(cFrom, "c_from"); err != nil {
		return nil, err
	}
	if err := warmupInRange(replyPct, 5, 45, "reply_pct"); err != nil {
		return nil, err
	}
	if err := warmupInRange(importantPct, 5, 40, "important_pct"); err != nil {
		return nil, err
	}
	if err := warmupInRange(clickPct, 0, 70, "click_pct"); err != nil {
		return nil, err
	}
	if float64(clickPct) > float64(importantPct)*1.75 {
		return nil, fmt.Errorf("invalid parameter: click_pct (at most important_pct x 1.75)")
	}
	return map[string]interface{}{
		"c_from":        cFrom,
		"reply_pct":     replyPct,
		"important_pct": importantPct,
		"click_pct":     clickPct,
	}, nil
}

// -- sending accounts --------------------------------------------------------

// ListSendingAccounts returns every sending account of the key's team.
func (c *WarmupClient) ListSendingAccounts() (map[string]interface{}, error) {
	return c.json("GET", "/sending-accounts", nil, nil)
}

// GetSendingAccount returns one sending account.
func (c *WarmupClient) GetSendingAccount(fromAddress string) (map[string]interface{}, error) {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return nil, err
	}
	return c.json("GET", "/sending-account", url.Values{"from_address": {fromAddress}}, nil)
}

// WarmupSendingAccountOptions are the optional fields of AddSendingAccount.
type WarmupSendingAccountOptions struct {
	// EngagementRuleID is 0 to 4 and required for self_managed.
	EngagementRuleID *int
	// IgnoreConflict is self-managed only: do not fail when already registered.
	IgnoreConflict *bool
	// EmailContent and ManagedSending are zb_managed only.
	EmailContent   map[string]interface{}
	ManagedSending map[string]interface{}
}

// AddSendingAccount adds a from-address to warmup.
//
// onboardingModel is "self_managed" (you run the mailbox; EngagementRuleID is
// required) or "zb_managed" (ZeroBounce provisions it). For zb_managed a 200
// means accepted; the mailbox is provisioned afterwards.
func (c *WarmupClient) AddSendingAccount(fromAddress, onboardingModel string, opts *WarmupSendingAccountOptions) (map[string]interface{}, error) {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return nil, err
	}
	known := false
	for _, model := range WarmupOnboardingModels {
		if onboardingModel == model {
			known = true
		}
	}
	if !known {
		return nil, fmt.Errorf("invalid parameter: onboarding_model (must be self_managed or zb_managed)")
	}

	body := map[string]interface{}{"from_address": fromAddress, "onboarding_model": onboardingModel}
	if opts == nil {
		opts = &WarmupSendingAccountOptions{}
	}
	if onboardingModel == "self_managed" {
		if opts.EngagementRuleID == nil {
			return nil, fmt.Errorf("empty parameter: engagement_rule_id (required for self_managed)")
		}
		if err := warmupInRange(*opts.EngagementRuleID, 0, 4, "engagement_rule_id"); err != nil {
			return nil, err
		}
		body["engagement_rule_id"] = *opts.EngagementRuleID
		if opts.EmailContent != nil || opts.ManagedSending != nil {
			return nil, fmt.Errorf("invalid parameter: email_content and managed_sending are for zb_managed")
		}
	}
	if opts.IgnoreConflict != nil {
		body["ignore_conflict"] = *opts.IgnoreConflict
	}
	if opts.EmailContent != nil {
		content := map[string]interface{}{}
		for key, value := range opts.EmailContent {
			content[key] = value
		}
		if types, found := content["types_of_emails"]; found {
			list, ok := types.([]string)
			if !ok {
				return nil, fmt.Errorf("invalid parameter: types_of_emails (transactional, newsletter and/or marketing)")
			}
			for _, item := range list {
				valid := false
				for _, allowed := range WarmupEmailTypes {
					if item == allowed {
						valid = true
					}
				}
				if !valid {
					return nil, fmt.Errorf("invalid parameter: types_of_emails (transactional, newsletter and/or marketing)")
				}
			}
		}
		body["email_content"] = content
	}
	if opts.ManagedSending != nil {
		sending := map[string]interface{}{}
		for key, value := range opts.ManagedSending {
			sending[key] = value
		}
		if offset, found := sending["timezone_offset"]; found {
			value, ok := offset.(int)
			if !ok || value < -11 || value > 14 {
				return nil, fmt.Errorf("invalid parameter: timezone_offset (must be -11 to 14)")
			}
		}
		if start, found := sending["warmup_start_date"]; found && start != nil {
			formatted, err := warmupISO(start, "warmup_start_date")
			if err != nil {
				return nil, err
			}
			sending["warmup_start_date"] = formatted
		}
		body["managed_sending"] = sending
	}
	return c.json("POST", "/sending-accounts", nil, body)
}

// UpdateSendingAccount changes an account's engagement rule, status and
// auto-enable flag. statusID is 0 to 8 (4 Paused, 5 Disabled); autoEnable true
// is accepted only for an account that is not active.
func (c *WarmupClient) UpdateSendingAccount(fromAddress string, engagementRuleID, statusID int, autoEnable bool) (map[string]interface{}, error) {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return nil, err
	}
	if err := warmupInRange(engagementRuleID, 0, 4, "engagement_rule_id"); err != nil {
		return nil, err
	}
	if err := warmupInRange(statusID, 0, 8, "status_id"); err != nil {
		return nil, err
	}
	return c.json("PUT", "/sending-accounts", nil, map[string]interface{}{
		"team_id":            warmupAnyTeamID,
		"from_address":       fromAddress,
		"engagement_rule_id": engagementRuleID,
		"status_id":          statusID,
		"auto_enable":        autoEnable,
	})
}

// DeleteSendingAccount removes a sending account. Destructive and final.
func (c *WarmupClient) DeleteSendingAccount(fromAddress string) error {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return err
	}
	_, err := c.json("DELETE", "/sending-accounts", url.Values{"from_address": {fromAddress}}, nil)
	return err
}

// ChangeFromAddress moves an account and its history to a new from-address.
// Despite the path, no SMTP credentials are stored.
func (c *WarmupClient) ChangeFromAddress(oldFromAddress, newFromAddress string) error {
	if err := warmupRequired(oldFromAddress, "old_from_address"); err != nil {
		return err
	}
	if err := warmupRequired(newFromAddress, "new_from_address"); err != nil {
		return err
	}
	_, err := c.json("PUT", "/smtp-credentials", nil, map[string]interface{}{
		"team_id":          warmupAnyTeamID,
		"old_from_address": oldFromAddress,
		"new_from_address": newFromAddress,
	})
	return err
}

// WarmupListFromAddressesOptions are the optional filters of ListFromAddresses.
type WarmupListFromAddressesOptions struct {
	// SearchFrom takes letters, digits and . - _ @ + only.
	SearchFrom string
	Offset     *int
	Limit      *int
}

// ListFromAddresses returns the registered from-addresses.
func (c *WarmupClient) ListFromAddresses(opts *WarmupListFromAddressesOptions) (map[string]interface{}, error) {
	params := url.Values{}
	if opts != nil {
		if opts.SearchFrom != "" {
			if !warmupSearchFrom.MatchString(opts.SearchFrom) {
				return nil, fmt.Errorf("invalid parameter: search_from (letters, digits and . - _ @ + only)")
			}
			params.Set("search_from", opts.SearchFrom)
		}
		// Offset and limit travel as strings.
		if opts.Offset != nil {
			if *opts.Offset < 0 {
				return nil, fmt.Errorf("invalid parameter: offset (must be a non-negative integer)")
			}
			params.Set("offset", fmt.Sprintf("%d", *opts.Offset))
		}
		if opts.Limit != nil {
			if *opts.Limit < 0 {
				return nil, fmt.Errorf("invalid parameter: limit (must be a non-negative integer)")
			}
			params.Set("limit", fmt.Sprintf("%d", *opts.Limit))
		}
	}
	return c.json("GET", "/from-addresses", params, nil)
}

// -- engagement rules --------------------------------------------------------

// GetEngagementRules returns the rule catalogue: -1 Custom, 0 Default,
// 1 Slow Pace, 2 Repair, 3 Boost, 4 Protect.
func (c *WarmupClient) GetEngagementRules() (map[string]interface{}, error) {
	return c.json("GET", "/engagement-rules", nil, nil)
}

// SetEngagementRule sets a standard rule (0 to 4) on an address.
func (c *WarmupClient) SetEngagementRule(cFrom string, ruleID int) error {
	if err := warmupRequired(cFrom, "c_from"); err != nil {
		return err
	}
	if err := warmupInRange(ruleID, 0, 4, "rule_id"); err != nil {
		return err
	}
	_, err := c.json("PUT", "/engagement-rule", url.Values{"c_from": {cFrom}},
		map[string]interface{}{"rule_id": ruleID})
	return err
}

// GetCustomEngagement returns an address's custom rule, with can_update and
// remaining_seconds.
func (c *WarmupClient) GetCustomEngagement(cFrom string) (map[string]interface{}, error) {
	if err := warmupRequired(cFrom, "c_from"); err != nil {
		return nil, err
	}
	if cFrom == "." || cFrom == ".." {
		return nil, fmt.Errorf("invalid parameter: c_from")
	}
	return c.json("GET", "/custom-engagement/"+url.PathEscape(strings.TrimSpace(cFrom)), nil, nil)
}

// CreateCustomEngagement creates a custom rule. It starts a 7 day cooldown on
// updates.
func (c *WarmupClient) CreateCustomEngagement(cFrom string, replyPct, importantPct, clickPct int) (map[string]interface{}, error) {
	body, err := warmupEngagementBody(cFrom, replyPct, importantPct, clickPct)
	if err != nil {
		return nil, err
	}
	return c.json("POST", "/custom-engagement", nil, body)
}

// UpdateCustomEngagement updates a custom rule. During the 7 day cooldown the
// API answers 429.
func (c *WarmupClient) UpdateCustomEngagement(cFrom string, replyPct, importantPct, clickPct int) (map[string]interface{}, error) {
	body, err := warmupEngagementBody(cFrom, replyPct, importantPct, clickPct)
	if err != nil {
		return nil, err
	}
	return c.json("PUT", "/custom-engagement", nil, body)
}

// DeleteCustomEngagement deletes a custom rule and falls back to standard rule
// engagementRuleID (1 to 4).
func (c *WarmupClient) DeleteCustomEngagement(cFrom string, engagementRuleID int) (map[string]interface{}, error) {
	if err := warmupRequired(cFrom, "c_from"); err != nil {
		return nil, err
	}
	if err := warmupInRange(engagementRuleID, 1, 4, "engagement_rule_id"); err != nil {
		return nil, err
	}
	return c.json("DELETE", "/custom-engagement", nil, map[string]interface{}{
		"c_from":             cFrom,
		"engagement_rule_id": engagementRuleID,
	})
}

// -- statistics, seeds and utilities -----------------------------------------

// GetSeeds returns the seed list.
func (c *WarmupClient) GetSeeds() (map[string]interface{}, error) {
	return c.json("GET", "/seeds", nil, nil)
}

// DownloadSeeds returns the seed list as CSV (columns seed,provider).
func (c *WarmupClient) DownloadSeeds() (string, error) {
	return c.csv("/seeds/download", nil)
}

// GetSeedCount returns the number of seeds.
func (c *WarmupClient) GetSeedCount() (map[string]interface{}, error) {
	return c.json("GET", "/seed-count", nil, nil)
}

// GetProgressToday returns today's warmup progress.
func (c *WarmupClient) GetProgressToday() (map[string]interface{}, error) {
	return c.json("GET", "/progress-today", nil, nil)
}

// GetSentToday returns the messages sent today.
func (c *WarmupClient) GetSentToday() (map[string]interface{}, error) {
	return c.json("GET", "/sent-today", nil, nil)
}

// GetEmailsPerDay returns the current daily volume.
func (c *WarmupClient) GetEmailsPerDay() (map[string]interface{}, error) {
	return c.json("GET", "/emails-per-day", nil, nil)
}

// GetStatistics returns statistics for an address. An address with no history
// returns zeros.
func (c *WarmupClient) GetStatistics(fromAddress string) (map[string]interface{}, error) {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return nil, err
	}
	return c.json("GET", "/statistics", url.Values{"from_address": {fromAddress}}, nil)
}

// GetStatisticsPerDay returns daily statistics between two dates, inclusive.
// startDate and endDate are a time.Time or a YYYY-MM-DD string.
func (c *WarmupClient) GetStatisticsPerDay(fromAddress string, startDate, endDate interface{}) (map[string]interface{}, error) {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return nil, err
	}
	start, err := warmupISO(startDate, "start_date")
	if err != nil {
		return nil, err
	}
	end, err := warmupISO(endDate, "end_date")
	if err != nil {
		return nil, err
	}
	if start > end {
		return nil, fmt.Errorf("invalid parameter: start_date is later than end_date")
	}
	return c.json("GET", "/statistics-per-day", url.Values{
		"from_address": {fromAddress},
		"start_date":   {start},
		"end_date":     {end},
	}, nil)
}

// GetStatisticsDetailed returns detailed statistics for an address.
func (c *WarmupClient) GetStatisticsDetailed(cFrom string) (map[string]interface{}, error) {
	if err := warmupRequired(cFrom, "c_from"); err != nil {
		return nil, err
	}
	return c.json("GET", "/statistics-detailed", url.Values{"c_from": {cFrom}}, nil)
}

// DownloadStatistics returns an address's statistics as CSV.
func (c *WarmupClient) DownloadStatistics(cFrom string) (string, error) {
	if err := warmupRequired(cFrom, "c_from"); err != nil {
		return "", err
	}
	return c.csv("/statistics/download", url.Values{"c_from": {cFrom}})
}

// GetFromAddressStatistics returns statistics for one from-address.
func (c *WarmupClient) GetFromAddressStatistics(fromAddress string) (map[string]interface{}, error) {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return nil, err
	}
	return c.json("GET", "/from-address-statistics", url.Values{"from_address": {fromAddress}}, nil)
}

// CheckDomainDmarc returns the SPF, DKIM and DMARC status of a from-address's
// domain.
//
// LIVE: pass an EMAIL ADDRESS. The live API answers "Please provide a valid
// email address" for a bare domain, although the docs describe a domain.
func (c *WarmupClient) CheckDomainDmarc(fromAddress, dkimSelector string) (map[string]interface{}, error) {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return nil, err
	}
	if !strings.Contains(fromAddress, "@") {
		return nil, fmt.Errorf("invalid parameter: from_address (an email address, e.g. sender@example.com)")
	}
	params := url.Values{"from_address": {fromAddress}}
	if dkimSelector != "" {
		params.Set("dkim_selector", dkimSelector)
	}
	return c.json("GET", "/check-domain-dmarc", params, nil)
}

// TestSMTPConfiguration checks that SMTP credentials connect and hand off a
// message. Nothing is stored, and the test message goes to a ZeroBounce test
// recipient, not fromAddress.
//
// LIVE: the API answers any failed SMTP login with HTTP 500. That is an API
// bug. The username and password are masked in every error from this method.
func (c *WarmupClient) TestSMTPConfiguration(fromAddress, username, password, server string, port int) (map[string]interface{}, error) {
	if err := warmupRequired(fromAddress, "from_address"); err != nil {
		return nil, err
	}
	if err := warmupRequired(username, "username"); err != nil {
		return nil, err
	}
	if err := warmupRequired(password, "password"); err != nil {
		return nil, err
	}
	if err := warmupRequired(server, "server"); err != nil {
		return nil, err
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid parameter: port (must be 1 to 65535)")
	}
	return c.json("POST", "/test-smtp-configuration", nil, map[string]interface{}{
		"from_address": fromAddress,
		"username":     username,
		"password":     password,
		"server":       server,
		"port":         port,
	}, password, username)
}

// warmupMessageFromBody turns a Warmup error body into a short message.
func warmupMessageFromBody(body []byte, status int) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return fmt.Sprintf("Empty response from ZeroBounce (HTTP %d)", status)
	}
	head := strings.ToLower(trimmed)
	if len(head) > 2000 {
		head = head[:2000]
	}
	if strings.Contains(head, "<!doctype html") || strings.Contains(head, "<html") {
		return "ZeroBounce returned an HTML error page instead of an API response."
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var parsed interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return "ZeroBounce returned a response that is not valid JSON."
		}
		// A top-level list of strings, e.g. ["In-progress tests cannot be deleted."].
		if list, ok := parsed.([]interface{}); ok {
			for _, item := range list {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					return text
				}
			}
		}
		if mapped, ok := parsed.(map[string]interface{}); ok {
			for _, key := range []string{"message", "error", "error_message", "detail"} {
				switch value := mapped[key].(type) {
				case string:
					if strings.TrimSpace(value) != "" {
						return value
					}
				case []interface{}:
					if len(value) == 0 {
						continue
					}
					if text, ok := value[0].(string); ok && strings.TrimSpace(text) != "" {
						return text
					}
					// A validation error list: [{loc: [...], msg: "..."}].
					if item, ok := value[0].(map[string]interface{}); ok {
						if msg, ok := item["msg"].(string); ok {
							if loc, ok := item["loc"].([]interface{}); ok && len(loc) > 1 {
								parts := make([]string, 0, len(loc)-1)
								for _, piece := range loc[1:] {
									parts = append(parts, fmt.Sprintf("%v", piece))
								}
								return fmt.Sprintf("%s: %s", strings.Join(parts, "."), msg)
							}
							return msg
						}
					}
				}
			}
		}
		return fmt.Sprintf("ZeroBounce returned an error (HTTP %d).", status)
	}
	if len(trimmed) > 1000 {
		return "ZeroBounce returned a response that is not valid JSON."
	}
	return trimmed
}
