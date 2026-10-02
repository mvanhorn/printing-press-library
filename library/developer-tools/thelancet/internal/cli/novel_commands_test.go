// Hand-authored command-contract coverage for Lancet analytics commands.

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNovelCommandContracts(t *testing.T) {
	flags := &rootFlags{}
	tests := []struct {
		name          string
		cmd           *cobra.Command
		requiredFlags []string
	}{
		{"affiliation-growth", newNovelAffiliationGrowthCmd(flags), []string{"years", "threshold", "limit", "db"}},
		{"curate", newNovelCurateCmd(flags), []string{"topic", "sort", "output", "limit"}},
		{"drift", newNovelDriftCmd(flags), []string{"window1", "window2", "top-n", "db"}},
		{"mesh", newNovelMeshCmd(flags), []string{"org", "limit", "db"}},
		{"rank-authors", newNovelRankAuthorsCmd(flags), []string{"institution", "journal", "limit", "db"}},
		{"visibility-gap", newNovelVisibilityGapCmd(flags), []string{"institution", "min-works", "limit", "db"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cmd.Use != tc.name {
				t.Fatalf("Use = %q, want %q", tc.cmd.Use, tc.name)
			}
			if tc.cmd.RunE == nil {
				t.Fatal("RunE must be configured")
			}
			if tc.cmd.Annotations["mcp:read-only"] != "true" {
				t.Fatal("command must remain read-only for MCP")
			}
			for _, name := range tc.requiredFlags {
				if tc.cmd.Flags().Lookup(name) == nil {
					t.Errorf("missing --%s flag", name)
				}
			}
		})
	}
}
