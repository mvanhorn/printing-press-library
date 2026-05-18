// Copyright 2026 dstevens. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

// hookSpy stubs the runAutoRefresh dispatcher with a counter so wiring
// tests can assert that PersistentPreRunE actually reaches the
// auto-refresh entry point. Restored via t.Cleanup.
type hookSpy struct {
	count   int64
	lastCmd string
}

func (s *hookSpy) install(t *testing.T) {
	t.Helper()
	prev := runAutoRefresh
	runAutoRefresh = func(cmd *cobra.Command, flags *rootFlags) {
		atomic.AddInt64(&s.count, 1)
		s.lastCmd = cmd.Name()
	}
	t.Cleanup(func() { runAutoRefresh = prev })
}

func (s *hookSpy) calls() int64 { return atomic.LoadInt64(&s.count) }

// TestHook_WiredIntoPersistentPreRun proves the PersistentPreRunE
// callback in newRootCmd actually invokes runAutoRefresh. Without this
// test, a refactor that removes the call site would compile cleanly
// and silently disable auto-refresh in production while every unit
// test continues to pass.
//
// We dispatch a real command that has minimal side effects (`which`
// with a query) and assert the spy fires. The spy is unconditional at
// the dispatcher level; the skip-list / opt-out gating lives inside
// runAutoRefreshImpl and is covered by unit tests in autorefresh_test.go.
func TestHook_WiredIntoPersistentPreRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GRANOLA_NO_AUTO_REFRESH", "")
	spy := &hookSpy{}
	spy.install(t)

	rc := RootCmd()
	rc.SetOut(&bytes.Buffer{})
	rc.SetErr(&bytes.Buffer{})
	rc.SetArgs([]string{"which", "snooze a thread"})
	_ = rc.Execute()

	if spy.calls() == 0 {
		t.Fatal("expected runAutoRefresh to be invoked via PersistentPreRunE")
	}
}

// TestHook_NoRefreshFlagShortCircuits exercises the real dispatcher
// (not the spy) with --no-refresh and asserts no provenance line lands.
// Covers the "user passes --no-refresh" leg of the precedence rules.
func TestHook_NoRefreshFlagShortCircuits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GRANOLA_NO_AUTO_REFRESH", "")
	prev := stderrIsTerminal
	stderrIsTerminal = func() bool { return true }
	t.Cleanup(func() { stderrIsTerminal = prev })

	flags := &rootFlags{noRefresh: true}
	var stderr bytes.Buffer
	cmd := &cobra.Command{Use: "meetings"}
	cmd.SetErr(&stderr)
	runAutoRefreshImpl(cmd, flags)
	if stderr.Len() != 0 {
		t.Errorf("--no-refresh should produce no stderr output, got %q", stderr.String())
	}
}

// TestHook_EnvOptOutShortCircuits covers the env-var precedence path.
// Important for CI usage where the flag cannot be threaded through
// every nested invocation.
func TestHook_EnvOptOutShortCircuits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GRANOLA_NO_AUTO_REFRESH", "1")
	prev := stderrIsTerminal
	stderrIsTerminal = func() bool { return true }
	t.Cleanup(func() { stderrIsTerminal = prev })

	flags := &rootFlags{}
	var stderr bytes.Buffer
	cmd := &cobra.Command{Use: "meetings"}
	cmd.SetErr(&stderr)
	runAutoRefreshImpl(cmd, flags)
	if stderr.Len() != 0 {
		t.Errorf("env opt-out should produce no stderr output, got %q", stderr.String())
	}
}

// TestHook_SkipListShortCircuits covers the skip-list inside the real
// impl. Verifies that for a synthesized command named "sync", no
// stderr output happens even with a populated environment.
func TestHook_SkipListShortCircuits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GRANOLA_NO_AUTO_REFRESH", "")
	prev := stderrIsTerminal
	stderrIsTerminal = func() bool { return true }
	t.Cleanup(func() { stderrIsTerminal = prev })

	flags := &rootFlags{}
	for _, name := range []string{"sync", "sync-api", "doctor", "auth", "version"} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			cmd := &cobra.Command{Use: name}
			cmd.SetErr(&stderr)
			runAutoRefreshImpl(cmd, flags)
			if stderr.Len() != 0 {
				t.Errorf("%s should be skipped, but got stderr: %q", name, stderr.String())
			}
		})
	}
}

// TestAgentContext_AutoRefreshExposed proves the agent-context JSON
// includes the auto_refresh contract object so introspecting agents
// can discover the opt-out surface without scraping --help.
func TestAgentContext_AutoRefreshExposed(t *testing.T) {
	ctx := buildAutoRefreshContext()
	if ctx.Default != "on" {
		t.Errorf("Default = %q, want \"on\"", ctx.Default)
	}
	if ctx.Flag != "--no-refresh" {
		t.Errorf("Flag = %q, want \"--no-refresh\"", ctx.Flag)
	}
	if ctx.Env != "GRANOLA_NO_AUTO_REFRESH" {
		t.Errorf("Env = %q, want \"GRANOLA_NO_AUTO_REFRESH\"", ctx.Env)
	}
	want := []string{"cache", "api"}
	if !equalSlice(ctx.Surfaces, want) {
		t.Errorf("Surfaces = %v, want %v", ctx.Surfaces, want)
	}
	// Skip list should be sorted and contain every name in
	// noRefreshCommands. A regression that adds a skip without
	// updating the agent-context disclosure will fail this.
	if len(ctx.SkipList) != len(noRefreshCommands) {
		t.Errorf("SkipList length = %d, want %d", len(ctx.SkipList), len(noRefreshCommands))
	}
	for _, name := range ctx.SkipList {
		if _, ok := noRefreshCommands[name]; !ok {
			t.Errorf("SkipList contains %q which is not in noRefreshCommands", name)
		}
	}
	// Spot-check sorting: cooperative check that we joined into a
	// stable order so the JSON shape stays diff-friendly.
	joined := strings.Join(ctx.SkipList, ",")
	if joined != strings.Join(sortedKeysOf(noRefreshCommands), ",") {
		t.Errorf("SkipList not sorted: %q", joined)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedKeysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Avoid importing sort just for this helper duplicate — use a
	// minimal insertion sort. The map has <20 entries.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
