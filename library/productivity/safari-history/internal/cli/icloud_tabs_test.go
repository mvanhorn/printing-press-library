package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	safariSource "github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source/safari"
)

// withStubbedICloudSeams overrides the package-level indirection seams for the
// duration of a test so the command's RunE runs against in-memory stubs — never
// the real CloudTabs.db and never osascript. Returns a pointer the test can read
// to assert whether RefreshSafari was invoked.
func withStubbedICloudSeams(t *testing.T, tabs []safariSource.CloudTab) *bool {
	t.Helper()
	refreshCalled := false
	origLocate, origRead, origSummarize, origRefresh, origSafariRunning := locateCloudTabsDBFn, readCloudTabsFn, summarizeCloudTabsFn, refreshSafariFn, safariRunningFn
	t.Cleanup(func() {
		locateCloudTabsDBFn, readCloudTabsFn, summarizeCloudTabsFn, refreshSafariFn, safariRunningFn = origLocate, origRead, origSummarize, origRefresh, origSafariRunning
	})
	locateCloudTabsDBFn = func() (string, error) { return "/stub/CloudTabs.db", nil }
	readCloudTabsFn = func(_ string, f safariSource.CloudTabsFilter) ([]safariSource.CloudTab, error) {
		// Honor the explicit-limit cap so a test can prove the cap reaches the
		// reader, while leaving the default-unlimited decision to RunE.
		out := tabs
		if f.Limit > 0 && len(out) > f.Limit {
			out = out[:f.Limit]
		}
		return out, nil
	}
	summarizeCloudTabsFn = func(_ string, _ safariSource.CloudTabsFilter) ([]safariSource.CloudTabDeviceSummary, error) {
		return nil, nil
	}
	refreshSafariFn = func(time.Duration) error { refreshCalled = true; return nil }
	safariRunningFn = func() (bool, error) { return true, nil }
	return &refreshCalled
}

// runICloudTabsJSON builds the icloud-tabs command in isolation with a
// controlled RootOptions (JSON forced on), runs it with the given args, and
// returns the decoded rows. output.Render writes to os.Stdout, so this captures
// os.Stdout for the duration of the run.
func runICloudTabsJSON(t *testing.T, args ...string) []map[string]any {
	t.Helper()
	opts := &RootOptions{Output: output.Flags{JSON: true, Limit: 20, Command: "icloud-tabs"}}
	cmd := newICloudTabsCmd(opts)
	// Re-declare the persistent --limit flag locally so the subcommand parses it
	// in isolation (root normally owns it); binding to opts.Output.Limit and
	// defaulting to 20 mirrors root, so cmd.Flags().Changed("limit") behaves
	// exactly as in production.
	cmd.Flags().IntVar(&opts.Output.Limit, "limit", 20, "row limit")
	cmd.Flags().BoolVar(&opts.Output.JSON, "json", true, "JSON output")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	runErr := cmd.Execute()
	_ = w.Close()
	os.Stdout = orig
	data, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("execute: %v", runErr)
	}
	var rows []map[string]any
	if strings.TrimSpace(string(data)) != "" {
		if jerr := json.Unmarshal(data, &rows); jerr != nil {
			t.Fatalf("decode %q: %v", string(data), jerr)
		}
	}
	return rows
}

func fixtureTabs() []safariSource.CloudTab {
	base := time.Unix(700_000_000+978307200, 0).UTC()
	out := make([]safariSource.CloudTab, 0, 25)
	for i := 0; i < 25; i++ {
		out = append(out, safariSource.CloudTab{
			DeviceName:     "Vinny's iPhone",
			DeviceType:     "iPhone",
			Title:          fmt.Sprintf("tab %d", i),
			URL:            fmt.Sprintf("https://example/%d", i),
			LastViewedTime: base.Add(time.Duration(i) * time.Minute),
		})
	}
	return out
}

// BLOCKER fix: the no-silent-cap default must return ALL tabs when --limit is
// not passed, and only cap when it is. This drives the real RunE so the
// cmd.Flags().Changed("limit") logic is load-bearing.
func TestICloudTabs_NoSilentCapByDefault(t *testing.T) {
	withStubbedICloudSeams(t, fixtureTabs())
	rows := runICloudTabsJSON(t) // no --limit
	if len(rows) != 25 {
		t.Fatalf("default should return ALL 25 tabs (no silent cap), got %d", len(rows))
	}
}

func TestICloudTabs_ExplicitLimitCaps(t *testing.T) {
	withStubbedICloudSeams(t, fixtureTabs())
	rows := runICloudTabsJSON(t, "--limit", "3")
	if len(rows) != 3 {
		t.Fatalf("explicit --limit 3 should cap to 3, got %d", len(rows))
	}
}

func TestICloudTabs_FilterFlagsReachReader(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		assertions func(t *testing.T, got safariSource.CloudTabsFilter)
	}{
		{
			name: "pinned",
			args: []string{"--pinned"},
			assertions: func(t *testing.T, got safariSource.CloudTabsFilter) {
				t.Helper()
				if !got.PinnedOnly {
					t.Fatal("--pinned did not reach reader filter")
				}
			},
		},
		{
			name: "device-name",
			args: []string{"--device-name", "iPhone"},
			assertions: func(t *testing.T, got safariSource.CloudTabsFilter) {
				t.Helper()
				if got.DeviceName != "iPhone" {
					t.Fatalf("--device-name filter = %q, want %q", got.DeviceName, "iPhone")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withStubbedICloudSeams(t, fixtureTabs()[:1])
			var got safariSource.CloudTabsFilter
			readCloudTabsFn = func(_ string, f safariSource.CloudTabsFilter) ([]safariSource.CloudTab, error) {
				got = f
				return fixtureTabs()[:1], nil
			}

			runICloudTabsJSON(t, tt.args...)
			tt.assertions(t, got)
		})
	}
}

func TestICloudTabs_SummaryJSONRendersDeviceSummaries(t *testing.T) {
	withStubbedICloudSeams(t, fixtureTabs()[:1])
	summarizeCloudTabsFn = func(_ string, _ safariSource.CloudTabsFilter) ([]safariSource.CloudTabDeviceSummary, error) {
		return []safariSource.CloudTabDeviceSummary{{
			DeviceName: "Vinny's iPhone",
			DeviceType: "iPhone",
			TabCount:   7,
		}}, nil
	}

	rows := runICloudTabsJSON(t, "--summary", "--json")
	if len(rows) != 1 {
		t.Fatalf("expected 1 summary row, got %d", len(rows))
	}
	if got := rows[0]["device_name"]; got != "Vinny's iPhone" {
		t.Fatalf("device_name = %v, want %q", got, "Vinny's iPhone")
	}
	if got := rows[0]["tab_count"]; got != float64(7) {
		t.Fatalf("tab_count = %v, want 7", got)
	}
	if _, ok := rows[0]["url"]; ok {
		t.Fatalf("summary row unexpectedly included per-tab url key: %#v", rows[0])
	}
}

// MAJOR fix: last_viewed_time must render as RFC3339 UTC (not local TZ, not a
// wrong layout) on the non-zero path.
func TestICloudTabs_RFC3339Output(t *testing.T) {
	withStubbedICloudSeams(t, fixtureTabs()[:1])
	rows := runICloudTabsJSON(t)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	got, _ := rows[0]["last_viewed_time"].(string)
	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("last_viewed_time %q is not RFC3339: %v", got, err)
	}
	if parsed.Location() != time.UTC && got[len(got)-1] != 'Z' {
		t.Fatalf("last_viewed_time %q is not UTC", got)
	}
}

// MAJOR fix: --refresh must be invoked only when requested, and never otherwise
// (it foregrounds Safari). Guards the side-effecting seam.
func TestICloudTabs_RefreshOnlyWhenRequested(t *testing.T) {
	called := withStubbedICloudSeams(t, fixtureTabs()[:1])
	runICloudTabsJSON(t) // no --refresh
	if *called {
		t.Fatal("RefreshSafari invoked without --refresh (would foreground Safari)")
	}

	called2 := withStubbedICloudSeams(t, fixtureTabs()[:1])
	runICloudTabsJSON(t, "--refresh", "--wait", "0")
	if !*called2 {
		t.Fatal("--refresh did not invoke RefreshSafari")
	}
}

// The missing-DB path produced by RunE must map to exit 4.
func TestICloudTabs_MissingDBExit4(t *testing.T) {
	origLocate := locateCloudTabsDBFn
	t.Cleanup(func() { locateCloudTabsDBFn = origLocate })
	locateCloudTabsDBFn = func() (string, error) {
		return "", fmt.Errorf("%w: /nope", safariSource.ErrCloudTabsDBMissing)
	}
	opts := &RootOptions{Output: output.Flags{JSON: true, Limit: 20, Command: "icloud-tabs"}}
	cmd := newICloudTabsCmd(opts)
	cmd.Flags().IntVar(&opts.Output.Limit, "limit", 20, "row limit")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for missing CloudTabs.db")
	}
	if !errors.Is(err, ErrSourceDBMissing) {
		t.Fatalf("RunE error does not wrap ErrSourceDBMissing: %v", err)
	}
	if got := ExitCodeForError(err); got != ExitSourceDBMissing {
		t.Fatalf("ExitCodeForError = %d, want %d", got, ExitSourceDBMissing)
	}
}

// The permission-denied path produced by RunE must also map to exit 4.
func TestICloudTabs_PermissionExit4(t *testing.T) {
	origLocate := locateCloudTabsDBFn
	t.Cleanup(func() { locateCloudTabsDBFn = origLocate })
	locateCloudTabsDBFn = func() (string, error) {
		return "", fmt.Errorf("%w: /nope", safariSource.ErrCloudTabsDBPermission)
	}
	opts := &RootOptions{Output: output.Flags{JSON: true, Limit: 20, Command: "icloud-tabs"}}
	cmd := newICloudTabsCmd(opts)
	cmd.Flags().IntVar(&opts.Output.Limit, "limit", 20, "row limit")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if !errors.Is(err, ErrSourceDBMissing) {
		t.Fatalf("permission RunE error does not wrap ErrSourceDBMissing: %v", err)
	}
	if got := ExitCodeForError(err); got != ExitSourceDBMissing {
		t.Fatalf("ExitCodeForError = %d, want %d", got, ExitSourceDBMissing)
	}
}

// A permission denial surfaced at READ time (the real Full Disk Access failure
// point — inside snapshotCloudTabs, not the initial locate stat) must ALSO map
// to exit 4, not the generic exit 1. Guards the mapCloudTabsDBErr remap at the
// read/summarize sites.
func TestICloudTabs_ReadPermissionExit4(t *testing.T) {
	origLocate := locateCloudTabsDBFn
	origRead := readCloudTabsFn
	t.Cleanup(func() { locateCloudTabsDBFn = origLocate; readCloudTabsFn = origRead })
	locateCloudTabsDBFn = func() (string, error) { return "/stub/CloudTabs.db", nil }
	readCloudTabsFn = func(_ string, _ safariSource.CloudTabsFilter) ([]safariSource.CloudTab, error) {
		return nil, fmt.Errorf("%w: /stub/CloudTabs.db", safariSource.ErrCloudTabsDBPermission)
	}
	opts := &RootOptions{Output: output.Flags{JSON: true, Limit: 20, Command: "icloud-tabs"}}
	cmd := newICloudTabsCmd(opts)
	cmd.Flags().IntVar(&opts.Output.Limit, "limit", 20, "row limit")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if !errors.Is(err, ErrSourceDBMissing) {
		t.Fatalf("read-time permission error does not wrap ErrSourceDBMissing: %v", err)
	}
	if got := ExitCodeForError(err); got != ExitSourceDBMissing {
		t.Fatalf("read-time permission ExitCodeForError = %d, want %d", got, ExitSourceDBMissing)
	}
}

func executeICloudTabsForErr(t *testing.T, opts *RootOptions, args ...string) string {
	t.Helper()
	cmd := newICloudTabsCmd(opts)
	cmd.Flags().IntVar(&opts.Output.Limit, "limit", 20, "row limit")
	var errBuf bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	if err := cmd.Execute(); err != nil {
		_ = w.Close()
		os.Stdout = orig
		t.Fatalf("execute: %v", err)
	}
	_ = w.Close()
	os.Stdout = orig
	_, _ = io.ReadAll(r)
	return errBuf.String()
}

// A plain read while Safari is not running must emit the --refresh hint on
// STDERR even with non-zero tabs. Safari running, unknown running state, and
// --refresh must suppress it.
func TestICloudTabs_StaleHint(t *testing.T) {
	opts := &RootOptions{Output: output.Flags{JSON: true, Limit: 20, Command: "icloud-tabs"}}

	withStubbedICloudSeams(t, fixtureTabs()[:2])
	safariRunningFn = func() (bool, error) { return false, nil }
	errText := executeICloudTabsForErr(t, opts)
	if !strings.Contains(errText, "these 2 tab(s)") || !strings.Contains(errText, "--refresh") {
		t.Fatalf("expected stale hint with non-zero tab count on stderr, got %q", errText)
	}

	withStubbedICloudSeams(t, nil)
	safariRunningFn = func() (bool, error) { return true, nil }
	errText = executeICloudTabsForErr(t, opts)
	if errText != "" {
		t.Fatalf("hint must be suppressed when Safari is running, got %q", errText)
	}

	withStubbedICloudSeams(t, fixtureTabs()[:1])
	safariRunningFn = func() (bool, error) { return false, fmt.Errorf("unknown") }
	errText = executeICloudTabsForErr(t, opts)
	if errText != "" {
		t.Fatalf("hint must be suppressed when Safari running state is unknown, got %q", errText)
	}

	withStubbedICloudSeams(t, fixtureTabs()[:1])
	safariRunningFn = func() (bool, error) { return false, nil }
	errText = executeICloudTabsForErr(t, opts, "--refresh")
	if errText != "" {
		t.Fatalf("hint must be suppressed under --refresh, got %q", errText)
	}
}

// formatRFC3339 must render the zero time as empty, not a 0001 date.
func TestFormatRFC3339Zero(t *testing.T) {
	if got := formatRFC3339(time.Time{}); got != "" {
		t.Fatalf("zero time formatted as %q, want empty", got)
	}
}
