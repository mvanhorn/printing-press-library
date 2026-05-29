// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: monthly meetings status report, computed via the ever-had
// query against the local hubspot_property_history table.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

// canonicalMeetingStatuses maps user-friendly status aliases to the canonical
// values HubSpot reports in hs_meeting_outcome history. Stay tolerant: pass
// through anything we don't recognize so users can target custom outcomes.
var canonicalMeetingStatuses = map[string]string{
	"scheduled": "SCHEDULED",
	"no-show":   "NO_SHOW",
	"no_show":   "NO_SHOW",
	"noshow":    "NO_SHOW",
	"completed": "COMPLETED",
	"cancelled": "CANCELED",
	"canceled":  "CANCELED",
}

// meetingStatusBuckets is the breakdown surface in the monthly summary; using a
// stable ordered list (instead of a map) keeps the report shape deterministic.
var meetingStatusBuckets = []string{"SCHEDULED", "NO_SHOW", "COMPLETED", "CANCELED"}

func newNovelMeetingsStatusReportCmd(flags *rootFlags) *cobra.Command {
	var statusArg string
	var monthArg string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "status-report",
		Short:       "Monthly meetings status report: every meeting that touched the given status in the given month",
		Long:        `Composes the meetings ever-had query into the canonical monthly-report shape. Output includes a per-current-outcome breakdown (Scheduled / No Show / Completed / Cancelled) so a single command answers "of the meetings I scheduled in May, how many actually happened?".`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  hubspot-pp-cli meetings status-report --status scheduled --month 2026-05`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			status := canonicalizeStatus(statusArg)
			month := monthArg
			if month == "" {
				month = time.Now().Format("2006-01")
			}
			from, to, err := parseMonth(month)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := queryMeetingsEverHad(cmd, db, "hs_meeting_outcome", status, from, to, cliutil.FilterExpr{})
			if err != nil {
				return err
			}

			// Per-current-outcome breakdown.
			breakdown := map[string]int{}
			for _, b := range meetingStatusBuckets {
				breakdown[b] = 0
			}
			for _, r := range rows {
				cur := r.CurrentOutcome
				if cur == "" {
					cur = "UNKNOWN"
				}
				breakdown[cur]++
			}

			// Stable ordered breakdown for output: known buckets first (in
			// declared order), then any unexpected outcomes alphabetically.
			type bucketRow struct {
				Outcome string `json:"outcome"`
				Count   int    `json:"count"`
			}
			var ordered []bucketRow
			seen := map[string]bool{}
			for _, b := range meetingStatusBuckets {
				ordered = append(ordered, bucketRow{Outcome: b, Count: breakdown[b]})
				seen[b] = true
			}
			var extras []string
			for k := range breakdown {
				if !seen[k] {
					extras = append(extras, k)
				}
			}
			sort.Strings(extras)
			for _, k := range extras {
				ordered = append(ordered, bucketRow{Outcome: k, Count: breakdown[k]})
			}

			summary := map[string]any{
				"status":             status,
				"month":              month,
				"window_from":        from.Format(time.RFC3339),
				"window_to":          to.Format(time.RFC3339),
				"total":              len(rows),
				"by_current_outcome": ordered,
				"results":            rows,
			}
			if flags.asJSON {
				return flags.printJSON(cmd, summary)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Meetings whose %s was ever %q in %s: %d\n", "hs_meeting_outcome", status, month, len(rows))
			fmt.Fprintln(cmd.OutOrStdout(), "Breakdown by current outcome:")
			for _, b := range ordered {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d\n", b.Outcome, b.Count)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "")
			headers := []string{"meeting_id", "title", "owner_id", "first_hit_at", "current_outcome"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{r.MeetingID, r.Title, r.OwnerID, r.FirstHitAt, r.CurrentOutcome})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&statusArg, "status", "scheduled", "Status to report on (scheduled, no-show, completed, cancelled — or pass a custom value verbatim)")
	cmd.Flags().StringVar(&monthArg, "month", "", "YYYY-MM (default: current month)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func canonicalizeStatus(s string) string {
	if v, ok := canonicalMeetingStatuses[s]; ok {
		return v
	}
	// Pass through unrecognized values (custom HubSpot outcomes) unchanged.
	return s
}

// parseMonth accepts YYYY-MM and returns [first-of-month, first-of-next-month).
func parseMonth(m string) (time.Time, time.Time, error) {
	t, err := time.Parse("2006-01", m)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--month must be YYYY-MM, got %q: %w", m, err)
	}
	from := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	return from, to, nil
}
