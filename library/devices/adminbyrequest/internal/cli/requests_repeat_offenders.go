// Copyright 2026 joltsconsulting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/adminbyrequest/internal/store"
	"github.com/spf13/cobra"
)

type repeatOffender struct {
	User     string `json:"user"`
	Requests int    `json:"requests"`
	Approved int    `json:"approved"`
	Denied   int    `json:"denied"`
}

func newRequestsRepeatOffendersCmd(flags *rootFlags) *cobra.Command {
	var windowStr string
	var topN int
	var dbPath string

	cmd := &cobra.Command{
		Use:     "repeat-offenders",
		Short:   "Top users by elevation-request count over a window (offline)",
		Long:    "Aggregate the locally-synced requests table by user account, return the highest-count requestors over the given window.",
		Example: "  adminbyrequest-pp-cli requests repeat-offenders --window 30d --top 10 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window, err := parseDurationExtras(windowStr)
			if err != nil {
				return fmt.Errorf("invalid --window %q (try 30d, 7d, 24h): %w", windowStr, err)
			}
			cutoff := time.Now().Add(-window).Format("2006-01-02T15:04:05")

			if dbPath == "" {
				dbPath = defaultDBPath("adminbyrequest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store at %s: %w (run sync first)", dbPath, err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT
				  COALESCE(json_extract(data, '$.user.account'), 'unknown') AS user,
				  COUNT(*) AS total,
				  SUM(CASE WHEN LOWER(status) = 'approved' OR LOWER(status) = 'finished' THEN 1 ELSE 0 END) AS approved,
				  SUM(CASE WHEN LOWER(status) = 'denied' THEN 1 ELSE 0 END) AS denied
				 FROM requests
				 WHERE COALESCE(request_time, '') >= ?
				 GROUP BY user
				 ORDER BY total DESC
				 LIMIT ?`, cutoff, topN)
			if err != nil {
				return fmt.Errorf("querying requests: %w", err)
			}
			defer rows.Close()

			var out []repeatOffender
			for rows.Next() {
				var r repeatOffender
				if err := rows.Scan(&r.User, &r.Requests, &r.Approved, &r.Denied); err != nil {
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating requests: %w", err)
			}
			if out == nil {
				out = []repeatOffender{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Top %d requestors since %s\n", len(out), cutoff)
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-40s %d total (%d approved, %d denied)\n", r.User, r.Requests, r.Approved, r.Denied)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&windowStr, "window", "30d", "Look-back window (e.g. 7d, 24h, 30d)")
	cmd.Flags().IntVar(&topN, "top", 10, "Return this many top users")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard CLI location)")
	return cmd
}
