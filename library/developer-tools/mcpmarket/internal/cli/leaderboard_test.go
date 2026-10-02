// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/store"
)

// TestNovelLeaderboardHelpWires smoke-tests that the leaderboard command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelLeaderboardHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"leaderboard", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("leaderboard --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "leaderboard"} {
		if !strings.Contains(help, want) {
			t.Fatalf("leaderboard --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestLeaderboardCurrentEmptyCatalogDoesNotShowOldEntries(t *testing.T) {
	withTempLearnHome(t)
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, defaultDBPath("mcpmarket-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if _, err := db.DB().ExecContext(ctx, `INSERT INTO resource_snapshots (resource_type, resource_id, data, snapshot_date, captured_at) VALUES ('server', 'removed', '{"name":"removed"}', ?, ?)`, yesterday, yesterday+"T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runRootArgs(t, "leaderboard", "--agent")
	if err != nil {
		t.Fatalf("current leaderboard failed: %v (stderr=%q)", err, stderr)
	}
	var result struct {
		AsOf    string             `json:"as_of"`
		Entries []leaderboardEntry `json:"entries"`
	}
	unmarshalAgentResults(t, stdout, &result)
	if result.AsOf != time.Now().UTC().Format("2006-01-02") || len(result.Entries) != 0 {
		t.Fatalf("current empty catalog fell back to old entries: %#v", result)
	}
}
