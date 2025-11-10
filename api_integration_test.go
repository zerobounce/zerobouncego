package zerobouncego

import (
	"testing"
)

func TestValidate(t *testing.T) {
	Initialize("mock_key")
	for _, e := range emailsToValidate {

		r, err := Validate(e.Email, SANDBOX_IP)
		if err != nil {
			t.Errorf(err.Error())
		}

		if r.FreeEmail != e.FreeEmail {
			t.Errorf("Email %s: FreeEmail: %v/%v", e.Email, r.FreeEmail, e.FreeEmail)
		}
	}
}
