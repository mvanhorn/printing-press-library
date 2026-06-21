// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/valyu/internal/store"
)

func newCostCmd(flags *rootFlags) *cobra.Command {
	var since string
	var budget float64
	var breakdown bool

	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Show cumulative API spend tracked by this CLI.",
		Long: `Show cumulative Valyu API spend from commands run by this CLI.

Spend is recorded when using: semantic-search compare, semantic-search batch,
cited-answer, and deepresearch queue. Use --breakdown to see per-command totals.

Use this command for reviewing local spend history.
Do NOT use this command for live billing or invoice data; check your Valyu dashboard instead.`,
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			var sinceTS string
			if since != "" {
				d, parseErr := parseSinceDur(since)
				if parseErr != nil {
					return fmt.Errorf("invalid --since %q: expected format like 7d, 24h, 1w", since)
				}
				sinceTS = time.Now().Add(-d).UTC().Format(time.RFC3339)
			}

			totalQuery := `SELECT COALESCE(SUM(cost), 0.0) FROM api_cost_ledger`
			totalArgs := []any{}
			if sinceTS != "" {
				totalQuery += ` WHERE created_at >= ?`
				totalArgs = append(totalArgs, sinceTS)
			}
			var total float64
			if err := db.DB().QueryRowContext(cmd.Context(), totalQuery, totalArgs...).Scan(&total); err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("querying cost ledger: %w", err)
			}

			if flags.asJSON {
				if breakdown {
					rows, err := costBreakdownRows(cmd.Context(), db.DB(), sinceTS)
					if err != nil {
						return err
					}
					out, _ := json.Marshal(map[string]any{
						"total":     total,
						"since":     since,
						"breakdown": rows,
					})
					return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
				}
				out, _ := json.Marshal(map[string]any{"total": total, "since": since})
				return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
			}

			if budget > 0 && total >= budget {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: total spend $%.4f exceeds --budget $%.4f\n", total, budget)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			sinceStr := "all time"
			if since != "" {
				sinceStr = "last " + since
			}
			fmt.Fprintf(tw, "Total spend (%s):\t$%.4f\n", sinceStr, total)
			if budget > 0 {
				remaining := budget - total
				if remaining < 0 {
					remaining = 0
				}
				fmt.Fprintf(tw, "Budget:\t$%.4f\n", budget)
				fmt.Fprintf(tw, "Remaining:\t$%.4f\n", remaining)
			}

			if breakdown {
				brows, err := costBreakdownRows(cmd.Context(), db.DB(), sinceTS)
				if err != nil {
					return err
				}
				fmt.Fprintln(tw, "")
				fmt.Fprintln(tw, "Command\tCount\tTotal Cost")
				fmt.Fprintln(tw, "-------\t-----\t----------")
				for _, r := range brows {
					name, _ := r["command"].(string)
					count, _ := r["count"].(float64)
					cost, _ := r["total"].(float64)
					fmt.Fprintf(tw, "%s\t%d\t$%.4f\n", name, int(count), cost)
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Filter to costs since this duration ago (e.g. 7d, 24h, 1w)")
	cmd.Flags().Float64Var(&budget, "budget", 0, "Warn if total spend exceeds this amount (USD)")
	cmd.Flags().BoolVar(&breakdown, "breakdown", false, "Show per-command subtotals")
	return cmd
}

func costBreakdownRows(ctx context.Context, db *sql.DB, sinceTS string) ([]map[string]any, error) {
	query := `SELECT command, COUNT(*) as count, COALESCE(SUM(cost), 0.0) as total FROM api_cost_ledger`
	args := []any{}
	if sinceTS != "" {
		query += ` WHERE created_at >= ?`
		args = append(args, sinceTS)
	}
	query += ` GROUP BY command ORDER BY total DESC`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying breakdown: %w", err)
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		var cmdName string
		var count int
		var total float64
		if err := rows.Scan(&cmdName, &count, &total); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"command": cmdName, "count": float64(count), "total": total})
	}
	return result, rows.Err()
}

// recordCost writes a single cost entry to the local SQLite cost ledger.
// Non-fatal: errors are silently ignored so cost recording never breaks the main command flow.
func recordCost(db *store.Store, command string, cost float64) {
	if db == nil || cost <= 0 {
		return
	}
	db.DB().Exec(`INSERT INTO api_cost_ledger (command, cost) VALUES (?, ?)`, command, cost)
}

// parseSinceDur parses strings like "7d", "24h", "1w", "30m" into time.Duration.
func parseSinceDur(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("too short")
	}
	unit := s[len(s)-1]
	val := s[:len(s)-1]
	var n int
	if _, err := fmt.Sscanf(val, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid number %q", val)
	}
	switch unit {
	case 'm':
		return time.Duration(n) * time.Minute, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("unknown unit %q (use m, h, d, w)", string(unit))
}
