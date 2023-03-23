package zerobouncego

import (
	"testing"
)


func TestValidate(t *testing.T) {
	ImportApiKeyFromEnvFile()
	for _, e := range emailsToValidate {

		r, err := Validate(e.Email, SANDBOX_IP)
		if err != nil {
			t.Errorf(err.Error())
		}

		if r.Status != e.Status || r.SubStatus != e.SubStatus || r.FreeEmail != e.FreeEmail {
			t.Errorf("Email %s: Status: %s/%s; SubStatus: %s/%s: FreeEmail: %v/%v", e.Email, r.Status, e.Status, r.SubStatus, e.SubStatus, r.FreeEmail, e.FreeEmail)
		}
	}
}
