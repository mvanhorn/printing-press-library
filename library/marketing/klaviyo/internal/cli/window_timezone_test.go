// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func pinKlaviyoWindowClock(t *testing.T, now time.Time) {
	t.Helper()
	previous := klaviyoWindowNow
	klaviyoWindowNow = func() time.Time { return now }
	t.Cleanup(func() { klaviyoWindowNow = previous })
}

func TestMetricWindowUsesTimezoneCalendarDays(t *testing.T) {
	for _, tt := range []struct {
		name, timezone, localTimezone, now, wantSince, wantUntil string
	}{
		{"default UTC after UTC midnight", "", "America/Chicago", "2026-11-10T00:30:00Z", "2026-08-12", "2026-11-10"},
		{"explicit UTC after UTC midnight", "UTC", "America/Chicago", "2026-11-10T00:30:00Z", "2026-08-12", "2026-11-10"},
		{"configured calendar across DST", "America/Chicago", "UTC", "2026-11-10T05:30:00Z", "2026-08-11", "2026-11-09"},
		{"configured timezone ahead of UTC", "Asia/Tokyo", "America/Chicago", "2026-11-09T16:30:00Z", "2026-08-12", "2026-11-10"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KLAVIYO_TIMEZONE", tt.timezone)
			local, err := time.LoadLocation(tt.localTimezone)
			if err != nil {
				t.Fatal(err)
			}
			now, err := time.Parse(time.RFC3339, tt.now)
			if err != nil {
				t.Fatal(err)
			}
			pinKlaviyoWindowClock(t, now.In(local))
			fixPassClientEnvironment(t, "https://example.invalid")
			_, since, until, err := clientAndWindow(&rootFlags{}, "90d")
			if err != nil {
				t.Fatal(err)
			}
			if since.Format("2006-01-02") != tt.wantSince || until.Format("2006-01-02") != tt.wantUntil {
				t.Fatalf("window = %s..%s; want %s..%s", since, until, tt.wantSince, tt.wantUntil)
			}
			c := &fakeCouponPoolClient{}
			if _, err := topAggregateRows(c, "metric", "$flow", "count", since, until, 0); err != nil {
				t.Fatal(err)
			}
			attrs := anyPath(c.posts[0].body, "data.attributes").(map[string]any)
			wantFilters := []string{"greater-or-equal(datetime," + tt.wantSince + "T00:00:00)", "less-than(datetime," + tt.wantUntil + "T23:59:59)"}
			if attrs["timezone"] != defaultMetricTimezone() || !reflect.DeepEqual(attrs["filter"], wantFilters) {
				t.Fatalf("aggregate attributes = %v; want timezone %s and filters %v", attrs, defaultMetricTimezone(), wantFilters)
			}
			flags := &rootFlags{platformSession: platformCommandTestSession(t)}
			cmd := &cobra.Command{Use: "open-rates"}
			cmd.Flags().String("last", "90d", "fixture lookback")
			if err := adoptPlatformCommandWindow(cmd, flags); err != nil {
				t.Fatal(err)
			}
			window := flags.platformSession.OutputMetadata().ResolvedWindow
			if window == nil || window.Timezone != defaultMetricTimezone() || !window.Start.Equal(since) || !window.End.Equal(until) {
				t.Fatalf("platform window = %+v; want %s..%s in %s", window, since, until, defaultMetricTimezone())
			}
			if err := AdoptMCPOutputSemantics(flags.platformSession, map[string]any{"last": "90d"}); err != nil {
				t.Fatal(err)
			}
			window = flags.platformSession.OutputMetadata().ResolvedWindow
			if window == nil || window.Timezone != defaultMetricTimezone() || !window.Start.Equal(since) || !window.End.Equal(until) {
				t.Fatalf("MCP window = %+v; want %s..%s in %s", window, since, until, defaultMetricTimezone())
			}
		})
	}
}

func TestMetricAggregateBodyFormatsBoundsInRequestTimezone(t *testing.T) {
	t.Setenv("KLAVIYO_TIMEZONE", "UTC")
	local, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	until := time.Date(2026, time.November, 10, 0, 30, 0, 0, time.UTC)
	since := until.AddDate(0, 0, -90)
	body := metricAggregateBody("metric", []string{"count"}, nil, since.In(local), until.In(local))
	want := []string{"greater-or-equal(datetime,2026-08-12T00:00:00)", "less-than(datetime,2026-11-10T23:59:59)"}
	if got := anyPath(body, "data.attributes.filter"); !reflect.DeepEqual(got, want) {
		t.Fatalf("filters = %v; want %v", got, want)
	}
}

func TestOptimalSendWindowUsesExplicitTimezoneAcrossDST(t *testing.T) {
	t.Setenv("KLAVIYO_TIMEZONE", "Asia/Tokyo")
	pinKlaviyoWindowClock(t, time.Date(2026, time.November, 10, 5, 30, 0, 0, time.UTC))
	fixPassClientEnvironment(t, "https://example.invalid")
	_, since, until, err := clientAndWindow(&rootFlags{}, "90d", "America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	body := metricAggregateBodyWithInterval("metric", []string{"count"}, nil, since, until, "hour", "America/Chicago")
	attrs := anyPath(body, "data.attributes").(map[string]any)
	want := []string{"greater-or-equal(datetime,2026-08-11T00:00:00)", "less-than(datetime,2026-11-09T23:59:59)"}
	if attrs["timezone"] != "America/Chicago" || attrs["interval"] != "hour" || !reflect.DeepEqual(attrs["filter"], want) {
		t.Fatalf("aggregate attributes = %v; want hourly Chicago dates %v", attrs, want)
	}
}

func TestRevenueWindowUsesDeclaredTimezoneAndPreservesLiteralDates(t *testing.T) {
	for _, tt := range []struct {
		name, since, until, wantSince, wantUntil string
	}{
		{"defaults", "", "", "2026-08-09", "2026-11-09"},
		{"explicit dates", "2026-08-01", "2026-11-01", "2026-08-01", "2026-11-01"},
		{"explicit start", "2026-08-01", "", "2026-08-01", "2026-11-09"},
		{"explicit end", "", "2026-11-01", "2026-08-09", "2026-11-01"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pinKlaviyoWindowClock(t, time.Date(2026, time.November, 10, 5, 30, 0, 0, time.UTC))
			posts := 0
			baseURL := rxHTTPFixture(t, "recorder", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				switch r.Method + " " + r.URL.Path {
				case "GET /api/metrics":
					fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order"}}],"links":{"next":null}}`)
				case "POST /api/metric-aggregates":
					posts++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					want := []any{"greater-or-equal(datetime," + tt.wantSince + "T00:00:00)", "less-than(datetime," + tt.wantUntil + "T23:59:59)"}
					if got := anyPath(body, "data.attributes.filter"); !reflect.DeepEqual(got, want) || anyPath(body, "data.attributes.timezone") != "US/Central" {
						t.Errorf("aggregate body = %v; want Central filters %v", body, want)
					}
					fmt.Fprint(w, `{"data":{"attributes":{"dates":[],"data":[]}}}`)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
					http.NotFound(w, r)
				}
			}))
			fixPassClientEnvironment(t, baseURL)
			cmd := newReportRevenueCmd(&rootFlags{asJSON: true, noCache: true})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--by", "channel", "--since", tt.since, "--until", tt.until})
			if err := cmd.Execute(); err != nil || posts != 1 {
				t.Fatalf("posts = %d, error = %v; want one revenue aggregate", posts, err)
			}
		})
	}
}
