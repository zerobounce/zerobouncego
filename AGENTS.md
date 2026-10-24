# AGENTS.md

Guidance for AI coding agents working in this repository.

## Project

`zerobouncego` is the ZeroBounce Go SDK. Callers set a process-wide API key with `Initialize` and call package functions, or they construct a `Client` with `New` when each user needs a separate key. The module path is `github.com/zerobounce/zerobouncego/v2`. Releases are git tags; pkg.go.dev indexes those tags. The Publish workflow creates the GitHub release and checks that proxy.golang.org can see the tag.

## Layout

- `version.go`: the only runtime version, `Version`.
- `api.go`: `Validate`, credits, and API usage.
- `batch_validate.go`: `ValidateBatch`.
- `file_validation.go`, `file_management.go`, `ai_scoring.go`: bulk validation and AI scoring files.
- `score_email.go`: real-time scoring.
- `email_finder.go`, `domain_search.go`, `activity_data.go`.
- `priority3_bulk.go`, `bulk_root_helpers.go`, `filters.go`: bulk finder, domain search, list evaluator, filters, and S3.
- `http_error.go`: turns HTTP failures, HTML pages, and empty or non-JSON bodies into a short `error`.
- `client.go`: `Client` / `New`.
- `offline_api_test.go`: offline suite. A fake `http.RoundTripper` records every request and answers without the network.
- `go.mod`: module path. It is not a version field.

## Commands

```bash
go test -short -count=1 .
go test -short -race .
```

CI (`.github/workflows/sdk_ci.yml`) runs `go test ./... -cover -short -v` on Go 1.21. `-short` skips tests that call the live API. That test run is the typecheck; there is no separate lint step.

## Conventions

- **Do not share the package-level client across users.** `API_KEY`, `URI`, and `BULK_URI` are process-wide. `Initialize` and `SetURI` replace them for every later package call. Concurrent package calls mix keys. Use `New(apiKey)` and `Client.Validate` when two keys must be in flight. `Client` does not read or write those globals.
- **Column indexes are 1-based.** `CsvFile` name, gender, and IP columns of `0` are left out. The email column is required and is sent as given, including `0`. `ImportCsvFile` treats column `0` as column `1`. `ValidateS3FileOptions.EmailAddressColumn` is `*int`: when it is set, `0` is sent. Bulk email finder and domain search reject domain column `0`.
- **Optional parameters are omitted when unset.** `Timeout == ""`, nil `ActivityData` / `VerifyPlus` / `AllowPhase2`, and nil S3 bools are not sent. Explicit `false` is sent as `false`. `EmailFinderCsvFile.HasHeaderRow` and `DomainSearchCsvFile.HasHeaderRow` are plain bools, so `false` is always sent.
- **Booleans on the wire are the strings `true` and `false`.**
- **`allow_phase_2` and the name, gender, and IP columns are validation `sendfile` only.** Scoring `sendfile` ignores them. `activity_data` on get-file is validation only.
- **`ValidateBatch` uses `api.zerobounce.net`**, not `bulkapi.zerobounce.net`.
- **Timeouts are checked before the request.** Single validate accepts 3–60 seconds. Batch validate accepts 10–120 seconds. `0` is a real value and is rejected. An empty string or a nil pointer is omitted.
- **An empty API key or a nil upload is rejected before any request.**
- **Failures are `error` values.** The message must be short: no raw HTML, no megabyte body, and no `Error: Error:` prefix. An account-level `ValidateBatch` error (`email_address` `all`) is a returned error. A per-address error stays on `ValidateBatchResponse.Errors` with a nil error.
- **Tests replace `http.DefaultTransport`.** The SDK client leaves `Transport` nil, so it uses that default, and it does not follow redirects. Do not call the live API from `go test -short`.
- Match the surrounding style. Do not reformat unrelated code.

## Changelog

`CHANGELOG.md` follows [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and the project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

- Add every user-visible change to `## [Unreleased]` in the same commit as the change.
- Group entries under these headings, in this order, omitting empty ones: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.
- Write for SDK users: name the function, option, or behaviour, and say what changed. Leave out CI-only changes, internal refactors, and fixes to code that was never released.
- At 2.y.z, a breaking change bumps the major version, a new function or option bumps the minor version, and fixes alone bump the patch version.

## Releasing

Publishing and pushing tags need a maintainer's explicit approval. Do not bump the version, tag, or publish unless they asked.

1. Move `[Unreleased]` entries under `## [x.y.z] - YYYY-MM-DD`, leave an empty `[Unreleased]` section, and update the compare links at the bottom of `CHANGELOG.md`.
2. Set the same version in `Version` in `version.go`. The publish workflow fails if it differs from the tag. Do not edit `go.mod` for a version bump. The module line must stay `github.com/zerobounce/zerobouncego/v2` for every 2.y.z tag.
3. `go test -short -count=1 .`
4. Tag `vx.y.z` on that commit and run the Publish workflow with that tag. It creates the GitHub release. proxy.golang.org indexes the tag; the workflow waits until `https://proxy.golang.org/github.com/zerobounce/zerobouncego/v2/@v/vx.y.z.info` returns 200.

- **Four base URLs.** `URI`, `BULK_URI` (ends in `/v2/`), plus `LIST_EVALUATOR_URI` and `INBOX_PLACEMENT_URI`. The last two are separate values rather than paths built from `BULK_URI`, and `SetURI` does not touch them. The S3/SFTP jobs use `bulkAPIRoot()`, which strips the `/v2`.
- **Two bulk shapes are easy to get wrong.** `ValidateSFTPFile`'s `FileName` is optional — leaving it empty uses the input bucket from the account's settings, and every other job requires it. `ExportSFTPFile` sends `export_path`, not the `export_bucket` that `ExportS3File` sends.
