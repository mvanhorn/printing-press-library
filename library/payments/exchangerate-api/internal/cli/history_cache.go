// Novel command: history-cache. Reconstructs historical rates from prior
// /latest syncs in the local rates_snapshots table — free-tier time travel
// without the paid /history endpoint.
package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/internal/store"
	"github.com/spf13/cobra"
)

func newHistoryCacheCmd(flags *rootFlags) *cobra.Command {
	var (
		since  string
		limit  int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "history-cache <base_code> <target_code>",
		Short: "Show historical rates for a pair from local rates_snapshots (free-tier time travel)",
		Long: `Reads rates_snapshots populated by 'sync-rates' (or any prior 'latest'
call) and returns the rate history for <base_code> -> <target_code>.

This is the free-tier alternative to /history: rather than paying for a
Pro plan to access /history, you sync /latest daily and query your own
local archive.

Use --since to limit to recent history (e.g. "30d", "7d", "24h").`,
		Example:     "  exchangerate-api-pp-cli history-cache USD EUR\n  exchangerate-api-pp-cli history-cache USD EUR --since 30d --json\n  exchangerate-api-pp-cli history-cache GBP JPY --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("target_code is required\nUsage: %s <base_code> <target_code>", cmd.CommandPath()))
			}
			base := strings.ToUpper(args[0])
			target := strings.ToUpper(args[1])
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
			query := `SELECT rate, source, captured_at FROM rates_snapshots WHERE base_code = ? AND target_code = ?`
			qArgs := []any{base, target}
			if since != "" {
				cutoff, err := parseDurationOrDate(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since %q: %w", since, err))
				}
				query += ` AND captured_at >= ?`
				qArgs = append(qArgs, cutoff.UTC().Format("2006-01-02 15:04:05"))
			}
			query += ` ORDER BY captured_at DESC`
			if limit > 0 {
				query += ` LIMIT ?`
				qArgs = append(qArgs, limit)
			}
			rows, err := s.DB().QueryContext(cmd.Context(), query, qArgs...)
			if err != nil {
				return fmt.Errorf("query rates_snapshots: %w", err)
			}
			defer rows.Close()
			type row struct {
				Rate       float64 `json:"rate"`
				Source     string  `json:"source"`
				CapturedAt string  `json:"captured_at"`
			}
			results := []row{}
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Rate, &r.Source, &r.CapturedAt); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				results = append(results, r)
			}
			payload := map[string]any{
				"base":    base,
				"target":  target,
				"count":   len(results),
				"history": results,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintf(out, "No snapshots found for %s/%s. Run 'sync-rates --base %s' to start building history.\n", base, target, base)
				return nil
			}
			fmt.Fprintf(out, "%-22s %12s  %s\n", "CAPTURED_AT", "RATE", "SOURCE")
			for _, r := range results {
				fmt.Fprintf(out, "%-22s %12.6f  %s\n", r.CapturedAt, r.Rate, r.Source)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Filter to snapshots within the last duration (e.g. 30d, 24h) or since date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

// parseDurationOrDate accepts "30d", "24h", "15m", "2w", or YYYY-MM-DD.
func parseDurationOrDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// shortcut: 30d, 2w, 24h, 15m
	if len(s) > 1 {
		unit := s[len(s)-1]
		val, err := strconv.Atoi(s[:len(s)-1])
		if err == nil {
			now := time.Now().UTC()
			switch unit {
			case 'd':
				return now.AddDate(0, 0, -val), nil
			case 'w':
				return now.AddDate(0, 0, -val*7), nil
			case 'h':
				return now.Add(-time.Duration(val) * time.Hour), nil
			case 'm':
				return now.Add(-time.Duration(val) * time.Minute), nil
			}
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected duration (e.g. 30d, 24h) or date (YYYY-MM-DD)")
	}
	return time.Now().UTC().Add(-d), nil
}
