# GoZeroBounce
Go implementation for [ZeroBounce Email Validation API v2](https://www.zerobounce.net/docs/email-validation-api-quickstart/) 
THIS PROJECT IS STILL IN DEVELOPMENT!

[Link to the original repo](https://github.com/antsanchez/gozerobounce)

## Installation and Usage
```sh
go get github.com/antsanchez/gozerobounce
```

You can use it like this:
```go
package main

import (
    "fmt"
    "os"

    "github.com/antsanchez/gozerobounce"
)

func main() {

    gozerobounce.APIKey = "... Your API KEY ..." 

    // For Querying a single E-Mail and IP
    // IP can also be an empty string
    response := gozerobounce.Validate("email@example.com", "123.123.123.123")

    // Now you can check status
    if response.Status == "valid" {
        fmt.Println("This email is valid")
    }

    // .. or Substatus
    if response.SubStatus == "disposable" {
        fmt.Println("This email is disposable")
    }

    // You can also check your credits 
    credits := gozerobounce.GetCredits()
    fmt.Println("Credits left:", credits.Credits)
}
```

## Already implemented
- Validate single email
- Get gredits
