package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

func g4LocalRoot(t *testing.T) (*cobra.Command, *rootFlags) {
	t.Helper()
	root, flags := g2Root(t, g2NoAPI(t))
	st, err := store.OpenVerifiedWithContext(context.Background(), flags.platformSession.Paths.DataFile, flags.platformSession)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	// Keep the fixture events in one monthly bucket even at month boundaries.
	now := time.Now().UTC().Truncate(24 * time.Hour)
	for i := range 8 {
		data, err := json.Marshal(map[string]any{
			"profile_id": "profile-1", "metric_name": "Received Email",
			"datetime": now.Add(-24*time.Hour + time.Duration(i)*time.Minute).Format(time.RFC3339),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.DB().Exec(`INSERT INTO events (id, data) VALUES (?, ?)`, fmt.Sprintf("event-%d", i), string(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	return root, flags
}

func TestG4FilteredLocalReportsUseFullAnalytics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		args    []string
		data    string
		metrics map[string]float64
	}{
		{"select", "send-fatigue", []string{"--json", "--select", "window"}, `{"window":"7d"}`, map[string]float64{"fatigued_profiles": 1, "fatigued_percentage": 100}},
		{"select with compact", "send-fatigue", []string{"--json", "--compact", "--select", "window"}, `{"window":"7d"}`, map[string]float64{"fatigued_profiles": 1, "fatigued_percentage": 100}},
		{"selected CSV", "send-fatigue", []string{"--csv", "--select", "window"}, `{"window":"7d"}`, map[string]float64{"fatigued_profiles": 1, "fatigued_percentage": 100}},
		{"compact array", "cohort", []string{"--json", "--compact"}, `[{"cohort":"2026-11","profiles":1,"retained":0,"retention_rate":0}]`, map[string]float64{"profiles": 1, "retained": 0, "retention_rate": 0}},
		{"selected dry run", "send-fatigue", []string{"--json", "--dry-run", "--select", "window"}, `{"window":"7d"}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, flags := g4LocalRoot(t)
			var output, diagnostics bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&diagnostics)
			root.SetArgs(append([]string{tc.command, "--db", flags.platformSession.Paths.DataFile}, tc.args...))
			err := root.Execute()
			finalizePlatformInvocation(flags, &err)
			if err != nil {
				t.Fatalf("filtered report rejected: %v; stdout=%s stderr=%s", err, &output, &diagnostics)
			}
			data := json.RawMessage(output.Bytes())
			if flags.csv {
				records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
				if err != nil || len(records) != 2 || len(records[0]) != 1 || len(records[1]) != 1 {
					t.Fatalf("selected CSV = %q, error = %v", data, err)
				}
				data, err = json.Marshal(map[string]string{records[0][0]: records[1][0]})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var envelope struct {
					Data json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					t.Fatal(err)
				}
				data = envelope.Data
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, data); err != nil {
				t.Fatal(err)
			}
			if compact.String() != tc.data {
				t.Fatalf("displayed data = %s, want only %s", &compact, tc.data)
			}
			if tc.metrics == nil {
				if flags.platformAnalytics != nil {
					t.Fatalf("dry run declared analytics: %+v", flags.platformAnalytics)
				}
				return
			}
			if flags.platformAnalytics == nil || len(flags.platformAnalytics.BusinessMetrics) != len(tc.metrics) {
				t.Fatalf("missing full analytics declaration: %+v", flags.platformAnalytics)
			}
			for name, want := range tc.metrics {
				if got := flags.platformAnalytics.BusinessMetrics[name]; got == nil || *got != want {
					t.Errorf("declared %s = %v, want %v", name, got, want)
				}
			}
		})
	}
}

func TestG4CompactCohortPreservesObservedZeroRetention(t *testing.T) {
	report := cohort([]resourceRow{
		{ID: "event-1", Data: map[string]any{"profile_id": "profile-1", "datetime": "2026-11-08T00:00:00Z"}},
		{ID: "event-2", Data: map[string]any{"profile_id": "profile-1", "datetime": "2026-11-08T00:01:00Z"}},
	}, "", "month")
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	const wantReport = `[{"cohort":"2026-11","profiles":1,"retained":0,"retention_rate":0}]`
	if string(raw) != wantReport {
		t.Fatalf("unfiltered cohort = %s, want %s", raw, wantReport)
	}
	flags := &rootFlags{platformSession: platformCommandTestSession(t)}
	root := &cobra.Command{Use: "fixture-pp-cli"}
	root.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "JSON output")
	root.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Compact output")
	root.AddCommand(&cobra.Command{Use: "cohort", RunE: func(cmd *cobra.Command, _ []string) error {
		return printJSONFiltered(cmd.OutOrStdout(), report, flags)
	}})
	wrapPlatformCommandOutputs(root, flags)
	var output, diagnostics bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&diagnostics)
	root.SetArgs([]string{"cohort", "--json", "--compact"})
	if err := root.Execute(); err != nil {
		t.Fatalf("compact cohort rejected: %v; stdout=%s stderr=%s", err, &output, &diagnostics)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, platformTestOutputPayload(t, output.Bytes())); err != nil {
		t.Fatal(err)
	}
	if compact.String() != wantReport {
		t.Fatalf("displayed cohort = %s, want %s", &compact, wantReport)
	}
	if flags.platformAnalytics == nil {
		t.Fatal("missing unfiltered analytics declaration")
	}
	for name, want := range map[string]float64{"profiles": 1, "retained": 0, "retention_rate": 0} {
		got := flags.platformAnalytics.BusinessMetrics[name]
		if got == nil {
			t.Fatalf("declared %s = null, want %v", name, want)
		}
		t.Logf("declared %s = %v", name, *got)
		if *got != want {
			t.Errorf("declared %s = %v, want %v", name, *got, want)
		}
	}
}

func TestG4SelectedReportStillRejectsZeroBusinessMetrics(t *testing.T) {
	root, flags := g4LocalRoot(t)
	var output, diagnostics bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&diagnostics)
	root.SetArgs([]string{"send-fatigue", "--db", flags.platformSession.Paths.DataFile, "--json", "--threshold", "9", "--select", "window"})
	err := root.Execute()
	finalizePlatformInvocation(flags, &err)
	const wantError = "analytics result rejected: all_business_metrics_zero_or_null"
	if err == nil || err.Error() != wantError {
		t.Fatalf("error = %v, want %q; stdout=%s stderr=%s", err, wantError, &output, &diagnostics)
	}
	var result struct {
		Error struct {
			Code   string `json:"code"`
			Reason string `json:"reason"`
		} `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Error.Code != "analytics_result_rejected" || result.Error.Reason != "all_business_metrics_zero_or_null" || strings.Contains(output.String(), `"window":"7d"`) {
		t.Fatalf("rejected report output = %s", &output)
	}
	if flags.platformAnalytics == nil {
		t.Fatal("missing full analytics declaration")
	}
	for _, name := range []string{"fatigued_profiles", "fatigued_percentage"} {
		if got := flags.platformAnalytics.BusinessMetrics[name]; got == nil || *got != 0 {
			t.Errorf("declared %s = %v, want an observed zero", name, got)
		}
	}
}
