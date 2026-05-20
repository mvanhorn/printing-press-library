// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/internal/store"
	"github.com/spf13/cobra"
)

type salaryStats struct {
	Count        int   `json:"count"`
	WithSalary   int   `json:"with_salary"`
	NullSalary   int   `json:"null_salary"`
	P25Midpoint  int64 `json:"p25_midpoint"`
	P50Midpoint  int64 `json:"p50_midpoint"`
	P75Midpoint  int64 `json:"p75_midpoint"`
	MinMidpoint  int64 `json:"min_midpoint"`
	MaxMidpoint  int64 `json:"max_midpoint"`
	MeanMidpoint int64 `json:"mean_midpoint"`
}

func newOpeningsSalaryStatsCmd(flags *rootFlags) *cobra.Command {
	var sector, experience, state, dbPath string

	cmd := &cobra.Command{
		Use:   "salary-stats",
		Short: "p25 / p50 / p75 of midpoint salary across filtered openings",
		Long: "Compute salary band quartiles of (salary_min+salary_max)/2 over locally synced " +
			"openings (joined with companies for sector/state filters). Null-salary count is " +
			"reported separately so missing data is honest.",
		Example:     "  american-reindustrialization-pp-cli openings salary-stats --sector robotics --experience senior --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("american-reindustrialization-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'american-reindustrialization-pp-cli sync' first.", err)
			}
			defer db.Close()

			q := `SELECT COALESCE(o.salary_min,0), COALESCE(o.salary_max,0)
			      FROM openings o
			      LEFT JOIN companies c ON c.id = o.company_id
			      WHERE 1=1`
			args2 := []any{}
			if sector != "" {
				q += " AND lower(c.primary_sector) = lower(?)"
				args2 = append(args2, strings.TrimSpace(sector))
			}
			if experience != "" {
				q += " AND lower(o.experience_level) = lower(?)"
				args2 = append(args2, strings.TrimSpace(experience))
			}
			if state != "" {
				q += " AND (upper(o.location_state) = upper(?) OR upper(c.hq_state) = upper(?))"
				args2 = append(args2, strings.TrimSpace(state), strings.TrimSpace(state))
			}

			rows, err := db.DB().QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			stats := salaryStats{}
			midpoints := make([]int64, 0)
			for rows.Next() {
				var smin, smax sql.NullInt64
				if err := rows.Scan(&smin, &smax); err != nil {
					continue
				}
				stats.Count++
				if smin.Int64 <= 0 && smax.Int64 <= 0 {
					stats.NullSalary++
					continue
				}
				stats.WithSalary++
				var mid int64
				switch {
				case smin.Int64 > 0 && smax.Int64 > 0:
					mid = (smin.Int64 + smax.Int64) / 2
				case smin.Int64 > 0:
					mid = smin.Int64
				default:
					mid = smax.Int64
				}
				midpoints = append(midpoints, mid)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating salary-stats rows: %w", err)
			}

			if len(midpoints) > 0 {
				sort.Slice(midpoints, func(i, j int) bool { return midpoints[i] < midpoints[j] })
				stats.MinMidpoint = midpoints[0]
				stats.MaxMidpoint = midpoints[len(midpoints)-1]
				stats.P25Midpoint = quantile(midpoints, 0.25)
				stats.P50Midpoint = quantile(midpoints, 0.50)
				stats.P75Midpoint = quantile(midpoints, 0.75)
				var sum int64
				for _, m := range midpoints {
					sum += m
				}
				stats.MeanMidpoint = sum / int64(len(midpoints))
			}

			return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
		},
	}

	cmd.Flags().StringVar(&sector, "sector", "", "Filter by company's primary_sector")
	cmd.Flags().StringVar(&experience, "experience", "", "Filter by experience_level (entry/mid/senior/lead)")
	cmd.Flags().StringVar(&state, "state", "", "Filter by location state or company HQ state (2-letter code)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}

// quantile returns the q-th quantile of a SORTED slice of int64 (q in [0,1])
// using linear interpolation between the two nearest ranks.
func quantile(sorted []int64, q float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	w := pos - float64(lo)
	return int64(float64(sorted[lo])*(1-w) + float64(sorted[hi])*w)
}
