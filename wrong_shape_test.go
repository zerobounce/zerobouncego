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
	typed, lists := 0, 0
	for _, call := range calls {
		if downloads[call.name] || strings.HasPrefix(call.name, "WarmupClient.Download") {
			continue
		}
		fake.body = `"x"`
		if call.call() == nil {
			continue // raw JSON: any JSON value is the result
		}
		typed++
		// A JSON array is never read positionally into a result object: [],
		// ["x"] and a positional array are refused by every call whose result
		// is an object. A call that takes [] returns a list, so it must
		// refuse an object ({}) instead.
		refused := []string{`null`, `0`, `true`, `"x"`, `[1]`, ` null `, `["x"]`, `["a@example.com",10]`, `[true,"3"]`}
		fake.body = `[]`
		if call.call() == nil {
			lists++
			refused = append(refused, `{}`)
		} else {
			refused = append(refused, `[]`)
		}
		for _, body := range refused {
			fake.body = body
			err := call.call()
			var apiErr *ZBApiError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != fake.status {
				t.Errorf("%s %s: err = %#v, want a *ZBApiError with status %d", call.name, body, err, fake.status)
			}
		}
	}
	if typed < 50 || lists > 2 {
		t.Fatalf("%d typed calls, %d of them taking a list", typed, lists)
	}
}
