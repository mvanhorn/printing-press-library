// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

func TestNovelPulseHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"pulse", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pulse --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "pulse [flags]", "Do NOT use this command for a current snapshot"} {
		if !strings.Contains(help, want) {
			t.Fatalf("pulse --help missing %q in output:\n%s", want, help)
		}
	}
}

// pulseSeries builds baseline days at baselinePerDay then window days at
// windowPerDay, with optional per-day bounces and spam in the window.
func pulseSeries(baselineDays, baselinePerDay, windowDays, windowPerDay, windowBounces, windowSpam int) []postmarkDay {
	out := make([]postmarkDay, 0, baselineDays+windowDays)
	for i := 0; i < baselineDays; i++ {
		out = append(out, postmarkDay{Sent: baselinePerDay})
	}
	for i := 0; i < windowDays; i++ {
		d := postmarkDay{Sent: windowPerDay}
		if i == 0 {
			d.Bounced, d.Spam = windowBounces, windowSpam
		}
		out = append(out, d)
	}
	return out
}

func flagNames(r pulseRow) []string {
	names := make([]string, 0, len(r.Flags))
	for _, f := range r.Flags {
		names = append(names, f.Flag)
	}
	return names
}

func TestPulseEvaluate(t *testing.T) {
	cases := []struct {
		name       string
		series     []postmarkDay
		wantStatus string
		wantFlags  []string
	}{
		{"stopped: 0 in window, 10/day baseline", pulseSeries(28, 10, 7, 0, 0, 0), pulseFlagStopped, []string{pulseFlagStopped}},
		{"dropped: 3/day vs 10/day", pulseSeries(28, 10, 7, 3, 0, 0), pulseFlagDropped, []string{pulseFlagDropped}},
		{"steady: 9/day vs 10/day", pulseSeries(28, 10, 7, 9, 0, 0), pulseStatusOK, []string{}},
		{"bounce spike: 7 of 70 bounced", pulseSeries(28, 10, 7, 10, 7, 0), pulseFlagSpiked, []string{pulseFlagSpiked}},
		{"spam spike: 1 complaint in 70", pulseSeries(28, 10, 7, 10, 0, 1), pulseFlagSpiked, []string{pulseFlagSpiked}},
		{"low volume never stops", pulseSeries(28, 1, 7, 0, 0, 0), pulseStatusLowVolume, []string{}},
		{"empty everywhere", pulseSeries(28, 0, 7, 0, 0, 0), pulseStatusLowVolume, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := pulseEvaluate(tc.series, 7, 28, 5)
			if row.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q (row %+v)", row.Status, tc.wantStatus, row)
			}
			got := flagNames(row)
			if strings.Join(got, ",") != strings.Join(tc.wantFlags, ",") {
				t.Fatalf("flags = %v, want %v", got, tc.wantFlags)
			}
		})
	}
}

func TestPulseEvaluateStoppedNumbers(t *testing.T) {
	row := pulseEvaluate(pulseSeries(28, 10, 7, 0, 0, 0), 7, 28, 5)
	if row.WindowSent != 0 || row.BaselineSent != 280 || row.BaselineDailyAvg != 10 || row.ExpectedWindowSent != 70 {
		t.Fatalf("numbers = %+v", row)
	}
	if row.ChangePct == nil || *row.ChangePct != -100 {
		t.Fatalf("change = %v", row.ChangePct)
	}
}

func TestPulseEvaluateBounceNeedsBaselineContrast(t *testing.T) {
	// 6% bounces in the window but the baseline also bounced at 6%: not a spike.
	series := pulseSeries(28, 100, 7, 100, 42, 0)
	for i := 0; i < 28; i++ {
		series[i].Bounced = 6
	}
	row := pulseEvaluate(series, 7, 28, 5)
	for _, f := range row.Flags {
		if f.Flag == pulseFlagSpiked {
			t.Fatalf("flagged spike without 2x contrast: %+v", row)
		}
	}
}

func TestPulseWindowZeroFillMatchesWindowPlusBaseline(t *testing.T) {
	now := time.Date(2026, 9, 30, 15, 0, 0, 0, postmarkEastern())
	w := newPulseWindow(now, 7, 28)
	if got := w.windowStart.Format(postmarkDateLayout); got != "2026-09-24" {
		t.Fatalf("window start = %s", got)
	}
	if got := w.baselineStart.Format(postmarkDateLayout); got != "2026-08-27" {
		t.Fatalf("baseline start = %s", got)
	}
	// Postmark omits empty days: only two dates come back.
	sparse := map[string]*postmarkDay{
		"2026-08-28": {Sent: 4},
		"2026-09-29": {Sent: 2},
	}
	series := zeroFillDays(sparse, w.baselineStart, w.totalDays())
	if len(series) != 35 {
		t.Fatalf("day count = %d, want window+baseline = 35", len(series))
	}
	if series[0].Date != "2026-08-27" || series[34].Date != "2026-09-30" {
		t.Fatalf("series spans %s..%s", series[0].Date, series[34].Date)
	}
	row := pulseEvaluate(series, 7, 28, 0)
	if row.WindowSent != 2 || row.BaselineSent != 4 {
		t.Fatalf("window/baseline split wrong: %+v", row)
	}
}

func TestPulseAttachNextScopesCommands(t *testing.T) {
	w := newPulseWindow(time.Date(2026, 9, 30, 12, 0, 0, 0, postmarkEastern()), 7, 28)
	row := pulseEvaluate(pulseSeries(28, 10, 7, 0, 0, 0), 7, 28, 5)
	row.Server, row.Stream = "Main App", "broadcast"
	pulseAttachNext(&row, w)
	next := row.Flags[0].Next
	if !strings.Contains(next, `--server 'Main App'`) || !strings.Contains(next, "--messagestream broadcast") || !strings.HasPrefix(next, "postmark-pp-cli messages list") {
		t.Fatalf("next = %q", next)
	}
}

// TestPulseFanoutPartialFailure runs pulse across two fake servers where one
// server's stats fail: the failure is reported, not averaged in as zeros.
func TestPulseFanoutPartialFailure(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{1, "Alpha", "tok-alpha"}, [3]any{2, "Beta", "tok-beta"}))
	today := postmarkDayStart(time.Now()).Format(postmarkDateLayout)
	f.handle("GET /stats/outbound/sends", func(r *http.Request, _ string) (int, any) {
		if r.Header.Get(postmarkServerTokenHeader) == "tok-beta" {
			return 422, `{"ErrorCode":1501,"Message":"boom"}`
		}
		return 200, map[string]any{"Days": []map[string]any{{"Date": today, "Sent": 3}}, "Sent": 3}
	})
	f.reply("GET /stats/outbound/bounces", 200, map[string]any{"Days": []any{}})
	f.reply("GET /stats/outbound/spam", 200, map[string]any{"Days": []any{}})

	stdout, stderr, err := postmarkRun(t, "pulse", "--json", "--min-baseline", "0")
	if err != nil {
		t.Fatalf("pulse error = %v\nstderr=%s", err, stderr)
	}
	var res pulseResult
	postmarkResults(t, stdout, &res)
	if len(res.Rows) != 1 || res.Rows[0].Server != "Alpha" || res.Rows[0].WindowSent != 3 {
		t.Fatalf("rows = %+v", res.Rows)
	}
	if len(res.FetchFailures) != 1 || res.FetchFailures[0].Server != "Beta" {
		t.Fatalf("fetch_failures = %+v", res.FetchFailures)
	}
	if !strings.Contains(stderr, "1 of 2 server fetches failed") {
		t.Fatalf("stderr missing partial-failure warning: %s", stderr)
	}
	for _, req := range f.log() {
		if strings.HasPrefix(req.Path, "/stats") && req.AccountTok != "" {
			t.Fatalf("stats request carried the account token: %+v", req)
		}
	}
}
