// Novel command: watch. Watchlist for currency pairs with per-pair
// threshold (% change). 'watch check' fetches fresh rates for every base
// in the watchlist and reports pairs whose change since last_known_rate
// exceeds threshold.
package cli

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/internal/store"
	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watchlist for currency pairs with movement-threshold alerts",
		Long:  "Persistent watchlist stored in SQLite. 'watch add' registers a pair with a threshold; 'watch check' fetches the current rate and flags pairs that moved more than the threshold since the last check.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchCheckCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var (
		threshold float64
		dbPath    string
	)
	cmd := &cobra.Command{
		Use:         "add <base_code> <target_code>",
		Short:       "Add a currency pair to the watchlist",
		Example:     "  exchangerate-api-pp-cli watch add USD EUR --threshold 1.0\n  exchangerate-api-pp-cli watch add GBP JPY --threshold 2.5",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			base := strings.ToUpper(args[0])
			target := strings.ToUpper(args[1])
			if threshold <= 0 {
				threshold = 1.0
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openExrateStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			_, err = s.DB().ExecContext(cmd.Context(),
				`INSERT INTO watchlist(base_code, target_code, threshold_pct) VALUES(?, ?, ?)
				 ON CONFLICT(base_code, target_code) DO UPDATE SET threshold_pct = excluded.threshold_pct`,
				base, target, threshold)
			if err != nil {
				return fmt.Errorf("insert watchlist: %w", err)
			}
			payload := map[string]any{"base": base, "target": target, "threshold_pct": threshold, "added": true}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Watching %s/%s @ %.2f%% threshold\n", base, target, threshold)
			return nil
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 1.0, "Alert when |%% change since last check| exceeds this")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all watched currency pairs",
		Example:     "  exchangerate-api-pp-cli watch list\n  exchangerate-api-pp-cli watch list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openExrateStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT base_code, target_code, threshold_pct, COALESCE(last_known_rate, 0), COALESCE(last_checked_at, '') FROM watchlist ORDER BY base_code, target_code`)
			if err != nil {
				return fmt.Errorf("query watchlist: %w", err)
			}
			defer rows.Close()
			type entry struct {
				Base          string  `json:"base"`
				Target        string  `json:"target"`
				ThresholdPct  float64 `json:"threshold_pct"`
				LastKnownRate float64 `json:"last_known_rate"`
				LastCheckedAt string  `json:"last_checked_at"`
			}
			out := []entry{}
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Base, &e.Target, &e.ThresholdPct, &e.LastKnownRate, &e.LastCheckedAt); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				out = append(out, e)
			}
			// PATCH exchangerate-rows-err-checks: see drift.go rationale.
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating watchlist: %w", err)
			}
			payload := map[string]any{"count": len(out), "watchlist": out}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No watched pairs. Add one with 'watch add <base> <target>'.")
				return nil
			}
			fmt.Fprintf(w, "%-6s %-6s %10s %14s  %s\n", "BASE", "TARGET", "THRESH%", "LAST_RATE", "LAST_CHECKED_AT")
			for _, e := range out {
				fmt.Fprintf(w, "%-6s %-6s %10.2f %14.6f  %s\n", e.Base, e.Target, e.ThresholdPct, e.LastKnownRate, e.LastCheckedAt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "remove <base_code> <target_code>",
		Short:       "Remove a currency pair from the watchlist",
		Example:     "  exchangerate-api-pp-cli watch remove USD EUR",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			base := strings.ToUpper(args[0])
			target := strings.ToUpper(args[1])
			if dryRunOK(flags) {
				return nil
			}
			s, err := openExrateStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			res, err := s.DB().ExecContext(cmd.Context(),
				`DELETE FROM watchlist WHERE base_code = ? AND target_code = ?`, base, target)
			if err != nil {
				return fmt.Errorf("delete watchlist: %w", err)
			}
			n, _ := res.RowsAffected()
			payload := map[string]any{"base": base, "target": target, "removed": n}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d watch entries for %s/%s\n", n, base, target)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

func newWatchCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Fetch current rates for every watched pair and report threshold crossings",
		Long: `Iterates the watchlist, groups pairs by base, and fetches /latest once per
distinct base. Compares each pair's current rate to the stored last_known_rate
and reports pairs whose absolute % change exceeds threshold_pct. Updates the
stored rate in either case.`,
		Example:     "  exchangerate-api-pp-cli watch check\n  exchangerate-api-pp-cli watch check --json --select alerts",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if c.Config == nil || c.Config.ExchangerateApiKey == "" {
				return usageErr(fmt.Errorf("EXCHANGERATE_API_KEY is required; export it or run 'auth set-token <key>'"))
			}
			s, err := openExrateStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT base_code, target_code, threshold_pct, COALESCE(last_known_rate, 0) FROM watchlist ORDER BY base_code, target_code`)
			if err != nil {
				return fmt.Errorf("query watchlist: %w", err)
			}
			type entry struct {
				Base      string
				Target    string
				Threshold float64
				LastRate  float64
			}
			var entries []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Base, &e.Target, &e.Threshold, &e.LastRate); err != nil {
					rows.Close()
					return fmt.Errorf("scan: %w", err)
				}
				entries = append(entries, e)
			}
			// PATCH exchangerate-rows-err-checks: surface mid-iteration sql
			// errors so threshold crossings for the dropped pairs aren't
			// silently missed. Greptile P1 review of PR #635.
			if err := rows.Err(); err != nil {
				rows.Close()
				return fmt.Errorf("iterating watchlist for check: %w", err)
			}
			rows.Close()
			if len(entries) == 0 {
				payload := map[string]any{"count": 0, "alerts": []any{}, "note": "watchlist is empty"}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Add a pair with 'watch add <base> <target>'.")
				return nil
			}
			// Group by base; one /latest per distinct base.
			byBase := map[string][]int{}
			bases := []string{}
			for i, e := range entries {
				if _, ok := byBase[e.Base]; !ok {
					bases = append(bases, e.Base)
				}
				byBase[e.Base] = append(byBase[e.Base], i)
			}
			sort.Strings(bases)
			type alert struct {
				Base      string  `json:"base"`
				Target    string  `json:"target"`
				LastRate  float64 `json:"last_rate"`
				CurrRate  float64 `json:"current_rate"`
				ChangePct float64 `json:"change_pct"`
				Threshold float64 `json:"threshold_pct"`
				Crossed   bool    `json:"crossed"`
			}
			var alerts []alert
			now := time.Now().UTC().Format("2006-01-02 15:04:05")
			for _, base := range bases {
				count, _, rates, syncErr := syncRatesOnce(cmd.Context(), c, base, dbPath)
				if syncErr != nil {
					return syncErr
				}
				_ = count
				for _, idx := range byBase[base] {
					e := entries[idx]
					curr, ok := rates[e.Target]
					if !ok {
						continue
					}
					var changePct float64
					if e.LastRate > 0 {
						changePct = (curr - e.LastRate) / e.LastRate * 100.0
					}
					a := alert{
						Base: e.Base, Target: e.Target, LastRate: e.LastRate, CurrRate: curr,
						ChangePct: changePct, Threshold: e.Threshold,
						Crossed: e.LastRate > 0 && math.Abs(changePct) >= e.Threshold,
					}
					alerts = append(alerts, a)
					// Route through the writeMu-protected helper so the
					// per-pair UPDATE serializes against concurrent
					// InsertRateSnapshots in syncRatesOnce above.
					_ = s.UpdateWatchObservation(cmd.Context(), e.Base, e.Target, curr, now)
				}
			}
			crossed := []alert{}
			for _, a := range alerts {
				if a.Crossed {
					crossed = append(crossed, a)
				}
			}
			payload := map[string]any{
				"checked":       len(alerts),
				"crossed_count": len(crossed),
				"alerts":        alerts,
				"checked_at":    now,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Checked %d pair(s); %d crossed threshold:\n", len(alerts), len(crossed))
			fmt.Fprintf(w, "%-6s %-6s %14s %14s %10s %8s %s\n", "BASE", "TARGET", "LAST", "CURRENT", "CHANGE%", "THRESH%", "ALERT")
			for _, a := range alerts {
				flag := ""
				if a.Crossed {
					flag = "*"
				}
				fmt.Fprintf(w, "%-6s %-6s %14.6f %14.6f %9.3f%% %7.2f%% %s\n", a.Base, a.Target, a.LastRate, a.CurrRate, a.ChangePct, a.Threshold, flag)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

// openExrateStore opens the SQLite store and ensures novel-feature tables exist.
func openExrateStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("exchangerate-api-pp-cli")
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening store at %s: %w", dbPath, err)
	}
	if err := s.EnsureExrateTables(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// (unused but kept for parity with the docs example showing strconv import)
var _ = strconv.Itoa
