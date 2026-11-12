# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `BulkValidationSubmitWithOptions`, `AiScoringFileSubmitWithOptions` and `GenericFileSubmitWithOptions`, which take a `*SendFileOptions`. `remove_duplicate` is sent only when `SendFileOptions.RemoveDuplicate` is set, so the API's default (`true`) applies otherwise; a nil `*SendFileOptions` sends neither.

### Deprecated

- `BulkValidationSubmit`, `AiScoringFileSubmit` and `GenericFileSubmit`. Their `remove_duplicate bool` cannot be left out, so they always send it (the README example passed `false`, which turned off the API's default duplicate removal). They still compile and send the value given; use the `...WithOptions` functions above.

### Fixed

- The bulk downloads (`BulkValidationResult[WithOptions]`, `AiScoringResult[WithOptions]`, `GenericResultFetch[WithOptions]`, `BulkEmailFinderResult`, `BulkDomainSearchResult`) returned a nil error for an empty HTTP 200 body (or one of only whitespace), whatever its content type, and wrote nothing, so the caller was told an empty result file had been saved. It is now a `*ZBApiError` with `StatusCode` 200 and the message `Empty response from ZeroBounce (HTTP 200)`, and nothing is written. Affects 2.2.0 to 3.0.1.
- A 2xx that is not an API response (a `text/html` page, an HTML body under any content type, text that is not JSON, JSON that does not parse, or a download sent with a JSON content type) was a plain `error` with no status on every call except the Warmup ones. It is now a `*ZBApiError` with `StatusCode` set to that 2xx status, with the same message as before; `ErrorFromResponse` does the same. A body that cannot be read on a 2xx is still a plain error. Code that treated "not a `*ZBApiError`" as "the API never answered" now sees these as API errors. `ListFilters` (2.2.0 to 3.0.1) and `DeleteInboxPlacementTest` (3.0.1) returned malformed JSON that starts with `{` or `[` (`{"success": tru`) as a success; it is now that error too, and on the other calls it carries the status instead of being a plain error. Affects 2.2.0 to 3.0.1.
- A bulk base URL with more than one trailing slash (`SetURI("", "https://host/v2//")`, or `BULK_URI` set that way) sent the bulk email finder and domain search calls, the S3 and SFTP/FTP file jobs and, after `SetURI`, the Inbox Placement calls to `/v2/<path>` and the list evaluator to `/v2/v2/listevaluator`. Every trailing slash is now stripped before `/v2`. Affects 2.2.0 to 3.0.1.

- `WarmupClient.AddSendingAccount` rejected a `ManagedSending["timezone_offset"]` that was not a Go `int` with `invalid parameter: timezone_offset (must be -11 to 14)`: `3.0` (the API docs' own example, and what a config decoded from JSON holds), a fractional offset such as `5.5`, an `int64` or a `json.Number`. It now accepts any Go numeric type and `json.Number` from -11 to 14, fractional included, and sends it unchanged; a value outside that range, NaN, an infinity or a non-number is rejected with `invalid parameter: timezone_offset (must be a number from -11 to 14)`. `ManagedSending["weekdays_only"]` must now be a `bool` and `ManagedSending["timezone"]` a `string`; anything else is rejected before the request (`"false"` was sent as a string). The documented `EmailContent` and `ManagedSending` keys are now listed in the `WarmupSendingAccountOptions` docs. Affects 2.2.0 to 3.0.1.
- `WarmupClient.DownloadSeeds` and `DownloadStatistics` returned an HTML page (a `text/html` 200, or an HTML body under any content type) and a JSON body sent with a non-JSON content type as the CSV text. They now return a `*ZBApiError` with the status (200), like an empty body or a JSON content type already did. Affects 2.2.0 to 3.0.1.

### Security

- An error built from a response body no longer carries the API key. A body that echoed the key (`{"error": "Invalid API key <key>"}` or `{"success": false, "message": "... <key>"}` on an HTTP 200, a 400 quoting the request URL with `?api_key=<key>`, a JSON error on a 4xx/5xx, a getfile JSON error, `ValidateBatch`'s error for address `all`) was passed on as the error text or `ZBApiError.Message` by every call except the Warmup ones, and by `ErrorFromResponse`. Every key a request has been sent with is now masked as `***` in such a message, as sent, URL-encoded and JSON-escaped, and so is any `api_key=` value. The Warmup client already masked its key and shares the same masking now.

## [3.0.1] - 2026-11-11

### Fixed

- The Inbox Placement Tester calls (`GetInboxPlacementProviders`, `GetInboxPlacementTestTypes`, `CreateInboxPlacementTest`, `GetInboxPlacementTest`, `ListInboxPlacementTests`, `DeleteInboxPlacementTest`, `GetInboxPlacementWebhook`, `SetInboxPlacementWebhook`) now check an HTTP 2xx body like every other call. In 2.2.0 and 3.0.0 an error-only body (`{"error": "Invalid API key"}`) or `{"success": false, ...}` came back as the result with a nil error; it is now a `*ZBApiError` with `StatusCode` 200 and the API's message. An HTML page or other non-JSON text on a 2xx was returned as the raw `encoding/json` error (`invalid character '<' looking for beginning of value`); it is now a plain error with the same short message as the other calls (`ZeroBounce returned an HTML error page ("<title>") instead of an API response.`). `DeleteInboxPlacementTest` now reads its response body too; an empty 2xx is still a success.

## [3.0.0] - 2026-11-12

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

## [2.1.3] - 2026-06-10

### Added

- `ValidateResponse.CatchallDomain`, read from `catchall_domain`.

## [2.1.1] - 2026-04-08

### Fixed

- The module path is `github.com/zerobounce/zerobouncego/v2`, so v2 tags resolve on proxy.golang.org.

## [2.1.0] - 2026-04-05

### Added

- `BulkValidationResultWithOptions` and `AiScoringResultWithOptions`, with `DownloadType` (`phase_1`, `phase_2`, `combined`) and `ActivityData` (validation `getfile` only).
- `CsvFile.AllowPhase2`, sent as `allow_phase_2` on validation `sendfile` only, and `CsvFile.ReturnURL`.
- `FileStatusResponse.FilePhase2Status`.

### Fixed

- Bulk `getfile` returns an error when the body is a JSON error, including HTTP 200 with `"success": false`, instead of saving that body as the results file.

## [2.0.2] - 2026-03-03

### Changed

- License the project under MIT.

## [2.0.0] - 2026-03-03

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
[2.1.3]: https://github.com/zerobounce/zerobouncego/compare/v2.1.1...v2.1.3
[2.1.1]: https://github.com/zerobounce/zerobouncego/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/zerobounce/zerobouncego/compare/v2.0.2...v2.1.0
[2.0.2]: https://github.com/zerobounce/zerobouncego/compare/v2.0.0...v2.0.2
[2.0.0]: https://github.com/zerobounce/zerobouncego/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/zerobounce/zerobouncego/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/zerobounce/zerobouncego/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/zerobounce/zerobouncego/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/zerobounce/zerobouncego/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/zerobounce/zerobouncego/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/zerobounce/zerobouncego/releases/tag/v1.0.0
