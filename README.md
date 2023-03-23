# GoZeroBounce
Go implementation for [ZeroBounce Email Validation API v2](https://www.zerobounce.net/docs/email-validation-api-quickstart/) 
THIS PROJECT IS STILL IN DEVELOPMENT!

[Link to the original repo](https://github.com/antsanchez/gozerobounce)

## Installation and Usage
```sh
go get github.com/zerobounce-llc/zerobouncego
```

This package uses the zero-bounce API which requires an API key. This key can either be provide in two ways:
1. through an environment variable `ZERO_BOUNCE_API_KEY` (either explicit or provided trough an `.env` file)
1. by settings explicitly in code, using the following method:
```go
zerobouncego.SetApiKey("mysecretapikey")
```


You can use it like this:
```go
package main

import (
    "fmt"
    "os"
    "github.com/zerobounce-llc/zerobouncego"
)

func main() {

    zerobouncego.APIKey = "... Your API KEY ..." 

    // For Querying a single E-Mail and IP
    // IP can also be an empty string
    response := zerobouncego.Validate("email@example.com", "123.123.123.123")

    // Now you can check status
    if response.Status == "valid" {
        fmt.Println("This email is valid")
    }

    // .. or Substatus
    if response.SubStatus == "disposable" {
        fmt.Println("This email is disposable")
    }

    // You can also check your credits 
    credits := zerobouncego.GetCredits()
    fmt.Println("Credits left:", credits.Credits)
}
```

## Testing

This package contains both unit tests and integration tests (which are excluded from the test suite). Unit test files are the one ending in "_test.go" (as go requires) and the integration tests are ending in ("_integration_t.go").

In order to run the integration tests:
- set appropriate `ZERO_BOUNCE_API_KEY` environment variable
- rename all "_integration_t.go" into "_integration_test.go"
- run either individual or all tests (`go test .`)


## Already implemented
- Validate single email
- Get gredits
