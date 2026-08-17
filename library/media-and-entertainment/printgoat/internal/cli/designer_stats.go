// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: pure local aggregation over outcomes logged by `log fail`
// (and any future `log success`) for a given designer.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

type failureReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// pp:data-source local
func newNovelDesignerStatsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "stats <name>",
		Short:   "See your own success/failure rate with a specific designer, pooled across every site they publish on.",
		Example: "  printgoat-pp-cli designer stats PrintedSolid --agent",
		// pp:no-error-path-probe: this is a pure local-database aggregation
		// query, not an API lookup — a designer name with zero logged
		// outcomes (including a nonsense/unknown one) is a legitimate empty
		// result (total_logged: 0), not an error condition to detect.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("missing required argument <name>\nUsage: %s <name>", cmd.CommandPath()))
			}
			designer := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			var successCount, failCount int
			if err := s.DB().QueryRowContext(ctx,
				`SELECT COUNT(*) FROM printgoat_print_outcomes WHERE lower(designer) = lower(?) AND outcome = 'success'`,
				designer,
			).Scan(&successCount); err != nil {
				return fmt.Errorf("counting successes: %w", err)
			}
			if err := s.DB().QueryRowContext(ctx,
				`SELECT COUNT(*) FROM printgoat_print_outcomes WHERE lower(designer) = lower(?) AND outcome = 'fail'`,
				designer,
			).Scan(&failCount); err != nil {
				return fmt.Errorf("counting failures: %w", err)
			}

			rows, err := s.DB().QueryContext(ctx,
				`SELECT reason, COUNT(*) AS c FROM printgoat_print_outcomes
				 WHERE lower(designer) = lower(?) AND outcome = 'fail' AND reason IS NOT NULL AND TRIM(reason) != ''
				 GROUP BY reason ORDER BY c DESC, reason ASC LIMIT 5`,
				designer,
			)
			if err != nil {
				return fmt.Errorf("aggregating failure reasons: %w", err)
			}
			var reasons []failureReasonCount
			for rows.Next() {
				var r failureReasonCount
				if serr := rows.Scan(&r.Reason, &r.Count); serr != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning failure reasons: %w", serr)
				}
				reasons = append(reasons, r)
			}
			closeErr := rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("aggregating failure reasons: %w", err)
			}
			if closeErr != nil {
				return fmt.Errorf("aggregating failure reasons: %w", closeErr)
			}

			total := successCount + failCount
			out := map[string]any{
				"designer":            designer,
				"success_count":       successCount,
				"fail_count":          failCount,
				"total_logged":        total,
				"top_failure_reasons": reasons,
			}
			if total == 0 {
				out["message"] = "no logged outcomes for this designer yet; use 'log fail <model-key> --reason ...' to start tracking"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
