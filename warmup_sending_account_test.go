package zerobouncego

import (
	"encoding/json"
	"math"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// addSendingAccountBody adds a zb_managed account with these maps and returns
// the JSON body that was sent, or the error.
func addSendingAccountBody(t *testing.T, content, sending map[string]interface{}) (map[string]interface{}, error) {
	t.Helper()
	var sent string
	responder, warmup := useWarmupResponder(t, "warmup-key", func(req *http.Request, body string) (*http.Response, error) {
		sent = body
		return textResponse(req, 200, "application/json", `{"success":true}`), nil
	})
	_, err := warmup.AddSendingAccount("a@example.com", "zb_managed", &WarmupSendingAccountOptions{
		IgnoreConflict: boolPtr(true),
		EmailContent:   content,
		ManagedSending: sending,
	})
	requests := responder.take()
	if err != nil {
		if len(requests) != 0 {
			t.Fatalf("a rejected argument still sent %d request(s)", len(requests))
		}
		return nil, err
	}
	if len(requests) != 1 {
		t.Fatalf("want one request, got %d", len(requests))
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(sent), &body); err != nil {
		t.Fatalf("body is not JSON: %q", sent)
	}
	return body, nil
}

// The documented zb_managed payload, every field set, goes out as given.
func TestAddSendingAccountSendsTheDocumentedZBManagedPayload(t *testing.T) {
	body, err := addSendingAccountBody(t,
		map[string]interface{}{
			"company_name":        "Acme",
			"company_website":     "https://acme.example",
			"company_description": "We make anvils.",
			"types_of_emails":     []string{"newsletter", "marketing"},
			"example_email":       "Hi, our new anvil is out.",
		},
		map[string]interface{}{
			"weekdays_only":     false,
			"timezone":          "Asia/Kolkata",
			"timezone_offset":   5.5,
			"warmup_start_date": "2026-03-04",
		})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"from_address":     "a@example.com",
		"onboarding_model": "zb_managed",
		"ignore_conflict":  true,
		"email_content": map[string]interface{}{
			"company_name":        "Acme",
			"company_website":     "https://acme.example",
			"company_description": "We make anvils.",
			"types_of_emails":     []interface{}{"newsletter", "marketing"},
			"example_email":       "Hi, our new anvil is out.",
		},
		"managed_sending": map[string]interface{}{
			"weekdays_only":     false,
			"timezone":          "Asia/Kolkata",
			"timezone_offset":   5.5,
			"warmup_start_date": "2026-03-04",
		},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body = %#v\nwant %#v", body, want)
	}
}

// timezone_offset is -11.0 to 14.0 hours, whole or fractional, in any Go
// numeric type: the docs' own example sends 3.0, and a config decoded from
// JSON holds float64. Only an int was accepted.
func TestAddSendingAccountAcceptsAnyNumericTimezoneOffsetInRange(t *testing.T) {
	accepted := map[string]interface{}{
		"int 3":         3,
		"float 3.0":     3.0,
		"float 5.5":     5.5,
		"int -11":       -11,
		"int 14":        14,
		"float -11.0":   -11.0,
		"float 14.0":    14.0,
		"int64 -3":      int64(-3),
		"float32 5.75":  float32(5.75),
		"uint8 9":       uint8(9),
		"json.Number 3": json.Number("3.5"),
	}
	for name, offset := range accepted {
		t.Run(name, func(t *testing.T) {
			body, err := addSendingAccountBody(t, nil, map[string]interface{}{"timezone_offset": offset})
			if err != nil {
				t.Fatalf("%v rejected: %v", offset, err)
			}
			sent := body["managed_sending"].(map[string]interface{})["timezone_offset"]
			if _, ok := sent.(float64); !ok {
				t.Fatalf("timezone_offset sent as %#v, want a JSON number", sent)
			}
		})
	}
	rejected := map[string]interface{}{
		"-11.5":       -11.5,
		"14.5":        14.5,
		"15":          15,
		"-12":         -12,
		"NaN":         math.NaN(),
		"+Inf":        math.Inf(1),
		"string 3":    "3",
		"bool":        true,
		"nil":         nil,
		"json.Number": json.Number("abc"),
	}
	for name, offset := range rejected {
		t.Run(name, func(t *testing.T) {
			_, err := addSendingAccountBody(t, nil, map[string]interface{}{"timezone_offset": offset})
			if err == nil || !strings.Contains(err.Error(), "timezone_offset") {
				t.Fatalf("%#v: err = %v, want a timezone_offset error", offset, err)
			}
		})
	}
}

// weekdays_only is a strict boolean and timezone a string, checked before a
// request like the other managed_sending fields.
func TestAddSendingAccountChecksWeekdaysOnlyAndTimezone(t *testing.T) {
	for name, sending := range map[string]map[string]interface{}{
		"weekdays_only string": {"weekdays_only": "false"},
		"weekdays_only int":    {"weekdays_only": 0},
		"timezone number":      {"timezone": 3},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := addSendingAccountBody(t, nil, sending); err == nil {
				t.Fatalf("%v was accepted", sending)
			}
		})
	}
	if _, err := addSendingAccountBody(t, nil, map[string]interface{}{"weekdays_only": true, "timezone": "Europe/Bucharest"}); err != nil {
		t.Fatal(err)
	}
}
