// Novel command: drift. Compares the latest snapshot per (base, target)
// against the oldest snapshot within a --since window and reports the
// biggest movers. Joins rows from rates_snapshots that no API endpoint
// can return.
package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/internal/store"
	"github.com/spf13/cobra"
)

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		base   string
		since  string
		top    int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Top FX movers since a prior snapshot (computed from local rates_snapshots)",
		Long: `Diffs the latest snapshot per (base, target) against the oldest snapshot
within --since (default 24h) and reports the largest absolute percent change.
Requires you to have been running 'sync-rates' periodically.`,
		Example:     "  exchangerate-api-pp-cli drift --base USD --since 24h\n  exchangerate-api-pp-cli drift --base EUR --since 7d --top 5 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if base == "" {
				base = "USD"
			}
			base = strings.ToUpper(base)
			if since == "" {
				since = "24h"
			}
			if top == 0 {
				top = 10
			}
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
			cutoff, err := parseDurationOrDate(since)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", since, err))
			}
			// Group by target; for each target take the latest rate now and
			// the oldest rate at-or-after the cutoff.
			query := `
				SELECT target_code,
					(SELECT rate FROM rates_snapshots WHERE base_code = ? AND target_code = t.target_code
						ORDER BY captured_at DESC LIMIT 1) AS now_rate,
					(SELECT rate FROM rates_snapshots WHERE base_code = ? AND target_code = t.target_code
						AND captured_at >= ? ORDER BY captured_at ASC LIMIT 1) AS then_rate
				FROM (SELECT DISTINCT target_code FROM rates_snapshots WHERE base_code = ?) t
				WHERE now_rate IS NOT NULL AND then_rate IS NOT NULL AND now_rate != then_rate
			`
			cutoffStr := cutoff.UTC().Format("2006-01-02 15:04:05")
			rows, err := s.DB().QueryContext(cmd.Context(), query, base, base, cutoffStr, base)
			if err != nil {
				return fmt.Errorf("query drift: %w", err)
			}
			defer rows.Close()
			type mover struct {
				Target    string  `json:"target"`
				From      float64 `json:"from_rate"`
				To        float64 `json:"to_rate"`
				ChangePct float64 `json:"change_pct"`
			}
			movers := []mover{}
			for rows.Next() {
				var m mover
				if err := rows.Scan(&m.Target, &m.To, &m.From); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				if m.From == 0 {
					continue
				}
				m.ChangePct = (m.To - m.From) / m.From * 100.0
				movers = append(movers, m)
			}
			// PATCH exchangerate-rows-err-checks: surface mid-iteration sql errors.
			// Without this rows.Err() check a transient SQLite error after the
			// first row truncates movers silently and the sort emits a partial
			// ranking with no diagnostic. Greptile P1 review of PR #635.
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating drift rows: %w", err)
			}
			sort.Slice(movers, func(i, j int) bool {
				return math.Abs(movers[i].ChangePct) > math.Abs(movers[j].ChangePct)
			})
			if top > 0 && len(movers) > top {
				movers = movers[:top]
			}
			payload := map[string]any{
				"base":   base,
				"since":  since,
				"cutoff": cutoffStr,
				"top":    top,
				"count":  len(movers),
				"movers": movers,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			out := cmd.OutOrStdout()
			if len(movers) == 0 {
				fmt.Fprintf(out, "No drift detected (need snapshots both at-or-after %s and now). Run 'sync-rates --base %s' regularly.\n", cutoffStr, base)
				return nil
			}
			fmt.Fprintf(out, "Top %d movers vs %s (base=%s):\n", len(movers), since, base)
			fmt.Fprintf(out, "%-8s %14s %14s %12s\n", "TARGET", "FROM", "TO", "CHANGE %")
			for _, m := range movers {
				fmt.Fprintf(out, "%-8s %14.6f %14.6f %+11.3f%%\n", m.Target, m.From, m.To, m.ChangePct)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "USD", "Base currency for the drift comparison")
	cmd.Flags().StringVar(&since, "since", "24h", "Window for the prior snapshot (e.g. 24h, 7d, 30d)")
	cmd.Flags().IntVar(&top, "top", 10, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}
