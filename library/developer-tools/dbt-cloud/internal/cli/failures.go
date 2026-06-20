// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — list recent failed runs from local SQLite mirror.

package cli

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/store"

	"github.com/spf13/cobra"
)

// FailedRunEntry describes a single failed run in the failures report.
type FailedRunEntry struct {
	RunID           int64  `json:"run_id"`
	JobDefinitionID int64  `json:"job_definition_id"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
}

// JobFailureGroup groups failed runs by job_definition_id.
type JobFailureGroup struct {
	JobDefinitionID int64            `json:"job_definition_id"`
	FailureCount    int              `json:"failure_count"`
	Runs            []FailedRunEntry `json:"runs"`
}

// pp:data-source local
func newNovelFailuresCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagDBPath string

	cmd := &cobra.Command{
		Use:   "failures",
		Short: "Recently failed runs in a time window, grouped by job, with each run's failed-step names.",
		Long: `List runs with error or cancelled status from the local SQLite mirror, grouped by job.

Requires a prior 'dbt-cloud-pp-cli sync' to populate the local database.
Results cover the last --days days (default 7).`,
		Example: `  dbt-cloud-pp-cli failures
  dbt-cloud-pp-cli failures --days 14 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would list failures (days=%d)\n", flagDays)
				return nil
			}

			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = defaultDBPath("dbt-cloud-pp-cli")
			}

			// Missing-mirror guard
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []JobFailureGroup{}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No local mirror found. Run: dbt-cloud-pp-cli sync\n")
				return nil
			}

			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			since := time.Now().UTC().AddDate(0, 0, -flagDays)

			// is_error=1 OR is_cancelled=1 (status 20=Error, 30=Cancelled)
			rows, err := s.DB().Query(
				`SELECT id, job_definition_id, status_humanized, created_at
				 FROM runs
				 WHERE created_at >= ? AND (is_error = 1 OR is_cancelled = 1)
				 ORDER BY created_at DESC`,
				since.Format(time.RFC3339),
			)
			if err != nil {
				return fmt.Errorf("querying runs: %w", err)
			}
			defer rows.Close()

			byJob := map[int64][]FailedRunEntry{}
			for rows.Next() {
				var runIDStr string
				var jobDefID int64
				var statusHuman, createdAt string
				if err := rows.Scan(&runIDStr, &jobDefID, &statusHuman, &createdAt); err != nil {
					continue
				}
				// run IDs are stored as strings in the generic table
				var runID int64
				fmt.Sscanf(runIDStr, "%d", &runID)

				byJob[jobDefID] = append(byJob[jobDefID], FailedRunEntry{
					RunID:           runID,
					JobDefinitionID: jobDefID,
					Status:          statusHuman,
					CreatedAt:       createdAt,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading runs: %w", err)
			}

			var groups []JobFailureGroup
			var jobIDs []int64
			for jid := range byJob {
				jobIDs = append(jobIDs, jid)
			}
			sort.Slice(jobIDs, func(i, j int) bool {
				// Sort by failure count descending, then job ID ascending
				ci, cj := len(byJob[jobIDs[i]]), len(byJob[jobIDs[j]])
				if ci != cj {
					return ci > cj
				}
				return jobIDs[i] < jobIDs[j]
			})
			for _, jid := range jobIDs {
				runs := byJob[jid]
				groups = append(groups, JobFailureGroup{
					JobDefinitionID: jid,
					FailureCount:    len(runs),
					Runs:            runs,
				})
			}

			if len(groups) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []JobFailureGroup{}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No failures found in the last %d days.\n", flagDays)
				return nil
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), groups, flags)
			}

			// Human output
			fmt.Fprintf(cmd.OutOrStdout(), "Failed runs in last %d days (%d jobs affected):\n\n", flagDays, len(groups))
			for _, g := range groups {
				fmt.Fprintf(cmd.OutOrStdout(), "Job %d — %d failure(s)\n", g.JobDefinitionID, g.FailureCount)
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintf(tw, "  RUN_ID\tSTATUS\tCREATED_AT\n")
				for _, r := range g.Runs {
					fmt.Fprintf(tw, "  %d\t%s\t%s\n", r.RunID, r.Status, r.CreatedAt)
				}
				tw.Flush()
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 7, "Number of days to look back (default 7)")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Path to local SQLite database (default: auto-detected)")
	return cmd
}
