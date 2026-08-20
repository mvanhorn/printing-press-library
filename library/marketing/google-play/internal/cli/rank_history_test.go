package cli

import (
	"bytes"
	"testing"
)

func TestNewRankHistoryCmd(t *testing.T) {
	cmd := newNovelRankHistoryCmd(&rootFlags{})
	if cmd.Use != "rank-history <appId>" {
		t.Errorf("Use = %q", cmd.Use)
	}
	for _, f := range []string{"collection", "category", "db"} {
		if cmd.Flag(f) == nil {
			t.Errorf("rank-history missing --%s", f)
		}
	}
}

func TestRankHistoryDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelRankHistoryCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"com.example.app", "--dry-run"})
	// dryRunOK reads flags.dryRun; set via the persistent flag path is not wired
	// in this isolated command, so call RunE directly with a fixture arg.
	if err := cmd.RunE(cmd, []string{"com.example.app"}); err != nil {
		t.Errorf("dry-run RunE returned error: %v", err)
	}
}

func TestRankHistoryRejectsBadCollection(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelRankHistoryCmd(flags)
	_ = cmd.Flags().Set("collection", "NONSENSE")
	err := cmd.RunE(cmd, []string{"com.example.app"})
	if err == nil {
		t.Error("expected an error for an invalid --collection")
	}
}
