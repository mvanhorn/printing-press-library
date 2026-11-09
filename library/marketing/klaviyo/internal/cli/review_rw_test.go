// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

func TestReviewRWHistoricalRevenueWindows(t *testing.T) {
	for _, tt := range []struct {
		name        string
		since       string
		until       string
		maximumDays int
		wantStart   string
		wantEnd     string
		wantClamp   bool
	}{
		{"full leap year fits", "2024-01-01", "2024-12-31", 366, "2024-01-01T00:00:00", "2024-12-31T23:59:59", false},
		{"historical range fits provider limit", "2024-01-01", "2024-11-30", 364, "2024-01-01T00:00:00", "2024-11-30T23:59:59", false},
		{"full leap year exceeds provider limit", "2024-01-01", "2024-12-31", 364, "2024-01-02T23:59:59", "2024-12-31T23:59:59", true},
		{"multiple years keep historical end", "2022-01-01", "2024-12-31", 364, "2024-01-02T23:59:59", "2024-12-31T23:59:59", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requests := []string{}
			root, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				if got := r.Header.Get("User-Agent"); got != "klaviyo-pp-cli/2026-04-15" {
					t.Errorf("User-Agent = %q", got)
				}
				switch r.Method + " " + r.URL.Path {
				case "GET /api/metrics":
					if !reflect.DeepEqual(r.URL.Query(), url.Values{"fields[metric]": {"name"}}) {
						t.Errorf("metrics query = %v", r.URL.Query())
					}
					fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order"}}],"links":{"next":null}}`)
				case "POST /api/metric-aggregates":
					if r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("aggregate query = %q, Content-Type = %q", r.URL.RawQuery, r.Header.Get("Content-Type"))
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					want := fmt.Sprintf(`{"data":{"attributes":{"by":["$attributed_channel"],"filter":["greater-or-equal(datetime,%s)","less-than(datetime,%s)"],"interval":"month","measurements":["sum_value","count","unique"],"metric_id":"orders","page_size":500,"timezone":"US/Central"},"type":"metric-aggregate"}}`, tt.wantStart, tt.wantEnd)
					if string(body) != want {
						t.Errorf("aggregate body = %s; want %s", body, want)
					}
					fmt.Fprint(w, `{"data":{"attributes":{"dates":["2024-12-01"],"data":[{"dimensions":["email"],"measurements":{"sum_value":[12],"count":[2],"unique":[1]}}]}}}`)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
					http.NotFound(w, r)
				}
			})
			registeredPlatformSource.WindowMaximumDays = tt.maximumDays
			registeredPlatformSource.WindowMaximumName = "fixture_provider_maximum"
			var output, diagnostics bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&diagnostics)
			root.SetArgs([]string{"report", "revenue", "--by", "channel", "--since", tt.since, "--until", tt.until, "--json"})
			if err := root.Execute(); err != nil {
				t.Fatalf("revenue failed: %v; stderr = %s", err, &diagnostics)
			}
			if want := []string{"GET /api/metrics", "POST /api/metric-aggregates"}; !reflect.DeepEqual(requests, want) {
				t.Fatalf("requests = %v; want %v", requests, want)
			}
			var envelope struct {
				Meta platform.OutputMetadata `json:"meta"`
			}
			if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
				t.Fatalf("decode output metadata: %v: %s", err, &output)
			}
			window := envelope.Meta.ResolvedWindow
			recorded := flags.platformSession.OutputMetadata().ResolvedWindow
			if window == nil || recorded == nil || !window.Start.Equal(recorded.Start) || !window.End.Equal(recorded.End) {
				t.Fatalf("output window = %+v; session metadata = %+v", window, flags.platformSession.OutputMetadata())
			}
			if window.Start.Format("2006-01-02T15:04:05") != tt.wantStart || window.End.Format("2006-01-02T15:04:05") != tt.wantEnd || window.Timezone != "US/Central" || window.Clamped != tt.wantClamp || !window.StartInclusive || !window.EndExclusive || !window.CompletePeriods {
				t.Fatalf("resolved window = %+v", window)
			}
			if window.Requested.Start == nil || window.Requested.End == nil || window.Requested.Start.Format("2006-01-02T15:04:05") != tt.since+"T00:00:00" || window.Requested.End.Format("2006-01-02T15:04:05") != tt.until+"T23:59:59" || window.Requested.Days != 0 {
				t.Fatalf("original explicit request = %+v", window.Requested)
			}
			wantReason, wantDiagnostics := "", ""
			if tt.wantClamp {
				wantReason = "requested_window_exceeds_provider_maximum"
				wantDiagnostics = "warning: requested window clamped: " + wantReason + "\n"
			}
			if window.ProviderMaximum != "fixture_provider_maximum" || window.ClampReason != wantReason || diagnostics.String() != wantDiagnostics {
				t.Fatalf("clamp metadata = %+v; stderr = %q", window, diagnostics.String())
			}
		})
	}
}

func TestReviewRWRelativeSinceUnchanged(t *testing.T) {
	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		since string
		days  int
		start time.Time
	}{
		{"7d", 7, now.AddDate(0, 0, -7)},
		{"2w", 14, now.AddDate(0, 0, -14)},
		{"36h", 0, now.Add(-36 * time.Hour)},
		{"2026-11-01", 0, time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(tt.since, func(t *testing.T) {
			cmd := &cobra.Command{Use: "fixture"}
			cmd.Flags().String("since", "", "fixture lookback")
			cmd.Flags().String("until", "", "fixture optional end")
			if err := cmd.ParseFlags([]string{"--since", tt.since}); err != nil {
				t.Fatal(err)
			}
			request, exclusive, ok := platformWindowRequestFromFlags(cmd, now, time.UTC, "UTC")
			if !ok || !exclusive || request.Days != tt.days {
				t.Fatalf("request = %+v, exclusive = %v, ok = %v", request, exclusive, ok)
			}
			window, err := platform.ResolveWindow(request, platform.WindowPolicy{Now: func() time.Time { return now }, EndExclusive: exclusive})
			if err != nil {
				t.Fatal(err)
			}
			if !window.Start.Equal(tt.start) || !window.End.Equal(now) || window.Clamped || window.CompletePeriods {
				t.Fatalf("relative window = %+v; want %s through %s", window, tt.start, now)
			}
		})
	}
}

func TestReviewRWMCPExplicitSinceUntil(t *testing.T) {
	session := platformCommandTestSession(t)
	if err := AdoptMCPOutputSemantics(session, map[string]any{"since": "2024-01-01", "until": "2024-12-31", "timezone": "US/Central"}); err != nil {
		t.Fatal(err)
	}
	window := session.OutputMetadata().ResolvedWindow
	if window == nil || window.Start.Format("2006-01-02T15:04:05Z07:00") != "2024-01-01T00:00:00-06:00" || window.End.Format("2006-01-02T15:04:05Z07:00") != "2024-12-31T23:59:59-06:00" || window.Clamped || !window.CompletePeriods {
		t.Fatalf("MCP explicit window = %+v", window)
	}
}

func TestReviewRWRejectsReversedWindow(t *testing.T) {
	flags := &rootFlags{platformSession: platformCommandTestSession(t)}
	cmd := newReportRevenueCmd(flags)
	if err := cmd.ParseFlags([]string{"--since", "2024-12-31", "--until", "2024-01-01"}); err != nil {
		t.Fatal(err)
	}
	err := adoptPlatformCommandWindow(cmd, flags)
	if err == nil || err.Error() != "resolve command window: window start must be before end" {
		t.Fatalf("error = %v; want exact reversed-window rejection", err)
	}
}

func TestReviewRWRevenueUsesUnclampedRelativeWindow(t *testing.T) {
	posts := 0
	_, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/metrics":
			fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order"}}],"links":{"next":null}}`)
		case "POST /api/metric-aggregates":
			posts++
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			want := `{"data":{"attributes":{"by":["$attributed_channel"],"filter":["greater-or-equal(datetime,2026-11-02T12:30:00)","less-than(datetime,2026-11-09T12:30:00)"],"interval":"month","measurements":["sum_value","count","unique"],"metric_id":"orders","page_size":500,"timezone":"UTC"},"type":"metric-aggregate"}}`
			if string(body) != want || r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("aggregate query = %q, Content-Type = %q, body = %s; want %s", r.URL.RawQuery, r.Header.Get("Content-Type"), body, want)
			}
			fmt.Fprint(w, `{"data":{"attributes":{"dates":["2026-11-01"],"data":[]}}}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	})
	flags.asJSON = true
	end := time.Date(2026, time.November, 9, 12, 30, 0, 0, time.UTC)
	flags.platformSession.SetResolvedWindow(platform.ResolvedWindow{Start: end.AddDate(0, 0, -7), End: end, Timezone: "UTC", StartInclusive: true, EndExclusive: true})
	cmd := newReportRevenueCmd(flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--by", "channel", "--since", "7d"})
	if err := cmd.Execute(); err != nil || posts != 1 {
		t.Fatalf("revenue error = %v, posts = %d; want one aggregate using the unclamped relative window", err, posts)
	}
}
