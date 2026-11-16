package zerobouncego

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const maxJSONBodyBytes = 2 << 20

// oversizeBodyMessage reports a response larger than maxJSONBodyBytes. It is
// a *ZBApiError with the response's status, a 2xx included.
const oversizeBodyMessage = "ZeroBounce returned a response larger than 2 MiB, which is not a valid API response."

// ZBApiError is a failed API response. It carries the HTTP status so callers
// can tell a 403 (Cloudflare) from a 400/404 (API) or a 429 (rate limit).
// It may be wrapped, so match it with errors.As.
//
// The validation functions return it:
//
//   - for every response with an HTTP status above 299, a refused redirect
//     (3xx) included, whatever the body;
//   - for a 2xx whose JSON body reports a failure ({"success": false, ...}, or
//     an error-only body such as {"error": "Invalid API key ..."}), with
//     StatusCode set to that 2xx status. That includes a JSON body from a
//     file download (getfile) and ValidateBatch's account-level error (an
//     error for address "all").
//
// A 2xx that is not an API response at all (an HTML page, an empty body,
// non-JSON text, a download with a JSON content type or an empty body) is a
// ZBApiError too, with StatusCode set to that 2xx status. The Warmup client
// returns a ZBApiError for every failed response, whatever its status.
//
// A transport failure such as a timeout, a body that cannot be read, and an
// argument rejected before any request is sent are plain errors.
type ZBApiError struct {
	StatusCode int
	Message    string
}

func (e *ZBApiError) Error() string { return e.Message }

// redirectRefusedMessage reports a 3xx. Redirects are never followed: the API
// key travels with the request, and a redirect could send it to another host.
const redirectRefusedMessage = "ZeroBounce returned a redirect, which this client does not follow."

// sentKeys holds every API key a request has been built with: requireKey
// records each one before it is sent. A message built from a response body
// is cleaned of all of them, so a server or proxy that echoes the key (an
// error quoting the request URL, {"error": "Invalid API key <key>"}) does not
// hand it to the caller. It grows by one entry per distinct key.
var sentKeys sync.Map

// rememberKey records a key that is about to be sent.
func rememberKey(key string) {
	if key != "" {
		sentKeys.Store(key, struct{}{})
	}
}

// apiKeyParameter matches an api_key query or form value, whichever key it
// holds.
var apiKeyParameter = regexp.MustCompile(`(?i)(api_key=)[^&\s"'<>]+`)

// maskSecrets replaces every form of each secret in text with ***, composed
// from the encoders up to depth 2: {as sent, JSON-escaped (plain, and
// HTML-safe the way encoding/json writes &, < and >, each with and without
// \/ for /)} x {as is, URL-encoded with spaces as + and as %20}, the path
// escaping of the secret, and each JSON form escaped twice (a JSON error body
// quoting the JSON request body). Longer forms are replaced first, so a
// shorter one never leaves part of a longer one behind.
func maskSecrets(text string, secrets ...string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		text = maskForms(text, secretForms(secret))
	}
	return text
}

// maskForms replaces every occurrence of each form in text with ***, ignoring
// case under Unicode full case folding (so "ß" is found as "SS" and "ss", and
// "ﬁ" as "FI"). Text and forms are folded and the matches are mapped back to
// the original text, which is masked whole: a match that covers part of one
// original character masks that character. Overlapping matches become one ***.
func maskForms(text string, forms []string) string {
	for _, form := range forms {
		text = strings.ReplaceAll(text, form, "***")
	}
	folded, starts, ends, offsets := foldIndex(text)
	var spans [][2]int
	seen := map[string]bool{}
	for _, form := range forms {
		needle := foldString(form)
		if needle == "" || seen[needle] {
			continue
		}
		seen[needle] = true
		for from := 0; from < len(folded); {
			at := strings.Index(folded[from:], needle)
			if at < 0 {
				break
			}
			first := from + at
			last := first + len(needle) - 1
			spans = append(spans, [2]int{starts[runeAt(offsets, first)], ends[runeAt(offsets, last)]})
			from = first + 1
		}
	}
	if len(spans) == 0 {
		return text
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
	var out strings.Builder
	pos := 0
	for i := 0; i < len(spans); {
		start, end := spans[i][0], spans[i][1]
		for i++; i < len(spans) && spans[i][0] <= end; i++ {
			if spans[i][1] > end {
				end = spans[i][1]
			}
		}
		out.WriteString(text[pos:start])
		out.WriteString("***")
		pos = end
	}
	out.WriteString(text[pos:])
	return out.String()
}

// foldRune is the full case folding of one rune: the status-F table, else the
// smallest rune of its simple-folding orbit.
func foldRune(r rune, out *strings.Builder) {
	if multi, ok := fullCaseFolds[r]; ok {
		for _, m := range multi {
			out.WriteRune(simpleFold(m))
		}
		return
	}
	out.WriteRune(simpleFold(r))
}

func simpleFold(r rune) rune {
	smallest := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f < smallest {
			smallest = f
		}
	}
	return smallest
}

// foldString is s under full case folding.
func foldString(s string) string {
	var out strings.Builder
	for _, r := range s {
		foldRune(r, &out)
	}
	return out.String()
}

// foldIndex folds text and records, for every original rune, where it starts
// and ends in text and where its folding starts in the folded string.
func foldIndex(text string) (folded string, starts, ends, offsets []int) {
	var out strings.Builder
	for i := 0; i < len(text); {
		r, width := utf8.DecodeRuneInString(text[i:])
		starts = append(starts, i)
		ends = append(ends, i+width)
		offsets = append(offsets, out.Len())
		foldRune(r, &out)
		i += width
	}
	return out.String(), starts, ends, offsets
}

// runeAt is the index of the original rune whose folding holds byte offset.
func runeAt(offsets []int, offset int) int {
	return sort.Search(len(offsets), func(i int) bool { return offsets[i] > offset }) - 1
}

// containsFold reports whether text holds secret, ignoring case under full
// case folding.
func containsFold(text, secret string) bool {
	return strings.Contains(text, secret) || strings.Contains(foldString(text), foldString(secret))
}

// secretForms lists every form maskSecrets masks, longest first.
func secretForms(secret string) []string {
	queryForms := func(value string) []string {
		escaped := url.QueryEscape(value)
		return []string{escaped, strings.ReplaceAll(escaped, "+", "%20")}
	}
	// Upper and lower copies too: a proxy that echoes the key in another
	// case must not put it back in the error (lessons 11 and 31).
	seeds := []string{secret}
	if folded := strings.ToUpper(secret); folded != secret {
		seeds = append(seeds, folded)
	}
	if folded := strings.ToLower(secret); folded != secret {
		seeds = append(seeds, folded)
	}
	var forms []string
	for _, seed := range seeds {
		forms = append(forms, seed, url.PathEscape(seed))
		forms = append(forms, queryForms(seed)...)
		for _, escape := range jsonEscapers {
			escaped := escape(seed)
			forms = append(forms, escaped)
			forms = append(forms, queryForms(escaped)...)
			// A JSON error body quoting the JSON request body.
			forms = append(forms, escape(escaped))
		}
	}
	sort.SliceStable(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	return forms
}

// jsonEscapers write text as it sits inside a JSON string: HTML-safe (&, <
// and > as \u0026, \u003c and \u003e, as json.Marshal writes them) or plain,
// each with / as is or as \/.
var jsonEscapers = []func(string) string{
	func(text string) string { return jsonEscaped(text, true) },
	func(text string) string { return jsonEscaped(text, false) },
	func(text string) string { return strings.ReplaceAll(jsonEscaped(text, true), "/", `\/`) },
	func(text string) string { return strings.ReplaceAll(jsonEscaped(text, false), "/", `\/`) },
}

func jsonEscaped(text string, htmlSafe bool) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(htmlSafe)
	_ = encoder.Encode(text)
	quoted := strings.TrimSpace(out.String())
	return quoted[1 : len(quoted)-1]
}

// redactKeys masks every API key this process has sent, and any api_key
// parameter value, in a message built from a response body. A message that
// still holds a key in a decoded view is replaced (omitIfSecretSurvives).
func redactKeys(message string) string {
	keys := []string{API_KEY}
	sentKeys.Range(func(key, _ interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	masked := apiKeyParameter.ReplaceAllString(maskSecrets(message, keys...), "${1}***")
	return omitIfSecretSurvives(masked, keys...)
}

// credentialOmittedMessage replaces a message that still held a secret after
// masking.
const credentialOmittedMessage = "ZeroBounce's response was omitted from this error because it contained a credential."

// omitIfSecretSurvives closes redaction under encoding: maskSecrets replaces
// the spellings this package can generate, and any other spelling (lowercase
// percent escapes, another JSON escaper) is caught here by decoding what is
// left. When any view of text, decoded up to maxSecretDecodeLayers layers
// deep in any order (percent-decoding, percent-decoding with + as a space,
// JSON-unescaping with \/ and \uXXXX, a backslash or \xHH escape, a doubled
// quote, an HTML character reference), compared ignoring case, contains a secret, the whole text is
// dropped for credentialOmittedMessage.
func omitIfSecretSurvives(text string, secrets ...string) string {
	var present []string
	for _, secret := range secrets {
		if secret != "" {
			present = append(present, secret)
		}
	}
	if len(present) == 0 {
		return text
	}
	views := []string{text}
	seen := map[string]bool{text: true}
	for layer := 0; layer <= maxSecretDecodeLayers; layer++ {
		var next []string
		for _, view := range views {
			for _, secret := range present {
				if containsFold(view, secret) {
					return credentialOmittedMessage
				}
			}
			if layer == maxSecretDecodeLayers {
				continue
			}
			for _, decode := range secretDecoders {
				decoded := decode(view)
				if !seen[decoded] {
					seen[decoded] = true
					next = append(next, decoded)
				}
			}
		}
		views = next
	}
	return text
}

// maxSecretDecodeLayers is how deep omitIfSecretSurvives decodes: the SDK's
// own request encoding plus three layers a server or proxy may echo it in.
const maxSecretDecodeLayers = 4

// secretDecoders undo one layer of each encoding a server may echo a secret
// in. They are lenient: a sequence that does not decode is kept as it is.
var secretDecoders = []func(string) string{
	func(text string) string { return percentDecoded(text, false) },
	func(text string) string { return percentDecoded(text, true) },
	jsonUnescaped,
	quoteDecoded,
}

// quoteDecoded undoes one layer of quote spellings JSON does not: a
// backslash before a single quote, a \xHH byte, an HTML character reference
// (&#39;, &quot;), and a quote doubled the way SQL writes it. A secret that
// contains a quote comes back in these forms, and none of them is a spelling
// maskSecrets generates.
func quoteDecoded(text string) string {
	text = strings.ReplaceAll(text, `\'`, "'")
	text = hexByteEscapesDecoded(text)
	// html.UnescapeString matches a named entity in one case. &QUOT; is
	// unescaped and &APOS; is not, so a secret that contains ' comes back
	// when the entity name is upper-cased.
	text = html.UnescapeString(htmlNamedEntitiesFolded(text))
	text = strings.ReplaceAll(text, "''", "'")
	text = strings.ReplaceAll(text, `""`, `"`)
	return text
}

// htmlNamedEntitiesFolded lowercases the name of each &Name; entity.
// Numeric references (&#39;, &#x27;) are left as they are.
func htmlNamedEntitiesFolded(text string) string {
	if !strings.Contains(text, "&") {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '&' {
			out.WriteByte(text[i])
			continue
		}
		semi := strings.IndexByte(text[i:], ';')
		if semi <= 1 {
			out.WriteByte(text[i])
			continue
		}
		name := text[i+1 : i+semi]
		if name[0] == '#' || !lettersOnly(name) {
			out.WriteString(text[i : i+semi+1])
			i += semi
			continue
		}
		out.WriteByte('&')
		out.WriteString(strings.ToLower(name))
		out.WriteByte(';')
		i += semi
	}
	return out.String()
}

func lettersOnly(text string) bool {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c < 'A' || (c > 'Z' && c < 'a') || c > 'z' {
			return false
		}
	}
	return text != ""
}

// hexByteEscapesDecoded turns \xHH and \XHH into that byte. A sequence that
// is not two hex digits is left as it is.
func hexByteEscapesDecoded(text string) string {
	if !strings.Contains(text, `\x`) && !strings.Contains(text, `\X`) {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+3 < len(text) && (text[i+1] == 'x' || text[i+1] == 'X') {
			high, okHigh := hexValue(text[i+2])
			low, okLow := hexValue(text[i+3])
			if okHigh && okLow {
				out.WriteByte(high<<4 | low)
				i += 3
				continue
			}
		}
		out.WriteByte(text[i])
	}
	return out.String()
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func percentDecoded(text string, plusIsSpace bool) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '%' && i+2 < len(text) {
			high, okHigh := hexValue(text[i+1])
			low, okLow := hexValue(text[i+2])
			if okHigh && okLow {
				out.WriteByte(high<<4 | low)
				i += 2
				continue
			}
		}
		if c == '+' && plusIsSpace {
			out.WriteByte(' ')
			continue
		}
		out.WriteByte(c)
	}
	return out.String()
}

func jsonUnescaped(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '\\' || i+1 >= len(text) {
			out.WriteByte(c)
			continue
		}
		switch next := text[i+1]; next {
		case '"', '\\', '/':
			out.WriteByte(next)
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'u':
			if i+5 < len(text) {
				if code, err := strconv.ParseUint(text[i+2:i+6], 16, 16); err == nil {
					r := rune(code)
					// A surrogate pair (\ud83d\ude00) is one character.
					if utf16.IsSurrogate(r) && i+11 < len(text) && text[i+6] == '\\' && text[i+7] == 'u' {
						if low, err := strconv.ParseUint(text[i+8:i+12], 16, 16); err == nil {
							if pair := utf16.DecodeRune(r, rune(low)); pair != unicode.ReplacementChar {
								out.WriteRune(pair)
								i += 11
								continue
							}
						}
					}
					out.WriteRune(r)
					i += 5
					continue
				}
			}
			out.WriteByte(c)
			continue
		default:
			out.WriteByte(c)
			continue
		}
		i++
	}
	return out.String()
}

// statusError is the error for a response that is not a usable API result: a
// *ZBApiError carrying the response's HTTP status, 2xx included (an HTML page
// or non-JSON text on a 200 is still the server's answer), with the API key
// masked (redactKeys).
func statusError(statusCode int, message string) error {
	message = redactKeys(message)
	if statusCode > 0 {
		return &ZBApiError{StatusCode: statusCode, Message: message}
	}
	return errors.New(message)
}

// apiError builds a ZBApiError. It never returns nil: it is called only on an
// error path, and a blank message (a body that was only an "Error:" prefix)
// becomes a fixed status-based one. The API key is masked in the message
// (redactKeys).
func apiError(statusCode int, message string) error {
	if strings.TrimSpace(message) == "" {
		message = noMessageText
	}
	return &ZBApiError{StatusCode: statusCode, Message: redactKeys(message)}
}

// RequestConfig is the API key and base URLs for one caller.
// Package functions use the process-wide key. A Client keeps its own copy.
type RequestConfig struct {
	APIKey  string
	URI     string
	BulkURI string
}

func packageConfig() RequestConfig {
	return RequestConfig{APIKey: API_KEY, URI: URI, BulkURI: BULK_URI}
}

func (c RequestConfig) requireKey() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return errors.New("api_key is required: call Initialize or New with a non-empty api_key")
	}
	if err := checkKeyText(c.APIKey); err != nil {
		return err
	}
	rememberKey(c.APIKey)
	return nil
}

const (
	apiKeyCharactersProblem = "invalid parameter: api_key has whitespace, control or invisible characters"
	apiKeyHeaderProblem     = "invalid parameter: api_key has characters that cannot be sent in an HTTP header"
)

// invisibleRune reports the invisible runes that are not whitespace, control
// or format (Cf) characters: the default ignorables, variation selectors,
// Hangul and braille blanks, and the Mongolian free variation selectors and
// vowel separator.
func invisibleRune(r rune) bool {
	switch r {
	case 0x034F, 0x115F, 0x1160, 0x17B4, 0x17B5, 0x2065, 0x2800, 0x3164, 0xFFA0:
		return true
	}
	return (r >= 0x180B && r <= 0x180F) || (r >= 0xFE00 && r <= 0xFE0F) ||
		(r >= 0xFFF0 && r <= 0xFFF8) || (r >= 0xE0000 && r <= 0xE0FFF)
}

// checkKeyText refuses a key that cannot be sent as an HTTP header value:
// whitespace of any kind (NBSP, U+0085, U+2028, U+3000, a space inside),
// control and Unicode format characters (U+200B, U+FEFF, U+180E, U+2060)
// anywhere in it, then anything outside printable ASCII. The HTTP library's
// own error for such a key would quote it unredacted.
func checkKeyText(key string) error {
	for _, r := range key {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zs, unicode.Zl, unicode.Zp) || invisibleRune(r) {
			return errors.New(apiKeyCharactersProblem)
		}
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 0x21 || key[i] > 0x7e {
			return errors.New(apiKeyHeaderProblem)
		}
	}
	return nil
}

// readLimitedBody reads a JSON response body of at most maxJSONBodyBytes. A
// larger body is a *ZBApiError with the response's status (a 500 stays a 500,
// a 2xx is an API error with that status); a body that cannot be read is a
// transport failure, returned as it is.
func readLimitedBody(reader io.Reader, statusCode int) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxJSONBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxJSONBodyBytes {
		return nil, &ZBApiError{StatusCode: statusCode, Message: oversizeBodyMessage}
	}
	return body, nil
}

// readDownloadBody reads a download response. A 2xx result file is returned
// whole, whatever its size (downloads are not capped). Every other body is
// read with the maxJSONBodyBytes limit, as readLimitedBody does, and so is a
// 2xx body that is not a file (HTML, or JSON, which the callers report as an
// error): an oversized one is a *ZBApiError with the response's status.
func readDownloadBody(reader io.Reader, statusCode int, contentType string) ([]byte, error) {
	if statusCode < 200 || statusCode > 299 {
		return readLimitedBody(reader, statusCode)
	}
	head, err := io.ReadAll(io.LimitReader(reader, maxJSONBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(head) <= maxJSONBodyBytes {
		return head, nil
	}
	// Past the cap: only a body that is a file stays uncapped.
	headText := string(head)
	if contentTypeIsHTML(contentType) || bodyLooksLikeHTML(headText) || shouldTreatGetFileBodyAsError(headText, contentType) {
		return nil, &ZBApiError{StatusCode: statusCode, Message: oversizeBodyMessage}
	}
	rest, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return append(head, rest...), nil
}

func bodyLooksLikeHTML(body string) bool {
	sample := body
	if len(sample) > 2000 {
		sample = sample[:2000]
	}
	lower := strings.ToLower(sample)
	return strings.Contains(lower, "<!doctype html") ||
		strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<head") ||
		strings.Contains(lower, "<body")
}

func contentTypeIsHTML(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/html")
}

// noMessageText is the message for an error whose text is blank once a
// leading "Error:" or "[Error]:" is removed (the other SDKs use the same text).
const noMessageText = "ZeroBounce returned an error with no message text."

// blankErrorMessage is the message for an empty response body.
func blankErrorMessage(status int) string {
	return fmt.Sprintf("Empty response from ZeroBounce (HTTP %d)", status)
}

func stripErrorPrefix(message string) string {
	for {
		trimmed := strings.TrimSpace(message)
		switch {
		case strings.HasPrefix(trimmed, "Error:"):
			message = strings.TrimSpace(trimmed[len("Error:"):])
		case strings.HasPrefix(trimmed, "[Error]:"):
			message = strings.TrimSpace(trimmed[len("[Error]:"):])
		default:
			return trimmed
		}
	}
}

// indexFoldASCII is strings.Index of an ASCII-lower-case needle in s, ignoring
// ASCII case, on the bytes of s as they are. (strings.ToLower can change a
// string's byte length, e.g. "İ", so an index into a lower-cased copy is not an
// index into the original.)
func indexFoldASCII(s, needle string) int {
	for i := 0; i+len(needle) <= len(s); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func htmlTitle(body string) string {
	start := indexFoldASCII(body, "<title")
	if start < 0 {
		return ""
	}
	openEnd := strings.Index(body[start:], ">")
	if openEnd < 0 {
		return ""
	}
	openEnd += start
	closeRel := indexFoldASCII(body[openEnd:], "</title>")
	if closeRel < 0 {
		return ""
	}
	return strings.TrimSpace(body[openEnd+1 : openEnd+closeRel])
}

// errorOnlyKeys only ever carry an error. "message" and "Message" are not
// among them: successful responses use those too (a bulk submit answers
// {"success": true, "message": "File Accepted"}, and a repeated greylist
// validate answers {"Message": "Greylist validation still in progress..."}).
var errorOnlyKeys = []string{"error", "error_message", "Error"}

// messageKeys are every key a message may sit under.
var messageKeys = []string{"error", "message", "error_message", "Message", "Error", "detail"}

func firstAPIErrorField(object map[string]interface{}) string {
	return apiErrorField(object, messageKeys)
}

// errorOnlyMessage is the API's own message when a 2xx body is an error with
// no "success" field to gate on. LIVE: an invalid or drained key answers
// validate, activity and getapiusage with {"error": "..."}, and the filters
// with {"Error": "..."}, all on an HTTP 200.
func errorOnlyMessage(object map[string]interface{}) string {
	if _, found := object["success"]; found {
		return ""
	}
	return apiErrorField(object, errorOnlyKeys)
}

// messageOnlyBody reports whether a JSON body is nothing but a message: an
// object with no "success" field whose keys are all message keys. The keys
// decide, whatever the message's value (a string, blank, null or a list), since
// no result object has these keys. An empty object {} is one too: it carries
// nothing. Only for calls whose result is never a bare
// message. LIVE: guessformat answers an invalid or drained key with an HTTP
// 200 and {"Message": "Invalid API key or your account ran out of credits"}.
// validate is not one of them: a repeated greylist validate answers with a
// bare Message, and that is a result.
func messageOnlyBody(body []byte) bool {
	var object map[string]interface{}
	if json.Unmarshal(body, &object) != nil || object == nil {
		return false
	}
	if _, found := object["success"]; found {
		return false
	}
	for key := range object {
		known := false
		for _, candidate := range messageKeys {
			if key == candidate {
				known = true
				break
			}
		}
		if !known {
			return false
		}
	}
	return true
}

func apiErrorField(object map[string]interface{}, keys []string) string {
	for _, key := range keys {
		value, ok := object[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case []interface{}:
			// The non-blank strings of the list, one per line.
			var lines []string
			for _, item := range typed {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					lines = append(lines, strings.TrimSpace(text))
				}
			}
			if len(lines) > 0 {
				return strings.Join(lines, "\n")
			}
		}
	}
	return ""
}

func messageFromBody(status int, contentType, body string) string {
	if status >= 300 && status < 400 {
		return redirectRefusedMessage
	}
	if bodyLooksLikeHTML(body) || contentTypeIsHTML(contentType) {
		title := htmlTitle(body)
		if title == "" {
			return "ZeroBounce returned an HTML error page instead of an API response."
		}
		return fmt.Sprintf("ZeroBounce returned an HTML error page (%q) instead of an API response.", title)
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return blankErrorMessage(status)
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		var parsed interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return "ZeroBounce returned a response that is not valid JSON."
		}
		if object, ok := parsed.(map[string]interface{}); ok {
			if title, ok := object["title"].(string); ok {
				accessDenied := strings.Contains(title, "Access denied") || strings.Contains(title, "1020")
				cloudflare, _ := object["cloudflare_error"].(bool)
				if accessDenied && cloudflare {
					return title
				}
			}
			if message := firstAPIErrorField(object); message != "" {
				if stripped := stripErrorPrefix(message); stripped != "" {
					return stripped
				}
				return noMessageText
			}
		}
		if len(trimmed) > 1000 {
			return "ZeroBounce returned a response that is not valid JSON."
		}
		if stripped := stripErrorPrefix(trimmed); stripped != "" {
			return stripped
		}
		return noMessageText
	}
	if len(trimmed) > 1000 {
		return "ZeroBounce returned a response that is not valid JSON."
	}
	message := stripErrorPrefix(trimmed)
	if message == "" {
		return noMessageText
	}
	return message
}

// ensureJSONAPIBody accepts a 2xx JSON document. Anything else becomes a short error.
func ensureJSONAPIBody(status int, contentType string, body []byte) error {
	success := status >= 200 && status < 300
	text := string(body)
	if !success || strings.TrimSpace(text) == "" || bodyLooksLikeHTML(text) || contentTypeIsHTML(contentType) {
		return statusError(status, messageFromBody(status, contentType, text))
	}
	trimmed := strings.TrimSpace(text)
	// Strict RFC 8259 JSON on the body as received: json.Valid allows only
	// space, tab, CR and LF around the value, and no BOM, comment, trailing
	// comma or value, NaN or other extension (testdata/strict-json-corpus.tsv).
	if !json.Valid(body) {
		if trimmed[0] != '{' && trimmed[0] != '[' {
			return statusError(status, messageFromBody(status, contentType, text))
		}
		return statusError(status, "ZeroBounce returned a response that is not valid JSON.")
	}
	// {"success": false, ...} on an HTTP 200 is a failure, not a result. The
	// live API answers an unknown file id that way, with a 200.
	if trimmed[0] == '{' {
		var probe map[string]interface{}
		if json.Unmarshal(body, &probe) == nil {
			if failed, found := probe["success"]; found && isFalsyAPISuccess(failed) {
				return apiError(status, messageFromBody(status, contentType, text))
			}
			if errorOnlyMessage(probe) != "" {
				return apiError(status, messageFromBody(status, contentType, text))
			}
		}
	}
	return nil
}

// isFalsyAPISuccess reports whether a "success" field means failure: every
// value a strict boolean reads as false (Python and PHP read the same list). The
// API sends it as a JSON false, "false" or "False" depending on the endpoint;
// 0 and "0" are failures too, whitespace around a string ignored.
func isFalsyAPISuccess(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return !typed
	case float64:
		return typed == 0
	case string:
		text := strings.ToLower(strings.TrimSpace(typed))
		return text == "false" || text == "0"
	}
	return false
}

func emptyJSONObject(body []byte) bool {
	// Whitespace inside the braces is still an empty object: `{ }`, a
	// newline, a tab. Comparing the trimmed text with "{}" misses those.
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(trimmed, &object) != nil || object == nil {
		return false
	}
	return len(object) == 0
}

// decodeJSONBody decodes a typed call's result. A body that is nothing but a
// message, or is an empty object, is an error with the response's status
// (standard.md §3): no typed result looks like that.
func decodeJSONBody(status int, contentType string, body []byte, dest interface{}) error {
	if err := ensureJSONAPIBody(status, contentType, body); err != nil {
		return err
	}
	if emptyJSONObject(body) {
		// Valid JSON of the wrong shape. The corpus accepts only this message
		// for an object that parses and is not the call's result.
		return statusError(status, unexpectedJSONShapeMessage)
	}
	if messageOnlyBody(body) {
		return apiError(status, messageFromBody(status, contentType, string(body)))
	}
	return decodeJSONBodyAllowMessage(status, contentType, body, dest)
}

// decodeValidateJSONBody is decodeJSONBody for validate: a repeated greylist
// validate answers a bare {"Message": "<text>"}, which is a result. Only that
// key, capitalised and with text; any other message-only body is an error.
func decodeValidateJSONBody(status int, contentType string, body []byte, dest interface{}) error {
	if err := ensureJSONAPIBody(status, contentType, body); err != nil {
		return err
	}
	if emptyJSONObject(body) {
		return statusError(status, unexpectedJSONShapeMessage)
	}
	if messageOnlyBody(body) && !greylistBody(body) {
		return apiError(status, messageFromBody(status, contentType, string(body)))
	}
	return decodeJSONBodyAllowMessage(status, contentType, body, dest)
}

// greylistBody reports whether a body is exactly {"Message": "<non-blank text>"}.
func greylistBody(body []byte) bool {
	var object map[string]interface{}
	if json.Unmarshal(body, &object) != nil || len(object) != 1 {
		return false
	}
	text, ok := object["Message"].(string)
	return ok && strings.TrimSpace(text) != ""
}

// decodeJSONBodyAllowMessage is decodeJSONBody without the message-only check,
// for the calls whose result may be a bare message or a free-form object (raw
// JSON filters, the list evaluator result, Inbox Placement).
func decodeJSONBodyAllowMessage(status int, contentType string, body []byte, dest interface{}) error {
	if err := ensureJSONAPIBody(status, contentType, body); err != nil {
		return err
	}
	// The body is valid JSON (ensureJSONAPIBody), so this fails only when it
	// is not the shape the call returns, e.g. a bare string or number.
	// encoding/json takes null as "leave dest as it is", which would hand back
	// a result of zero values; only a raw-JSON result (*interface{}) takes it.
	if _, raw := dest.(*interface{}); !raw && string(bytes.Trim(body, " \t\r\n")) == "null" {
		return statusError(status, unexpectedJSONShapeMessage)
	}
	// encoding/json already refuses a field of another JSON type; a result
	// with fields its call cannot answer without also checks those are there.
	if err := json.Unmarshal(body, dest); err != nil {
		return statusError(status, unexpectedJSONShapeMessage)
	}
	if checked, ok := dest.(requiredFieldsChecker); ok && !checked.hasRequiredFields(body) {
		return statusError(status, unexpectedJSONShapeMessage)
	}
	return nil
}

// requiredFieldsChecker is a typed result with fields its call cannot answer
// without (standard.md §3, "field types are part of the shape"). body is the
// JSON the result was decoded from, already known to decode into it.
type requiredFieldsChecker interface {
	hasRequiredFields(body []byte) bool
}

// validateRequiredFields are the fields a validate result cannot do without.
// A pointer stays nil when the key is missing or null.
type validateRequiredFields struct {
	Address *string `json:"address"`
	Status  *string `json:"status"`
	Message *string `json:"Message"`
}

// complete reports whether a result has its address and status. The greylist
// answer (exactly {"Message": "<text>"}) is accepted by the validate call only
// (see ValidateResponse.hasRequiredFields), never for a batch row.
func (f validateRequiredFields) complete() bool {
	return f.Address != nil && f.Status != nil
}

func getFileBodyError(status int, contentType, body string) error {
	// An empty 200 is not a result file: there is nothing to save. TrimSpace
	// keeps a BOM, so a body of only BOMs would otherwise be written out.
	stripped := trimBOMSpace(body)
	if bodyLooksLikeHTML(body) || contentTypeIsHTML(contentType) || status != 200 || stripped == "" {
		text := body
		if stripped == "" {
			text = ""
		}
		return statusError(status, messageFromBody(status, contentType, text))
	}
	if shouldTreatGetFileBodyAsError(body, contentType) {
		message := stripErrorPrefix(strings.TrimSpace(FormatGetFileErrorMessage(trimBOMSpace(body))))
		if message == "" || len(message) > 500 || bodyLooksLikeHTML(message) || message == trimBOMSpace(body) {
			message = messageFromBody(status, contentType, body)
		}
		// A JSON error body, or a JSON content type on a download, is the API
		// reporting a failure, so it carries the status like any other.
		return statusError(status, message)
	}
	return nil
}

// unexpectedJSONShapeMessage reports a 2xx body that is valid JSON but not
// the object or list the call returns (a bare string or number, say).
const unexpectedJSONShapeMessage = "ZeroBounce returned JSON that is not the expected response."
