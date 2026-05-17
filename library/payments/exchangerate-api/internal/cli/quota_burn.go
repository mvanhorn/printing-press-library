// Novel command: quota burn. Reads quota_snapshots over time, computes
// average burn rate, and projects when the monthly quota will exhaust
// before the refresh day. Synthesizes a snapshot from a fresh /quota call
// if none exist yet.
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/internal/store"
	"github.com/spf13/cobra"
)

func newQuotaBurnCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath  string
		noFetch bool
	)
	cmd := &cobra.Command{
		Use:   "burn",
		Short: "Project quota exhaustion from quota_snapshots history",
		Long: `Reads the local quota_snapshots table (populated by 'quota refresh' or
auto-captured by 'burn'), computes the burn rate, and projects the date
at which your quota will hit zero — or whether you'll comfortably reach
the next refresh day.

If there are zero local snapshots, this command fetches one fresh /quota
reading before computing (use --no-fetch to suppress).`,
		Example: "  exchangerate-api-pp-cli quota burn\n  exchangerate-api-pp-cli quota burn --json --select projected_exhaustion,requests_remaining",
		// Not mcp:read-only: appends one row to quota_snapshots when fetching
		// a fresh /quota reading (the default). Pass --no-fetch for the
		// read-only form.
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("exchangerate-api-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()
			if err := s.EnsureExrateTables(cmd.Context()); err != nil {
				return err
			}

			// Best-effort fresh snapshot capture before reading history.
			if !noFetch {
				if c, cErr := flags.newClient(); cErr == nil && c.Config != nil && c.Config.ExchangerateApiKey != "" {
					path := fmt.Sprintf("/v6/%s/quota", c.Config.ExchangerateApiKey)
					if body, getErr := c.Get(path, nil); getErr == nil {
						var qr struct {
							Result            string `json:"result"`
							PlanQuota         int    `json:"plan_quota"`
							RequestsRemaining int    `json:"requests_remaining"`
							RefreshDay        int    `json:"refresh_day_of_month"`
						}
						if json.Unmarshal(body, &qr) == nil && qr.Result == "success" {
							_ = s.InsertQuotaSnapshot(cmd.Context(), qr.PlanQuota, qr.RequestsRemaining, qr.RefreshDay)
						}
					}
				}
			}

			type snap struct {
				PlanQuota         int
				RequestsRemaining int
				RefreshDay        int
				At                time.Time
			}
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT plan_quota, requests_remaining, refresh_day_of_month, captured_at FROM quota_snapshots ORDER BY captured_at ASC`)
			if err != nil {
				return fmt.Errorf("query quota_snapshots: %w", err)
			}
			defer rows.Close()
			var snaps []snap
			for rows.Next() {
				var sn snap
				var at string
				if err := rows.Scan(&sn.PlanQuota, &sn.RequestsRemaining, &sn.RefreshDay, &at); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				if t, err := time.Parse("2006-01-02 15:04:05", at); err == nil {
					sn.At = t
				}
				snaps = append(snaps, sn)
			}
			// PATCH exchangerate-rows-err-checks: surface mid-iteration sql errors.
			// Without this rows.Err() check, a truncated snaps slice produces a
			// falsely-optimistic burn projection with no diagnostic. Greptile P1
			// review of PR #635.
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating quota_snapshots: %w", err)
			}
			payload := map[string]any{
				"snapshot_count": len(snaps),
			}
			if len(snaps) == 0 {
				payload["status"] = "no-data"
				payload["hint"] = "run 'quota burn' a few times (or 'quota' which captures a snapshot) to populate quota_snapshots"
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No quota snapshots yet. Run 'quota burn' a few times over the next day to seed history.")
				return nil
			}
			latest := snaps[len(snaps)-1]
			payload["plan_quota"] = latest.PlanQuota
			payload["requests_remaining"] = latest.RequestsRemaining
			payload["refresh_day_of_month"] = latest.RefreshDay
			payload["latest_captured_at"] = latest.At.UTC().Format(time.RFC3339)
			nextRefresh := nextRefreshDate(time.Now().UTC(), latest.RefreshDay)
			payload["next_refresh"] = nextRefresh.Format(time.RFC3339)

			if len(snaps) >= 2 {
				first := snaps[0]
				elapsed := latest.At.Sub(first.At).Hours()
				delta := first.RequestsRemaining - latest.RequestsRemaining
				if elapsed <= 0 || delta <= 0 {
					payload["burn_rate_per_hour"] = 0.0
					payload["status"] = "stable"
				} else {
					perHour := float64(delta) / elapsed
					payload["burn_rate_per_hour"] = perHour
					hoursUntilZero := float64(latest.RequestsRemaining) / perHour
					// Cap at 1 year so an effectively-zero burn rate doesn't
					// produce +Inf, which time.Duration cannot represent and
					// would silently overflow the resulting time.Time.
					const maxHours = 8760.0
					if hoursUntilZero > maxHours {
						hoursUntilZero = maxHours
					}
					exhaustion := time.Now().UTC().Add(time.Duration(hoursUntilZero * float64(time.Hour)))
					payload["projected_exhaustion"] = exhaustion.Format(time.RFC3339)
					if exhaustion.Before(nextRefresh) {
						payload["status"] = "WARNING: projected to exhaust before refresh"
					} else {
						payload["status"] = "OK"
					}
				}
			} else {
				payload["status"] = "warming-up"
				payload["hint"] = "need at least 2 snapshots to project burn rate; run again later"
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Quota: %d / %d remaining (refresh day %d)\n", latest.RequestsRemaining, latest.PlanQuota, latest.RefreshDay)
			fmt.Fprintf(out, "Snapshots: %d, latest %s\n", len(snaps), latest.At.UTC().Format(time.RFC3339))
			if pr, ok := payload["projected_exhaustion"].(string); ok {
				fmt.Fprintf(out, "Burn rate: %.2f req/hr | Projected exhaustion: %s\n", payload["burn_rate_per_hour"], pr)
			}
			fmt.Fprintf(out, "Status: %s\n", payload["status"])
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "Don't capture a fresh /quota snapshot before computing")
	return cmd
}

// PATCH exchangerate-next-refresh-month-clamp: clamp refresh day to the last
// day of the month before constructing the candidate. Without this, a
// refresh_day_of_month of 31 in a 30-day month (e.g. April) normalises via
// time.Date to May 1, pushing next_refresh one month later than it really is
// and suppressing a real WARNING projected-exhaustion status. Greptile P2
// review of PR #635. The original refreshDay is preserved across the
// next-month rollover branch — clamping for the current month must not
// shorten the intended day for next month (Jun 30 day=31 → Jul 31, not Jul 30).
func nextRefreshDate(now time.Time, refreshDay int) time.Time {
	lastDay := func(year int, month time.Month) int {
		return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	}
	year, month, _ := now.Date()
	thisDay := refreshDay
	if max := lastDay(year, month); thisDay > max {
		thisDay = max
	}
	candidate := time.Date(year, month, thisDay, 0, 0, 0, 0, time.UTC)
	if !candidate.After(now) {
		nextYear, nextMonth, _ := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0).Date()
		nextDay := refreshDay
		if max := lastDay(nextYear, nextMonth); nextDay > max {
			nextDay = max
		}
		candidate = time.Date(nextYear, nextMonth, nextDay, 0, 0, 0, 0, time.UTC)
	}
	return candidate
}
