package traveloka

// Simulated regression for source partial-sentinel refresh across OW -> RT flows.
import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestSimulatedCoreOneWayThenReturnPreservesRequestSentinelSchema(t *testing.T) {
	c := simulatedCoreClient(t)
	for _, path := range []string{flightInitialPath, flightPollPath, flightPrefetchPath} {
		profile := c.session.Profiles[path]
		profile.Body["sentinel"] = map[string]any{"signals": []any{}, "token": "example-captured-request-token"}
		c.session.Profiles[path] = profile
	}
	firstSearchID := ""
	rtInitialCount := 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		b, e := io.ReadAll(r.Body)
		if e != nil {
			t.Fatal(e)
		}
		var payload map[string]any
		if decodeJSON(b, &payload) != nil {
			t.Fatal("request data malformed")
		}
		sentinel := sourceObject(payload["sentinel"])
		if _, ok := sentinel["signals"].([]any); !ok {
			return simulatedResponse(r, 400, `{"error":{"code":"MISSING_REQUIRED_REQUEST_FIELD"}}`, http.Header{}), nil
		}
		d := sourceObject(payload["data"])
		var data map[string]any
		switch r.URL.Path {
		case flightInitialPath:
			if d["tripType"] == "ONE_WAY" {
				firstSearchID = sourceString(d["searchId"])
				data = coreFlightResult([]any{coreFlightRow("ow", "SIN", "CGK", "15000")}, true)
			} else {
				rtInitialCount++
				if sourceString(d["searchId"]) == firstSearchID || len(sourceList(d["journeys"])) != 2 || len(sourceList(d["selectedFlights"])) != 0 {
					t.Fatal("return reused prior search or stale selection")
				}
				data = coreFlightResult([]any{coreFlightRow("rt_out", "SIN", "CGK", "12000")}, true)
			}
		case flightPollPath:
			if sourceString(d["journeyIndex"]) != "1" || len(sourceList(d["selectedFlights"])) != 1 || sourceList(d["selectedFlights"])[0] != "rt_out" {
				t.Fatal("return poll is missing outbound context")
			}
			data = coreFlightResult([]any{coreFlightRow("rt_in", "CGK", "SIN", "0")}, true)
		case flightPrefetchPath:
			total := "15000"
			if len(sourceList(d["journeyIds"])) == 2 {
				total = "25000"
			}
			data = map[string]any{"totalPrice": coreDisplay(total), "displayedPricePerPax": coreDisplay(total)}
		default:
			t.Fatal("unexpected flight operation")
		}
		// Source refresh is partial: token changes, captured required signals is omitted.
		response, _ := json.Marshal(map[string]any{"data": data, "sentinel": map[string]any{"token": "example-refreshed-source-token"}})
		return simulatedResponse(r, 200, string(response), http.Header{}), nil
	}))
	q := coreFlightQuery()
	q.Adults = 1
	q.Children = 0
	q.Infants = 0
	q.MaxCandidates = 1
	q.Limit = 1
	ow, e := c.SearchFlights(context.Background(), q)
	if e != nil || len(ow.Offers) != 1 || ow.Offers[0].Price.Total.Amount != "150.00" {
		t.Fatalf("one-way source flow failed: %v", e)
	}
	q.ReturnDate = time.Now().AddDate(0, 3, 7).Format("2006-01-02")
	rt, e := c.SearchFlights(context.Background(), q)
	if e != nil {
		t.Fatalf("partial source sentinel refresh broke the next return request: %v", e)
	}
	if rtInitialCount != 1 || len(rt.Offers) != 1 || len(rt.Offers[0].Legs) != 2 || rt.Offers[0].Price.Total.Amount != "250.00" {
		t.Fatal("OW -> RT flow did not retain both source journeys and authoritative total")
	}
}
