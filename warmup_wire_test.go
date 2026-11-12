package zerobouncego

import (
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"
)

// warmupWire is one Warmup request as it went out: method, path, raw query
// and raw JSON body. url.Values and encoding/json write keys in sorted order,
// so both compare as exact strings.
type warmupWire struct {
	method, path, query, body string
}

// sendWarmup runs call against a fake Warmup host and returns the requests it
// sent, with the error.
func sendWarmup(t *testing.T, call func(w *WarmupClient) error) ([]warmupWire, error) {
	t.Helper()
	var sent []warmupWire
	responder, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, body string) (*http.Response, error) {
		sent = append(sent, warmupWire{req.Method, req.URL.EscapedPath(), req.URL.RawQuery, body})
		if strings.Contains(req.URL.Path, "/download") {
			return textResponse(req, 200, "text/csv", "seed,provider\nseed@example.com,gmail\n"), nil
		}
		return textResponse(req, 200, CONTENT_TYPE_JSON, `{"success":true}`), nil
	})
	err := call(warmup)
	responder.take()
	return sent, err
}

func expectOneWarmupRequest(t *testing.T, call func(w *WarmupClient) error, want warmupWire) {
	t.Helper()
	sent, err := sendWarmup(t, call)
	if err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 {
		t.Fatalf("sent %d requests, want 1", len(sent))
	}
	want.path = warmupBasePath + want.path
	if sent[0] != want {
		t.Fatalf("sent %+v\nwant %+v", sent[0], want)
	}
}

func expectNoWarmupRequest(t *testing.T, call func(w *WarmupClient) error, wantError string) {
	t.Helper()
	sent, err := sendWarmup(t, call)
	if err == nil || !strings.Contains(err.Error(), wantError) {
		t.Fatalf("err = %v, want %q", err, wantError)
	}
	if len(sent) != 0 {
		t.Fatalf("a rejected argument still sent %d request(s)", len(sent))
	}
}

// Every Warmup request is pinned whole: method, path, every query parameter
// and the exact JSON body, so renaming or adding a wire field, or sending an
// unset option, fails here. A new WarmupClient method must join the table.
func TestEveryWarmupRequestSendsExactlyTheDocumentedFields(t *testing.T) {
	start := time.Date(2026, 3, 4, 23, 30, 0, 0, time.FixedZone("x", -5*3600))
	cases := map[string]struct {
		call func(w *WarmupClient) error
		want warmupWire
	}{
		"ListSendingAccounts": {func(w *WarmupClient) error { _, err := w.ListSendingAccounts(); return err },
			warmupWire{"GET", "/sending-accounts", "", ""}},
		"GetSendingAccount": {func(w *WarmupClient) error { _, err := w.GetSendingAccount("a+b@example.com"); return err },
			warmupWire{"GET", "/sending-account", "from_address=a%2Bb%40example.com", ""}},
		"AddSendingAccount": {func(w *WarmupClient) error {
			_, err := w.AddSendingAccount("a@example.com", "self_managed", &WarmupSendingAccountOptions{EngagementRuleID: intPtr(2)})
			return err
		}, warmupWire{"POST", "/sending-accounts", "", `{"engagement_rule_id":2,"from_address":"a@example.com","onboarding_model":"self_managed"}`}},
		"UpdateSendingAccount": {func(w *WarmupClient) error { _, err := w.UpdateSendingAccount("a@example.com", 2, 4, true); return err },
			warmupWire{"PUT", "/sending-accounts", "", `{"auto_enable":true,"engagement_rule_id":2,"from_address":"a@example.com","status_id":4,"team_id":1}`}},
		"DeleteSendingAccount": {func(w *WarmupClient) error { return w.DeleteSendingAccount("a@example.com") },
			warmupWire{"DELETE", "/sending-accounts", "from_address=a%40example.com", ""}},
		"ChangeFromAddress": {func(w *WarmupClient) error { return w.ChangeFromAddress("a@example.com", "b@example.com") },
			warmupWire{"PUT", "/smtp-credentials", "", `{"new_from_address":"b@example.com","old_from_address":"a@example.com","team_id":1}`}},
		"ListFromAddresses": {func(w *WarmupClient) error {
			_, err := w.ListFromAddresses(&WarmupListFromAddressesOptions{SearchFrom: "a@b", Offset: intPtr(5), Limit: intPtr(10)})
			return err
		}, warmupWire{"GET", "/from-addresses", "limit=10&offset=5&search_from=a%40b", ""}},
		"GetEngagementRules": {func(w *WarmupClient) error { _, err := w.GetEngagementRules(); return err },
			warmupWire{"GET", "/engagement-rules", "", ""}},
		"SetEngagementRule": {func(w *WarmupClient) error { return w.SetEngagementRule("a@example.com", 3) },
			warmupWire{"PUT", "/engagement-rule", "c_from=a%40example.com", `{"rule_id":3}`}},
		"GetCustomEngagement": {func(w *WarmupClient) error { _, err := w.GetCustomEngagement("a@example.com"); return err },
			warmupWire{"GET", "/custom-engagement/a@example.com", "", ""}},
		"CreateCustomEngagement": {func(w *WarmupClient) error {
			_, err := w.CreateCustomEngagement("a@example.com", 10, 20, 30)
			return err
		},
			warmupWire{"POST", "/custom-engagement", "", `{"c_from":"a@example.com","click_pct":30,"important_pct":20,"reply_pct":10}`}},
		"UpdateCustomEngagement": {func(w *WarmupClient) error {
			_, err := w.UpdateCustomEngagement("a@example.com", 11, 21, 31)
			return err
		},
			warmupWire{"PUT", "/custom-engagement", "", `{"c_from":"a@example.com","click_pct":31,"important_pct":21,"reply_pct":11}`}},
		"DeleteCustomEngagement": {func(w *WarmupClient) error { _, err := w.DeleteCustomEngagement("a@example.com", 4); return err },
			warmupWire{"DELETE", "/custom-engagement", "", `{"c_from":"a@example.com","engagement_rule_id":4}`}},
		"GetSeeds": {func(w *WarmupClient) error { _, err := w.GetSeeds(); return err },
			warmupWire{"GET", "/seeds", "", ""}},
		"DownloadSeeds": {func(w *WarmupClient) error { _, err := w.DownloadSeeds(); return err },
			warmupWire{"GET", "/seeds/download", "", ""}},
		"GetSeedCount": {func(w *WarmupClient) error { _, err := w.GetSeedCount(); return err },
			warmupWire{"GET", "/seed-count", "", ""}},
		"GetProgressToday": {func(w *WarmupClient) error { _, err := w.GetProgressToday(); return err },
			warmupWire{"GET", "/progress-today", "", ""}},
		"GetSentToday": {func(w *WarmupClient) error { _, err := w.GetSentToday(); return err },
			warmupWire{"GET", "/sent-today", "", ""}},
		"GetEmailsPerDay": {func(w *WarmupClient) error { _, err := w.GetEmailsPerDay(); return err },
			warmupWire{"GET", "/emails-per-day", "", ""}},
		"GetStatistics": {func(w *WarmupClient) error { _, err := w.GetStatistics("a@example.com"); return err },
			warmupWire{"GET", "/statistics", "from_address=a%40example.com", ""}},
		"GetStatisticsPerDay": {func(w *WarmupClient) error {
			_, err := w.GetStatisticsPerDay("a@example.com", start, "2026-03-09")
			return err
		}, warmupWire{"GET", "/statistics-per-day", "end_date=2026-03-09&from_address=a%40example.com&start_date=2026-03-04", ""}},
		"GetStatisticsDetailed": {func(w *WarmupClient) error { _, err := w.GetStatisticsDetailed("a@example.com"); return err },
			warmupWire{"GET", "/statistics-detailed", "c_from=a%40example.com", ""}},
		"DownloadStatistics": {func(w *WarmupClient) error { _, err := w.DownloadStatistics("a@example.com"); return err },
			warmupWire{"GET", "/statistics/download", "c_from=a%40example.com", ""}},
		"GetFromAddressStatistics": {func(w *WarmupClient) error { _, err := w.GetFromAddressStatistics("a@example.com"); return err },
			warmupWire{"GET", "/from-address-statistics", "from_address=a%40example.com", ""}},
		"CheckDomainDmarc": {func(w *WarmupClient) error { _, err := w.CheckDomainDmarc("a@example.com", "s1"); return err },
			warmupWire{"GET", "/check-domain-dmarc", "dkim_selector=s1&from_address=a%40example.com", ""}},
		"TestSMTPConfiguration": {func(w *WarmupClient) error {
			_, err := w.TestSMTPConfiguration("a@example.com", "user", "secret", "smtp.example.com", 587)
			return err
		}, warmupWire{"POST", "/test-smtp-configuration", "", `{"from_address":"a@example.com","password":"secret","port":587,"server":"smtp.example.com","username":"user"}`}},
	}
	var missing []string
	for _, call := range warmupCalls(nil) {
		name := strings.TrimPrefix(call.name, "WarmupClient.")
		if _, ok := cases[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("Warmup methods with no exact-request case: %v", missing)
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) { expectOneWarmupRequest(t, tc.call, tc.want) })
	}
}

// The optional parameters go out only when set: no dkim_selector, no list
// filters, and for self_managed no ignore_conflict unless the caller set it,
// in which case false is sent as false.
func TestWarmupOptionalFieldsAreSentOnlyWhenSet(t *testing.T) {
	expectOneWarmupRequest(t, func(w *WarmupClient) error { _, err := w.CheckDomainDmarc("a@example.com", ""); return err },
		warmupWire{"GET", "/check-domain-dmarc", "from_address=a%40example.com", ""})
	expectOneWarmupRequest(t, func(w *WarmupClient) error { _, err := w.ListFromAddresses(nil); return err },
		warmupWire{"GET", "/from-addresses", "", ""})
	expectOneWarmupRequest(t, func(w *WarmupClient) error {
		_, err := w.ListFromAddresses(&WarmupListFromAddressesOptions{Offset: intPtr(0)})
		return err
	}, warmupWire{"GET", "/from-addresses", "offset=0", ""})
	for _, flag := range []bool{false, true} {
		expectOneWarmupRequest(t, func(w *WarmupClient) error {
			_, err := w.AddSendingAccount("a@example.com", "self_managed",
				&WarmupSendingAccountOptions{EngagementRuleID: intPtr(0), IgnoreConflict: boolPtr(flag)})
			return err
		}, warmupWire{"POST", "/sending-accounts", "",
			`{"engagement_rule_id":0,"from_address":"a@example.com","ignore_conflict":` + formatBoolField(flag) + `,"onboarding_model":"self_managed"}`})
	}
	// zb_managed with no options sends the two required fields only.
	expectOneWarmupRequest(t, func(w *WarmupClient) error {
		_, err := w.AddSendingAccount("a@example.com", "zb_managed", nil)
		return err
	}, warmupWire{"POST", "/sending-accounts", "", `{"from_address":"a@example.com","onboarding_model":"zb_managed"}`})
}

// AddSendingAccount checks its options before sending anything: the
// engagement rule is required for self_managed and must be 0 to 4, the
// zb_managed blocks are refused for self_managed, and types_of_emails takes
// only the documented values.
func TestAddSendingAccountRefusesBadOptionsBeforeSending(t *testing.T) {
	add := func(model string, opts *WarmupSendingAccountOptions) func(w *WarmupClient) error {
		return func(w *WarmupClient) error { _, err := w.AddSendingAccount("a@example.com", model, opts); return err }
	}
	expectNoWarmupRequest(t, add("self_managed", nil), "engagement_rule_id (required for self_managed)")
	expectNoWarmupRequest(t, add("self_managed", &WarmupSendingAccountOptions{EngagementRuleID: intPtr(5)}), "engagement_rule_id (must be 0 to 4)")
	expectNoWarmupRequest(t, add("self_managed", &WarmupSendingAccountOptions{EngagementRuleID: intPtr(-1)}), "engagement_rule_id (must be 0 to 4)")
	for name, opts := range map[string]*WarmupSendingAccountOptions{
		"email_content":   {EngagementRuleID: intPtr(1), EmailContent: map[string]interface{}{"company_name": "Acme"}},
		"managed_sending": {EngagementRuleID: intPtr(1), ManagedSending: map[string]interface{}{"weekdays_only": true}},
		"empty maps":      {EngagementRuleID: intPtr(1), EmailContent: map[string]interface{}{}, ManagedSending: map[string]interface{}{}},
	} {
		t.Run("self_managed with "+name, func(t *testing.T) {
			expectNoWarmupRequest(t, add("self_managed", opts), "email_content and managed_sending are for zb_managed")
		})
	}
	for name, types := range map[string]interface{}{
		"unknown value":     []string{"newsletter", "spam"},
		"wrong case":        []string{"Newsletter"},
		"not a string list": []interface{}{"newsletter"},
		"a string":          "newsletter",
	} {
		t.Run("types_of_emails "+name, func(t *testing.T) {
			expectNoWarmupRequest(t, add("zb_managed", &WarmupSendingAccountOptions{
				EmailContent: map[string]interface{}{"types_of_emails": types},
			}), "invalid parameter: types_of_emails")
		})
	}
	expectNoWarmupRequest(t, add("zb_managed", &WarmupSendingAccountOptions{
		ManagedSending: map[string]interface{}{"warmup_start_date": "2026/03/04"},
	}), "warmup_start_date (expected YYYY-MM-DD)")
	expectNoWarmupRequest(t, add("zb_managed", &WarmupSendingAccountOptions{
		ManagedSending: map[string]interface{}{"warmup_start_date": 20260304},
	}), "warmup_start_date (expected YYYY-MM-DD)")
}

// A time.Time warmup_start_date goes out as its own calendar date,
// YYYY-MM-DD, not as encoding/json's RFC 3339 timestamp.
func TestAddSendingAccountSendsAStartDateAsYYYYMMDD(t *testing.T) {
	for name, start := range map[string]interface{}{
		"time.Time late in a zone west of UTC": time.Date(2026, 3, 4, 23, 30, 0, 0, time.FixedZone("x", -5*3600)),
		"time.Time in UTC":                     time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC),
		"string":                               "2026-03-04",
	} {
		t.Run(name, func(t *testing.T) {
			expectOneWarmupRequest(t, func(w *WarmupClient) error {
				_, err := w.AddSendingAccount("a@example.com", "zb_managed", &WarmupSendingAccountOptions{
					EmailContent:   map[string]interface{}{"types_of_emails": []string{"transactional"}},
					ManagedSending: map[string]interface{}{"warmup_start_date": start},
				})
				return err
			}, warmupWire{"POST", "/sending-accounts", "",
				`{"email_content":{"types_of_emails":["transactional"]},"from_address":"a@example.com",` +
					`"managed_sending":{"warmup_start_date":"2026-03-04"},"onboarding_model":"zb_managed"}`})
		})
	}
}
