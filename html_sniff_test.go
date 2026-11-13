package zerobouncego

import (
	"net/http"
	"testing"
)

// Each marker of the HTML sniff on its own. A page with no other HTML, served
// as text/csv, must not be saved as a result file: every download is an error
// with the status and writes nothing (standard.md §3, downloads).
func TestEachHTMLSniffMarkerAloneMakesEveryCSVDownloadAnError(t *testing.T) {
	downloads := downloadCalls(t)
	pages := []string{
		"<!DOCTYPE html>\nemail\na@b.c\n",
		"<!doctype html>\nemail\na@b.c\n",
		"<HTML>\nemail\na@b.c\n",
		"<html>\nemail\na@b.c\n",
		"<head>\nemail\na@b.c\n",
		"<body>\nemail\na@b.c\n",
	}
	for _, page := range pages {
		calls := sweepWith(t, func(req *http.Request) *http.Response {
			return textResponse(req, 200, "text/csv", page)
		})
		for _, call := range calls {
			if !downloads[call.name] {
				continue
			}
			t.Run(call.name, func(t *testing.T) {
				sweepDownloads.Reset()
				err := call.call()
				assertStatus200(t, err)
				if sweepDownloads.Len() != 0 {
					t.Fatalf("%q: wrote %d bytes", page, sweepDownloads.Len())
				}
			})
		}
	}
}
