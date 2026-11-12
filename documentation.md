#### Installation and Usage
```sh
go get github.com/zerobounce/zerobouncego/v3
```

This package uses the zero-bounce API which requires an API key. This key can be provided in three ways:
1. through an environment variable `ZEROBOUNCE_API_KEY` (or legacy `ZERO_BOUNCE_API_KEY`), loaded automatically in code
2. through an .env file that contains `ZEROBOUNCE_API_KEY` and then calling the following method before usage:
```go
zerobouncego.LoadEnvFromFile()
```
3. by settings explicitly in code, using the following method, where a preferred URI can also be provided:
```go
zerobouncego.InitializeWithURI("mysecretapikey", zerobouncego.ZB_API_URL_DEFAULT)
```

`LoadEnvFromFile` loads `.env` and then calls `Initialize` with
`ZEROBOUNCE_API_KEY` (or `ZERO_BOUNCE_API_KEY`), so it replaces a key set earlier
with `Initialize` (with an empty one if neither variable is set). Use one or the
other. To use a second key alongside the package-level one, see `New` under
Validation.

#### Generic API methods

```go
package main

import (
	"fmt"
	"time"

	"github.com/zerobounce/zerobouncego/v3"
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

#### Validation

###### 1. Single email validation

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego/v3"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")

	// Validate one address. The IP address can be an empty string.
	response, error_ := zerobouncego.Validate("possible_typo@example.com", "123.123.123.123")
	if error_ != nil {
		fmt.Println("error occurred: ", error_.Error())
		return
	}

	// Now you can check the status
	switch response.Status {
	case zerobouncego.S_VALID:
		fmt.Println("This email is valid")
	case zerobouncego.S_INVALID:
		fmt.Println("This email is invalid")
	}

	// .. or the sub-status
	if response.SubStatus == zerobouncego.SS_POSSIBLE_TYPO {
		fmt.Println("This email might have a typo")
	}

	// A timeout (3-60 seconds) can also be specified
	timeoutResponse, error_ := zerobouncego.ValidateWithTimeout("possible_typo@example.com", "123.123.123.123", "10")
	if error_ != nil {
		fmt.Println("error occurred: ", error_.Error())
		return
	}
	fmt.Println("status:", timeoutResponse.Status)
}
```

###### 2. Batch validation

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego/v3"
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

###### Validation options and batch options

`ValidateWithOptions` takes the optional `/validate` parameters: `Timeout`
(3-60 seconds, as a string), `ActivityData`, `VerifyPlus`, `DomainInfo`,
`GreylistProcessing`, `CallbackURL`, and `UsePost` to send a form-encoded
`POST /validate` instead of a `GET`. A nil pointer is not sent.
`ValidateBatchWithOptions` sends `timeout` (10-120 seconds), `activity_data` and
`verify_plus` when they are set.

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego/v3"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")

	yes := true
	response, err := zerobouncego.ValidateWithOptions("valid@example.com", zerobouncego.ValidateOptions{
		Timeout:      "10",
		ActivityData: &yes, // fills ActiveInDays / ActiveFirstSeen
		VerifyPlus:   &yes,
		UsePost:      true,
	})
	if err != nil {
		fmt.Println("error occurred: ", err.Error())
		return
	}
	fmt.Println(response.Status, response.ActiveInDays.String)

	timeout := 30
	batch, err := zerobouncego.ValidateBatchWithOptions(
		[]zerobouncego.EmailToValidate{{EmailAddress: "valid@example.com"}},
		&zerobouncego.ValidateBatchOptions{Timeout: &timeout, ActivityData: &yes},
	)
	if err != nil {
		fmt.Println("error occurred: ", err.Error())
		return
	}
	fmt.Println(len(batch.EmailBatch), "results")
}
```

###### A client with its own key

`New` returns a `Client` that keeps its own API key and base URLs (`APIKey`,
`URI`, `BulkURI`), so two keys can be used side by side. It does not read or
change the package-level key set by `Initialize`. Its one method so far is
`Validate`; every other call uses the package-level key.

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego/v3"
)

func main() {
	client := zerobouncego.New("... Another API KEY ...")
	response, err := client.Validate("valid@example.com", "")
	if err != nil {
		fmt.Println("error occurred: ", err.Error())
		return
	}
	fmt.Println(response.Status)
}
```

###### AI scoring for one email

`ScoreEmail` calls `GET /scoring`; `ScoreEmailPost` sends the same request as a
form-encoded `POST`.

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego/v3"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")
	score, err := zerobouncego.ScoreEmail("valid@example.com")
	if err != nil {
		fmt.Println("error occurred: ", err.Error())
		return
	}
	fmt.Printf("%s scores %d\n", score.Email, score.Score)
}
```

###### Errors

Every failed call returns an `error`. When the API answered (an HTTP status
above 299, a refused redirect included, or a 2xx whose body is not a usable
result: a JSON body reporting a failure, an HTML page, an empty body, non-JSON
text, or a download with a JSON content type), that error is a
`*zerobouncego.ZBApiError` carrying `StatusCode`, so a Cloudflare 403 can be
told apart from an API 400 or 404 or a 429. A timeout, a body that could not be
read, and an argument rejected before the request are plain errors.

```go
package main

import (
	"errors"
	"fmt"

	"github.com/zerobounce/zerobouncego/v3"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")
	_, err := zerobouncego.Validate("valid@example.com", "")
	var apiErr *zerobouncego.ZBApiError
	switch {
	case err == nil:
		fmt.Println("ok")
	case errors.As(err, &apiErr) && apiErr.StatusCode == 403:
		fmt.Println("blocked before reaching the API:", apiErr.Message)
	case errors.As(err, &apiErr):
		fmt.Printf("API error (HTTP %d): %s\n", apiErr.StatusCode, apiErr.Message)
	default:
		fmt.Println("request failed:", err)
	}
}
```

Requests time out after 120 seconds, and the time covers reading the whole
response, so a large bulk results download on a slow connection can fail with a
timeout. A non-zero `http.DefaultClient.Timeout` is used instead of the 120
seconds: set it before the call (and before `NewWarmupClient`) to allow longer.
Redirects are never followed, because the API key travels with the request; a
3xx answer is the error `ZeroBounce returned a redirect, which this client does
not follow.`

###### Bulk API v2 (validation and getfile)

Bulk validation and scoring target the v2 bulk API. Docs: [v2 send file](https://www.zerobounce.net/docs/email-validation-api-quickstart/v2-send-file), [v2 file status](https://www.zerobounce.net/docs/email-validation-api-quickstart/v2-file-status), [v2 get file](https://www.zerobounce.net/docs/email-validation-api-quickstart/v2-get-file).

Optional `CsvFile` fields for **validation** send only: `ReturnURL`, `AllowPhase2` (`*bool`; nil omits `allow_phase_2`). Status responses include `FilePhase2Status` when present.

Optional getfile query parameters: `GetFileOptions` with `DownloadType` (`DownloadTypePhase1`, `DownloadTypePhase2`, `DownloadTypeCombined`) and `ActivityData` for **validation** getfile only. Use `BulkValidationResultWithOptions`, `AiScoringResultWithOptions`, or `GenericResultFetchWithOptions`.

JSON error bodies (including HTTP 200 with `"success": false`) return an error; no bytes are written to the result writer. Helpers: `GetFileJSONIndicatesError`, `FormatGetFileErrorMessage`.

###### 3. Bulk file validation

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego/v3"
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
	// remove_duplicate is sent only when set; the API's default is true.
	// Pass &zerobouncego.SendFileOptions{RemoveDuplicate: &keep} to choose.
	submit_response, error_ := zerobouncego.BulkValidationSubmitWithOptions(csv_file, nil)
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


###### 4. AI scoring

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego/v3"
)


func main() {
	// Or zerobouncego.LoadEnvFromFile() to read the key from .env instead.
	// Calling it after Initialize would replace this key with the file's.
	zerobouncego.Initialize("... Your API KEY ...")
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
		// ReturnURL is optional for scoring send when supported by the API.
	}
	submit_response, error_ := zerobouncego.AiScoringFileSubmitWithOptions(csv_file, nil)
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


###### Bulk email finder, bulk domain search and list evaluator

These upload a CSV like `BulkValidationSubmit` and follow the same
submit / status / result / delete pattern. `BulkEmailFinderSubmit` needs
`DomainColumn` and one of `FirstNameColumn` or `FullNameColumn` (columns are
1-based), and says which is missing before uploading. `has_header_row` is always
sent, `false` included.

```go
package main

import (
	"fmt"
	"os"

	"github.com/zerobounce/zerobouncego/v3"
)

func main() {
	zerobouncego.Initialize("... Your API KEY ...")

	file, err := os.Open("people.csv") // domain,first_name,last_name
	if err != nil {
		fmt.Println("error while opening the file: ", err.Error())
		return
	}
	defer file.Close()

	submitted, err := zerobouncego.BulkEmailFinderSubmit(zerobouncego.EmailFinderCsvFile{
		File: file, FileName: "people.csv", HasHeaderRow: true,
		DomainColumn: 1, FirstNameColumn: 2, LastNameColumn: 3,
	})
	if err != nil {
		fmt.Println("error while submitting data: ", err.Error())
		return
	}

	status, err := zerobouncego.BulkEmailFinderFileStatus(submitted.FileId)
	if err != nil {
		fmt.Println("error while polling: ", err.Error())
		return
	}
	if status.Percentage() == 100 {
		result, err := os.Create("people_found.csv")
		if err != nil {
			fmt.Println("error on creating result file: ", err.Error())
			return
		}
		defer result.Close()
		if err := zerobouncego.BulkEmailFinderResult(submitted.FileId, result); err != nil {
			fmt.Println("error on fetching the result: ", err.Error())
		}
	}
}
```

`BulkDomainSearchSubmit` takes a `DomainSearchCsvFile` (`File`, `FileName`,
`HasHeaderRow`, `DomainColumn`) and pairs with `BulkDomainSearchFileStatus`,
`BulkDomainSearchResult` and `BulkDomainSearchFileDelete`.
`ListEvaluatorSubmit` takes a `ListEvaluatorCsvFile` (`File`, `FileName`,
`EmailAddressColumn`); `GetListEvaluatorResult` reads the evaluation, and
`SendEvaluatedFile` sends the evaluated file on for validation. The list
evaluator lives at `LIST_EVALUATOR_URI`.

###### Filters

Custom allow/block filters on the main API. Each call takes a `rule`, a
`target` and a `value`; see the ZeroBounce API docs for the accepted rule and
target values.

```go
added, err := zerobouncego.AddFilter("allow", "domain", "example.com")
if err != nil {
	log.Fatal(err)
}
fmt.Println(added)

filters, err := zerobouncego.ListFilters(false) // true sends a POST
if err != nil {
	log.Fatal(err)
}
fmt.Println(string(filters))

if _, err := zerobouncego.DeleteFilter("allow", "domain", "example.com"); err != nil {
	log.Fatal(err)
}
```

#### S3 and SFTP/FTP bulk

Queue a job on a file that already sits in your Amazon S3 bucket, or on your
SFTP/FTP server. See the [S3](https://www.zerobounce.net/docs/api-integrations/amazon-s3-bulk-api)
and [SFTP](https://www.zerobounce.net/docs/api-integrations/sftp_bulk_api) docs.
These live on the bulkapi host *without* the `/v2` path.

Every one of these jobs needs a `return_url`: the API posts the finished job
there, and it answers `Provided return URL is not permitted: None` without one,
so the SDK rejects the call before it goes out.

```go
// Amazon S3. FileName is the full path, as returned by ListS3Files.
zerobouncego.ListS3Files(zerobouncego.ListS3FilesOptions{Bucket: "my-bucket"})

zerobouncego.ValidateS3File(zerobouncego.ValidateS3FileOptions{
    FileName:  "lists/leads.csv",
    ReturnURL: "https://example.com/zerobounce-done", // required
})

zerobouncego.ScoreS3File(zerobouncego.ScoreS3FileOptions{
    FileName:  "lists/leads.csv",
    ReturnURL: "https://example.com/zerobounce-done", // required
})

// export_bucket is optional: "" uses the bucket from your ZeroBounce settings.
zerobouncego.ExportS3File("<FILE_ID>", "my-export-bucket")

zerobouncego.EmailFinderS3File(zerobouncego.BulkFileJobOptions{
    FileName:  "lists/leads.csv",
    HasHeaderRow: true,
    ReturnURL: "https://example.com/zerobounce-done",
})

removeDuplicates := true
zerobouncego.DomainSearchS3File(zerobouncego.BulkFileJobOptions{
    FileName:         "lists/leads.csv",
    HasHeaderRow:     true,
    RemoveDuplicates: &removeDuplicates,
    ReturnURL:        "https://example.com/zerobounce-done",
})

// SFTP/FTP. The server, path and protocol come from your ZeroBounce settings.
// The docs call FileName optional for ValidateSFTPFile; the live API answers
// "file_name: This field is required", so every job here needs it.
done := "https://example.com/zerobounce-done"
zerobouncego.ValidateSFTPFile(zerobouncego.BulkFileJobOptions{
    FileName: "my_file.txt", HasHeaderRow: true, ReturnURL: done,
})
zerobouncego.ScoreSFTPFile(zerobouncego.BulkFileJobOptions{FileName: "my_file.txt", ReturnURL: done})
zerobouncego.EmailFinderSFTPFile(zerobouncego.BulkFileJobOptions{FileName: "my_file.txt", ReturnURL: done})
zerobouncego.DomainSearchSFTPFile(zerobouncego.BulkFileJobOptions{FileName: "my_file.txt", ReturnURL: done})

// Note the field: this one takes export_path, not export_bucket.
// Pass "" to use the export path from your settings.
zerobouncego.ExportSFTPFile("<FILE_ID>", "/exports")
```

#### Validate options

`ValidateWithOptions` also accepts `DomainInfo`, `GreylistProcessing` and
`CallbackURL`. A nil pointer is not sent, and booleans go on the wire as the
strings `"true"`/`"false"`.

```go
yes := true
zerobouncego.ValidateWithOptions("possible_typo@example.com", zerobouncego.ValidateOptions{
    DomainInfo:         &yes, // fills DomainWebsiteExists / DomainRegistrantCompanyName
    GreylistProcessing: &yes,
    CallbackURL:        "https://example.com/greylist", // needs GreylistProcessing
})
```

`CallbackURL` is where the greylist retry posts its result, so it is rejected
without `GreylistProcessing` — before any request is sent. While that retry is
still running the API answers with a `Message` instead of a result, which lands
on `response.Message`.

#### Inbox Placement Tester

Runs on its own host (`INBOX_PLACEMENT_URI`), takes the API key in an
`Authorization: Bearer` header rather than on the query string, and sends
camelCase JSON bodies. It spends the account's inbox-test credits.

A test name is at most 64 characters, `TestType` is 1, 2 or 3, and type 1 needs
a `SenderEmail`. On the list, `Status` is `"running"` or `"finished"` and
`PageSize` is 1-100. Each of those is checked before the request goes out, so a
rejected call costs nothing.

```go
zerobouncego.GetInboxPlacementProviders()
zerobouncego.GetInboxPlacementTestTypes()

testType := 1
created, err := zerobouncego.CreateInboxPlacementTest("Launch", &zerobouncego.CreateInboxPlacementTestOptions{
    TestType:    &testType,
    SenderEmail: "news@example.com",
    WebhookURL:  "https://example.com/ipt-webhook",
})
if err != nil {
    log.Fatal(err)
}

// Send your campaign to the seeds in `created`, then read the results.
fmt.Println(created)
zerobouncego.GetInboxPlacementTest("<TEST_ID>")

page, pageSize := 1, 50
zerobouncego.ListInboxPlacementTests(&zerobouncego.ListInboxPlacementTestsOptions{
    Status: zerobouncego.IPT_STATUS_FINISHED, Page: &page, PageSize: &pageSize,
})
zerobouncego.SetInboxPlacementWebhook("https://example.com/ipt-webhook")
zerobouncego.GetInboxPlacementWebhook()
zerobouncego.DeleteInboxPlacementTest("<TEST_ID>")
```

#### Warmup API

The [Warmup API](https://www.zerobounce.net/docs/warmup-api) is a separate
product from email validation: it has its own host (`WarmupDefaultBaseURL`,
`https://warmup-api.zerobounce.net`) and its own **Warmup API key**, which is
bound to one team and sent in an `API-Key` header. Use a `WarmupClient` for it.
The client does not use the validation key, `Initialize`, or `SetURI`, and the
validation functions never see the Warmup key.

`NewWarmupClient(apiKey, baseURL)` rejects an empty key, and a base URL that is
not `https://`; pass `""` for the default host. Methods return the decoded JSON
as a `map[string]interface{}` (a list answer is wrapped under `"data"`),
`DownloadSeeds` and `DownloadStatistics` return CSV text, and a failed call is a
`*ZBApiError` with the HTTP status (for example 429 while a custom engagement
rule is in its 7-day cooldown). Out-of-range arguments are rejected before the
request.

```go
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego/v3"
)

func main() {
	// A Warmup API key, not the validation API key. "" uses WarmupDefaultBaseURL.
	warmup, err := zerobouncego.NewWarmupClient(os.Getenv("ZEROBOUNCE_WARMUP_API_KEY"), "")
	if err != nil {
		log.Fatal(err)
	}

	accounts, err := warmup.ListSendingAccounts()
	if err != nil {
		var apiErr *zerobouncego.ZBApiError
		if errors.As(err, &apiErr) {
			log.Fatalf("Warmup API error (HTTP %d): %s", apiErr.StatusCode, apiErr.Message)
		}
		log.Fatal(err)
	}
	fmt.Println(accounts)

	// Register a mailbox you run yourself, with the Default engagement rule (0).
	rule := 0
	_, err = warmup.AddSendingAccount("sender@example.com", "self_managed",
		&zerobouncego.WarmupSendingAccountOptions{EngagementRuleID: &rule})
	if err != nil {
		log.Fatal(err)
	}

	// Dates are a time.Time or a "YYYY-MM-DD" string.
	perDay, err := warmup.GetStatisticsPerDay("sender@example.com", "2026-11-01", time.Now())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(perDay)

	seeds, err := warmup.DownloadSeeds() // CSV: seed,provider
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(seeds)
}
```

All 26 methods:

| Area | Methods |
| --- | --- |
| Sending accounts | `ListSendingAccounts`, `GetSendingAccount`, `AddSendingAccount` (`WarmupSendingAccountOptions`), `UpdateSendingAccount`, `DeleteSendingAccount`, `ChangeFromAddress`, `ListFromAddresses` (`WarmupListFromAddressesOptions`) |
| Engagement rules | `GetEngagementRules`, `SetEngagementRule`, `GetCustomEngagement`, `CreateCustomEngagement`, `UpdateCustomEngagement`, `DeleteCustomEngagement` |
| Seeds and statistics | `GetSeeds`, `DownloadSeeds`, `GetSeedCount`, `GetProgressToday`, `GetSentToday`, `GetEmailsPerDay`, `GetStatistics`, `GetStatisticsPerDay`, `GetStatisticsDetailed`, `DownloadStatistics`, `GetFromAddressStatistics` |
| Utilities | `CheckDomainDmarc` (takes an email address), `TestSMTPConfiguration` |

`AddSendingAccount` takes an onboarding model from `WarmupOnboardingModels`
(`self_managed`, which needs `EngagementRuleID`, or `zb_managed`). For
`zb_managed`, `EmailContent` and `ManagedSending` are sent as given, with these
documented keys:

- `EmailContent`: `company_name`, `company_website`, `company_description`,
  `types_of_emails` (a `[]string` drawn from `WarmupEmailTypes`) and
  `example_email`.
- `ManagedSending`: `weekdays_only` (a `bool`), `timezone` (a `string`, e.g.
  `"Europe/Bucharest"`), `timezone_offset` (any Go number from -11 to 14,
  fractional allowed, e.g. `5.5`) and `warmup_start_date` (a `time.Time` or
  `"YYYY-MM-DD"`).

```go
_, err = warmup.AddSendingAccount("sender@example.com", "zb_managed",
	&zerobouncego.WarmupSendingAccountOptions{
		EmailContent: map[string]interface{}{
			"company_name":    "Acme",
			"company_website": "https://acme.example",
			"types_of_emails": []string{"newsletter"},
		},
		ManagedSending: map[string]interface{}{
			"weekdays_only":   true,
			"timezone":        "Asia/Kolkata",
			"timezone_offset": 5.5,
		},
	})
```

`DownloadSeeds` and `DownloadStatistics` return a `*ZBApiError` instead of the
CSV when the answer is empty, JSON or an HTML page. `DeleteSendingAccount`
cannot be undone.

#### Testing

Unit tests mock every request and never reach the network:

```bash
go test -short ./...
```

A plain `go test ./...` is offline as well. The integration tests in
`api_integration_test.go` and `batch_validate_integration_test.go` call the live
API, so they skip unless you opt in with `ZEROBOUNCE_INTEGRATION=1` (and they
always skip under `-short`). `TestValidate` and `TestBulkEmailValidation` also
need a key in `ZEROBOUNCE_API_KEY` (or `ZERO_BOUNCE_API_KEY`); they validate the
`@example.com` sandbox addresses.

```bash
ZEROBOUNCE_INTEGRATION=1 ZEROBOUNCE_API_KEY=... go test -run 'TestValidate$|TestInvalidApiKey|TestBulkEmailValidation' -v .
```

`live_api_test.go` is a fuller live suite behind the `live` build tag
(`go test -tags live -v -run TestLive ./...`); its header lists the environment
variables it reads and which calls spend credits.
