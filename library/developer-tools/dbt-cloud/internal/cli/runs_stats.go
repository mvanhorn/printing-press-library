// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — per-job run statistics from local SQLite mirror.

package cli

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/store"

	"github.com/spf13/cobra"
)

// JobStats holds computed statistics for a single job over a time window.
type JobStats struct {
	JobDefinitionID int64   `json:"job_definition_id"`
	TotalRuns       int     `json:"total_runs"`
	SuccessCount    int     `json:"success_count"`
	SuccessRatePct  float64 `json:"success_rate_pct"`
	FailureCount    int     `json:"failure_count"`
	AvgDurationSec  float64 `json:"avg_duration_sec"`
	P95DurationSec  float64 `json:"p95_duration_sec"`
}

// computeJobStats calculates statistics given a slice of (is_success int, duration_seconds float64).
func computeJobStats(jobID int64, rows []runsStatRow) JobStats {
	s := JobStats{JobDefinitionID: jobID, TotalRuns: len(rows)}
	var durations []float64
	for _, r := range rows {
		if r.IsSuccess {
			s.SuccessCount++
		}
		if r.DurationSec > 0 {
			durations = append(durations, r.DurationSec)
		}
	}
	s.FailureCount = s.TotalRuns - s.SuccessCount
	if s.TotalRuns > 0 {
		s.SuccessRatePct = math.Round(float64(s.SuccessCount)/float64(s.TotalRuns)*10000) / 100
	}
	if len(durations) > 0 {
		sum := 0.0
		for _, d := range durations {
			sum += d
		}
		s.AvgDurationSec = math.Round(sum/float64(len(durations))*100) / 100
		sort.Float64s(durations)
		idx := int(math.Ceil(0.95*float64(len(durations)))) - 1
		if idx < 0 {
			idx = 0
		}
		s.P95DurationSec = math.Round(durations[idx]*100) / 100
	}
	return s
}

// runsStatRow is a minimal row from the runs table.
type runsStatRow struct {
	JobDefinitionID int64
	IsSuccess       bool
	DurationSec     float64
	CreatedAt       time.Time
}

// parseDurationToSeconds converts dbt Cloud's "HH:MM:SS" or numeric-seconds duration to float64.
func parseDurationToSeconds(s string) float64 {
	if s == "" {
		return 0
	}
	// Try numeric first
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// Try HH:MM:SS
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		h, _ := strconv.ParseFloat(parts[0], 64)
		m, _ := strconv.ParseFloat(parts[1], 64)
		sec, _ := strconv.ParseFloat(parts[2], 64)
		return h*3600 + m*60 + sec
	}
	return 0
}

// pp:data-source local
func newNovelRunsStatsCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagJobID int64
	var flagDBPath string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Success rate, average and p95 duration, and failure counts per job over a time window from the local run mirror.",
		Long: `Read the local synced run mirror and compute per-job statistics.

Requires a prior 'dbt-cloud-pp-cli sync' to populate the local database.
Statistics are computed over runs in the last --days days (default 30).
Use --job to filter to a single job definition ID.`,
		Example: `  dbt-cloud-pp-cli runs stats
  dbt-cloud-pp-cli runs stats --days 7 --json
  dbt-cloud-pp-cli runs stats --job 12345`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compute run stats (days=%d)\n", flagDays)
				return nil
			}

			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = defaultDBPath("dbt-cloud-pp-cli")
			}

			// Missing-mirror guard
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []JobStats{}, flags)
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

			// Query runs table directly — it has dedicated columns for is_success, duration, created_at, job_definition_id
			query := `SELECT job_definition_id, is_success, duration, created_at FROM runs WHERE created_at >= ? `
			qargs := []any{since.Format(time.RFC3339)}
			if flagJobID > 0 {
				query += `AND job_definition_id = ? `
				qargs = append(qargs, flagJobID)
			}
			query += `ORDER BY created_at DESC`

			rows, err := s.DB().Query(query, qargs...)
			if err != nil {
				return fmt.Errorf("querying runs: %w", err)
			}
			defer rows.Close()

			// Group rows by job_definition_id
			byJob := map[int64][]runsStatRow{}
			for rows.Next() {
				var jobDefID int64
				var isSuccess int
				var duration, createdAt string
				if err := rows.Scan(&jobDefID, &isSuccess, &duration, &createdAt); err != nil {
					continue
				}
				t, _ := time.Parse(time.RFC3339, createdAt)
				byJob[jobDefID] = append(byJob[jobDefID], runsStatRow{
					JobDefinitionID: jobDefID,
					IsSuccess:       isSuccess == 1,
					DurationSec:     parseDurationToSeconds(duration),
					CreatedAt:       t,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading runs: %w", err)
			}

			// Sort job IDs for deterministic output
			var jobIDs []int64
			for jid := range byJob {
				jobIDs = append(jobIDs, jid)
			}
			sort.Slice(jobIDs, func(i, j int) bool { return jobIDs[i] < jobIDs[j] })

			var stats []JobStats
			for _, jid := range jobIDs {
				stats = append(stats, computeJobStats(jid, byJob[jid]))
			}

			if len(stats) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []JobStats{}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No runs found in the last %d days. Run: dbt-cloud-pp-cli sync\n", flagDays)
				return nil
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
			}

			// Human table output
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "JOB_ID\tTOTAL\tSUCCESS\tFAILED\tSUCCESS%%\tAVG_SEC\tP95_SEC\n")
			for _, s := range stats {
				fmt.Fprintf(tw, "%d\t%d\t%d\t%d\t%.1f%%\t%.1f\t%.1f\n",
					s.JobDefinitionID, s.TotalRuns, s.SuccessCount, s.FailureCount,
					s.SuccessRatePct, s.AvgDurationSec, s.P95DurationSec)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of days to look back (default 30)")
	cmd.Flags().Int64Var(&flagJobID, "job", 0, "Filter to a specific job definition ID")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Path to local SQLite database (default: auto-detected)")
	return cmd
}
