// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

// pp:data-source auto
// Supported strategies: auto, local, live, or computed.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelHistoryCmd(flags *rootFlags) *cobra.Command {
	var target string
	var limit int
	cmd := &cobra.Command{
		Use: "history", Short: "Keep a local, timestamped ledger of readiness reports and their provenance.",
		Example:     "  is-agentic-pp-cli history --target https://is-agentic.com --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "read local report history")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, path, err := openAgenticStore(ctx, flags)
			if err != nil {
				return err
			}
			if s == nil {
				return missingStore(cmd, flags, path)
			}
			defer s.Close()
			items, err := s.ListAgenticSnapshots(ctx, target, limit)
			if err != nil {
				return fmt.Errorf("reading local history: %w", err)
			}
			return emitSnapshots(cmd, flags, items)
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "filter history to one public URL")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum snapshots to return")
	return cmd
}
