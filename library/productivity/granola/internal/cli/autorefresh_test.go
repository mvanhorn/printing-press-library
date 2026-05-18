// Copyright 2026 dstevens. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// TestShouldSkipAutoRefresh_TopLevelSkips covers each command name in
// the skip list directly. A regression here would silently start running
// auto-refresh inside commands like `sync` itself (recursion) or `auth`
// (refresh requires the auth those commands establish), so each entry
// gets its own assertion.
func TestShouldSkipAutoRefresh_TopLevelSkips(t *testing.T) {
	skips := []string{
		"sync", "sync-api", "auth", "doctor",
		"help", "version", "completion", "agent-context",
		"profile", "feedback", "which",
	}
	for _, name := range skips {
		t.Run(name, func(t *testing.T) {
			cmd := &cobra.Command{Use: name}
			if !shouldSkipAutoRefresh(cmd) {
				t.Errorf("expected %q to be skipped", name)
			}
		})
	}
}

// TestShouldSkipAutoRefresh_NestedAncestors covers deep subcommands —
// `auth login`, `profile save`, `feedback list` — each of which inherits
// the skip from its parent. Without ancestor walking, sub-flows under
// skip-listed parents would silently auto-refresh.
func TestShouldSkipAutoRefresh_NestedAncestors(t *testing.T) {
	cases := []struct{ parent, leaf string }{
		{"auth", "login"},
		{"auth", "status"},
		{"profile", "save"},
		{"profile", "list"},
		{"feedback", "list"},
	}
	for _, tc := range cases {
		t.Run(tc.parent+"/"+tc.leaf, func(t *testing.T) {
			p := &cobra.Command{Use: tc.parent}
			c := &cobra.Command{Use: tc.leaf}
			p.AddCommand(c)
			if !shouldSkipAutoRefresh(c) {
				t.Errorf("expected %q under %q to be skipped", tc.leaf, tc.parent)
			}
		})
	}
}

// TestShouldSkipAutoRefresh_DataCommands covers commands that should NOT
// be skipped — the whole point of auto-refresh is to fire on these. A
// false positive here is the most damaging kind of regression because
// it silently defeats auto-refresh on real data reads.
func TestShouldSkipAutoRefresh_DataCommands(t *testing.T) {
	data := []string{
		"meetings", "panel", "attendee", "folder", "transcript",
		"notes-show", "export", "extract", "memo", "talktime",
		"calendar", "recipes", "chat", "stats", "show",
	}
	for _, name := range data {
		t.Run(name, func(t *testing.T) {
			cmd := &cobra.Command{Use: name}
			if shouldSkipAutoRefresh(cmd) {
				t.Errorf("expected %q to NOT be skipped (it's a data command)", name)
			}
		})
	}
}

// TestAutoRefreshOptedOut_FlagWins covers the highest-precedence opt-out.
// The --no-refresh flag must beat any env state; otherwise users couldn't
// override a CI-wide GRANOLA_NO_AUTO_REFRESH for a single ad-hoc command.
func TestAutoRefreshOptedOut_FlagWins(t *testing.T) {
	t.Setenv("GRANOLA_NO_AUTO_REFRESH", "")
	flags := &rootFlags{noRefresh: true}
	if !autoRefreshOptedOut(flags) {
		t.Fatal("expected opt-out when --no-refresh is true")
	}
}

// TestAutoRefreshOptedOut_EnvVar covers the env-var fallback. When the
// flag is unset (zero value), env decides. Critical for CI usage where
// scripts cannot pass the flag through every nested invocation.
func TestAutoRefreshOptedOut_EnvVar(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"YES", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"banana", false},
	}
	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv("GRANOLA_NO_AUTO_REFRESH", tc.val)
			flags := &rootFlags{}
			if got := autoRefreshOptedOut(flags); got != tc.want {
				t.Errorf("GRANOLA_NO_AUTO_REFRESH=%q: got %v want %v", tc.val, got, tc.want)
			}
		})
	}
}

// TestRefreshPlan_Empty covers the no-auth case. When neither surface
// is configured, run() must return an empty slice so the dispatcher
// emits no provenance line — this is the legitimate "fresh install,
// auth not yet configured" state and should be silent.
func TestRefreshPlan_Empty(t *testing.T) {
	p := refreshPlan{}
	if !p.empty() {
		t.Fatal("empty plan should report empty()")
	}
	flags := &rootFlags{}
	results := p.run(testCtx(t), flags)
	if len(results) != 0 {
		t.Errorf("empty plan should produce no results, got %d", len(results))
	}
}

// TestEmitProvenanceLine_Format covers the exact stderr line format.
// Locked-down so accidental Sprintf reordering doesn't silently change
// what users see.
func TestEmitProvenanceLine_Format(t *testing.T) {
	var buf bytes.Buffer
	results := []refreshResult{
		{surface: refreshSurfaceCache, ok: true, rows: 47, duration: 1234 * time.Millisecond},
		{surface: refreshSurfaceAPI, ok: true, rows: 12, duration: 820 * time.Millisecond},
	}
	emitProvenanceLine(&buf, results)
	got := buf.String()
	want := "auto-refresh: cache=ok (1.2s, 47 rows)  api=ok (820ms, 12 rows)\n"
	if got != want {
		t.Errorf("provenance line:\n got:  %q\n want: %q", got, want)
	}
}

// TestEmitProvenanceLine_CacheOnly covers the single-surface case.
// The line must NOT show "api=skipped" or any other noise — users
// without an API key should see only their cache result.
func TestEmitProvenanceLine_CacheOnly(t *testing.T) {
	var buf bytes.Buffer
	emitProvenanceLine(&buf, []refreshResult{
		{surface: refreshSurfaceCache, ok: true, rows: 5, duration: 50 * time.Millisecond},
	})
	got := buf.String()
	want := "auto-refresh: cache=ok (50ms, 5 rows)\n"
	if got != want {
		t.Errorf("cache-only line:\n got:  %q\n want: %q", got, want)
	}
}

// TestEmitProvenanceLine_Empty covers the no-results path — empty slice
// must produce no output at all, not "auto-refresh:" with nothing after.
func TestEmitProvenanceLine_Empty(t *testing.T) {
	var buf bytes.Buffer
	emitProvenanceLine(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("empty results should produce no output, got %q", buf.String())
	}
}

// TestFormatRefreshFragment_Failure covers the failure rendering. The
// error must be shortened (no newlines, no 200-char stacks) and the
// surface label must remain stable.
func TestFormatRefreshFragment_Failure(t *testing.T) {
	r := refreshResult{
		surface:  refreshSurfaceCache,
		ok:       false,
		duration: 30 * time.Millisecond,
		err:      errors.New("keychain unavailable\n\ndeeper detail follows"),
	}
	got := formatRefreshFragment(r)
	want := "cache=failed: keychain unavailable (30ms)"
	if got != want {
		t.Errorf("failure fragment:\n got:  %q\n want: %q", got, want)
	}
}

// TestFormatRefreshDuration covers each branch of the duration
// formatter. Easy to regress by switching to Duration.String() which
// emits microsecond-precision junk on a refresh line.
func TestFormatRefreshDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{50 * time.Millisecond, "50ms"},
		{999 * time.Millisecond, "999ms"},
		{1 * time.Second, "1.0s"},
		{1234 * time.Millisecond, "1.2s"},
		{30 * time.Second, "30.0s"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := formatRefreshDuration(tc.in); got != tc.want {
				t.Errorf("formatRefreshDuration(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestShortErr covers the truncation and newline-handling paths.
// Without these guards the provenance line would expand to two or three
// lines when an upstream error happens to be multi-line.
func TestShortErr(t *testing.T) {
	if got := shortErr(nil); got != "" {
		t.Errorf("nil err should produce empty string, got %q", got)
	}
	long := strings.Repeat("x", 200)
	got := shortErr(errors.New(long))
	if len(got) > 80 {
		t.Errorf("long err should be truncated to <=80 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated err should end in '...', got %q", got)
	}
	got = shortErr(errors.New("first line\nsecond line"))
	if got != "first line" {
		t.Errorf("multi-line err should keep first line only, got %q", got)
	}
}

// TestShouldEmitProvenance covers each suppression input. Critical for
// the agent-mode contract — JSON consumers must NOT see the line.
func TestShouldEmitProvenance(t *testing.T) {
	// Pin TTY check so suppression-by-mode is the only signal under test.
	prev := stderrIsTerminal
	stderrIsTerminal = func() bool { return true }
	t.Cleanup(func() { stderrIsTerminal = prev })

	cases := []struct {
		name  string
		flags *rootFlags
		want  bool
	}{
		{"default", &rootFlags{}, true},
		{"json", &rootFlags{asJSON: true}, false},
		{"compact", &rootFlags{compact: true}, false},
		{"quiet", &rootFlags{quiet: true}, false},
		{"agent", &rootFlags{agent: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldEmitProvenance(tc.flags, nil); got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

// TestShouldEmitProvenance_NonTTY covers the non-terminal suppression.
// Pipes and CI logs must NOT see the line even in default mode.
func TestShouldEmitProvenance_NonTTY(t *testing.T) {
	prev := stderrIsTerminal
	stderrIsTerminal = func() bool { return false }
	t.Cleanup(func() { stderrIsTerminal = prev })

	if shouldEmitProvenance(&rootFlags{}, nil) {
		t.Fatal("expected suppression when stderr is not a TTY")
	}
}

// testCtx returns a context bounded to the test duration. Tiny helper
// to keep refresh-plan tests from leaking goroutines if a future code
// change starts spawning them.
func testCtx(t *testing.T) (ctx interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}) {
	t.Helper()
	c, cancel := contextWithTimeout(t, 5*time.Second)
	t.Cleanup(cancel)
	return c
}

// contextWithTimeout factored out so testCtx stays a one-liner.
func contextWithTimeout(t *testing.T, d time.Duration) (interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}, func()) {
	t.Helper()
	return autoRefreshContext(nil, d)
}
