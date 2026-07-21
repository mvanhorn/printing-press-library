package cli

import (
	"bytes"
	"testing"
)

func TestPersonalCommandDryRunContracts(t *testing.T) {
	for _, args := range [][]string{
		{"schedule", "tonight", "--dry-run", "--timezone", "America/New_York", "--json"},
		{"backlog", "pick", "--dry-run", "--max-episodes", "13", "--max-runtime-minutes", "30", "--json"},
		{"progress", "check-in", "1", "--episode", "1", "--dry-run", "--json"},
		{"progress", "catch-up", "--dry-run", "--as-of", "2026-07-21T20:00:00Z", "--json"},
	} {
		cmd := RootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestPersonalCommandsRejectInvalidBounds(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"backlog", "pick", "--max-episodes", "0", "--max-runtime-minutes", "30"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("backlog pick accepted zero episode bound")
	}
	cmd = RootCmd()
	cmd.SetArgs([]string{"schedule", "tonight", "--dry-run", "--timezone", "not/a/zone"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("schedule tonight accepted invalid IANA zone")
	}
}
