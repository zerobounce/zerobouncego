# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `BulkValidationSubmitWithOptions`, `AiScoringFileSubmitWithOptions` and `GenericFileSubmitWithOptions`, which take a `*SendFileOptions`. `remove_duplicate` is sent only when `SendFileOptions.RemoveDuplicate` is set, so the API's default (`true`) applies otherwise; a nil `*SendFileOptions` sends neither.
- `InitializeWithURIChecked(key, ZbApiURL) error`: like `InitializeWithURI`, and returns `invalid parameter: api url` for a value that is not a `ZB_API_URL_*` constant (key and URI unchanged).
- Runnable godoc examples (`example_test.go`): `ExampleNew` against a local test server, `ExampleClient_Validate`, `ExampleInitialize`, `ExampleZBApiError`.

### Changed

- The SDK now uses its own `*http.Transport` (cloned from `http.DefaultTransport`: proxy from the environment, TLS verification on) and a fresh `http.Client` with no cookie jar, the 120 s timeout and no redirect following. A global `http.DefaultClient.Jar` no longer receives or sends cookies for SDK calls. A replacement `http.DefaultTransport` (fake, httpmock) is still honoured.
- **Behaviour change:** API keys with an ordinary space inside or any non-ASCII character are refused too, on both clients (texts as the other SDKs: "api_key has whitespace, control or invisible characters" / "api_key has characters that cannot be sent in an HTTP header").
- `Initialize` and the exported URL variables cannot return an error; the send-time check is authoritative for them.
- Publish workflow: the tag input reaches shell code only through `env`; a second run waits instead of overlapping (`concurrency`); the tag must match `^v[0-9]+\.[0-9]+\.[0-9]+$`; the test job confirms it checked out the commit `validate-tag` validated, no longer uses the `release` environment, runs `go vet` and tests both the `go.mod` floor and Go 1.27 with `GOTOOLCHAIN=local`. The `setup-go` pin comment names its exact tag (`v7.0.0`).
- CHANGELOG: v2.0.0, v2.0.1, v2.0.2 and v2.1.0 are marked "tagged, never published" (go.mod path mismatch); added v2.0.1 and v2.1.4 entries and compare links; the v3.0.0 date is UTC (2026-11-11). AGENTS.md describes the two CI lanes and `go vet`.
- `GetFileJSONIndicatesError` is simplified with the same result (true for any valid JSON object or list); its doc comment now says so.

### Deprecated

- `BulkValidationSubmit`, `AiScoringFileSubmit` and `GenericFileSubmit`. Their `remove_duplicate bool` cannot be left out, so they always send it (the README example passed `false`, which turned off the API's default duplicate removal). They still compile and send the value given; use the `...WithOptions` functions above.

### Fixed

- **Behaviour change:** a 200 whose body is only a message (`message`, `Message`, `detail`, a blank or null message, a `null`, a number or an empty list as the message, `{"error": ""}`, `{"error":"x"}` or `{"error":[...]}`) or an empty object (`{}`, `{ }`, or braces with a newline or tab inside) was a zero-value result, or on `FindEmail*` and `DomainSearch*` a nil error with empty fields, whatever the message's value. It is now a `*ZBApiError` with `StatusCode` 200 on the typed calls (about 45): file submit, status and delete, the S3 and SFTP jobs, `SendEvaluatedFile`, list-evaluator submit, batch, activity, score, usage, validate and its variants (only `{"Message": "<non-blank text>"}` is validate's greylist result), email finder and domain search (an empty object is an error there too). Exempt: validate's greylist answer, the raw-JSON filter calls, the list-evaluator result, Inbox Placement and Warmup. Affects 2.2.0 to 3.0.1.
- `FileStatusResponse` now includes `error_reason`, which the file-status response sends and the previous struct dropped.
- **Behaviour change:** a bulk or Warmup download (`DownloadSeeds`, `DownloadStatistics`) whose body is JSON is now an error and nothing is written. Before, only an object with `success`/`message`/`error` keys under a non-JSON content type was caught; a UTF-8 BOM before the JSON (more than one BOM too), a JSON list, any other JSON object, or an `application/problem+json` / `*+json` content type was saved as the result file, and a BOM-prefixed JSON error was rejected with the raw body (the BOM included) instead of the message inside it. Now every leading BOM is stripped, the message is the one in the JSON, any JSON object or list is refused (also after a BOM), and `application/json` and `*/*+json` content types are refused. A body that starts with `{` or `[` but is not valid JSON is an error too, not a saved file. A bulk download body that is only BOMs was saved as the file; it is now an empty-body error, as the Warmup CSV downloads already treated it. `GetFileJSONIndicatesError` follows the same rules.
- `NewWarmupClient` accepts `http://` to `127.0.0.1`, `localhost` and `[::1]`, as the other base URLs already did, and refuses a URL with userinfo.
- `types_of_emails` in `EmailContent` accepts a `[]interface{}` of strings, which is what a JSON config decodes to, as well as a `[]string`.
- Setting `EngagementRuleID` on a `zb_managed` account is an error. It used to be ignored. `IgnoreConflict` is still sent for either onboarding model.
- A negative `http.DefaultClient.Timeout` is rejected before the request. Go would otherwise treat it as no timeout.
- `InitializeWithURI` with a value that is not one of the `ZB_API_URL_*` constants now leaves the key and URI unchanged. It set the URI to `""` before. No breaking change: the function keeps its signature (no return value); use `InitializeWithURIChecked` to get the error.
- The bulk downloads (`BulkValidationResult[WithOptions]`, `AiScoringResult[WithOptions]`, `GenericResultFetch[WithOptions]`, `BulkEmailFinderResult`, `BulkDomainSearchResult`) returned a nil error for an empty HTTP 200 body (or one of only whitespace), whatever its content type, and wrote nothing, so the caller was told an empty result file had been saved. It is now a `*ZBApiError` with `StatusCode` 200 and the message `Empty response from ZeroBounce (HTTP 200)`, and nothing is written. Affects 2.2.0 to 3.0.1.
- A 2xx that is not an API response (a `text/html` page, an HTML body under any content type, text that is not JSON, JSON that does not parse, or a download sent with a JSON content type) was a plain `error` with no status on every call except the Warmup ones. It is now a `*ZBApiError` with `StatusCode` set to that 2xx status, with the same message as before; `ErrorFromResponse` does the same. A body that cannot be read on a 2xx is still a plain error. Code that treated "not a `*ZBApiError`" as "the API never answered" now sees these as API errors. `ListFilters` (2.2.0 to 3.0.1) and `DeleteInboxPlacementTest` (3.0.1) returned malformed JSON that starts with `{` or `[` (`{"success": tru`) as a success; it is now that error too, and on the other calls it carries the status instead of being a plain error. Affects 2.2.0 to 3.0.1.
- A bulk base URL with more than one trailing slash (`SetURI("", "https://host/v2//")`, or `BULK_URI` set that way) sent the bulk email finder and domain search calls, the S3 and SFTP/FTP file jobs and, after `SetURI`, the Inbox Placement calls to `/v2/<path>` and the list evaluator to `/v2/v2/listevaluator`. Every trailing slash is now stripped before `/v2`. Affects 2.2.0 to 3.0.1.
- `WarmupClient.AddSendingAccount` rejected a `ManagedSending["timezone_offset"]` that was not a Go `int` with `invalid parameter: timezone_offset (must be -11 to 14)`: `3.0` (the API docs' own example, and what a config decoded from JSON holds), a fractional offset such as `5.5`, an `int64` or a `json.Number`. It now accepts any Go numeric type and `json.Number` from -11 to 14, fractional included, and sends it unchanged; a value outside that range, NaN, an infinity or a non-number is rejected with `invalid parameter: timezone_offset (must be a number from -11 to 14)`. `ManagedSending["weekdays_only"]` must now be a `bool` and `ManagedSending["timezone"]` a `string`; anything else is rejected before the request (`"false"` was sent as a string). The documented `EmailContent` and `ManagedSending` keys are now listed in the `WarmupSendingAccountOptions` docs. Affects 2.2.0 to 3.0.1.
- `WarmupClient.DownloadSeeds` and `DownloadStatistics` returned an HTML page (a `text/html` 200, or an HTML body under any content type) and a JSON body sent with a non-JSON content type as the CSV text. They now return a `*ZBApiError` with the status (200), like an empty body or a JSON content type already did. Affects 2.2.0 to 3.0.1.
- The Warmup JSON calls returned a `text/html` 2xx whose body parsed as JSON (`{"a":1}`) as the API's answer. It is now a `*ZBApiError` with that status and the message `ZeroBounce returned an HTML error page instead of an API response.`, like every other call. Affects 2.2.0 to 3.0.1.
- A 2xx body of only whitespace (`"   "`) was a success with a nil result on every Warmup JSON call (2.2.0 to 3.0.1) and every Inbox Placement call (2.2.0 to 3.0.1). It is not JSON, so it is now a `*ZBApiError` with that status, like on every other call. A zero-length body (and a 204) is still the documented empty success there.
- A 2xx body that is valid JSON but not an object or list (`"x"`, `0`) was refused by every JSON call except the Warmup ones, with the body text as the message. It is now taken as JSON (strict RFC 8259, checked on the body as received: only space, tab, CR and LF around the value; no BOM, comment, trailing comma or trailing value, `NaN` or other extension). `ListFilters` now returns it as the raw JSON. Where valid JSON does not fit the call's result type (a bare value, `null` on any call that does not return raw JSON, or `{}` where `ListS3Files` expects a list), the error is now a `*ZBApiError` with that status and the message `ZeroBounce returned JSON that is not the expected response.`; it was a plain error reading `ZeroBounce returned a response that is not valid JSON.`. Affects 2.2.0 to 3.0.1.
- A response body larger than 2 MiB lost its HTTP status on the JSON calls that read it before checking the status (`GetCredits`, `Validate`, `ValidateBatch` and the Inbox Placement calls among them): a 500 or a 200 over the cap was a plain `error` (`ZeroBounce returned a response that is not valid JSON.`) with no status. It is now a `*ZBApiError` with the response's status (a 500 stays 500, a 2xx is an API error with that status) and the message `ZeroBounce returned a response larger than 2 MiB, which is not a valid API response.`; `ErrorFromResponse` does the same. Affects 2.2.0 to 3.0.1.
- `GetCredits` returned a 200 whose `Credits` was missing, `null` or not a string holding an integer (`{}`, `{"Credits":null}`, `{"Credits":"abc"}`, `{"Credits":"5.0"}`) as a success, and `Credits()` then returned `-1`, the API's answer for an invalid key. `Validate`, `ValidateWithTimeout`, `ValidateWithOptions` and `Client.Validate` returned a 200 without `address` or `status` (`{}`, or either one `null`) as a result with empty strings, and `ValidateBatch[WithOptions]` did the same for such a row. Each is now a `*ZBApiError` with `StatusCode` 200 and `ZeroBounce returned JSON that is not the expected response.`. The greylist answer `{"Message": "..."}` is still a result, `{"Credits":"-1"}` still returns `-1`, and a field of another JSON type (`{"Credits":true}`, `{"status":5}`) was already an error. `CreditsResponse.CreditsRaw` and `Credits()` are unchanged. Affects 2.2.0 to 3.0.1.
- `GetCredits` refused `Credits` sent as a JSON integer, the form zerobounce.net's own example `{"Credits":2375323}` shows, with an error. It now accepts an integer sent either as a JSON string (`"-1"`, `"12345"`, as the live API sends it) or as a JSON integer; `CreditsRaw` holds the integer's text in both cases and `Credits()` returns it. A number that is not an integer (`5.5`, `5.0`, `1e3`), one out of `int` range, a string that is not an integer, any other JSON type, and a missing or `null` value are still a `*ZBApiError` with `StatusCode` 200. Affects 2.2.0 to 3.0.1.
- The sendfile, deletefile and bulk-root file-job calls that return `*FileValidationResponse` (`BulkValidationSubmit[WithOptions]`, `AiScoringFileSubmit[WithOptions]`, `GenericFileSubmit[WithOptions]`, `BulkValidationFileDelete`, `AiScoringFileDelete`, `GenericFileDelete`, the S3 and SFTP/FTP file jobs) returned a 2xx whose `message` was neither a string nor a list of strings (`5`, `true`, `{}`, `[5]`, `["a",5]`) as a result. It is now a `*ZBApiError` with that 2xx status and `ZeroBounce returned JSON that is not the expected response.`; a string, a list of strings and `null` are still results. Affects 2.2.0 to 3.0.1.
- **Behaviour change:** an empty 2xx body on the seven non-DELETE Inbox Placement calls is an error (DELETE with an empty 204/200 stays a success).
- **Behaviour change:** `"success": 0`, `"0"` and `" False "` (any case, whitespace ignored) are failures, like `false`.
- **Behaviour change:** an error message given as a list keeps every non-blank line, joined with a line break (before: only the first).
- `EmailToValidate.IPAddress` is omitted from the batch request when empty (before: `"ip_address":""`).
- HTML error-page titles are found on the original bytes, so a page with a character whose lower-casing changes the byte length (e.g. "İ") no longer garbles or loses the title.
- Multipart uploads: CR and LF are removed from `FileName` (header injection into the part); `"` and `\` stay escaped. Errors from writing multipart fields and the file part are now returned instead of ignored.
- The Dockerfile's test image (`docker build` then `docker run`) failed with `failed to initialize build cache at /.cache/go-build: permission denied`: it runs as user 65532 with `HOME=/`. `GOCACHE` is now `/tmp/go-build`, and the base image is pinned by digest. CI builds the image and runs the tests in it (new `docker` job in `sdk_ci.yml`); Dependabot keeps the digest current (Docker entry, `golang` major/minor ignored).
- **Behaviour change:** an error body whose message is blank after the `Error:` / `[Error]:` prefix is removed (`{"error":"Error:"}`, `{"Error":"Error: "}`, `{"message":"Error:"}`, `{"success":false,"message":"[Error]: "}`) was a success with zero values on about 60 typed calls (and `{"success":false,…}` on IPT and filter calls). Now it is a `*ZBApiError` with the response status and the message `ZeroBounce returned an error with no message text.`. `apiError` never returns nil.
- **Behaviour change:** a validate-batch row with only `Message` (no address and status) is refused; before it was accepted. Validate accepts the greylist answer only as exactly `{"Message": "<text>"}`; `{"Message":"x","foo":1}` and `{"Message":"x","status":"valid"}` are refused as before-shape errors (before: accepted).
- A download writer's error (`could not write into given file`, `could not read response body`) now wraps the cause with `%w`, so `errors.Is` / `errors.As` work.
- **Behaviour change:** a nil `io.Writer` (or a nil pointer inside it) passed to `BulkValidationResult`, `AiScoringResult`, `BulkEmailFinderResult`, `BulkDomainSearchResult`, `GenericResultFetch` and the `*WithOptions` variants sent the request and then panicked on the first write. It is now refused before anything is sent, with `empty parameter: file_writer`.
- **Behaviour change:** a list of messages in an error body (`{"error":["a","b"]}`) kept only its first item in the getfile calls (`FormatGetFileErrorMessage`) and the Warmup client; it is now every non-blank item joined with a newline, as on the main calls.
- **Behaviour change:** `ImportCsvFile` sent the full local path as the upload's file name; it now sends the base name (`emails.csv`).
- **Behaviour change:** the 2 MiB response cap now also applies to the bulk JSON calls, the filters, the priority 3 calls and the Warmup JSON calls (a larger body is the same size-cap `*ZBApiError` with the response's status). Successful file downloads, including the Warmup CSV downloads, are not capped.
- A non-2xx (or JSON/HTML-on-200) response body of a file download or of the Email Finder / Domain Search result download is now read with the 2 MiB limit (an 8 MiB error body was read whole before); result files on a 2xx stay uncapped.
- **Behaviour change:** a Warmup request that fails in the transport (a timeout, a cancelled context) now keeps `errors.Is(err, context.DeadlineExceeded)` and the `net.Error` methods (`Timeout()` true); before, it was rebuilt as a plain string error. Warmup still masks its key and the call's SMTP/IMAP secrets.

### Security

- A base URL with userinfo (`https://api.zerobounce.net@evil.example/v2`) was accepted by `SetURI`, a `Client`'s `URI` and `NewWarmupClient`, and the API key was sent to the host after the `@`. It is now refused before the request.
- An error built from a response body now also masks a secret when the server echoes it in a different case, including a percent-encoded copy of that case. This covers the API key, the Warmup key and the SMTP password.
- Redaction also decodes HTML entities (named, decimal, hex) and `\'` before checking for a secret, for the API key, the Warmup key and the SMTP password.
- An API key padded with a format character (U+200B, U+FEFF, U+180E, or any other Unicode format character), a C1 control, a Unicode space in the middle of the key (U+00A0, U+1680, U+2028, U+2029, U+3000), or an invisible character that is not a format character (U+034F, U+115F, U+1160, U+17B4, U+17B5, U+180B–U+180F, U+2065, U+2800, U+3164, U+FFA0, U+FFF0–U+FFF8, and variation selectors and tags U+E0000–U+E0FFF) is rejected before a request, on the main client and on Warmup.
- An error built from a response body now also drops a secret when a named HTML entity in it is upper-cased (`&APOS;` for a secret that contains `'`). `encoding/html` only unescapes that name in lower case.
- `LoadEnvFromFile` no longer prints a rejected `ZERO_BOUNCE_URI`, and it does not leave that URL in place. The parse error quotes the whole URL, and that URL can contain the API key. The message is now fixed.
- `json.Marshal`, `xml.Marshal` and `gob` of a `Client` or a `RequestConfig` no longer include the API key. `fmt` already printed it as `***`. `Client.APIKey` stays an exported string (documented on the type and in the README): printing the field itself, or a `Client` stored by value in your own struct, prints the key.
- `LoadEnvFromFile` no longer prints a malformed `.env` line. That line can contain the API key; the message is now fixed.
- The client refuses to send while `GODEBUG` turns on net/http's HTTP/2 frame log. That includes `http2debug=1` or `http2debug=2` anywhere in the value (so `http2debug=0,http2debug=2` still refuses), any other non-zero `http2debug`, and a log that was already on when the process started. That log writes the request path, and the API key is in it.
- An error built from a response body no longer carries the API key. A body that echoed the key (`{"error": "Invalid API key <key>"}` or `{"success": false, "message": "... <key>"}` on an HTTP 200, a 400 quoting the request URL with `?api_key=<key>`, a JSON error on a 4xx/5xx, a getfile JSON error, `ValidateBatch`'s error for address `all`) was passed on as the error text or `ZBApiError.Message` by every call except the Warmup ones, and by `ErrorFromResponse`. Every key a request has been sent with is now masked as `***` in such a message, as sent, URL-encoded and JSON-escaped, and so is any `api_key=` value. The Warmup client already masked its key and shares the same masking now.
- Redaction is now closed under encoding. After the known spellings of a secret are masked, the rest of an error message built from a response body is decoded up to four layers deep (the SDK's own request encoding plus three layers a server may echo it in) in every order (percent-decoding, percent-decoding with `+` as a space, JSON-unescaping including `\/` and `\uXXXX`, a backslash-escaped single quote, a `\xHH` byte, a doubled quote, and an HTML character reference such as `&#39;` or `&quot;`); if any decoded view still holds an API key, the Warmup key or the SMTP username or password, the whole message is replaced with `ZeroBounce's response was omitted from this error because it contained a credential.` A body that echoed a secret in a spelling the masking did not generate (lowercase percent escapes, a URL encoding of a URL encoding, a third layer, `\'`, `''` or `&#39;` for a secret that contains a quote) used to hand it to the caller, on every call. Applies to `ZBApiError.Message`, the Warmup client's errors and `ErrorFromResponse`.
- Secrets (API keys, and the Warmup SMTP/IMAP credentials) are now also masked in URL encoding with spaces as `%20` as well as `+`, in the HTML-safe JSON escaping `encoding/json` writes (`\u003c`, `\u003e`, `\u0026`), as the URL encoding of each JSON-escaped form, which is how a server echoes a URL-encoded JSON request body, and JSON-escaped twice (a JSON error body quoting the JSON request body). Longer forms are masked first, so no fragment of a secret is left behind.
- **Behaviour change:** base URLs are validated with net/url: https scheme (http only to loopback), a valid host (DNS name or IP/[IPv6]), a valid port, no userinfo, no whitespace/control/backslash. Checked in `SetURI`, `NewWarmupClient` and again on every send (`Client.URI`/`BulkURI`, package variables, evaluator/IPT, Warmup). Before: `https://api.zerobounce.net@evil.example/v2` sent the key to evil.example, and Warmup accepted `https://exa mple.com` and `https://`.
- `Client`, `RequestConfig` and `WarmupClient` print base URLs without userinfo.
- **Behaviour change:** a base URL with a query (`?x=1`), a bare `?` or a fragment (`#f`) is now refused by `SetURI`, `NewWarmupClient` and on every send (package variables, `Client.URI`/`BulkURI`, `LIST_EVALUATOR_URI`, `INBOX_PLACEMENT_URI`, the Warmup base). Before, it was accepted and produced `…/getcredits?x=1?api_key=KEY`, which a transport error printed with the key. The send-time check also refuses a request URL with a fragment, a second `?` or a malformed query.
- Error URLs now mask any `api_key=<value>` by pattern (case-insensitive, also `api%5Fkey`), as a backstop to masking the parsed query value.
- Redaction of a key echoed by a server matches under Unicode full case folding (`ß` as `SS`/`ss`, `ﬁ` as `FI`, `İ` as `i̇`), not only simple folding. Before, such an echo of the key was left in the error.
- `WarmupClient` keeps its key behind a function value, so a `WarmupClient` stored by value in another struct no longer prints the key with `%v`, `%+v`, `%#v`, `%s` or `%q`.
- `json`, `xml` and `gob` encodings of `Client` and `RequestConfig` mask the password of a base URL's userinfo, as `fmt` already did.
- A transport error from a caller's transport (for example httpmock's `no responder found for URL "..."`) that quotes the request (URL, headers, form, JSON or multipart body) no longer carries the API key, the Warmup key or an SMTP/IMAP secret in `err.Error()`, in any `fmt` verb or through `errors.Unwrap` on any request path (GET, form/JSON POST, multipart, Inbox Placement, Warmup). Every key the SDK has been given is masked, not only the one parsed from the request. `errors.Is` (for `context.DeadlineExceeded`, `context.Canceled`, `os.ErrDeadlineExceeded`, `net.ErrClosed`) and `Timeout()` still work.
- Printing, JSON/XML/gob-encoding or formatting a `Client` whose `URI` or `BulkURI` holds an `?api_key=<key>` query (any case) now masks the value as `REDACTED`.
- Transport errors now mask every credential the SDK knows, not only what it could parse out of the request: the `Authorization: Bearer` key (Inbox Placement), the key in `ValidateBatch`'s JSON body and in every multipart upload, the `api_key` query value under any spelling of the name (`?%61pi_key=`), the API-Key header and form body, and every key the SDK has sent. A caller's transport whose error dumps the request (`httputil.DumpRequestOut`) no longer leaks the key through `err.Error()`, `%v`, `%+v` or `%#v` on any call.
- `Client` / `RequestConfig` / `WarmupClient` printing: an `api_key` written as `%61pi_key` in a base URL is masked.

## [3.0.1] - 2026-11-11

### Fixed

- The Inbox Placement Tester calls (`GetInboxPlacementProviders`, `GetInboxPlacementTestTypes`, `CreateInboxPlacementTest`, `GetInboxPlacementTest`, `ListInboxPlacementTests`, `DeleteInboxPlacementTest`, `GetInboxPlacementWebhook`, `SetInboxPlacementWebhook`) now check an HTTP 2xx body like every other call. In 2.2.0 and 3.0.0 an error-only body (`{"error": "Invalid API key"}`) or `{"success": false, ...}` came back as the result with a nil error; it is now a `*ZBApiError` with `StatusCode` 200 and the API's message. An HTML page or other non-JSON text on a 2xx was returned as the raw `encoding/json` error (`invalid character '<' looking for beginning of value`); it is now a plain error with the same short message as the other calls (`ZeroBounce returned an HTML error page ("<title>") instead of an API response.`). `DeleteInboxPlacementTest` now reads its response body too; an empty 2xx is still a success.

## [3.0.0] - 2026-11-11

### Upgrading from 2.x

- **The module path is now `github.com/zerobounce/zerobouncego/v3`** (Go semantic import versioning: a new major needs a new path). Change your imports and run `go get github.com/zerobounce/zerobouncego/v3@v3.0.0`; `go get -u` on `/v2` stays on the 2.x line. The rest of this section lists the behaviour changes.

### Breaking

- Every request now refuses a URL that is not `https://` before anything is sent; the error contains `refusing to send the API key to a URL that is not https:// (http:// is allowed only for 127.0.0.1, localhost and [::1])`. In v2.2.0 only `SetURI` and the Warmup client checked: a base URL set through the `URI`, `BULK_URI`, `LIST_EVALUATOR_URI` or `INBOX_PLACEMENT_URI` variables, the `ZERO_BOUNCE_URI`, `ZERO_BOUNCE_BULK_URI`, `ZERO_BOUNCE_LIST_EVALUATOR_URI` or `ZERO_BOUNCE_INBOX_PLACEMENT_URI` environment variables, or a `Client`'s `URI` and `BulkURI` sent the API key over plain `http://` to any host. `SetURI` now also accepts `http://` to those loopback hosts. Upgrading: a test pointed at a plain-`http://` mock on another host must move it to `https://` or to a loopback address.
- An API key with surrounding whitespace or control characters is rejected before a request is sent, with `invalid parameter: api_key has surrounding whitespace or control characters`, as `NewWarmupClient` already did for the Warmup key. This covers `Initialize`, `InitializeWithURI`, `SetApiKey`, the environment variables and `New`. Upgrading: trim a key read from a file (a trailing newline is the usual cause).
- `MOCK_VALIDATE_RESPONSE` and `SingleTest` are no longer exported. They were the package's own test fixtures and now live in `_test.go` files, so `github.com/jarcoal/httpmock` is no longer compiled into programs that import this module. Upgrading: copy them into your own tests if you used them.

### Changed

- A file download (`BulkValidationResult`, `AiScoringResult`, `GenericResultFetch` and their `WithOptions` forms, `BulkEmailFinderResult`, `BulkDomainSearchResult`) that gets a JSON error body on an HTTP 200, and `ValidateBatch`'s account-level error (HTTP 200, an error for address `all`), now return a `*ZBApiError` with `StatusCode` 200 instead of a plain error, like every other JSON failure on a 2xx. The message text is unchanged.

### Fixed

- `Validate`, `ValidateWithTimeout`, `ValidateWithOptions` and `Client.Validate` no longer send an empty `ip_address` parameter when no IP address is given.

### Security

- `Client`, `WarmupClient` and `RequestConfig` print the API key as `***` with every `fmt` verb. `%v`, `%+v` and `%#v` printed the key in v2.2.0.

## [2.2.0] - 2026-11-11

### Breaking

Upgrading from 2.1.4:

- Go 1.21 or later is required. `go.mod` declares `go 1.21` (it said `go 1.16`); on Go 1.20 the build fails with `undefined: strings.ContainsFunc`. The import path is unchanged: `github.com/zerobounce/zerobouncego/v2`.
- API requests time out after 120 seconds, reading the response included, so a slow bulk results download now fails with a timeout error. Set a non-zero `http.DefaultClient.Timeout` before the call (and before `NewWarmupClient`) to allow longer.
- `SetURI` returns an `error` and applies nothing when either argument is rejected. Check it; a call used as a statement still compiles but keeps reaching the old host.
- An invalid or drained API key returns an error carrying the API's message instead of an empty result (`GetCredits` still returns `-1`).
- A response with an HTTP status above 299, a 3xx included (redirects are not followed), returns a `*ZBApiError`; match it with `errors.As`.
- `ValidateBatch` returns an error for an account-level failure (an error for address `all`).
- `ErrorFromResponse` and `DoGetRequest` return a short message instead of the raw body, and `DoGetRequest` treats more 2xx bodies as errors.
- An empty API key, a nil upload, and a timeout outside the API range are rejected before a request is sent.
- `Initialize` resets the base URL to `ZERO_BOUNCE_URI` when that is set.

Details for each are under Added, Changed and Fixed below.

### Added

- Warmup API client, for the [ZeroBounce Warmup API](https://www.zerobounce.net/docs/warmup-api). Warmup is a separate product with its own host, `WarmupDefaultBaseURL` (`https://warmup-api.zerobounce.net`), and its own team-scoped Warmup API key, which goes in an `API-Key` header rather than an `api_key` parameter. `NewWarmupClient(apiKey, baseURL)` returns a `*WarmupClient` bound to that key (an empty `baseURL` uses `WarmupDefaultBaseURL`; any other must be `https://`). It does not use `API_KEY`, `Initialize`, or `SetURI`, and the validation functions never send the Warmup key. Its 26 methods:
  - sending accounts: `ListSendingAccounts`, `GetSendingAccount`, `AddSendingAccount` (with `WarmupSendingAccountOptions`; `WarmupOnboardingModels` and `WarmupEmailTypes` list the accepted onboarding models and email types), `UpdateSendingAccount`, `DeleteSendingAccount`, `ChangeFromAddress`, `ListFromAddresses` (with `WarmupListFromAddressesOptions`);
  - engagement rules: `GetEngagementRules`, `SetEngagementRule`, `GetCustomEngagement`, `CreateCustomEngagement`, `UpdateCustomEngagement`, `DeleteCustomEngagement`;
  - seeds and statistics: `GetSeeds`, `DownloadSeeds`, `GetSeedCount`, `GetProgressToday`, `GetSentToday`, `GetEmailsPerDay`, `GetStatistics`, `GetStatisticsPerDay`, `GetStatisticsDetailed`, `DownloadStatistics`, `GetFromAddressStatistics`;
  - utilities: `CheckDomainDmarc`, `TestSMTPConfiguration`.

  JSON answers come back as a `map[string]interface{}` (a list is wrapped under `"data"`), the two downloads as CSV text, and a failed call as a `*ZBApiError` with the HTTP status (a 429, for example). Out-of-range arguments are rejected before the request.
- `ValidateWithOptions` takes `Timeout`, `ActivityData`, `VerifyPlus`, and `UsePost` (form-encoded `POST /validate`). `ValidateResponse` reads `active_in_days` and `active_first_seen`.
- `ValidateBatchWithOptions` sends `timeout`, `activity_data`, and `verify_plus` when they are set.
- `ScoreEmail` (`GET /scoring`) and `ScoreEmailPost` (form-encoded `POST /scoring`), returning `ScoreEmailResponse`. `ENDPOINT_SCORING` is their path.
- Bulk email finder (`BulkEmailFinderSubmit`, `BulkEmailFinderFileStatus`, `BulkEmailFinderResult`, `BulkEmailFinderFileDelete`). `BulkEmailFinderSubmit` needs `DomainColumn` and one of `FirstNameColumn` or `FullNameColumn`, and says which is missing before uploading the file.
- Bulk domain search (`BulkDomainSearchSubmit`, `BulkDomainSearchFileStatus`, `BulkDomainSearchResult`, `BulkDomainSearchFileDelete`). Both bulk email finder and bulk domain search always send `has_header_row`, `false` included.
- List evaluator (`ListEvaluatorSubmit`, `GetListEvaluatorResult`, `SendEvaluatedFile`) on its own base URL, `LIST_EVALUATOR_URI`. The file id is URL-encoded as a single path segment.
- Filters (`AddFilter`, `ListFilters`, `DeleteFilter`).
- S3 bulk (`ValidateS3File`, `ScoreS3File`, `ExportS3File`, `ListS3Files`, `EmailFinderS3File`, `DomainSearchS3File`). `ValidateS3FileOptions` and `ScoreS3FileOptions` take a `ReturnURL`, which every S3 and SFTP job requires: the API rejects one without it (`Provided return URL is not permitted: None`), so the SDK does too. `RemoveDuplicates` is sent as `remove_duplicates` when it is set. `ExportS3File`'s export bucket is optional and falls back to the bucket in your ZeroBounce settings, like `ExportSFTPFile`'s export path.
- SFTP/FTP bulk (`ValidateSFTPFile`, `ScoreSFTPFile`, `EmailFinderSFTPFile`, `DomainSearchSFTPFile`, `ExportSFTPFile`), sharing the new `BulkFileJobOptions`. `FileName` and `ReturnURL` are required by every job, `ValidateSFTPFile` included. The server, path and protocol come from your ZeroBounce settings. `ExportSFTPFile` takes `export_path` rather than the `export_bucket` `ExportS3File` sends.
- `ValidateOptions` accepts `DomainInfo`, `GreylistProcessing` and `CallbackURL`. `CallbackURL` needs `GreylistProcessing` set to true and is rejected without it, and when blank, before a request is sent.
- `ValidateResponse` carries `DomainWebsiteExists`, `DomainRegistrantCompanyName` and `Message`. `Message` is the API's `Message` key, returned instead of a result while a greylist retry is still running.
- Sub-status `ai_agent_mailbox`: `SS_AI_AGENT_MAILBOX` and `ValidateSubStatusAiAgentMailbox`.
- Inbox Placement Tester on `INBOX_PLACEMENT_URI`: `GetInboxPlacementProviders`, `GetInboxPlacementTestTypes`, `CreateInboxPlacementTest`, `GetInboxPlacementTest`, `ListInboxPlacementTests`, `DeleteInboxPlacementTest`, `GetInboxPlacementWebhook`, `SetInboxPlacementWebhook`. The API key goes in an `Authorization: Bearer` header, bodies are camelCase JSON, and unset fields are left out. A test name over 64 characters, a `TestType` outside 1|2|3, a type-1 test without a `SenderEmail`, a list `Status` other than `"running"` or `"finished"` (`IPT_STATUS_RUNNING`, `IPT_STATUS_FINISHED`), and a `PageSize` outside 1-100 are rejected before the request, so they cost no inbox-test credits.
- `LIST_EVALUATOR_URI` and `INBOX_PLACEMENT_URI`, the base URLs of the list evaluator and the Inbox Placement Tester. `SetURI` moves both onto the bulk host it is given. Set `ZERO_BOUNCE_LIST_EVALUATOR_URI` or `ZERO_BOUNCE_INBOX_PLACEMENT_URI` to pin either one; `SetURI` then leaves it alone.
- `New` returns a `Client` that keeps its own API key and base URLs (`APIKey`, `URI`, `BulkURI`). Its one method so far is `Validate`. Package functions still use the process-wide key. `RequestConfig` (an API key and base URLs) is exported as well; no exported function takes it yet.
- `ZBApiError` (`StatusCode`, `Message`). The validation functions return it for every response with an HTTP status above 299, a refused redirect included, and for a 2xx whose JSON body reports a failure; a 2xx HTML page or an empty or non-JSON body stays a plain error. The Warmup client returns it for every failed response. A transport failure such as a timeout is a plain error. Match it with `errors.As`.
- `DoPostFormRequest` posts a form-encoded body to the main API and decodes the JSON answer the way `DoGetRequest` does.

### Changed

- Requires Go 1.21 or later, and `go.mod` now says so. `NewWarmupClient` uses `strings.ContainsFunc`, added in Go 1.21; on Go 1.20 the build fails with `undefined: strings.ContainsFunc`. `go.mod` previously declared `go 1.16`, although every release since the first has needed at least Go 1.19 (`url.JoinPath`), so an older toolchain got a compile error instead of a version message. Every Go release before 1.21 is past end of life.
- API requests time out after 120 seconds and do not follow redirects; v2.1.4 had no timeout. The 120 seconds cover the whole exchange, reading the response included, so a bulk results download (`BulkValidationResult`, `AiScoringResult`, `GenericResultFetch` and their `WithOptions` forms, `BulkEmailFinderResult`, `BulkDomainSearchResult`) that takes longer on a slow connection now fails with a timeout error where it used to finish. A non-zero `http.DefaultClient.Timeout` replaces the 120 seconds; set it before the call (and before `NewWarmupClient`) to allow longer. A 3xx answer is an error, `ZeroBounce returned a redirect, which this client does not follow.`, carrying the 3xx status.
- `SetURI` returns an `error` instead of silently ignoring a malformed or non-`https://` value, and applies nothing when either argument is rejected. A call used as a statement still compiles; check the error to be told that an override was refused rather than keep reaching the old host.
- The exported `ErrorFromResponse` and `DoGetRequest` return different error text: the short message described under Fixed (the API's own message, or a one-line note for an HTML page, an empty body or a body that is not JSON) instead of v2.1.4's `error: ` followed by every JSON value, or the raw body. For a status above 299 the error is a `*ZBApiError`. `DoGetRequest` also treats more responses as errors: on a 2xx, an HTML page, an empty or non-JSON body, `{"success": false, ...}`, and a body that only carries an error (`{"error": ...}`). It accepts JSON on any 2xx, where v2.1.4 rejected every status but 200.

### Fixed

- An invalid or drained API key returned an empty result instead of an error. The API answers it with an HTTP 200 and no `success` field: `{"error": "Invalid API key or your account ran out of credits"}` from `Validate` and `GetActivityData`, `{"error": "Invalid API key"}` from `GetApiUsage`, `{"Error": ...}` from the filters, and `{"Message": ...}` from the email finder and domain search. Each now returns an error carrying that message; before, `Validate` came back with an empty status and `GetActivityData` with `Found: false`. A bare `Message` is still a result from `Validate`, which answers a repeated `greylist_processing` request with `{"Message": "Greylist validation still in progress. Please try again later."}`. `GetCredits` still returns `-1` for an invalid key, as documented.
- `Initialize` no longer discards a `ZERO_BOUNCE_URI` override. It still resets the base URL, but to that environment value when it is set, instead of always to the production host.
- The api-usage response now reads `sub_status_leading_period_removed` and `sub_status_alias_address`, two counters the API returns that were silently discarded.
- `ValidateBatch` posts to `api.zerobounce.net/v2/validatebatch` instead of `bulkapi.zerobounce.net`.
- A 403 HTML "Access denied" page is returned as a short error. An HTML page with status 200 is an error as well, instead of a JSON parse error, or a file `BulkValidationResult` saved as the results CSV.
- Empty and non-JSON bodies are a short error. A repeated `Error:` / `[Error]:` prefix is removed.
- `ValidateBatch` returns an error for an account-level failure (HTTP 200, error for address `all`, such as an invalid API key or no credits). An error for one address stays on the successful result.
- Scoring `sendfile` does not send validation-only name, gender, or IP columns.
- Calls reject an empty API key, a nil upload, and a timeout outside the API range before a request is sent. Single validate accepts 3–60 seconds. Batch validate accepts 10–120 seconds.
- A plain `go test ./...` no longer calls the live API. The three integration tests skip unless `ZEROBOUNCE_INTEGRATION=1` is set (and still skip under `-short`).

### Security

- Redirects are disabled, so a 3xx response cannot send the API key to another host.
- Error values no longer include the raw HTML access-denied page.

## [2.1.4] - 2026-08-14

### Changed

- Dependency and GitHub Actions bumps (Dependabot); `TestInvalidApiKey` is skipped in `-short` mode. No change to the library's behaviour.

## [2.1.3] - 2026-06-10

### Added

- `ValidateResponse.CatchallDomain`, read from `catchall_domain`.

## [2.1.1] - 2026-04-08

First v2 release the Go module proxy serves: v2.0.0, v2.0.1, v2.0.2 and v2.1.0 were tagged while `go.mod` still named the module without `/v2`, so `go get` could not resolve them.

### Fixed

- The module path is `github.com/zerobounce/zerobouncego/v2`, so v2 tags resolve on proxy.golang.org.

## [2.1.0] - 2026-04-05 (tagged, never published)

### Added

- `BulkValidationResultWithOptions` and `AiScoringResultWithOptions`, with `DownloadType` (`phase_1`, `phase_2`, `combined`) and `ActivityData` (validation `getfile` only).
- `CsvFile.AllowPhase2`, sent as `allow_phase_2` on validation `sendfile` only, and `CsvFile.ReturnURL`.
- `FileStatusResponse.FilePhase2Status`.

### Fixed

- Bulk `getfile` returns an error when the body is a JSON error, including HTTP 200 with `"success": false`, instead of saving that body as the results file.

## [2.0.2] - 2026-03-03 (tagged, never published)

### Changed

- License the project under MIT.

## [2.0.1] - 2026-03-03 (tagged, never published)

### Changed

- Repository workflows only (`sdk_ci.yml`, auto-assign). No change to the library.

## [2.0.0] - 2026-03-03 (tagged, never published)

### Added

- Named validation status and sub-status constants (`ValidateStatus*` and `ValidateSubStatus*`).

## [1.3.0] - 2026-01-26

### Added

- Sub-statuses `gold`, `role_based_accept_all`, `alternate`, `mx_forward`, `blocked`, and `allowed`.

## [1.2.1] - 2025-12-18

### Added

- `ValidateWithTimeout`.

## [1.2.0] - 2025-11-10

### Added

- `FindEmailByDomain*` and `FindEmailByCompany*` methods, and `DomainSearchByDomain` / `DomainSearchByCompanyName`.
- `InitializeWithURI` to select the default, US, or EU API base URL.
- Sub-status `accept_all`.

### Deprecated

- `FindEmail` and `DomainSearch`. Use the email-finder or domain-search methods instead.

## [1.1.1] - 2025-05-30

### Added

- `SetURI` to point requests at another base URL.

## [1.1.0] - 2023-09-04

### Added

- Email finder (`FindEmail`).

## [1.0.0] - 2023-04-12

### Added

- Initial release: `GetCredits`, `GetApiUsage`, `Validate`, `ValidateBatch`, bulk validation and AI scoring file methods, and `GetActivityData`.

[Unreleased]: https://github.com/zerobounce/zerobouncego/compare/v3.0.1...HEAD
[3.0.1]: https://github.com/zerobounce/zerobouncego/compare/v3.0.0...v3.0.1
[3.0.0]: https://github.com/zerobounce/zerobouncego/compare/v2.2.0...v3.0.0
[2.2.0]: https://github.com/zerobounce/zerobouncego/compare/v2.1.4...v2.2.0
[2.1.4]: https://github.com/zerobounce/zerobouncego/compare/v2.1.3...v2.1.4
[2.1.3]: https://github.com/zerobounce/zerobouncego/compare/v2.1.1...v2.1.3
[2.1.1]: https://github.com/zerobounce/zerobouncego/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/zerobounce/zerobouncego/compare/v2.0.2...v2.1.0
[2.0.2]: https://github.com/zerobounce/zerobouncego/compare/v2.0.1...v2.0.2
[2.0.1]: https://github.com/zerobounce/zerobouncego/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/zerobounce/zerobouncego/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/zerobounce/zerobouncego/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/zerobounce/zerobouncego/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/zerobounce/zerobouncego/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/zerobounce/zerobouncego/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/zerobounce/zerobouncego/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/zerobounce/zerobouncego/releases/tag/v1.0.0
