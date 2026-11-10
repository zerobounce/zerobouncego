package zerobouncego

import (
	"os"
	"strings"
	"testing"
)

// integrationOptIn is the environment variable that lets the integration
// tests below call the live API. Without it they skip, so a plain
// `go test ./...` never reaches the network; `-short` skips them as well.
const integrationOptIn = "ZEROBOUNCE_INTEGRATION"

// requireIntegration skips a live-API test unless ZEROBOUNCE_INTEGRATION=1.
func requireIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv(integrationOptIn) != "1" {
		t.Skip("integration test calls the live API: set " + integrationOptIn + "=1 to run it")
	}
}

// requireIntegrationKey is requireIntegration plus a real key from
// ZEROBOUNCE_API_KEY (or ZERO_BOUNCE_API_KEY), set as the package key.
func requireIntegrationKey(t *testing.T) {
	t.Helper()
	requireIntegration(t)
	key := strings.TrimSpace(getAPIKeyFromEnv())
	if key == "" {
		t.Skip("integration test needs ZEROBOUNCE_API_KEY")
	}
	Initialize(key)
}

func TestValidate(t *testing.T) {
	requireIntegrationKey(t)
	for _, e := range emailsToValidate {

		r, err := ValidateWithTimeout(e.Email, SANDBOX_IP, "10")
		if err != nil {
			t.Error(err)
			continue
		}

		if r.FreeEmail != e.FreeEmail {
			t.Errorf("Email %s: FreeEmail: %v/%v", e.Email, r.FreeEmail, e.FreeEmail)
		}
	}
}
