package zerobouncego_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	zerobouncego "github.com/zerobounce/zerobouncego/v3"
)

// A client holds its own key and base URLs, so two keys can be in flight at once.
// This example answers from a local test server, so it needs no network and no real key.
func ExampleNew() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"address":"valid@example.com","status":"valid","sub_status":""}`)
	}))
	defer srv.Close()

	client := zerobouncego.New("your-api-key")
	client.URI = srv.URL + "/" // loopback http is accepted; use the default https URL in real code

	result, err := client.Validate("valid@example.com", "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Address, result.Status, result.Status == zerobouncego.ValidateStatusValid)
	// Output: valid@example.com valid true
}

// Validate one address with a Client. The call is not run here (no Output line).
func ExampleClient_Validate() {
	client := zerobouncego.New("your-api-key")

	result, err := client.Validate("someone@example.com", "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Status, result.SubStatus)
}

// The package-level functions use one process-wide key set by Initialize.
// Do not share it between users; use New for that.
func ExampleInitialize() {
	zerobouncego.Initialize("your-api-key")

	credits, err := zerobouncego.GetCredits()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(credits.Credits())
}

// An API answer that is an error (a bad key, a quota, an HTTP error status) is a *ZBApiError.
func ExampleZBApiError() {
	_, err := zerobouncego.New("your-api-key").Validate("someone@example.com", "")
	if apiErr, ok := err.(*zerobouncego.ZBApiError); ok {
		fmt.Println(apiErr.StatusCode, apiErr.Message)
	}
}
