// Sweep findings: --limit truncation and either dependent report fetch/decode failure print a partial scorecard envelope and exit API-class non-zero.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
)

func TestNovelReportsScorecardHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"reports", "scorecard", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Usage:", "scorecard", "--user", "--limit", "--period", "--as-of", "UTC", "Monday-based", "must be inside that window"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, out.String())
		}
	}
}

func TestNovelReportsScorecardBehavior(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/reports/targets":
			if r.URL.Query().Get("userId") != "u1" {
				t.Errorf("userId=%q", r.URL.Query().Get("userId"))
			}
			_, _ = w.Write([]byte(`{"data":{"targets":[{"userId":"u1","metric":"calls","period":"monthly","value":100},{"userId":"u1","metric":"conversations","period":"monthly","value":20}]}}`))
		case "/reports/agent-performance":
			if r.URL.Query().Get("userIds") != "u1" || r.URL.Query().Get("from") != "2026-09-01T00:00:00Z" || r.URL.Query().Get("to") != "2026-10-01T00:00:00Z" {
				t.Errorf("query=%v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"data":{"rows":[{"userId":"u1","name":"Ada","metrics":{"calls":{"actual":50},"conversations":{"actual":5}}},{"userId":"u2","name":"No Target","metrics":{"calls":{"actual":7}}}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", server.URL)
	t.Setenv("CONDUYT_API_KEY", "test")
	flags := &rootFlags{asJSON: true}
	cmd := newNovelReportsScorecardCmd(flags)
	cmd.SetArgs([]string{"--from", "2026-09-01", "--to", "2026-10-01", "--user", "u1"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var view scorecardView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("decode %v: %s", err, out.String())
	}
	rows := view.Rows
	if len(rows) != 3 {
		t.Fatalf("rows=%+v", rows)
	}
	if rows[0].Metric != "calls" || rows[0].AttainmentPct != 50 {
		t.Fatalf("calls=%+v", rows[0])
	}
	if rows[0].Period != "monthly" {
		t.Fatalf("period=%q", rows[0].Period)
	}
	if rows[1].Metric != "conversations" || rows[1].AttainmentPct != 25 {
		t.Fatalf("conversations=%+v", rows[1])
	}
	if rows[2].UserID != "u2" || rows[2].Status != "no_target" {
		t.Fatalf("no-target=%+v", rows[2])
	}
}

func TestNovelReportsScorecardDefaultDoesNotCapLargeTenant(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reports/targets" {
			_, _ = w.Write([]byte(`{"data":{"targets":[]}}`))
			return
		}
		if r.URL.Path != "/reports/agent-performance" {
			http.NotFound(w, r)
			return
		}
		var agents []scorecardAgent
		for i := 0; i < 202; i++ {
			metrics := map[string]scorecardMetric{}
			for m := 0; m < 7; m++ {
				metrics[fmt.Sprintf("metric-%d", m)] = scorecardMetric{Actual: 1}
			}
			agents = append(agents, scorecardAgent{UserID: fmt.Sprintf("u-%03d", i), Metrics: metrics})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"rows": agents}})
	}))
	defer server.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", server.URL)
	t.Setenv("CONDUYT_API_KEY", "test")
	cmd := newNovelReportsScorecardCmd(&rootFlags{asJSON: true})
	cmd.SetArgs([]string{"--period", "month", "--as-of", "2026-09-10"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var view scorecardView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Partial || view.Checked != 1414 || len(view.Rows) != 1414 || view.Total == nil || *view.Total != 1414 {
		t.Fatalf("view=%+v", view)
	}
}

func TestNovelReportsScorecardPaceMath(t *testing.T) {
	rows := buildScorecardRows([]scorecardTarget{{UserID: "a", Metric: "calls", Period: "monthly", Value: 100}}, []scorecardAgent{{UserID: "a", Metrics: map[string]scorecardMetric{"calls": {Actual: 33}}}}, "monthly", mustDate(t, "2026-09-01"), mustDate(t, "2026-10-01"), mustDate(t, "2026-09-11"))
	if len(rows) != 1 || rows[0].AttainmentPct != 33 || rows[0].PacePct != 99 || rows[0].Status != "off_pace" {
		t.Fatalf("row=%+v", rows)
	}
}

func TestScorecardCalendarPeriodBounds(t *testing.T) {
	for _, tc := range []struct{ name, period, anchor, from, to string }{
		{"week", "weekly", "2026-09-16", "2026-09-14", "2026-09-21"},
		{"quarter", "quarterly", "2026-05-10", "2026-04-01", "2026-07-01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from, to := scorecardPeriodBounds(tc.period, mustDate(t, tc.anchor))
			if !from.Equal(mustDate(t, tc.from)) || !to.Equal(mustDate(t, tc.to)) {
				t.Fatalf("bounds=%s..%s", from, to)
			}
		})
	}
}

func TestScorecardRejectsNonAlignedExplicitPeriodWindow(t *testing.T) {
	testenv.Isolate(t)
	performanceCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reports/targets" {
			_, _ = w.Write([]byte(`{"data":{"targets":[]}}`))
			return
		}
		performanceCalls++
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_API_KEY", "test")
	cmd := newNovelReportsScorecardCmd(&rootFlags{asJSON: true})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--range", "10d", "--period", "month", "--as-of", "2026-09-10"})
	err := cmd.Execute()
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "not exactly one aligned month") || performanceCalls != 0 {
		t.Fatalf("err=%v calls=%d", err, performanceCalls)
	}
}

func TestScorecardRejectsAsOfOutsideAlignedWindow(t *testing.T) {
	for _, tc := range []struct {
		name, period, from, to, asOf string
	}{
		{"month", "month", "2026-08-01", "2026-09-01", "2026-09-10"},
		{"week", "week", "2026-09-07", "2026-09-14", "2026-09-14"},
		{"quarter", "quarter", "2026-04-01", "2026-07-01", "2026-03-31"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, _, err := scorecardWindow(tc.period, "", tc.from, tc.to, mustDate(t, tc.asOf), mustDate(t, "2026-09-16"), true, false, true)
			if err == nil || !strings.Contains(err.Error(), "--as-of must be inside") {
				t.Fatalf("err=%v", err)
			}

			testenv.Isolate(t)
			cmd := newNovelReportsScorecardCmd(&rootFlags{asJSON: true})
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--period", tc.period, "--from", tc.from, "--to", tc.to, "--as-of", tc.asOf})
			err = cmd.Execute()
			if ExitCode(err) != 2 || !strings.Contains(err.Error(), "--as-of must be inside") {
				t.Fatalf("command err=%v code=%d", err, ExitCode(err))
			}
		})
	}
}

func TestScorecardExplicitWindowDefaultsAsOfWithinWindow(t *testing.T) {
	for _, tc := range []struct {
		name, from, to, today, wantAsOf string
	}{
		{"past window uses final day", "2026-08-01", "2026-09-01", "2026-09-16", "2026-08-31"},
		{"current window uses today", "2026-09-01", "2026-10-01", "2026-09-16", "2026-09-16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from, to, period, asOf, err := scorecardWindow("month", "", tc.from, tc.to, mustDate(t, tc.today), mustDate(t, tc.today), false, false, true)
			if err != nil {
				t.Fatal(err)
			}
			if period != "monthly" || !from.Equal(mustDate(t, tc.from)) || !to.Equal(mustDate(t, tc.to)) || !asOf.Equal(mustDate(t, tc.wantAsOf)) {
				t.Fatalf("window=%s..%s period=%q asOf=%s", from, to, period, asOf)
			}
		})
	}
}

func TestScorecardPastExplicitWindowUsesFullPeriodForPace(t *testing.T) {
	from, to, _, asOf, err := scorecardWindow("month", "", "2026-08-01", "2026-09-01", mustDate(t, "2026-09-16"), mustDate(t, "2026-09-16"), false, false, true)
	if err != nil {
		t.Fatal(err)
	}
	rows := buildScorecardRows([]scorecardTarget{{UserID: "u", Metric: "calls", Period: "monthly", Value: 100}}, []scorecardAgent{{UserID: "u", Metrics: map[string]scorecardMetric{"calls": {Actual: 100}}}}, "monthly", from, to, asOf.AddDate(0, 0, 1))
	if len(rows) != 1 || rows[0].PacePct != 100 || rows[0].Status != "on_pace" {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestNovelReportsScorecardLimitPrintsPartialAndReturnsAPIErr(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reports/targets" {
			_, _ = w.Write([]byte(`{"data":{"targets":[{"userId":"u","metric":"calls","period":"daily","value":1},{"userId":"u","metric":"sms","period":"daily","value":1}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"rows":[]}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_API_KEY", "test")
	cmd := newNovelReportsScorecardCmd(&rootFlags{asJSON: true})
	cmd.SetArgs([]string{"--range", "1d", "--limit", "1"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	if ExitCode(err) != 5 {
		t.Fatalf("err=%v", err)
	}
	var view scorecardView
	if json.NewDecoder(&out).Decode(&view) != nil || !view.Partial || view.Checked != 1 || view.Total == nil || *view.Total != 2 || len(view.Rows) != 1 {
		t.Fatalf("out=%s", out.String())
	}
}

func TestNovelReportsScorecardIncludesTargetsWithoutPerformance(t *testing.T) {
	targets := []scorecardTarget{
		{UserID: "absent", Metric: "calls", Period: "monthly", Value: 100},
		{UserID: "present", Metric: "conversations", Period: "monthly", Value: 20},
	}
	agents := []scorecardAgent{{UserID: "present", Name: "Ada", Metrics: map[string]scorecardMetric{"calls": {Actual: 5}}}}
	rows := buildScorecardRows(targets, agents, "monthly", mustDate(t, "2026-01-01"), mustDate(t, "2026-01-11"), mustDate(t, "2026-01-06"))

	byKey := make(map[string]scorecardRow, len(rows))
	for _, row := range rows {
		byKey[row.UserID+"/"+row.Metric] = row
	}
	for _, key := range []string{"absent/calls", "present/conversations"} {
		row, ok := byKey[key]
		if !ok || row.Actual != 0 || row.Status == "on_pace" {
			t.Fatalf("%s=%+v present=%t", key, row, ok)
		}
	}
}

func TestNovelReportsScorecardEmptyAndFailure(t *testing.T) {
	for _, tc := range []struct {
		name          string
		targetsStatus int
		wantCode      int
		want          string
	}{{"empty", 200, 0, `"rows":[]`}, {"performance failure", 200, 5, "fetching agent performance"}} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/reports/targets" {
					if tc.name == "empty" {
						_, _ = w.Write([]byte(`{"data":{"targets":[]}}`))
					} else {
						_, _ = w.Write([]byte(`{"data":{"targets":[{"userId":"u","metric":"calls","period":"daily","value":1}]}}`))
					}
					return
				}
				if tc.name == "empty" && r.URL.Path == "/reports/agent-performance" {
					_, _ = w.Write([]byte(`{"data":{"rows":[]}}`))
					return
				}
				http.Error(w, "down", 400)
			}))
			defer srv.Close()
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
			t.Setenv("CONDUYT_API_KEY", "test")
			flags := &rootFlags{asJSON: true}
			cmd := newNovelReportsScorecardCmd(flags)
			cmd.SetArgs([]string{"--range", "1d"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			gotCode := 0
			if err != nil {
				gotCode = ExitCode(err)
			}
			if gotCode != tc.wantCode {
				t.Fatalf("err=%v code=%d", err, ExitCode(err))
			}
			if tc.wantCode == 0 && !strings.Contains(compactTestJSON(out.String()), tc.want) {
				t.Fatalf("out=%s", out.String())
			}
			if tc.wantCode != 0 && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestNovelReportsScorecardInvalidCollectionShapesPrintPartial(t *testing.T) {
	for _, endpoint := range []string{"targets", "performance"} {
		for _, body := range []string{`{}`, `{"data":{}}`, `{"error":"upstream failure"}`, `{"error":{"code":"down"},"data":{"targets":[],"rows":[]}}`, `{"data":{"message":"upstream failure"}}`} {
			t.Run(endpoint+"/"+body, func(t *testing.T) {
				testenv.Isolate(t)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/reports/targets" {
						if endpoint == "targets" {
							_, _ = w.Write([]byte(body))
						} else {
							_, _ = w.Write([]byte(`{"data":{"targets":[]}}`))
						}
						return
					}
					_, _ = w.Write([]byte(body))
				}))
				defer srv.Close()
				t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
				t.Setenv("CONDUYT_API_KEY", "test")
				cmd := newNovelReportsScorecardCmd(&rootFlags{asJSON: true, noCache: true})
				cmd.SilenceUsage = true
				cmd.SetArgs([]string{"--range", "1d"})
				var out bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&bytes.Buffer{})
				err := cmd.Execute()
				var got scorecardView
				if decodeErr := json.Unmarshal(out.Bytes(), &got); decodeErr != nil {
					t.Fatalf("decode %q: %v", out.String(), decodeErr)
				}
				if ExitCode(err) != 5 || !got.Partial || got.Checked != 0 || len(got.Failures) != 1 || got.Rows == nil {
					t.Fatalf("err=%v got=%+v out=%s", err, got, out.String())
				}
			})
		}
	}
}

func TestNovelReportsScorecardInfersWeekAndReportsOtherPeriodsSkipped(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reports/targets" {
			_, _ = w.Write([]byte(`{"data":{"targets":[{"userId":"u","metric":"calls","period":"weekly","value":10},{"userId":"u","metric":"calls","period":"monthly","value":40}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"rows":[{"userId":"u","name":"Ada","metrics":{"calls":{"actual":5}}}]}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_API_KEY", "test")
	cmd := newNovelReportsScorecardCmd(&rootFlags{asJSON: true})
	cmd.SetArgs([]string{"--from", "2026-09-07", "--to", "2026-09-14"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got scorecardView
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0].Period != "weekly" || got.Rows[0].Target != 10 || got.SkippedTargets != 1 || len(got.SkippedPeriods) != 1 || got.SkippedPeriods[0] != "monthly" {
		t.Fatalf("got=%+v out=%s", got, out.String())
	}
}

func TestNovelReportsScorecardAmbiguousWindowRequiresPeriod(t *testing.T) {
	testenv.Isolate(t)
	targetCalls := 0
	performanceCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reports/targets" {
			targetCalls++
			_, _ = w.Write([]byte(`{"data":{"targets":[{"userId":"u","metric":"calls","period":"weekly","value":10},{"userId":"u","metric":"calls","period":"monthly","value":40}]}}`))
			return
		}
		performanceCalls++
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_API_KEY", "test")
	cmd := newNovelReportsScorecardCmd(&rootFlags{asJSON: true})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--from", "2026-09-01", "--to", "2026-09-11"})
	err := cmd.Execute()
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "pass --period") || targetCalls != 0 || performanceCalls != 0 {
		t.Fatalf("err=%v target_calls=%d performance_calls=%d", err, targetCalls, performanceCalls)
	}
}

func TestNovelReportsScorecardPeriodMonthSelectsMonthlyTarget(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reports/targets" {
			_, _ = w.Write([]byte(`{"data":{"targets":[{"userId":"u","metric":"calls","period":"weekly","value":10},{"userId":"u","metric":"calls","period":"monthly","value":40}]}}`))
			return
		}
		if r.URL.Query().Get("from") != "2026-09-01T00:00:00Z" || r.URL.Query().Get("to") != "2026-10-01T00:00:00Z" {
			t.Errorf("query=%v", r.URL.Query())
		}
		_, _ = w.Write([]byte(`{"data":{"rows":[{"userId":"u","name":"Ada","metrics":{"calls":{"actual":20}}}]}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_API_KEY", "test")
	cmd := newNovelReportsScorecardCmd(&rootFlags{asJSON: true})
	cmd.SetArgs([]string{"--period", "month", "--as-of", "2026-09-10"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got scorecardView
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Timezone != "UTC" || got.From != "2026-09-01T00:00:00Z" || got.To != "2026-10-01T00:00:00Z" || got.AsOf != "2026-09-10" {
		t.Fatalf("window=%+v", got)
	}
	if len(got.Rows) != 1 || got.Rows[0].Period != "monthly" || got.Rows[0].Target != 40 || got.Rows[0].AttainmentPct != 50 || got.Rows[0].PacePct != 150 || got.Rows[0].Status != "on_pace" || got.SkippedTargets != 1 || got.SkippedPeriods[0] != "weekly" {
		t.Fatalf("got=%+v", got)
	}
}

func mustDate(t *testing.T, v string) time.Time {
	t.Helper()
	d, e := time.Parse("2006-01-02", v)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
