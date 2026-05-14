// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/store"
)

type rateCheckEntry struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowName   string  `json:"workflow_name,omitempty"`
	ExecutionCount int     `json:"execution_count"`
	WindowMinutes  int     `json:"window_minutes"`
	RatePerMinute  float64 `json:"rate_per_minute"`
	Threshold      float64 `json:"threshold_per_minute"`
	IsRunaway      bool    `json:"is_runaway"`
}

func newExecutionsRateCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var windowMins int
	var threshold float64
	var onlyRunaway bool

	cmd := &cobra.Command{
		Use:   "rate-check",
		Short: "Detect runaway workflow triggers exceeding an execution rate threshold",
		Long: `Count executions per workflow in a time window and flag any that exceed a
per-minute execution rate. Useful for catching infinite loops, webhook storms,
or misconfigured triggers before they impact n8n instance health.
Requires a local sync (n8n-pp-cli sync) first.`,
		Example: strings.Trim(`
  # Check last 30 minutes, flag anything above 10/min
  n8n-pp-cli executions rate-check

  # Tighter window and lower threshold
  n8n-pp-cli executions rate-check --window 5 --threshold 2.0

  # Only show runaway workflows
  n8n-pp-cli executions rate-check --runaway --json --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("n8n-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'n8n-pp-cli sync' first.", err)
			}
			defer db.Close()

			since := time.Now().UTC().Add(-time.Duration(windowMins) * time.Minute)
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT e.workflow_id, w.name, COUNT(*) as cnt
				   FROM executions e
				   LEFT JOIN workflows w ON w.id = e.workflow_id
				  WHERE e.started_at >= ?
				  GROUP BY e.workflow_id
				  ORDER BY cnt DESC`,
				since.Format(time.RFC3339),
			)
			if err != nil {
				return fmt.Errorf("querying executions: %w\nRun 'n8n-pp-cli sync' first.", err)
			}
			defer rows.Close()

			var results []rateCheckEntry
			for rows.Next() {
				var e rateCheckEntry
				var name *string
				if err := rows.Scan(&e.WorkflowID, &name, &e.ExecutionCount); err != nil {
					continue
				}
				if name != nil {
					e.WorkflowName = *name
				}
				e.WindowMinutes = windowMins
				e.RatePerMinute = float64(e.ExecutionCount) / float64(windowMins)
				e.Threshold = threshold
				e.IsRunaway = e.RatePerMinute > threshold
				if onlyRunaway && !e.IsRunaway {
					continue
				}
				results = append(results, e)
			}
			if rows.Err() != nil {
				return fmt.Errorf("scanning rows: %w", rows.Err())
			}

			if len(results) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				if onlyRunaway {
					fmt.Fprintf(cmd.OutOrStdout(), "No runaway workflows detected (threshold: %.1f/min over %d min)\n",
						threshold, windowMins)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No executions found in window. Run 'n8n-pp-cli sync' first.")
				}
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().IntVar(&windowMins, "window", 30, "Time window in minutes to check")
	cmd.Flags().Float64Var(&threshold, "threshold", 10.0, "Executions per minute above which a workflow is flagged as runaway")
	cmd.Flags().BoolVar(&onlyRunaway, "runaway", false, "Only show workflows exceeding the threshold")
	return cmd
}
