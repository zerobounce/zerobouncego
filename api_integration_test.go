package zerobouncego

import (
	"testing"
)

func TestValidate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	Initialize("mock_key")
	for _, e := range emailsToValidate {

		r, err := ValidateWithTimeout(e.Email, SANDBOX_IP, "10")
		if err != nil {
			t.Errorf(err.Error())
		}

		if r.FreeEmail != e.FreeEmail {
			t.Errorf("Email %s: FreeEmail: %v/%v", e.Email, r.FreeEmail, e.FreeEmail)
		}
	}
}
