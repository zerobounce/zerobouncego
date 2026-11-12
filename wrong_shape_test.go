package zerobouncego

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// A 2xx body that is strict JSON but not the shape the call returns is a
// *ZBApiError with that 2xx status on every typed call, never a result filled
// with zero values. A call that takes a bare string ("x") as its result
// returns raw JSON, so any JSON is its answer; every other call is typed and
// must refuse each body below. null is the case encoding/json lets through:
// unmarshalling it into a struct is a no-op.
func TestEveryTypedCallRefusesJSONOfTheWrongShape(t *testing.T) {
	downloads := downloadCalls(t)
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	calls := publicRequestCalls(client, warmup)
	typed := 0
	for _, call := range calls {
		if downloads[call.name] || strings.HasPrefix(call.name, "WarmupClient.Download") {
			continue
		}
		fake.body = `"x"`
		if call.call() == nil {
			continue // raw JSON: any JSON value is the result
		}
		typed++
		for _, body := range []string{`null`, `0`, `true`, `"x"`, `[1]`, ` null `} {
			fake.body = body
			err := call.call()
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != fake.status {
				t.Errorf("%s %s: err = %#v, want a *ZBApiError with status %d", call.name, body, err, fake.status)
			}
		}
	}
	if typed < 50 {
		t.Fatalf("only %d typed calls", typed)
	}
}
