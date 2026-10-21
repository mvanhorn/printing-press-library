// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil/testenv"
)

func newLoopbackTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("loopback listener unavailable: %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = listener
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func TestCompareWindowsAndSumDaily(t *testing.T) {
	end := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	cs, ce, ps, pe := compareWindows(end, 7)
	if cs.Format("2006-01-02") != "2026-10-14" || ce.Format("2006-01-02") != "2026-10-20" || ps.Format("2006-01-02") != "2026-10-07" || pe.Format("2006-01-02") != "2026-10-13" {
		t.Fatalf("windows = %s..%s and %s..%s", cs, ce, ps, pe)
	}
	got, err := sumDailyMetrics(json.RawMessage(`{"all":{"daily_metrics":[{"metrics":{"IMPRESSION":2,"SAVE":"bad"}},{"metrics":{"IMPRESSION":3,"SAVE":1}}]}}`), []string{"IMPRESSION", "SAVE"})
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if _, err := sumDailyMetrics(json.RawMessage(`{"all":"oops"}`), []string{"IMPRESSION"}); err == nil {
		t.Fatal("malformed analytics must be an error, not zero totals")
	}
	if got["IMPRESSION"] != 5 || got["SAVE"] != 1 {
		t.Fatalf("totals = %#v", got)
	}
}

func TestPerfCompareLiveAndValidation(t *testing.T) {
	testenv.Isolate(t)
	oldNow := nowUTC
	defer func() { nowUTC = oldNow }()
	nowUTC = func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }
	t.Setenv("PINTEREST_ACCESS_TOKEN", "test-token")
	srv := newLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start_date") == "2026-10-14" {
			_, _ = w.Write([]byte(`{"all":{"daily_metrics":[{"metrics":{"IMPRESSION":12,"SAVE":3}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"all":{"daily_metrics":[{"metrics":{"IMPRESSION":8,"SAVE":0}}]}}`))
	}))
	t.Setenv("PINTEREST_BASE_URL", srv.URL)
	out, err := runPinterestCmd(t, "", "perf", "compare", "--days=7", "--metrics=IMPRESSION,SAVE", "--json")
	if err != nil {
		t.Fatalf("compare: %v %s", err, out)
	}
	var got struct {
		Metrics []struct {
			Metric   string   `json:"metric"`
			Current  float64  `json:"current"`
			Prior    float64  `json:"prior"`
			Delta    float64  `json:"delta"`
			DeltaPct *float64 `json:"delta_pct"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got.Metrics) != 2 {
		t.Fatalf("parse comparison: %v\n%s", err, out)
	}
	imp, save := got.Metrics[0], got.Metrics[1]
	if imp.Metric != "IMPRESSION" || imp.Current != 12 || imp.Prior != 8 || imp.Delta != 4 || imp.DeltaPct == nil || *imp.DeltaPct != 50 {
		t.Fatalf("IMPRESSION row = %+v", imp)
	}
	if save.Metric != "SAVE" || save.Delta != 3 || save.DeltaPct != nil {
		t.Fatalf("SAVE row should have null delta_pct when prior is 0: %+v", save)
	}
	_, err = runPinterestCmd(t, "", "perf", "compare", "--days=40", "--end-date=2026-09-01", "--json")
	if err == nil || !strings.Contains(err.Error(), "--days and --end-date") {
		t.Fatalf("expected window usage error, got %v", err)
	}
}

// TestNovelPerfCompareHelpWires smoke-tests that the perf compare command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPerfCompareHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"perf", "compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("perf compare --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "compare"} {
		if !strings.Contains(help, want) {
			t.Fatalf("perf compare --help missing %q in output:\n%s", want, help)
		}
	}
}
