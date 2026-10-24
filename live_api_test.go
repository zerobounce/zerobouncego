//go:build live

// Opt-in live API suite: calls every SDK method against the real API to check
// payloads and response shapes.
//
// The `live` build tag keeps it out of `go test ./...` and CI. Run it with:
//
//	export ZEROBOUNCE_API_KEY=...          # validation API (skipped without it)
//	export ZEROBOUNCE_WARMUP_API_KEY=...   # Warmup API (skipped without it)
//	go test -tags live -v -run TestLive ./...
//
// By default only free calls run: sandbox addresses (no credits), reads, and
// requests expected to fail that still prove the API accepts the payload. The
// free tier checks that the credit balance did not change. Two flags opt in to
// more:
//
//	ZEROBOUNCE_LIVE_SPEND=1     credit-spending calls and a bulk round trip
//	                            (the job is deleted again)
//	ZEROBOUNCE_LIVE_CHANGES=1   reversible account changes, undone in cleanup
//	                            even when a test fails
//
// Never called, because the API cannot undo them: creating an Inbox Placement
// test, setting the Inbox Placement webhook, and ZB-managed Warmup onboarding.
//
// No key is written to a file or printed; keys come from the environment.

package zerobouncego

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	liveAPIKey    = strings.TrimSpace(os.Getenv("ZEROBOUNCE_API_KEY"))
	liveWarmupKey = strings.TrimSpace(os.Getenv("ZEROBOUNCE_WARMUP_API_KEY"))
	liveSpend     = os.Getenv("ZEROBOUNCE_LIVE_SPEND") == "1"
	liveChanges   = os.Getenv("ZEROBOUNCE_LIVE_CHANGES") == "1"
)

// A well-formed ID matching no file: a malformed one can be Cloudflare-blocked.
const (
	liveUnknownID = "0d1b7a4e-1111-2222-3333-444455556666"
	liveCallback  = "https://example.com/zerobounce-sdk-live-test"
	liveSandbox   = "valid@example.com"
)

func liveRedact(text string) string {
	for _, secret := range []string{liveAPIKey, liveWarmupKey} {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "***")
		}
	}
	return text
}

func requireValidationKey(t *testing.T) {
	t.Helper()
	if liveAPIKey == "" {
		t.Skip("ZEROBOUNCE_API_KEY not set")
	}
	Initialize(liveAPIKey)
}

func liveCredits(t *testing.T) int {
	t.Helper()
	credits, err := GetCredits()
	if err != nil {
		t.Fatalf("GetCredits: %s", liveRedact(err.Error()))
	}
	return credits.Credits()
}

func TestLiveValidationFree(t *testing.T) {
	requireValidationKey(t)
	before := liveCredits(t)

	t.Run("GetApiUsage", func(t *testing.T) {
		end := time.Now()
		start := end.AddDate(0, 0, -7)
		if _, err := GetApiUsage(start, end); err != nil {
			t.Fatalf("GetApiUsage: %s", liveRedact(err.Error()))
		}
	})

	t.Run("Validate on a sandbox address", func(t *testing.T) {
		response, err := Validate(liveSandbox, "")
		if err != nil {
			t.Fatalf("Validate: %s", liveRedact(err.Error()))
		}
		if response.Address != liveSandbox {
			t.Fatalf("address did not echo: %q", response.Address)
		}
	})

	t.Run("LIVE batch timeout is accepted as a string", func(t *testing.T) {
		// Sent as a JSON number the API rejects the whole batch with the
		// account-level "Invalid API Key or your account ran out of credits".
		timeout := 30
		response, err := ValidateBatchWithOptions(
			[]EmailToValidate{{EmailAddress: liveSandbox}},
			&ValidateBatchOptions{Timeout: &timeout},
		)
		if err != nil {
			t.Fatalf("ValidateBatch: %s", liveRedact(err.Error()))
		}
		for _, item := range response.Errors {
			if strings.EqualFold(item.EmailAddress, "all") {
				t.Fatalf("account-level error: %s", liveRedact(item.Error))
			}
		}
	})

	t.Run("an unknown file id is a short, clean error", func(t *testing.T) {
		_, err := BulkValidationFileStatus(liveUnknownID)
		if err == nil {
			t.Fatal("expected a failure")
		}
		message := liveRedact(err.Error())
		if len(message) > 500 || strings.Contains(message, "<html") {
			t.Fatalf("message is not short and clean: %q", message)
		}
	})

	t.Run("LIVE S3 jobs need return_url, checked before sending", func(t *testing.T) {
		_, err := EmailFinderS3File(BulkFileJobOptions{FileName: "f.csv", HasHeaderRow: true})
		if err == nil || !strings.Contains(err.Error(), "return_url") {
			t.Fatalf("expected a return_url rejection, got %v", err)
		}
	})

	t.Run("LIVE a complete S3 payload is accepted", func(t *testing.T) {
		// Without an S3 integration this fails on the integration, not the payload.
		_, err := EmailFinderS3File(BulkFileJobOptions{
			FileName:     "f.csv",
			HasHeaderRow: true,
			ReturnURL:    liveCallback,
		})
		if err != nil {
			message := liveRedact(err.Error())
			if strings.Contains(message, "return URL is not permitted") ||
				strings.Contains(message, "This field is required") {
				t.Fatalf("payload rejected: %s", message)
			}
		}
	})

	t.Run("ListFilters", func(t *testing.T) {
		if _, err := ListFilters(false); err != nil {
			t.Fatalf("ListFilters: %s", liveRedact(err.Error()))
		}
	})

	t.Run("LIVE the Inbox Placement Tester accepts a Bearer key", func(t *testing.T) {
		if _, err := GetInboxPlacementProviders(); err != nil {
			t.Fatalf("GetInboxPlacementProviders: %s", liveRedact(err.Error()))
		}
		if _, err := GetInboxPlacementTestTypes(); err != nil {
			t.Fatalf("GetInboxPlacementTestTypes: %s", liveRedact(err.Error()))
		}
		if _, err := ListInboxPlacementTests(nil); err != nil {
			t.Fatalf("ListInboxPlacementTests: %s", liveRedact(err.Error()))
		}
		if _, err := GetInboxPlacementWebhook(); err != nil {
			t.Fatalf("GetInboxPlacementWebhook: %s", liveRedact(err.Error()))
		}
	})

	t.Run("the free calls did not change the credit balance", func(t *testing.T) {
		if after := liveCredits(t); after != before {
			t.Fatalf("credits went from %d to %d", before, after)
		}
	})
}

func TestLiveValidationSpend(t *testing.T) {
	requireValidationKey(t)
	if !liveSpend {
		t.Skip("ZEROBOUNCE_LIVE_SPEND not set to 1")
	}

	t.Run("Validate with the billable add-ons", func(t *testing.T) {
		// activity_data, verify_plus and domain_info are charged even for a
		// sandbox address, so this is not a free call.
		yes := true
		timeout := "30"
		if _, err := ValidateWithOptions(liveSandbox, ValidateOptions{
			Timeout:            timeout,
			ActivityData:       &yes,
			VerifyPlus:         &yes,
			DomainInfo:         &yes,
			GreylistProcessing: &yes,
			CallbackURL:        liveCallback,
		}); err != nil {
			t.Fatalf("ValidateWithOptions: %s", liveRedact(err.Error()))
		}
	})

	t.Run("ScoreEmail", func(t *testing.T) {
		if _, err := ScoreEmail(liveSandbox); err != nil {
			t.Fatalf("ScoreEmail: %s", liveRedact(err.Error()))
		}
	})

	t.Run("GetActivityData", func(t *testing.T) {
		if _, err := GetActivityData(liveSandbox); err != nil {
			t.Fatalf("GetActivityData: %s", liveRedact(err.Error()))
		}
	})
}

func TestLiveValidationChanges(t *testing.T) {
	requireValidationKey(t)
	if !liveChanges {
		t.Skip("ZEROBOUNCE_LIVE_CHANGES not set to 1")
	}

	value := fmt.Sprintf("zb-sdk-live-%d.example.com", time.Now().UnixNano())
	if _, err := AddFilter("allow", "domain", value); err != nil {
		t.Fatalf("AddFilter: %s", liveRedact(err.Error()))
	}
	// Removed even if the assertions below fail.
	defer func() {
		if _, err := DeleteFilter("allow", "domain", value); err != nil {
			t.Errorf("cleanup DeleteFilter: %s", liveRedact(err.Error()))
		}
	}()
}

func liveWarmup(t *testing.T) *WarmupClient {
	t.Helper()
	if liveWarmupKey == "" {
		t.Skip("ZEROBOUNCE_WARMUP_API_KEY not set")
	}
	client, err := NewWarmupClient(liveWarmupKey, "")
	if err != nil {
		t.Fatalf("NewWarmupClient: %s", liveRedact(err.Error()))
	}
	return client
}

func TestLiveWarmupReads(t *testing.T) {
	client := liveWarmup(t)

	reads := map[string]func() (map[string]interface{}, error){
		"ListSendingAccounts": client.ListSendingAccounts,
		"GetEngagementRules":  client.GetEngagementRules,
		"GetSeeds":            client.GetSeeds,
		"GetSeedCount":        client.GetSeedCount,
		"GetProgressToday":    client.GetProgressToday,
		"GetSentToday":        client.GetSentToday,
		"GetEmailsPerDay":     client.GetEmailsPerDay,
	}
	for name, call := range reads {
		t.Run(name, func(t *testing.T) {
			if _, err := call(); err != nil {
				t.Fatalf("%s: %s", name, liveRedact(err.Error()))
			}
		})
	}

	t.Run("ListFromAddresses", func(t *testing.T) {
		if _, err := client.ListFromAddresses(nil); err != nil {
			t.Fatalf("ListFromAddresses: %s", liveRedact(err.Error()))
		}
	})

	t.Run("DownloadSeeds returns CSV", func(t *testing.T) {
		csv, err := client.DownloadSeeds()
		if err != nil {
			t.Fatalf("DownloadSeeds: %s", liveRedact(err.Error()))
		}
		if strings.TrimSpace(csv) == "" || strings.HasPrefix(strings.TrimSpace(csv), "{") {
			t.Fatalf("expected CSV, got %q", liveRedact(csv[:min(80, len(csv))]))
		}
	})

	t.Run("LIVE CheckDomainDmarc needs an email address", func(t *testing.T) {
		if _, err := client.CheckDomainDmarc("example.com", ""); err == nil ||
			!strings.Contains(strings.ToLower(err.Error()), "email address") {
			t.Fatalf("expected a bare domain to be rejected, got %v", err)
		}
		if _, err := client.CheckDomainDmarc("sender@example.com", ""); err != nil {
			t.Fatalf("CheckDomainDmarc: %s", liveRedact(err.Error()))
		}
	})

	t.Run("an unknown resource carries the HTTP status", func(t *testing.T) {
		_, err := client.GetCustomEngagement("no-such-address@example.com")
		if err == nil {
			return
		}
		if strings.Contains(err.Error(), liveWarmupKey) {
			t.Fatal("the key leaked into the error message")
		}
		if apiErr, ok := err.(*ZBApiError); ok && apiErr.StatusCode == 0 {
			t.Fatal("no HTTP status on the error")
		}
	})
}

func TestLiveWarmupChanges(t *testing.T) {
	client := liveWarmup(t)
	if !liveChanges {
		t.Skip("ZEROBOUNCE_LIVE_CHANGES not set to 1")
	}

	address := fmt.Sprintf("zb-sdk-live-%d@example.com", time.Now().UnixNano())
	ignore := true
	rule := 0
	if _, err := client.AddSendingAccount(address, "self_managed", &WarmupSendingAccountOptions{
		EngagementRuleID: &rule,
		IgnoreConflict:   &ignore,
	}); err != nil {
		t.Fatalf("AddSendingAccount: %s", liveRedact(err.Error()))
	}
	// Removed even if a later assertion fails.
	defer func() {
		if err := client.DeleteSendingAccount(address); err != nil {
			t.Errorf("cleanup DeleteSendingAccount: %s", liveRedact(err.Error()))
		}
	}()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
