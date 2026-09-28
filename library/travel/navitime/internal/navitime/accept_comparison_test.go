package navitime

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

// This experiment changes only one request's Accept value in a test wrapper.
// Every case uses a fresh cookie-free client and permits exactly one actual GET.
func TestLiveIsolatedAcceptComparison(t *testing.T) {
	if os.Getenv("NAVITIME_ACCEPT_COMPARE") != "1" {
		t.Skip("isolated two-GET public comparison is opt-in")
	}
	q := Query{From: "station:00006668", To: "station:00001756", DepartAt: "2026-10-01T09:00"}
	totalSent := 0
	for _, tc := range []struct{ name, override string }{{"current_html", ""}, {"overridden_json", "application/json"}} {
		c, err := NewClient(Options{NoCache: true, Timeout: 15 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if c.HTTP.Jar != nil {
			t.Fatal("experiment must not import or retain cookies")
		}
		base := c.HTTP.Transport
		sent := 0
		statuses := []int{}
		accept := ""
		c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if sent >= 1 {
				return nil, errors.New("single-GET case budget exhausted; retry prevented")
			}
			clone := req.Clone(req.Context())
			clone.Header = req.Header.Clone()
			if tc.override != "" {
				clone.Header.Set("Accept", tc.override)
			}
			accept = clone.Header.Get("Accept")
			if clone.Header.Get("Cookie") != "" || clone.Header.Get("Authorization") != "" {
				return nil, errors.New("credentials are forbidden in this experiment")
			}
			sent++
			totalSent++
			resp, err := base.RoundTrip(clone)
			if resp != nil {
				statuses = append(statuses, resp.StatusCode)
			}
			return resp, err
		})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		result, err := c.Routes(ctx, q)
		cancel()
		c.HTTP.CloseIdleConnections()
		record := map[string]any{"case": tc.name, "query": q, "accept": accept, "fresh_client": true, "no_cache": true, "actual_gets": sent, "statuses": statuses, "metrics": c.Metrics(), "normalization_succeeded": err == nil, "route_count": len(result.Routes)}
		if err != nil {
			record["error"] = err.Error()
		} else if len(result.Routes) > 0 {
			record["first_route"] = summary(result.Routes[0])
		}
		assertJSON(t, "isolated_accept_comparison", record)
	}
	assertJSON(t, "isolated_accept_totals", map[string]any{"actual_gets": totalSent})
}
