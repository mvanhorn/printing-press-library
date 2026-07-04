package cli

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestParseBatchArgRejectsZero(t *testing.T) {
	for _, input := range []string{"0", "release 0", "release_0"} {
		if got, err := parseBatchArg(input); err == nil {
			t.Fatalf("parseBatchArg(%q) = %d, nil; want error", input, got)
		}
	}
}

func TestReleasesCheckDryRunShortCircuitsBeforeOpeningStore(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newReleasesCheckCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	dbPath := filepath.Join(t.TempDir(), "missing", "data.db")
	cmd.SetArgs([]string{"--db", dbPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("dry-run output = %q, want empty", out.String())
	}
}
