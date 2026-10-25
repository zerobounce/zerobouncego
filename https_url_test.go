package zerobouncego

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequireHTTPS(t *testing.T) {
	ok, err := requireHTTPS("https://api.zerobounce.net/v2/")
	assert.NoError(t, err)
	assert.Equal(t, "https://api.zerobounce.net/v2/", ok)

	_, err = requireHTTPS("http://example.com")
	assert.Error(t, err)

	_, err = requireHTTPS("file:///etc/passwd")
	assert.Error(t, err)

	empty, err := requireHTTPS("")
	assert.NoError(t, err)
	assert.Equal(t, "", empty)
}

func TestSetURIIgnoresNonHTTPS(t *testing.T) {
	restoreBaseURLs(t)

	assert.NoError(t, SetURI("https://api-eu.zerobounce.net/v2/", "https://bulkapi.zerobounce.net/v2/"))
	assert.Equal(t, "https://api-eu.zerobounce.net/v2/", URI)

	// A rejected value is reported rather than swallowed, and nothing moves.
	err := SetURI("http://evil.example/", "http://evil.example/")
	assert.ErrorContains(t, err, "https")
	assert.Equal(t, "https://api-eu.zerobounce.net/v2/", URI)
	assert.Equal(t, "https://bulkapi.zerobounce.net/v2/", BULK_URI)

	// A bad bulk URI leaves the good main URI alone too: neither is applied.
	err = SetURI("https://api-us.zerobounce.net/v2/", "http://evil.example/")
	assert.ErrorContains(t, err, "bulk")
	assert.Equal(t, "https://api-eu.zerobounce.net/v2/", URI)
}

// restoreBaseURLs puts the four process-wide base URLs back after a test.
func restoreBaseURLs(t *testing.T) {
	t.Helper()
	prevURI, prevBulk := URI, BULK_URI
	prevEvaluator, prevIPT := LIST_EVALUATOR_URI, INBOX_PLACEMENT_URI
	t.Cleanup(func() {
		URI, BULK_URI = prevURI, prevBulk
		LIST_EVALUATOR_URI, INBOX_PLACEMENT_URI = prevEvaluator, prevIPT
	})
}

// SetURI used to leave LIST_EVALUATOR_URI and INBOX_PLACEMENT_URI pointing at
// production, so a caller who redirected the SDK still sent the list evaluator
// and the Inbox Placement Tester to the live API.
func TestSetURIRebasesTheSeparateHosts(t *testing.T) {
	restoreBaseURLs(t)
	t.Setenv("ZERO_BOUNCE_LIST_EVALUATOR_URI", "")
	t.Setenv("ZERO_BOUNCE_INBOX_PLACEMENT_URI", "")

	assert.NoError(t, SetURI("", "https://bulk.example.test/v2/"))
	assert.Equal(t, "https://bulk.example.test/v2/listevaluator", LIST_EVALUATOR_URI)
	assert.Equal(t, "https://bulk.example.test/api/ipt", INBOX_PLACEMENT_URI)
}

// An explicit environment override is a choice of host, so SetURI keeps it.
func TestSetURIKeepsEnvironmentOverrides(t *testing.T) {
	restoreBaseURLs(t)
	t.Setenv("ZERO_BOUNCE_LIST_EVALUATOR_URI", "https://evaluator.example.test/v2/listevaluator")
	t.Setenv("ZERO_BOUNCE_INBOX_PLACEMENT_URI", "https://ipt.example.test/api/ipt")
	LIST_EVALUATOR_URI = "https://evaluator.example.test/v2/listevaluator"
	INBOX_PLACEMENT_URI = "https://ipt.example.test/api/ipt"

	assert.NoError(t, SetURI("", "https://bulk.example.test/v2/"))
	assert.Equal(t, "https://evaluator.example.test/v2/listevaluator", LIST_EVALUATOR_URI)
	assert.Equal(t, "https://ipt.example.test/api/ipt", INBOX_PLACEMENT_URI)
}

// Initialize used to overwrite URI with the hardcoded production host, which
// threw away a ZERO_BOUNCE_URI override.
func TestInitializeKeepsEnvironmentURI(t *testing.T) {
	restoreBaseURLs(t)
	prevKey := API_KEY
	t.Cleanup(func() { API_KEY = prevKey })

	t.Setenv("ZERO_BOUNCE_URI", "https://api.example.test/v2/")
	Initialize("a-key")
	assert.Equal(t, "https://api.example.test/v2/", URI)

	t.Setenv("ZERO_BOUNCE_URI", "")
	Initialize("a-key")
	assert.Equal(t, "https://api.zerobounce.net/v2/", URI)
}
