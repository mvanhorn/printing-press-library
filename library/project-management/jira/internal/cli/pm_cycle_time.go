// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/jira/internal/store"
	"github.com/spf13/cobra"
)

func newCycleTimeCmd(flags *rootFlags) *cobra.Command {
	var project string
	var issueType string
	var lastStr string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "cycle-time",
		Short: "Show p50/p75/p90 resolution time for resolved issues",
		Long: `Compute cycle time (creation to resolution) for resolved issues in the local store.
Outputs p50/p75/p90 percentiles and a histogram bucket distribution.

Data must be synced first: run 'sync --project KEY'.`,
		Example: `  # Cycle time for all resolved issues in last 90 days
  jira-pp-cli cycle-time --project MYPROJ

  # Bug-specific cycle time
  jira-pp-cli cycle-time --project MYPROJ --type Bug --last 30

  # JSON output for agents
  jira-pp-cli cycle-time --project MYPROJ --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("jira-pp-cli")
			}

			lastDays, err := parseDurationDays(lastStr)
			if err != nil {
				return fmt.Errorf("invalid --last value %q: use a number (days) or duration like 90d, 6m, 1y", lastStr)
			}

			db, err2 := store.OpenWithContext(cmd.Context(), dbPath)
			if err2 != nil {
				return fmt.Errorf("opening local database: %w\nRun 'jira-pp-cli sync' first.", err2)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -lastDays).Format("2006-01-02")

			query := `
SELECT
  json_extract(data, '$.key') as issue_key,
  json_extract(data, '$.fields.summary') as summary,
  json_extract(data, '$.fields.issuetype.name') as itype,
  json_extract(data, '$.fields.created') as created,
  json_extract(data, '$.fields.resolutiondate') as resolved
FROM issue
WHERE json_extract(data, '$.fields.resolutiondate') IS NOT NULL
  AND json_extract(data, '$.fields.created') IS NOT NULL
  AND substr(json_extract(data, '$.fields.resolutiondate'), 1, 10) >= ?`

			qargs := []any{cutoff}

			if project != "" {
				query += ` AND json_extract(data, '$.fields.project.key') = ?`
				qargs = append(qargs, project)
			}
			if issueType != "" {
				query += ` AND json_extract(data, '$.fields.issuetype.name') = ?`
				qargs = append(qargs, issueType)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying cycle times: %w", err)
			}
			defer rows.Close()

			type issueTime struct {
				Key      string  `json:"key"`
				Summary  string  `json:"summary"`
				Type     string  `json:"type"`
				Created  string  `json:"created"`
				Resolved string  `json:"resolved"`
				Days     float64 `json:"days"`
			}

			var issuesList []issueTime
			var cycleDays []float64

			for rows.Next() {
				var key, summary, itype, created, resolved string
				if err := rows.Scan(&key, &summary, &itype, &created, &resolved); err != nil {
					continue
				}
				c, err1 := parseJiraTime(created)
				r, err2 := parseJiraTime(resolved)
				if err1 != nil || err2 != nil {
					continue
				}
				days := r.Sub(c).Hours() / 24
				if days < 0 {
					continue
				}
				cycleDays = append(cycleDays, days)
				issuesList = append(issuesList, issueTime{
					Key:      key,
					Summary:  summary,
					Type:     itype,
					Created:  created,
					Resolved: resolved,
					Days:     math.Round(days*10) / 10,
				})
			}

			if len(cycleDays) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No resolved issues found. Run 'sync --project KEY' first.")
				return nil
			}

			sort.Float64s(cycleDays)
			p50 := cyclePercentile(cycleDays, 50)
			p75 := cyclePercentile(cycleDays, 75)
			p90 := cyclePercentile(cycleDays, 90)
			avg := cycleAverage(cycleDays)

			buckets := cycleHistogram(cycleDays)

			if flags.asJSON {
				result := map[string]any{
					"count":   len(cycleDays),
					"p50":     math.Round(p50*10) / 10,
					"p75":     math.Round(p75*10) / 10,
					"p90":     math.Round(p90*10) / 10,
					"avg":     math.Round(avg*10) / 10,
					"buckets": buckets,
					"issues":  issuesList,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Cycle Time — %d resolved issues", len(cycleDays))
			if project != "" {
				fmt.Fprintf(out, " in %s", project)
			}
			if issueType != "" {
				fmt.Fprintf(out, " (type: %s)", issueType)
			}
			fmt.Fprintf(out, " (last %d days)\n\n", lastDays)
			fmt.Fprintf(out, "  p50: %5.1f days\n", p50)
			fmt.Fprintf(out, "  p75: %5.1f days\n", p75)
			fmt.Fprintf(out, "  p90: %5.1f days\n", p90)
			fmt.Fprintf(out, "  avg: %5.1f days\n\n", avg)

			fmt.Fprintln(out, "Distribution:")
			maxCount := 0
			for _, b := range buckets {
				if b.Count > maxCount {
					maxCount = b.Count
				}
			}
			for _, b := range buckets {
				barLen := 0
				if maxCount > 0 {
					barLen = b.Count * 30 / maxCount
				}
				bar := ""
				for i := 0; i < barLen; i++ {
					bar += "█"
				}
				fmt.Fprintf(out, "  %-12s │ %s %d\n", b.Label, bar, b.Count)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project key (e.g. MYPROJ)")
	cmd.Flags().StringVar(&issueType, "type", "", "Issue type filter (e.g. Bug, Story, Task)")
	cmd.Flags().StringVar(&lastStr, "last", "90", "Issues resolved in the last N days or duration (e.g. 90, 180d, 6m, 1y)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

type cycleHistogramBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

func cycleHistogram(days []float64) []cycleHistogramBucket {
	labels := []string{"< 1 day", "1-3 days", "3-7 days", "1-2 weeks", "2-4 weeks", "1-2 months", "> 2 months"}
	limits := []float64{1, 3, 7, 14, 30, 60, math.MaxFloat64}
	buckets := make([]cycleHistogramBucket, len(labels))
	for i, l := range labels {
		buckets[i].Label = l
	}
	for _, d := range days {
		prev := 0.0
		for i, lim := range limits {
			if d > prev && d <= lim {
				buckets[i].Count++
				break
			}
			prev = lim
		}
	}
	return buckets
}

func cyclePercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (idx-float64(lo))*(sorted[hi]-sorted[lo])
}

func cycleAverage(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func parseJiraTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.999-0700",
		"2006-01-02T15:04:05.000+0000",
		"2006-01-02T15:04:05.999Z",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}

// parseDurationDays converts a plain integer or duration string to a number of days.
// Accepts: "90" (days), "90d", "12w", "6m" (months), "1y".
// PATCH: novel-features
func parseDurationDays(s string) (int, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	if len(s) < 2 {
		return 0, fmt.Errorf("unrecognised duration %q", s)
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return 0, fmt.Errorf("unrecognised duration %q", s)
	}
	switch s[len(s)-1] {
	case 'd':
		return n, nil
	case 'w':
		return n * 7, nil
	case 'm':
		return n * 30, nil
	case 'y':
		return n * 365, nil
	default:
		return 0, fmt.Errorf("unknown unit %q in %q; use d, w, m, or y", string(s[len(s)-1]), s)
	}
}
