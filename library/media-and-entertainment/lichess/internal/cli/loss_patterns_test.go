// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type fakeGamesClient struct {
	data json.RawMessage
	path string
}

func (f *fakeGamesClient) GetWithHeaders(_ context.Context, path string, _ map[string]string, _ map[string]string) (json.RawMessage, error) {
	f.path = path
	return f.data, nil
}

// TestNovelLossPatternsHelpWires smoke-tests that the loss-patterns command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelLossPatternsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"loss-patterns", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("loss-patterns --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "loss-patterns"} {
		if !strings.Contains(help, want) {
			t.Fatalf("loss-patterns --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelLossPatternsWithoutUsernameUsesOptionalArgumentPath(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"loss-patterns", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("loss-patterns without username dry-run error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("loss-patterns without username output = %q: %v", out.String(), err)
	}
	if got["dry_run"] != true || got["username"] != "" {
		t.Fatalf("loss-patterns without username dry-run = %#v", got)
	}
}

func TestCollectLossPatternsCountsOnlyTheRequestedPlayersJudgments(t *testing.T) {
	data := json.RawMessage(`{"winner":"black","perf":"rapid","opening":{"name":"` + strings.Repeat("x", 70*1024) + `"},"players":{"white":{"user":{"id":"alice"}},"black":{"user":{"id":"bob"}}},"division":{"middle":2,"end":4},"analysis":[{"judgment":{"name":"Inaccuracy"}},{"judgment":{"name":"Good move"}},{"judgment":{"name":"Mistake"}}]}`)
	client := &fakeGamesClient{data: data}
	report, err := collectLossPatterns(&cobra.Command{}, client, "alice", 10)
	if err != nil {
		t.Fatalf("collectLossPatterns() error = %v", err)
	}
	if report.GamesScanned != 1 || report.Losses != 1 {
		t.Fatalf("games/losses = %d/%d, want 1/1", report.GamesScanned, report.Losses)
	}
	if len(report.Patterns) != 2 {
		t.Fatalf("patterns = %#v, want two white-player judgments", report.Patterns)
	}
	if report.Patterns[0].Judgment != "Inaccuracy" || report.Patterns[0].Phase != "opening" {
		t.Fatalf("first pattern = %#v, want opening inaccuracy", report.Patterns[0])
	}
	if report.Patterns[1].Judgment != "Mistake" || report.Patterns[1].Phase != "middlegame" {
		t.Fatalf("second pattern = %#v, want middlegame mistake", report.Patterns[1])
	}
}

func TestCollectLossPatternsEscapesUsernamePathSegment(t *testing.T) {
	client := &fakeGamesClient{data: json.RawMessage("")}
	_, _ = collectLossPatterns(&cobra.Command{}, client, "alice?max=500", 10)
	if client.path != "/api/games/user/alice%3Fmax=500" {
		t.Fatalf("path = %q", client.path)
	}
}
