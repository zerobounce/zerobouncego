# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `ValidateWithOptions` takes `Timeout`, `ActivityData`, `VerifyPlus`, and `UsePost` (form-encoded `POST /validate`). `ValidateResponse` reads `active_in_days` and `active_first_seen`.
- `ValidateBatchWithOptions` sends `timeout`, `activity_data`, and `verify_plus` when they are set.
- `ScoreEmail` (`GET /scoring`) and `ScoreEmailPost` (form-encoded `POST /scoring`).
- Bulk email finder (`BulkEmailFinderSubmit`, `BulkEmailFinderFileStatus`, `BulkEmailFinderResult`, `BulkEmailFinderFileDelete`).
- Bulk domain search (`BulkDomainSearchSubmit`, `BulkDomainSearchFileStatus`, `BulkDomainSearchResult`, `BulkDomainSearchFileDelete`).
- List evaluator (`ListEvaluatorSubmit`, `GetListEvaluatorResult`, `SendEvaluatedFile`).
- Filters (`AddFilter`, `ListFilters`, `DeleteFilter`).
- S3 bulk (`ValidateS3File`, `ScoreS3File`, `ExportS3File`, `ListS3Files`).
- `New` returns a `Client` that keeps its own API key. Package functions still use the process-wide key.

### Changed

- API requests time out after 120 seconds and do not follow redirects.
- `SetURI` leaves the current base URL in place when the new value is not `https://`.

### Fixed

- `ValidateBatch` posts to `api.zerobounce.net/v2/validatebatch` instead of `bulkapi.zerobounce.net`.
- A 403 HTML "Access denied" page is returned as a short error. An HTML page with status 200 is an error as well, including `BulkValidationResult`, `BulkEmailFinderResult`, and `BulkDomainSearchResult`, instead of a saved CSV or a JSON parse.
- Empty and non-JSON bodies are a short error. A repeated `Error:` / `[Error]:` prefix is removed.
- `ValidateBatch` returns an error for an account-level failure (HTTP 200, error for address `all`, such as an invalid API key or no credits). An error for one address stays on the successful result.
- Bulk email finder and domain search send `has_header_row` when it is false. Scoring `sendfile` does not send validation-only name, gender, or IP columns.
- Calls reject an empty API key, a nil upload, and a timeout outside the API range before a request is sent. Single validate accepts 3–60 seconds. Batch validate accepts 10–120 seconds.

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

[Unreleased]: https://github.com/zerobounce/zerobouncego/compare/v2.1.4...HEAD
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
