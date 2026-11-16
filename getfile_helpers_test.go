package zerobouncego

import "testing"

func TestGetFileJSONIndicatesError(t *testing.T) {
	if !GetFileJSONIndicatesError(`{"success":false,"message":""}`) {
		t.Fatal("expected true")
	}
	if !GetFileJSONIndicatesError("\uFEFF{\"success\":false,\"message\":\"File not found\"}") {
		t.Fatal("BOM-prefixed error object")
	}
	// Any JSON object is an error payload, never a result file.
	if !GetFileJSONIndicatesError(`{"file_id":"x"}`) {
		t.Fatal("expected true")
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
	if !shouldTreatGetFileBodyAsError("\uFEFF\uFEFF{\"error\":\"x\"}", "text/csv") {
		t.Fatal("two BOMs before a JSON object")
	}
	if !shouldTreatGetFileBodyAsError("{\"error\":\"x\"} trailing", "text/csv") {
		t.Fatal("JSON with trailing bytes")
	}
}

func TestFormatGetFileErrorMessage(t *testing.T) {
	msg := FormatGetFileErrorMessage(`{"success":false,"message":"not ready"}`)
	if msg != "not ready" {
		t.Fatalf("got %q", msg)
	}
	msg = FormatGetFileErrorMessage("\uFEFF{\"success\":false,\"message\":\"File not found\"}")
	if msg != "File not found" {
		t.Fatalf("BOM message: %q", msg)
	}
}
