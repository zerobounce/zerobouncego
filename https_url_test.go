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
	prevURI, prevBulk := URI, BULK_URI
	t.Cleanup(func() {
		URI, BULK_URI = prevURI, prevBulk
	})

	SetURI("https://api-eu.zerobounce.net/v2/", "https://bulkapi.zerobounce.net/v2/")
	assert.Equal(t, "https://api-eu.zerobounce.net/v2/", URI)

	SetURI("http://evil.example/", "http://evil.example/")
	assert.Equal(t, "https://api-eu.zerobounce.net/v2/", URI)
}
