package zerobouncego

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// testdata/strict-json-corpus.tsv is a byte-identical copy of
// sdk-docs/skills/zerobounce-sdk/references/strict-json-corpus.tsv (the shared
// strict JSON corpus, check E7). Copy it again when that file changes; do not
// edit it here. Each line is a verdict, a tab and a JSON-encoded string whose
// decoded value is the exact response body.
type corpusBody struct {
	accept bool
	body   string
}

func strictJSONCorpus(t *testing.T) []corpusBody {
	t.Helper()
	file, err := os.Open("testdata/strict-json-corpus.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var bodies []corpusBody
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 || (fields[0] != "ACCEPT" && fields[0] != "REJECT") {
			t.Fatalf("corpus line %q", line)
		}
		var body string
		if err := json.Unmarshal([]byte(fields[1]), &body); err != nil {
			t.Fatalf("corpus line %q: %v", line, err)
		}
		bodies = append(bodies, corpusBody{accept: fields[0] == "ACCEPT", body: body})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	accepted := 0
	for _, entry := range bodies {
		if entry.accept {
			accepted++
		}
	}
	if accepted < 5 || len(bodies)-accepted < 30 {
		t.Fatalf("corpus has %d ACCEPT and %d REJECT bodies", accepted, len(bodies)-accepted)
	}
	return bodies
}

// fixedBodyTransport answers every request with one JSON body, with the
// status the call takes as a success: 201 for a file submit (GenericFileSubmit
// and the calls built on it accept nothing else), 200 for the rest.
type fixedBodyTransport struct {
	body   string
	sent   int
	status int
}

func (f *fixedBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.sent++
	f.status = 200
	if strings.HasSuffix(req.URL.Path, "/sendfile") {
		f.status = 201
	}
	header := make(http.Header)
	header.Set("Content-Type", CONTENT_TYPE_JSON)
	return &http.Response{
		StatusCode:    f.status,
		Header:        header,
		Body:          io.NopCloser(strings.NewReader(f.body)),
		ContentLength: int64(len(f.body)),
		Request:       req,
	}, nil
}

// emptyBodySuccess are the calls that take a zero-length 2xx as a success
// with no result: every Inbox Placement call (open owner decision; the
// standard keeps it for the delete only) and every Warmup JSON call (the
// documented empty success). A body of whitespace is not empty.
func emptyBodySuccess(name string) bool {
	return strings.Contains(name, "InboxPlacement") || strings.HasPrefix(name, "WarmupClient.")
}

// Every JSON call, Warmup and raw-JSON (ListFilters, DoGetRequest) included,
// rejects every REJECT body of the shared corpus as an API error with the
// response's 2xx status, and takes every ACCEPT body as JSON: either a result, or the shape
// error for JSON that is valid but not the expected object.
func TestEveryJSONCallFollowsTheStrictJSONCorpus(t *testing.T) {
	bodies := strictJSONCorpus(t)
	// The file downloads, found by running every call, plus the Warmup CSV
	// downloads, which return the CSV as text.
	downloads := downloadCalls(t)
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	calls := publicRequestCalls(client, warmup)
	swept := 0
	for _, call := range calls {
		if downloads[call.name] || strings.HasPrefix(call.name, "WarmupClient.Download") {
			continue
		}
		swept++
		for _, entry := range bodies {
			verdict := "REJECT"
			if entry.accept {
				verdict = "ACCEPT"
			}
			t.Run(call.name+"/"+verdict+"/"+jsonQuoted(entry.body), func(t *testing.T) {
				fake.body = entry.body
				before := fake.sent
				err := call.call()
				if fake.sent == before {
					t.Fatal("no request reached the transport")
				}
				var apiErr *ZBApiError
				if entry.accept {
					if err != nil && !(errors.As(err, &apiErr) && apiErr.StatusCode == fake.status && apiErr.Message == unexpectedJSONShapeMessage) {
						t.Fatalf("valid JSON refused: %v", err)
					}
					return
				}
				if entry.body == "" && emptyBodySuccess(call.name) {
					return
				}
				if err == nil {
					t.Fatal("accepted a body that is not strict JSON")
				}
				if !errors.As(err, &apiErr) || apiErr.StatusCode != fake.status {
					t.Fatalf("err = %#v, want a *ZBApiError with status %d", err, fake.status)
				}
			})
		}
	}
	if swept != len(calls)-len(downloads)-2 || swept < 80 {
		t.Fatalf("swept %d of %d calls", swept, len(calls))
	}
}
