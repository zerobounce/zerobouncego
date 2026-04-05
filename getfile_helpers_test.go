package zerobouncego

import "testing"

func TestGetFileJSONIndicatesError(t *testing.T) {
	if !GetFileJSONIndicatesError(`{"success":false,"message":""}`) {
		t.Fatal("expected true")
	}
	if GetFileJSONIndicatesError(`{"file_id":"x"}`) {
		t.Fatal("expected false")
	}
}

func TestFormatGetFileErrorMessage(t *testing.T) {
	msg := FormatGetFileErrorMessage(`{"success":false,"message":"not ready"}`)
	if msg != "not ready" {
		t.Fatalf("got %q", msg)
	}
}
