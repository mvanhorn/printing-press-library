package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHandoffUnknownPartySuitabilityIsPartial(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		payload, _ := io.ReadAll(r.Body)
		body := `{"data":{"venue":{"id":"245672","name":"Example","realTimeBooking":true,"courses":[{"id":"182402","name":"Course"}]}}}`
		if strings.Contains(string(payload), "query Sessions") {
			body = `{"data":{"venue":{"id":"245672","name":"Example","realTimeBooking":true},"availabilitySearch":[{"__typename":"ReservableAvailability","id":"6871760","startTime":"2026-10-05T18:00:00+09:00","minPartySize":null,"maxPartySize":6,"course":{"id":"182402","name":"Course"}}]}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"booking", "handoff", "--id", "245672", "--session-id", "6871760", "--date", "2026-10-05", "--party", "2", "--no-cache"})
	if e := root.Execute(); e != nil {
		t.Fatal(e)
	}
	var result map[string]any
	if json.Unmarshal(out.Bytes(), &result) != nil {
		t.Fatal(out.String())
	}
	if result["meta"].(map[string]any)["partial"] != true || result["session"].(map[string]any)["party_eligible"] != nil {
		t.Fatal(out.String())
	}
}
