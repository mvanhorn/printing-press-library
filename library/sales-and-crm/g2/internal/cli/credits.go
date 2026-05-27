// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/store"
	"github.com/spf13/cobra"
)

// newCreditsCmd builds the `credits` parent + children: balance, log, forecast.
func newCreditsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "credits",
		Short:       "Inspect credit balance, deduction log, and burn forecast",
		Long:        "Read locally synced credit_account and credit_deductions to surface balance, recent deductions, and a month-end burn forecast.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCreditsBalanceCmd(flags))
	cmd.AddCommand(newCreditsLogCmd(flags))
	cmd.AddCommand(newCreditsForecastCmd(flags))
	return cmd
}

// creditBalanceRow mirrors what we surface from the credit_account table.
type creditBalanceRow struct {
	OrganizationID string  `json:"organization_id"`
	Balance        float64 `json:"balance"`
	TotalCredits   float64 `json:"total_credits"`
	UsedCredits    float64 `json:"used_credits"`
}

func newCreditsBalanceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "balance [flags]",
		Short:       "Show locally synced credit balance per organization",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  g2-pp-cli credits balance --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("g2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			maybeEmitSyncHints(cmd, db, "credit_account", flags.maxAge)

			rows, err := readCreditBalances(db.DB())
			if err != nil {
				return fmt.Errorf("reading credit_account: %w", err)
			}
			if len(rows) == 0 {
				return emitEmptyCreditsResult(cmd, flags, "credit_account", "no credit_account data — run `g2-pp-cli sync --resources credit_account` first")
			}

			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"organization_id", "balance", "total_credits", "used_credits"}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					r.OrganizationID,
					fmt.Sprintf("%.2f", r.Balance),
					fmt.Sprintf("%.2f", r.TotalCredits),
					fmt.Sprintf("%.2f", r.UsedCredits),
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/g2-pp-cli/data.db)")
	return cmd
}

// readCreditBalances scans credit_account rows. Many G2 organizations
// have exactly one row; we return all just in case a token has access
// to multiple orgs.
func readCreditBalances(db *sql.DB) ([]creditBalanceRow, error) {
	rs, err := db.Query(`SELECT organization_id, data FROM credit_account`)
	if err != nil {
		// Table may not exist on a fresh DB; treat as empty.
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rs.Close()
	var out []creditBalanceRow
	for rs.Next() {
		var orgID, data sql.NullString
		if err := rs.Scan(&orgID, &data); err != nil {
			return nil, err
		}
		row := creditBalanceRow{OrganizationID: orgID.String}
		if data.Valid {
			extractCreditAccountAttrs(data.String, &row)
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

// extractCreditAccountAttrs pulls balance/total/used from a JSON blob.
// Tries `attributes.<field>` first, falls back to flat keys.
func extractCreditAccountAttrs(data string, row *creditBalanceRow) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return
	}
	attrs, _ := obj["attributes"].(map[string]any)
	if attrs == nil {
		attrs = obj
	}
	if v, ok := attrs["balance"].(float64); ok {
		row.Balance = v
	}
	if v, ok := attrs["remaining_credits"].(float64); ok && row.Balance == 0 {
		row.Balance = v
	}
	if v, ok := attrs["total_credits"].(float64); ok {
		row.TotalCredits = v
	}
	if v, ok := attrs["used_credits"].(float64); ok {
		row.UsedCredits = v
	}
}

// creditLogRow groups credit deductions by endpoint.
type creditLogRow struct {
	Endpoint     string  `json:"endpoint"`
	Deductions   int64   `json:"deductions"`
	CreditsUsed  float64 `json:"credits_used"`
	LastUsedDate string  `json:"last_used_date,omitempty"`
}

func newCreditsLogCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "log [flags]",
		Short:       "List credit deductions grouped by endpoint over a window",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  g2-pp-cli credits log --since 14d --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("g2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			maybeEmitSyncHints(cmd, db, "credit_deductions", flags.maxAge)

			cutoff := time.Time{}
			if strings.TrimSpace(since) != "" {
				window, perr := cliutil.ParseDurationLoose(since)
				if perr != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, perr))
				}
				cutoff = time.Now().Add(-window)
			}

			rows, err := readCreditLog(db.DB(), cutoff)
			if err != nil {
				return fmt.Errorf("reading credit_deductions: %w", err)
			}
			if len(rows) == 0 {
				return emitEmptyCreditsResult(cmd, flags, "credit_deductions", "no credit_deductions data — run `g2-pp-cli sync --resources credit_deductions` first")
			}

			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"endpoint", "deductions", "credits_used", "last_used"}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					r.Endpoint,
					fmt.Sprintf("%d", r.Deductions),
					fmt.Sprintf("%.2f", r.CreditsUsed),
					r.LastUsedDate,
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window for the log (e.g. 14d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/g2-pp-cli/data.db)")
	return cmd
}

// readCreditLog reads credit_deductions and groups by endpoint.
// cutoff zero value disables the time filter.
func readCreditLog(db *sql.DB, cutoff time.Time) ([]creditLogRow, error) {
	var q string
	var rs *sql.Rows
	var err error
	if cutoff.IsZero() {
		q = `SELECT data, synced_at FROM credit_deductions`
		rs, err = db.Query(q)
	} else {
		q = `SELECT data, synced_at FROM credit_deductions WHERE synced_at >= ?`
		rs, err = db.Query(q, cutoff)
	}
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rs.Close()

	type acc struct {
		count    int64
		credits  float64
		lastUsed string
	}
	byEndpoint := map[string]*acc{}
	for rs.Next() {
		var data sql.NullString
		var syncedAt sql.NullTime
		if err := rs.Scan(&data, &syncedAt); err != nil {
			return nil, err
		}
		if !data.Valid {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data.String), &obj); err != nil {
			continue
		}
		attrs, _ := obj["attributes"].(map[string]any)
		if attrs == nil {
			attrs = obj
		}
		endpoint, _ := attrs["endpoint"].(string)
		if endpoint == "" {
			endpoint, _ = attrs["path"].(string)
		}
		if endpoint == "" {
			endpoint = "unknown"
		}
		var credits float64
		if v, ok := attrs["credits"].(float64); ok {
			credits = v
		} else if v, ok := attrs["amount"].(float64); ok {
			credits = v
		} else if v, ok := attrs["debit"].(float64); ok {
			credits = v
		}
		usedAt, _ := attrs["created_at"].(string)
		if usedAt == "" {
			usedAt, _ = attrs["used_at"].(string)
		}
		if usedAt == "" && syncedAt.Valid {
			usedAt = syncedAt.Time.UTC().Format(time.RFC3339)
		}

		a := byEndpoint[endpoint]
		if a == nil {
			a = &acc{}
			byEndpoint[endpoint] = a
		}
		a.count++
		a.credits += credits
		if usedAt > a.lastUsed {
			a.lastUsed = usedAt
		}
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	out := make([]creditLogRow, 0, len(byEndpoint))
	for ep, a := range byEndpoint {
		out = append(out, creditLogRow{
			Endpoint:     ep,
			Deductions:   a.count,
			CreditsUsed:  a.credits,
			LastUsedDate: a.lastUsed,
		})
	}
	return out, nil
}

// creditsForecast is the result emitted by `credits forecast`.
type creditsForecast struct {
	RemainingCredits      float64 `json:"remaining_credits"`
	DailyAvg              float64 `json:"daily_avg"`
	ProjectedMonthEndBurn float64 `json:"projected_month_end_burn"`
	OnTrack               bool    `json:"on_track"`
	WindowDaysAnalyzed    int     `json:"window_days_analyzed"`
}

func newCreditsForecastCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "forecast [flags]",
		Short:       "Project month-end credit burn from 14-day trailing usage",
		Long:        "Computes daily average credit usage from trailing 14 days of credit_deductions and projects month-end spend. `on_track` is true when projected burn fits inside remaining_credits.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  g2-pp-cli credits forecast --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("g2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			maybeEmitSyncHints(cmd, db, "credit_deductions", flags.maxAge)

			balances, err := readCreditBalances(db.DB())
			if err != nil {
				return fmt.Errorf("reading balances: %w", err)
			}
			cutoff := time.Now().Add(-14 * 24 * time.Hour)
			logRows, err := readCreditLog(db.DB(), cutoff)
			if err != nil {
				return fmt.Errorf("reading deductions: %w", err)
			}
			if len(balances) == 0 && len(logRows) == 0 {
				return emitEmptyCreditsResult(cmd, flags, "credit_deductions", "no credit_account or credit_deductions data — run `g2-pp-cli sync --resources credit_account,credit_deductions` first")
			}
			forecast := buildCreditsForecast(balances, logRows)

			if flags.asJSON {
				return flags.printJSON(cmd, forecast)
			}
			headers := []string{"remaining_credits", "daily_avg", "projected_month_end_burn", "on_track", "window_days"}
			table := [][]string{{
				fmt.Sprintf("%.2f", forecast.RemainingCredits),
				fmt.Sprintf("%.2f", forecast.DailyAvg),
				fmt.Sprintf("%.2f", forecast.ProjectedMonthEndBurn),
				fmt.Sprintf("%t", forecast.OnTrack),
				fmt.Sprintf("%d", forecast.WindowDaysAnalyzed),
			}}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/g2-pp-cli/data.db)")
	return cmd
}

// buildCreditsForecast composes a forecast from balance + log rows.
// Pure function — testable without the DB.
func buildCreditsForecast(balances []creditBalanceRow, logRows []creditLogRow) creditsForecast {
	var remaining float64
	for _, b := range balances {
		remaining += b.Balance
	}
	var totalCredits float64
	for _, r := range logRows {
		totalCredits += r.CreditsUsed
	}
	const window = 14
	dailyAvg := totalCredits / float64(window)
	now := time.Now()
	daysLeft := daysUntilMonthEnd(now)
	projected := dailyAvg * float64(daysLeft)
	return creditsForecast{
		RemainingCredits:      remaining,
		DailyAvg:              dailyAvg,
		ProjectedMonthEndBurn: projected,
		OnTrack:               projected <= remaining || remaining == 0,
		WindowDaysAnalyzed:    window,
	}
}

// daysUntilMonthEnd returns the number of whole days from `from` to the
// last day of `from`'s month (inclusive). Used by the forecast projection.
func daysUntilMonthEnd(from time.Time) int {
	year, month, _ := from.Date()
	firstOfNext := time.Date(year, month+1, 1, 0, 0, 0, 0, from.Location())
	delta := firstOfNext.Sub(from)
	days := int(delta.Hours() / 24)
	if days < 1 {
		days = 1
	}
	return days
}

// emitEmptyCreditsResult emits a structured "no data" result for credits
// commands. Returns nil so the exit code is 0 — empty local store is not
// an error condition, just a no-op until sync runs.
func emitEmptyCreditsResult(cmd *cobra.Command, flags *rootFlags, resource, prose string) error {
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{
			"resource": resource,
			"empty":    true,
			"hint":     prose,
		})
	}
	fmt.Fprintln(cmd.ErrOrStderr(), prose)
	return nil
}
