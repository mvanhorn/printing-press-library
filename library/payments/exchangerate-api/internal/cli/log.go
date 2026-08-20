// Novel command: log. Read-side surface for conversions_log so agents can
// recall recent conversions without re-querying the API.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newLogCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Inspect the local conversions_log",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newLogShowCmd(flags))
	return cmd
}

func newLogShowCmd(flags *rootFlags) *cobra.Command {
	var (
		since  string
		base   string
		target string
		limit  int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Show recent entries from conversions_log (filterable by --base, --target, --since)",
		Example:     "  exchangerate-api-pp-cli log show\n  exchangerate-api-pp-cli log show --since 7d --json\n  exchangerate-api-pp-cli log show --base USD --target EUR --limit 20",
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
			query := `SELECT base_code, target_code, amount, result, rate, source, captured_at FROM conversions_log WHERE 1=1`
			qArgs := []any{}
			if base != "" {
				query += ` AND base_code = ?`
				qArgs = append(qArgs, strings.ToUpper(base))
			}
			if target != "" {
				query += ` AND target_code = ?`
				qArgs = append(qArgs, strings.ToUpper(target))
			}
			if since != "" {
				cutoff, err := parseDurationOrDate(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since %q: %w", since, err))
				}
				query += ` AND captured_at >= ?`
				qArgs = append(qArgs, cutoff.UTC().Format("2006-01-02 15:04:05"))
			}
			query += ` ORDER BY captured_at DESC`
			if limit <= 0 {
				limit = 25
			}
			query += ` LIMIT ?`
			qArgs = append(qArgs, limit)
			rows, err := s.DB().QueryContext(cmd.Context(), query, qArgs...)
			if err != nil {
				return fmt.Errorf("query conversions_log: %w", err)
			}
			defer rows.Close()
			type row struct {
				Base       string  `json:"base"`
				Target     string  `json:"target"`
				Amount     float64 `json:"amount"`
				Result     float64 `json:"result"`
				Rate       float64 `json:"rate"`
				Source     string  `json:"source"`
				CapturedAt string  `json:"captured_at"`
			}
			results := []row{}
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Base, &r.Target, &r.Amount, &r.Result, &r.Rate, &r.Source, &r.CapturedAt); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				results = append(results, r)
			}
			// PATCH exchangerate-rows-err-checks: see drift.go rationale.
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating conversions_log: %w", err)
			}
			payload := map[string]any{"count": len(results), "entries": results, "limit": limit}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(out, "No conversion log entries match. Run 'convert' or 'convert-batch' to populate.")
				return nil
			}
			fmt.Fprintf(out, "%-22s %-6s %-6s %12s %12s %10s  %s\n", "CAPTURED_AT", "FROM", "TO", "AMOUNT", "RESULT", "RATE", "SOURCE")
			for _, r := range results {
				fmt.Fprintf(out, "%-22s %-6s %-6s %12.4f %12.4f %10.6f  %s\n",
					r.CapturedAt, r.Base, r.Target, r.Amount, r.Result, r.Rate, r.Source)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Filter to entries newer than this (e.g. 24h, 7d, YYYY-MM-DD)")
	cmd.Flags().StringVar(&base, "base", "", "Filter to a base currency")
	cmd.Flags().StringVar(&target, "target", "", "Filter to a target currency")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}
