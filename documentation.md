#### Installation and Usage
```sh
go get github.com/zerobounce/zerobouncego
```

This package uses the zero-bounce API which requires an API key. This key can be provide in three ways:
1. through an environment variable `ZERO_BOUNCE_API_KEY` (loaded automatically in code)
2. through an .env file that contains `ZERO_BOUNCE_API_KEY` and then calling following method before usage:
```go
zerobouncego.LoadEnvFromFile()
```
3. by settings explicitly in code, using the following method:
```go
zerobouncego.SetApiKey("mysecretapikey")
```

#### Generic API methods

```go
package main

import (
	"fmt"
	"time"

	"github.com/zerobounce/zerobouncego"
)

func main() {
	zerobouncego.SetApiKey("... Your API KEY ...")

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
    "os"
    "github.com/zerobounce/zerobouncego"
)

func main() {

    zerobouncego.APIKey = "... Your API KEY ..."

	// For Querying a single E-Mail and IP
	// IP can also be an empty string
	response, error_ := zerobouncego.Validate("possible_typo@example.com", "123.123.123.123")

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

###### 2. Batch validation

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego"
)

func main() {
	zerobouncego.SetApiKey("... Your API KEY ...")

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

###### 3. Bulk file validation

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego"
)


func main() {
	zerobouncego.SetApiKey("... Your API KEY ...")
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


###### 4. AI scoring

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego"
)


func main() {
	zerobouncego.SetApiKey("... Your API KEY ...")
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


#### Testing

This package contains both unit tests and integration tests (which are excluded from the test suite). Unit test files are the ones ending in "_test.go" (as go requires) and the integration tests are ending in ("_integration_t.go").

In order to run the integration tests:
- set appropriate `ZERO_BOUNCE_API_KEY` environment variable
- rename all "_integration_t.go" into "_integration_test.go"
- run either individual or all tests (`go test .`)

NOTE: currently, the unit tests can be updated such that, by removing the mocking and explicit API key setting, they should work as integration tests as well AS LONG AS a valid API key is provided via environment

