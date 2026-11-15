package zerobouncego

import "testing"

func TestGetFileJSONIndicatesError(t *testing.T) {
	if !GetFileJSONIndicatesError(`{"success":false,"message":""}`) {
		t.Fatal("expected true")
	}
	if !GetFileJSONIndicatesError("\uFEFF{\"success\":false,\"message\":\"File not found\"}") {
		t.Fatal("BOM-prefixed error object")
	}
	if GetFileJSONIndicatesError(`{"file_id":"x"}`) {
		t.Fatal("expected false")
	}
	if !shouldTreatGetFileBodyAsError(`{"file_id":"x"}`, "text/csv") {
		t.Fatal("any JSON object is a download error")
	}
	if !shouldTreatGetFileBodyAsError(`[{"error":"bad"}]`, "application/octet-stream") {
		t.Fatal("JSON array")
	}
	if !shouldTreatGetFileBodyAsError("\uFEFF{\"a\":1}", "text/plain") {
		t.Fatal("BOM before a JSON object")
	}
	if !shouldTreatGetFileBodyAsError("csv,body", "application/problem+json") {
		t.Fatal("+json content type")
	}
}

func TestFormatGetFileErrorMessage(t *testing.T) {
	msg := FormatGetFileErrorMessage(`{"success":false,"message":"not ready"}`)
	if msg != "not ready" {
		t.Fatalf("got %q", msg)
	}
}
