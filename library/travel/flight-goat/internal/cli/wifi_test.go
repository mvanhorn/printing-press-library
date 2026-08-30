// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the `wifi` (SeatWifi) command group.

package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/seatwifi"
	"github.com/spf13/cobra"
)

func runWifiCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	flags := &rootFlags{}
	cmd := newWifiCmd(flags)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errb.String(), err
}

func TestWifiCmd_HelpListsSubcommands(t *testing.T) {
	stdout, _, err := runWifiCmd(t, "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}
	for _, want := range []string{"flight", "airline", "airlines", "rollouts", "speed", "airline-speed", "search"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestWifiFlight_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "UA1234"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "/api/v1/flights/UA1234") {
		t.Errorf("dry-run should contain flight path, got: %s", s)
	}
	if !strings.Contains(s, "dry_run") && !strings.Contains(s, "dry run") {
		t.Errorf("dry-run marker missing: %s", s)
	}
}

func TestWifiAirline_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"airline", "UA"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/airlines/UA") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiAirlines_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"airlines"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/airlines") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiRollouts_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"rollouts", "UA"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/rollouts/UA") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiRollouts_All_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"rollouts"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/rollouts") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiSpeed_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"speed", "UA1234"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/speed-reports/stats/UA1234") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiSearch_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"search", "united"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/search") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiAirlineSpeed_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"airline-speed", "UA"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/speed-reports/airline/UA") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiFlight_DryRunQuiet(t *testing.T) {
	flags := &rootFlags{dryRun: true, quiet: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "UA1234"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("quiet dry-run should emit nothing, got %q", out.String())
	}
}

func TestWifiFlight_DryRunJSON(t *testing.T) {
	flags := &rootFlags{dryRun: true, asJSON: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "UA1234"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "dry run - no request sent") {
		t.Fatalf("json dry-run leaked human text: %s", got)
	}
	if !strings.Contains(got, `"dry_run"`) || !strings.Contains(got, `/api/v1/flights/UA1234`) {
		t.Fatalf("json dry-run missing payload, got: %s", got)
	}
}

func TestWifiFlight_RequiresArg(t *testing.T) {
	_, _, err := runWifiCmd(t, "flight")
	if err == nil {
		t.Fatal("expected error for missing flight arg")
	}
}

func TestWifiSearch_RequiresArg(t *testing.T) {
	_, _, err := runWifiCmd(t, "search")
	if err == nil {
		t.Fatal("expected error for missing search arg")
	}
}

func TestWifiFlight_WhitespaceArgRejectedBeforeDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "  "})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for whitespace-only flight")
	}
	if !strings.Contains(err.Error(), "flight number required") {
		t.Fatalf("error = %v, want flight number required", err)
	}
	got := out.String()
	if strings.Contains(got, "dry run") || strings.Contains(got, "dry_run") || strings.Contains(got, "/api/v1/flights/") {
		t.Fatalf("should not print a dry-run URL, got %q", got)
	}
}

func TestWifiSearch_WhitespaceArgRejectedBeforeDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"search", "  "})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for whitespace-only search query")
	}
	if !strings.Contains(err.Error(), "query required") {
		t.Fatalf("error = %v, want query required", err)
	}
	got := out.String()
	if strings.Contains(got, "dry run") || strings.Contains(got, "dry_run") || strings.Contains(got, "/api/search") {
		t.Fatalf("should not print a dry-run URL, got %q", got)
	}
}

func TestWifiSearch_DryRunEncodesQuery(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"search", "foo&bar"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "q=foo%26bar") {
		t.Errorf("dry-run should query-escape reserved chars, got: %s", got)
	}
	if strings.Contains(got, "q=foo&bar") {
		t.Errorf("dry-run leaked unescaped query: %s", got)
	}
}

func TestWifiFlight_DryRunPathEscapes(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "ua/1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "/api/v1/flights/UA%2F1") {
		t.Errorf("dry-run should path-escape flight, got: %s", got)
	}
}

func TestWifiMachineFlagsSkipHumanTable(t *testing.T) {
	// Greptile: --csv/--quiet/--select/--compact must not pick the TTY table.
	cases := map[string]*rootFlags{
		"csv":     {csv: true},
		"quiet":   {quiet: true},
		"compact": {compact: true},
		"plain":   {plain: true},
		"select":  {selectFields: "code"},
		"json":    {asJSON: true},
	}
	for name, flags := range cases {
		if wantsHumanTable(os.Stdout, flags) {
			t.Errorf("%s: still chose human table on TTY", name)
		}
	}
}

func TestHumanText_StripsANSIAndControlSequences(t *testing.T) {
	in := "keep\tcolumns\nlines\x00\x07\x1b[31mred\x1b[0m\x7f\u0085\u009b31m"
	got := humanText(in)
	if strings.ContainsAny(got, "\x00\x07\x1b\x7f") {
		t.Fatalf("control/ANSI bytes survived: %q", got)
	}
	if strings.ContainsRune(got, '\t') || strings.ContainsRune(got, '\n') {
		t.Fatalf("tab/newline should become spaces: %q", got)
	}
	if !strings.Contains(got, "keep") || !strings.Contains(got, "columns") || !strings.Contains(got, "red") {
		t.Fatalf("lost printable content: %q", got)
	}
	want := "keep columns lines[31mred[0m31m"
	if got != want {
		t.Fatalf("humanText = %q, want %q", got, want)
	}
}

func TestPrintWifiFlightHuman_ScrubsDetailsAndExplanation(t *testing.T) {
	details := "hi \x1b[31mred\x1b[0m\x07"
	res := &seatwifi.FlightWifi{
		FlightNumber: "UA1",
		Airline:      "United",
		AirlineCode:  "UA",
		WifiProvider: "starlink",
		Details:      &details,
		Explanation:  "note \x1b]0;title\x07 done",
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := printWifiFlightHuman(cmd, res); err != nil {
		t.Fatalf("print: %v", err)
	}
	got := out.String()
	if strings.ContainsAny(got, "\x1b\x07") {
		t.Fatalf("human path leaked control sequences: %q", got)
	}
	if !strings.Contains(got, "details:") || !strings.Contains(got, "note:") {
		t.Fatalf("missing details/note blocks: %q", got)
	}
	if !strings.Contains(got, "red") || !strings.Contains(got, "done") {
		t.Fatalf("lost printable content: %q", got)
	}
}

func TestPrintWifiFlightJSON_PreservesControlSequences(t *testing.T) {
	details := "hi \x1b[31mred"
	res := &seatwifi.FlightWifi{FlightNumber: "UA1", Details: &details, Explanation: "n\x1bote"}
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := printJSONFiltered(cmd.OutOrStdout(), res, flags); err != nil {
		t.Fatalf("json: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `\u001b`) {
		t.Fatalf("JSON path should keep ESC as \\u001b, got %s", got)
	}
}

func TestPrintWifiAirlineHuman_ScrubsFleetInfo(t *testing.T) {
	res := &seatwifi.Airline{
		Code:      "AS",
		Name:      "Alaska",
		FleetInfo: "Starlink \x1b[2Jwipe\x1b[0m fleet",
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := printWifiAirlineHuman(cmd, res); err != nil {
		t.Fatalf("print: %v", err)
	}
	got := out.String()
	if strings.ContainsRune(got, '\x1b') {
		t.Fatalf("fleetInfo leaked ESC: %q", got)
	}
	if !strings.Contains(got, "fleetInfo:") || !strings.Contains(got, "Starlink") {
		t.Fatalf("missing fleetInfo content: %q", got)
	}
}

func TestWifiCtx_AppliesCommandTimeout(t *testing.T) {
	flags := &rootFlags{timeout: 45 * time.Second}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	ctx, cancel := wifiCtx(cmd, flags)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline from --timeout")
	}
	remaining := time.Until(deadline)
	if remaining < 40*time.Second || remaining > 45*time.Second {
		t.Fatalf("deadline remaining = %v, want ~45s", remaining)
	}
}

func TestHumanText_RolloutNotesScrubbedBeforeTruncate(t *testing.T) {
	notes := "\x1b[31m" + strings.Repeat("x", 100)
	got := truncateRunes(humanText(notes), 80)
	if strings.ContainsRune(got, '\x1b') {
		t.Fatalf("truncated notes leaked ESC: %q", got)
	}
	if !strings.Contains(got, "xxx") {
		t.Fatalf("lost printable notes: %q", got)
	}
}

func TestRolloutDone_ScrubsExpectedCompletion(t *testing.T) {
	done := "Q2 \x1b[31m2026\x1b[0m\x07"
	got := rolloutDone(seatwifi.Rollout{ExpectedCompletion: &done})
	if strings.ContainsAny(got, "\x1b\x07") {
		t.Fatalf("rolloutDone leaked control sequences: %q", got)
	}
	if !strings.Contains(got, "Q2") || !strings.Contains(got, "2026") {
		t.Fatalf("lost printable completion: %q", got)
	}
}

func TestPrintWifiAirlineRolloutsHuman_ScrubsExpectedCompletion(t *testing.T) {
	done := "Q2 \x1b[31m2026\x1b[0m\x07"
	res := &seatwifi.AirlineRolloutsResponse{
		Airline: "UA",
		Rollouts: []seatwifi.Rollout{{
			AircraftType:       "737",
			Status:             "active",
			Notes:              "ok",
			ExpectedCompletion: &done,
		}},
	}
	cmd := &cobra.Command{}
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := printWifiAirlineRolloutsHuman(cmd, res); err != nil {
		t.Fatalf("print: %v", err)
	}
	got := out.String() + errb.String()
	if strings.ContainsAny(got, "\x1b\x07") {
		t.Fatalf("single-airline rollout table leaked control sequences: %q", got)
	}
	if !strings.Contains(got, "COMPLETION") {
		t.Fatalf("missing completion column: %q", got)
	}
	if !strings.Contains(got, "Q2") || !strings.Contains(got, "2026") {
		t.Fatalf("lost printable completion: %q", got)
	}
}
