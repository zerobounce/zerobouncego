# Go ZeroBounce API
Go implementation for [ZeroBounce Email Validation API v2](https://www.zerobounce.net/docs/email-validation-api-quickstart/).

[Link to the original repo](https://github.com/antsanchez/gozerobounce)

**Version and tagging:** Version is set in `version.go` and kept in sync with git tags. The SDKs monorepo scripts (`tag-version.sh`, `check-untagged-bump-and-push.sh`, `commit-and-tag-version.sh`) read and update it when creating or bumping releases. pkg.go.dev indexes the module from GitHub tags automatically. For **v2 and later**, the Go module path must include `/v2` ([semantic import versioning](https://go.dev/ref/mod#major-version-suffixes)); v1.x uses `github.com/zerobounce/zerobouncego` without the suffix.

## Installation and Usage
```sh
go get github.com/zerobounce/zerobouncego/v2
```

This package uses the zero-bounce API which requires an API key. This key can be provided in three ways:
1. through an environment variable `ZEROBOUNCE_API_KEY` (or legacy `ZERO_BOUNCE_API_KEY`), loaded automatically in code
2. through an .env file that contains `ZEROBOUNCE_API_KEY` (see `.env.example`) and then calling the following method before usage:
```go
zerobouncego.LoadEnvFromFile()
```
3. by settings explicitly in code, using the following method, where a preferred URI can also be provided:
```go
zerobouncego.InitializeWithURI("mysecretapikey", ZB_API_URL_DEFAULT)
```

### Mocking / Other URI
If you need to use a mock service in your tests or otherwise use a different URI you can:
Set it in the .env file (and calling LoadEnvFromFile):
```bash
ZERO_BOUNCE_URI=        # optional, defaults to the production URI
ZERO_BOUNCE_BULK_URI=   # optional, defaults to the production bulk URI
```

Call the setter function (passing empty strings will keep current values):
```go
zerobouncego.SetURI(new_uri, new_bulk_uri)
```

## Generic API methods

```go
package main

import (
	"fmt"
	"time"

	"github.com/zerobounce/zerobouncego/v2"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")

	// Check your account's credits
	credits, error_ := zerobouncego.GetCredits()
	if error_ != nil {
		fmt.Println("Error from credits: ", error_.Error())
	} else {
		fmt.Println("Credits left:", credits.Credits())
	}

	// Check your account's usage of the API
	start_time := time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local)
	end_time := time.Now()
	usage, error_ := zerobouncego.GetApiUsage(start_time, end_time)
	if error_ != nil {
		fmt.Println("Error from API usage: ", error_.Error())
	} else {
		fmt.Println("Total API calls: ", usage.Total)
	}
}

```

## Validation

#### 1. Single email validation

```go
package main

import (
    "fmt"
    "os"
    "github.com/zerobounce/zerobouncego/v2"
)

func main() {

    zerobouncego.APIKey = "... Your API KEY ..."

	// For Querying a single E-Mail and IP
	// IP can also be an empty string
	// Timeout can also be specified
	response, error_ := zerobouncego.Validate("possible_typo@example.com", "123.123.123.123")
	timeoutResponse, timeoutError_ := zerobouncego.ValidateWithTimeout("possible_typo@example.com", "123.123.123.123", "10")

	if error_ != nil {
		fmt.Println("error occurred: ", error_.Error())
	} else {
		// Now you can check status
		if response.Status == zerobouncego.S_INVALID {
			fmt.Println("This email is valid")
		}

		// .. or Sub-status
		if response.SubStatus == zerobouncego.SS_POSSIBLE_TYPO {
			fmt.Println("This email might have a typo")
		}
	}
}
```

#### 2. Batch validation

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego/v2"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")

	emails_to_validate := []zerobouncego.EmailToValidate{
		{EmailAddress: "disposable@example.com", IPAddress: "99.110.204.1"},
		{EmailAddress: "invalid@example.com", IPAddress: "1.1.1.1"},
		{EmailAddress: "valid@example.com"},
		{EmailAddress: "toxic@example.com"},
	}

	response, error_ := zerobouncego.ValidateBatch(emails_to_validate)
	if error_ != nil {
		fmt.Println("error ocurred while batch validating: ", error_.Error())
	} else {
		fmt.Printf("%d successful result fetched and %d error results\n", len(response.EmailBatch), len(response.Errors))
		if len(response.EmailBatch) > 0 {
			fmt.Printf(
				"email '%s' has status '%s' and sub-status '%s'\n",
				response.EmailBatch[0].Address,
				response.EmailBatch[0].Status,
				response.EmailBatch[0].SubStatus,
			)
		}
	}
}
```

#### Bulk API v2 (validation and getfile)

Bulk validation and scoring target the v2 bulk API. Docs: [v2 send file](https://www.zerobounce.net/docs/email-validation-api-quickstart/v2-send-file), [v2 file status](https://www.zerobounce.net/docs/email-validation-api-quickstart/v2-file-status), [v2 get file](https://www.zerobounce.net/docs/email-validation-api-quickstart/v2-get-file).

Optional `CsvFile` fields for **validation** send only: `ReturnURL`, `AllowPhase2` (use a `*bool`; omit or leave nil to skip sending `allow_phase_2`). Status responses include `FilePhase2Status` when the API returns it.

Optional [v2 getfile](https://www.zerobounce.net/docs/email-validation-api-quickstart/v2-get-file) query parameters: `GetFileOptions` with `DownloadType` pointing at `DownloadTypePhase1`, `DownloadTypePhase2`, or `DownloadTypeCombined`, and `ActivityData` (`*bool`) for **validation** getfile only (`AiScoringResultWithOptions` does not send `activity_data`). Use `BulkValidationResultWithOptions`, `AiScoringResultWithOptions`, or `GenericResultFetchWithOptions`.

Non-CSV JSON error bodies—including some HTTP 200 responses with `"success": false`—produce an error; nothing is written to the result writer. For custom handling of raw bodies, use `GetFileJSONIndicatesError` and `FormatGetFileErrorMessage`.

```go
dt := zerobouncego.DownloadTypeCombined
ad := true
opts := &zerobouncego.GetFileOptions{DownloadType: &dt, ActivityData: &ad}
err := zerobouncego.BulkValidationResultWithOptions(fileID, w, opts)
```

#### 3. Bulk file validation

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego/v2"
)


func main() {
	zerobouncego.Initialize("... Your API KEY ...")
	import_file_path := "PATH_TO_CSV_TO_IMPORT"
	result_file_path := "PATH_TO_CSV_TO_EXPORT"

	file, error_ := os.Open(import_file_path)
	if error_ != nil {
		fmt.Println("error while opening the file: ", error_.Error())
		return
	}

	defer file.Close()
	csv_file := zerobouncego.CsvFile{
		File: file, HasHeaderRow: false, EmailAddressColumn: 1, FileName: "emails.csv",
		// ReturnURL: "https://example.com/callback", // optional
		// AllowPhase2: &trueVal, // optional *bool; validation send only (define var trueVal = true)
	}
	submit_response, error_ := zerobouncego.BulkValidationSubmit(csv_file, false)
	if error_ != nil {
		fmt.Println("error while submitting data: ", error_.Error())
		return
	}

	fmt.Println("submitted file ID: ", submit_response.FileId)
	var file_status *zerobouncego.FileStatusResponse
	file_status, _ = zerobouncego.BulkValidationFileStatus(submit_response.FileId)
	fmt.Println("file status: ", file_status.FileStatus)
	fmt.Println("completion percentage: ", file_status.Percentage(), "%")

	// wait for the file to get completed
	fmt.Println()
	fmt.Println("Waiting for the file to get completed ")
	var seconds_waited int = 1
	for file_status.Percentage() != 100. {
		time.Sleep(time.Duration(seconds_waited) * time.Second)
		if seconds_waited < 10 {
			seconds_waited += 1
		}

		file_status, error_ = zerobouncego.BulkValidationFileStatus(submit_response.FileId)
		if error_ != nil {
			fmt.Print()
			fmt.Print("error ocurred while polling for status: ", error_.Error())
			return
		}
		fmt.Printf("..%.2f%% ", file_status.Percentage())
	}
	fmt.Println()
	fmt.Println("File validation complete")

	// save validation result
	result_file, error_ := os.OpenFile(result_file_path, os.O_RDWR | os.O_CREATE, 0644)
	if error_ != nil {
		fmt.Println("error on creating result file: ", error_.Error())
		return
	}
	error_ = zerobouncego.BulkValidationResult(submit_response.FileId, result_file)
	defer result_file.Close()
	if error_ != nil {
		fmt.Println("error on fetch validation result: ", error_.Error())
		return
	}
	fmt.Printf("Saved validation result at path: %s\n", result_file_path)

	// delete result file, after saving
	delete_status, error_ := zerobouncego.BulkValidationFileDelete(file_status.FileId)
	if error_ != nil {
		fmt.Println("error on fetch file delete: ", error_.Error())
		return
	}
	fmt.Println(delete_status)
}

```

Example import file:
```csv
disposable@example.com
invalid@example.com
valid@example.com
toxic@example.com

```

Example export file:
```csv
"Email Address","ZB Status","ZB Sub Status","ZB Account","ZB Domain","ZB First Name","ZB Last Name","ZB Gender","ZB Free Email","ZB MX Found","ZB MX Record","ZB SMTP Provider","ZB Did You Mean"
"disposable@example.com","do_not_mail","disposable","","","zero","bounce","male","False","true","mx.example.com","example",""
"invalid@example.com","invalid","mailbox_not_found","","","zero","bounce","male","False","true","mx.example.com","example",""
"valid@example.com","valid","","","","zero","bounce","male","False","true","mx.example.com","example",""
"toxic@example.com","do_not_mail","toxic","","","zero","bounce","male","False","true","mx.example.com","example",""
"mailbox_not_found@example.com","invalid","mailbox_not_found","","","zero","bounce","male","False","true","mx.example.com","example",""
"failed_syntax_check@example.com","invalid","failed_syntax_check","","","zero","bounce","male","False","true","mx.example.com","example",""

```


#### 4. AI scoring

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego/v2"
)


func main() {
	zerobouncego.Initialize("... Your API KEY ...")
	zerobouncego.LoadEnvFromFile()
	import_file_path := "./emails.csv"
	result_file_path := "./validation_result.csv"

	file, error_ := os.Open(import_file_path)
	if error_ != nil {
		fmt.Println("error while opening the file: ", error_.Error())
		return
	}

	defer file.Close()
	csv_file := zerobouncego.CsvFile{
		File: file, HasHeaderRow: false, EmailAddressColumn: 1, FileName: "emails.csv",
		// ReturnURL is optional for scoring send as well when supported by the API.
	}
	submit_response, error_ := zerobouncego.AiScoringFileSubmit(csv_file, false)
	if error_ != nil {
		fmt.Println("error while submitting data: ", error_.Error())
		return
	}

	fmt.Println("submitted file ID: ", submit_response.FileId)
	var file_status *zerobouncego.FileStatusResponse
	file_status, _ = zerobouncego.AiScoringFileStatus(submit_response.FileId)
	fmt.Println("file status: ", file_status.FileStatus)
	fmt.Println("completion percentage: ", file_status.Percentage(), "%")

	// wait for the file to get completed
	fmt.Println()
	fmt.Println("Waiting for the file to get completed ")
	var seconds_waited int = 1
	for file_status.Percentage() != 100. {
		time.Sleep(time.Duration(seconds_waited) * time.Second)
		if seconds_waited < 10 {
			seconds_waited += 1
		}

		file_status, error_ = zerobouncego.AiScoringFileStatus(submit_response.FileId)
		if error_ != nil {
			fmt.Print()
			fmt.Print("error ocurred while polling for status: ", error_.Error())
			return
		}
		fmt.Printf("..%.2f%% ", file_status.Percentage())
	}
	fmt.Println()
	fmt.Println("File validation complete")

	// save validation result
	result_file, error_ := os.OpenFile(result_file_path, os.O_RDWR | os.O_CREATE, 0644)
	if error_ != nil {
		fmt.Println("error on creating result file: ", error_.Error())
		return
	}
	error_ = zerobouncego.AiScoringResult(submit_response.FileId, result_file)
	defer result_file.Close()
	if error_ != nil {
		fmt.Println("error on fetch validation result: ", error_.Error())
		return
	}
	fmt.Printf("Saved validation result at path: %s\n", result_file_path)

	// delete result file, after saving
	delete_status, error_ := zerobouncego.AiScoringFileDelete(file_status.FileId)
	if error_ != nil {
		fmt.Println("error on fetch file delete: ", error_.Error())
		return
	}
	fmt.Println(delete_status)
}

```


Example import file:
```csv
disposable@example.com
invalid@example.com
valid@example.com
toxic@example.com

```

Example export file:
```csv
"Email Address","ZeroBounceQualityScore"
"disposable@example.com","0"
"invalid@example.com","10"
"valid@example.com","10"
"toxic@example.com","2"

```

## Email Finder
Email Finder allows you to search for new business email addresses using our proprietary technologies

#### 1. Single Email finder

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego/v2"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")

	response, error_ := zerobouncego.FindEmail("example.com", "John", "", "Doe")
	if error_ != nil {
		fmt.Println(error_.Error())
		return
	}
	fmt.Println(response)
}
```

#### 2. Single domain finder

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego/v2"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")
	response, error_ := zerobouncego.DomainSearch("example.com")
	if error_ != nil {
		fmt.Println(error_.Error())
		return
	}
	fmt.Println(response)
}
```

## Testing

### Run tests with Docker
From the **parent repository root** (the folder that contains all SDKs and `docker-compose.yml`):

```bash
docker compose build go
docker compose run --rm go
```

This runs unit tests only (`-short` skips integration tests that need a real API key).

### Unit and integration tests (local)
This package contains both unit tests and integration tests (which are excluded from the test suite). Unit test files are the ones ending in "_test.go" (as go requires) and the integration tests are ending in ("_integration_t.go").

In order to run the integration tests:
- set appropriate `ZEROBOUNCE_API_KEY` (or `ZERO_BOUNCE_API_KEY`) environment variable
- rename all "_integration_t.go" into "_integration_test.go"
- run either individual or all tests (`go test . -v`)

NOTE: currently, the unit tests can be updated such that, by removing the mocking and explicit API key setting, they should work as integration tests as well AS LONG AS a valid API key is provided via environment

## Publish

There is no npm-style “publish” command. [proxy.golang.org](https://proxy.golang.org) and [pkg.go.dev](https://pkg.go.dev/github.com/zerobounce/zerobouncego/v2) pull releases from **Git tags** on this repo.

### Release checklist (maintainers)

1. **`go.mod` `module` line must match the major version of the tag** ([semantic import versioning](https://go.dev/ref/mod#major-version-suffixes)):
   - Tags **`v1.x.x`** → module path **`github.com/zerobounce/zerobouncego`** (no major suffix).
   - Tags **`v2.x.x` and above** → module path **`github.com/zerobounce/zerobouncego/v2`** (must end with **`/v2`**). Consumer imports use the same path, e.g. `"github.com/zerobounce/zerobouncego/v2"`.
2. Bump **`version.go`** (`Version = "…"`) to match the release you are about to tag.
3. Commit, then create an **annotated** tag `vMAJOR.MINOR.PATCH` and push **`main`** and **`git push origin vMAJOR.MINOR.PATCH`**.

### Verify the release reached the proxy

After pushing the tag, confirm the proxy serves it (replace `v2` in the URL if you ever ship a v3 line with `/v3`):

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  "https://proxy.golang.org/github.com/zerobounce/zerobouncego/v2/@v/${TAG}.info"
```

Expect **`200`**. **`404`** here means the version is **not** installable with `go get`—almost always because the **`module` path and tag major version disagree** (e.g. `v2.1.0` tag with a module line missing `/v2`). That happened for **`v2.0.0`–`v2.1.0`**; **`v2.1.1`** was the first v2 tag published correctly.

You can also run `go list -m -versions github.com/zerobounce/zerobouncego/v2` locally.

### Reference

Monorepo maintainer notes: [sdk-docs/pkg-go.dev](../sdk-docs/pkg-go-dev/) in the SDKs repo.

