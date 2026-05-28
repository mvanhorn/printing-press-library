// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/internal/store"

	"github.com/spf13/cobra"
)

type reconcileDay struct {
	Date          string  `json:"date"`
	AccountSpend  float64 `json:"account_spend"`
	InsightsSpend float64 `json:"insights_spend"`
	Drift         float64 `json:"drift"`
	DriftPct      float64 `json:"drift_pct"`
	Flagged       bool    `json:"flagged"`
}

type reconcileView struct {
	Account     string         `json:"account,omitempty"`
	SinceDays   int            `json:"since_days"`
	Threshold   float64        `json:"threshold_pct"`
	TotalDays   int            `json:"total_days"`
	FlaggedDays int            `json:"flagged_days"`
	Rows        []reconcileDay `json:"rows"`
	Note        string         `json:"note,omitempty"`
}

func newNovelReconcileCmd(flags *rootFlags) *cobra.Command {
	var flagAccount string
	var flagSince string
	var flagThreshold string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Per-day diff between account spend and sum-of-insights spend.",
		Long: `For each day in the window, compare the account-level reported spend against
the sum of ad-level insights spend. Flags days where the absolute drift exceeds
the threshold percent (default 5%). Usually surfaces days with delayed conversion
attribution or pixel/server-side event discrepancies.

Requires synced account-level and ad-level insights with time_increment=1.`,
		Example: `  meta-ads-pp-cli reconcile --account act_4327210487520472 --since 30d --threshold 5 --agent
  meta-ads-pp-cli reconcile --since 7d --json --select flagged_days,rows.date,rows.drift_pct`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would reconcile per-day spend drift from local insights")
				return nil
			}
			sinceDays := 30
			if flagSince != "" {
				n, err := parseDayWindow(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				sinceDays = n
			}
			threshold := 5.0
			if flagThreshold != "" {
				n, err := strconv.ParseFloat(flagThreshold, 64)
				if err != nil || n < 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--threshold must be a non-negative number, got %q", flagThreshold))
				}
				threshold = n
			}

			if dbPath == "" {
				dbPath = defaultDBPath("meta-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			accountFilter := ""
			queryArgs := []any{fmt.Sprintf("-%d days", sinceDays)}
			if flagAccount != "" {
				accountFilter = ` AND json_extract(data, '$.account_id') = ?`
				queryArgs = append(queryArgs, flagAccount)
			}

			// account-level spend per day
			accountQ := `SELECT
					json_extract(data, '$.date_start') AS date,
					CAST(json_extract(data, '$.spend') AS REAL) AS spend
				FROM resources
				WHERE resource_type IN ('insights', 'adaccounts_insights')
				  AND date(json_extract(data, '$.date_start')) > date('now', ?)` + accountFilter + `
				  AND (json_extract(data, '$.ad_id') IS NULL OR json_extract(data, '$.ad_id') = '')`

			accountByDate := make(map[string]float64)
			rows, err := db.DB().QueryContext(cmd.Context(), accountQ, queryArgs...)
			if err != nil {
				return fmt.Errorf("account-insights query: %w", err)
			}
			for rows.Next() {
				var date string
				var spend float64
				if err := rows.Scan(&date, &spend); err == nil {
					accountByDate[date] += spend
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating account-insights rows: %w", err)
			}
			_ = rows.Close()

			// ad-level spend per day. Exclude rows where ad_id is empty-string in
			// addition to NULL — accountQ already covers BOTH conditions, so without
			// this exclusion an empty-string ad_id row would land in both partitions
			// and the drift subtraction would silently cancel out the duplication.
			adQ := `SELECT data FROM resources
				WHERE resource_type IN ('insights', 'ads_insights', 'adaccounts_insights')
				  AND date(json_extract(data, '$.date_start')) > date('now', ?)
				  AND json_extract(data, '$.ad_id') IS NOT NULL
				  AND json_extract(data, '$.ad_id') != ''`

			rows2, err := db.DB().QueryContext(cmd.Context(), adQ, fmt.Sprintf("-%d days", sinceDays))
			if err != nil {
				return fmt.Errorf("ad-insights query: %w", err)
			}
			insightsByDate := make(map[string]float64)
			for rows2.Next() {
				var data []byte
				if err := rows2.Scan(&data); err != nil {
					continue
				}
				var raw struct {
					DateStart string `json:"date_start"`
					AccountID string `json:"account_id"`
					Spend     string `json:"spend"`
				}
				if err := json.Unmarshal(data, &raw); err != nil {
					continue
				}
				if flagAccount != "" {
					// Strict include: row must explicitly match the requested account.
					// Rows with an empty/missing account_id cannot be safely attributed
					// to flagAccount, so they are excluded rather than admitted.
					if raw.AccountID == "" || (raw.AccountID != flagAccount && "act_"+raw.AccountID != flagAccount) {
						continue
					}
				}
				v, _ := strconv.ParseFloat(raw.Spend, 64)
				insightsByDate[raw.DateStart] += v
			}
			if err := rows2.Err(); err != nil {
				_ = rows2.Close()
				return fmt.Errorf("iterating ad-insights rows: %w", err)
			}
			_ = rows2.Close()

			// Build reconcile rows from union of dates
			dateSet := make(map[string]bool)
			for d := range accountByDate {
				dateSet[d] = true
			}
			for d := range insightsByDate {
				dateSet[d] = true
			}

			view := reconcileView{
				Account:   flagAccount,
				SinceDays: sinceDays,
				Threshold: threshold,
				Rows:      make([]reconcileDay, 0, len(dateSet)),
			}
			for d := range dateSet {
				a := accountByDate[d]
				b := insightsByDate[d]
				drift := a - b
				driftPct := 0.0
				if a > 0 {
					driftPct = (drift / a) * 100.0
				}
				flagged := abs64(driftPct) > threshold
				view.Rows = append(view.Rows, reconcileDay{
					Date:          d,
					AccountSpend:  a,
					InsightsSpend: b,
					Drift:         drift,
					DriftPct:      driftPct,
					Flagged:       flagged,
				})
				if flagged {
					view.FlaggedDays++
				}
			}
			view.TotalDays = len(view.Rows)
			// Sort newest first
			sortReconcile(view.Rows)
			if view.TotalDays == 0 {
				view.Note = "no insights in local store for window; sync with both account-level and ad-level insights"
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagAccount, "account", "", "Filter to a specific ad account (e.g., act_1234567890)")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Look-back window (e.g., 7d, 30d, 90d)")
	cmd.Flags().StringVar(&flagThreshold, "threshold", "5", "Drift percent threshold to flag a day")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.meta-ads-pp-cli/data.db)")
	return cmd
}

func abs64(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func sortReconcile(rows []reconcileDay) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j-1].Date < rows[j].Date; j-- {
			rows[j-1], rows[j] = rows[j], rows[j-1]
		}
	}
}
