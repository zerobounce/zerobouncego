package zerobouncego

import (
	"reflect"
	"testing"
)

// remove_duplicate is sent only when the caller sets it: the API defaults it
// to true, and the old functions could not leave it out.
func TestSendfileSendsRemoveDuplicateOnlyWhenSet(t *testing.T) {
	submits := map[string]func(CsvFile, *SendFileOptions) error{
		"BulkValidationSubmitWithOptions": func(f CsvFile, o *SendFileOptions) error {
			_, err := BulkValidationSubmitWithOptions(f, o)
			return err
		},
		"AiScoringFileSubmitWithOptions": func(f CsvFile, o *SendFileOptions) error {
			_, err := AiScoringFileSubmitWithOptions(f, o)
			return err
		},
		"GenericFileSubmitWithOptions": func(f CsvFile, o *SendFileOptions) error {
			_, err := GenericFileSubmitWithOptions(f, ENDPOINT_FILE_SEND, o)
			return err
		},
	}
	options := map[string]*SendFileOptions{
		"":      nil,
		"unset": {},
		"false": {RemoveDuplicate: boolPtr(false)},
		"true":  {RemoveDuplicate: boolPtr(true)},
	}
	for name, submit := range submits {
		for want, opts := range options {
			t.Run(name+"/"+want, func(t *testing.T) {
				fake := useFake(t)
				Initialize("sendfile-key")
				if err := submit(csvUpload(), opts); err != nil {
					t.Fatal(err)
				}
				got, sent := onlyRequest(t, fake).Fields["remove_duplicate"]
				if want == "" || want == "unset" {
					if sent {
						t.Fatalf("remove_duplicate sent as %q though it was not set", got)
					}
					return
				}
				if got != want {
					t.Fatalf("remove_duplicate = %q (sent %v), want %q", got, sent, want)
				}
			})
		}
	}
	// The old functions still send the value they are given.
	for _, value := range []bool{false, true} {
		fake := useFake(t)
		Initialize("sendfile-key")
		if _, err := BulkValidationSubmit(csvUpload(), value); err != nil {
			t.Fatal(err)
		}
		if got := onlyRequest(t, fake).Fields["remove_duplicate"]; got != formatBoolField(value) {
			t.Fatalf("BulkValidationSubmit(%v) sent %q", value, got)
		}
	}
}

// Each request body below is pinned field by field, so renaming a wire field
// fails here.
func TestRequestBodiesUseTheDocumentedFieldNames(t *testing.T) {
	csvWithReturnURL := csvUpload()
	csvWithReturnURL.ReturnURL = "https://example.com/done"
	csvWithReturnURL.HasHeaderRow = true
	cases := []struct {
		name string
		call func(warmup *WarmupClient) error
		want map[string]string
	}{
		{"sendfile", func(*WarmupClient) error {
			_, err := BulkValidationSubmitWithOptions(csvWithReturnURL, &SendFileOptions{RemoveDuplicate: boolPtr(true)})
			return err
		}, map[string]string{
			"api_key": "fields-key", "file": "<file>", "email_address_column": "1", "has_header_row": "true",
			"remove_duplicate": "true", "return_url": "https://example.com/done",
		}},
		{"scoring sendfile", func(*WarmupClient) error {
			_, err := AiScoringFileSubmitWithOptions(csvWithReturnURL, nil)
			return err
		}, map[string]string{
			"api_key": "fields-key", "file": "<file>", "email_address_column": "1", "has_header_row": "true",
			"return_url": "https://example.com/done",
		}},
		{"inbox placement create", func(*WarmupClient) error {
			_, err := CreateInboxPlacementTest("t", &CreateInboxPlacementTestOptions{
				TestType: intPtr(1), SenderEmail: "news@example.com", ProviderIDs: []int{1, 2}, SeedIDs: []int{3},
				WebhookURL: "https://example.com/hook",
			})
			return err
		}, map[string]string{
			"name": "t", "testType": "1", "senderEmail": "news@example.com", "providerIds": "[1,2]", "seedIds": "[3]",
			"webhookUrl": "https://example.com/hook",
		}},
		{"warmup update", func(w *WarmupClient) error {
			_, err := w.UpdateSendingAccount("a@example.com", 2, 4, true)
			return err
		}, map[string]string{
			"team_id": "", "from_address": "a@example.com", "engagement_rule_id": "2", "status_id": "4", "auto_enable": "true",
		}},
		{"warmup change from address", func(w *WarmupClient) error {
			return w.ChangeFromAddress("a@example.com", "b@example.com")
		}, map[string]string{
			"team_id": "", "old_from_address": "a@example.com", "new_from_address": "b@example.com",
		}},
		{"warmup smtp test", func(w *WarmupClient) error {
			_, err := w.TestSMTPConfiguration("a@example.com", "user", "secret", "smtp.example.com", 587)
			return err
		}, map[string]string{
			"from_address": "a@example.com", "username": "user", "password": "secret", "server": "smtp.example.com", "port": "587",
		}},
		{"warmup custom engagement", func(w *WarmupClient) error {
			_, err := w.CreateCustomEngagement("a@example.com", 10, 20, 30)
			return err
		}, map[string]string{
			"c_from": "a@example.com", "reply_pct": "10", "important_pct": "20", "click_pct": "30",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := useFake(t)
			Initialize("fields-key")
			INBOX_PLACEMENT_URI = "https://bulkapi.example.test/api/ipt"
			warmup, err := NewWarmupClient("warmup-key", "https://warmup.example.test")
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.call(warmup); err != nil {
				t.Fatal(err)
			}
			got := onlyRequest(t, fake).Fields
			want := map[string]string{}
			for key, value := range tc.want {
				want[key] = value
			}
			// team_id is the SDK's own placeholder; pin the name, not the value.
			if _, ok := want["team_id"]; ok {
				want["team_id"] = got["team_id"]
				if _, sent := got["team_id"]; !sent {
					t.Fatal("team_id not sent")
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("fields = %v\nwant     %v", got, want)
			}
		})
	}
}
