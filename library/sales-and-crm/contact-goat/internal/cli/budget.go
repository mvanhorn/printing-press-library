// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// budget: aggregate Deepline credit spend tracked in the local deepline_log
// table.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/store"

	"github.com/spf13/cobra"
)

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var historyLimit int

	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Deepline credit spend: totals, top tools, and history",
		Long: `Aggregate Deepline credit spend from the local log (no network calls).

Every deepline execute call — including dry-runs and errors — is logged to
~/.local/share/contact-goat-pp-cli/data.db#deepline_log by tool_id and
timestamp, along with its estimated credit cost. This command sums that log
so you can track spend without a confirmed upstream balance endpoint.`,
		Example: `  contact-goat-pp-cli budget
  contact-goat-pp-cli budget --since 7d --json
  contact-goat-pp-cli budget set-limit 500
  contact-goat-pp-cli budget history --limit 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBudgetDefault(cmd, flags, sinceStr)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "30d", "Window for top-tools breakdown (e.g. 24h, 7d, 30d, all)")
	cmd.Flags().IntVar(&historyLimit, "limit", 50, "Number of rows for the history subcommand")
	cmd.AddCommand(newBudgetSetLimitCmd(flags))
	cmd.AddCommand(newBudgetHistoryCmd(flags, &historyLimit))
	return cmd
}

func runBudgetDefault(cmd *cobra.Command, flags *rootFlags, sinceStr string) error {
	dbPath := defaultDBPath("contact-goat-pp-cli")
	s, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening local db: %w", err)
	}
	defer s.Close()

	thisMonthHours := hoursSinceMonthStart()
	last30d := 24 * 30
	sinceHours := parseSinceHours(sinceStr)

	thisMonth := s.SumDeeplineCost(thisMonthHours)
	last30 := s.SumDeeplineCost(last30d)
	within := s.SumDeeplineCost(sinceHours)

	top, err := s.DeeplineTopToolsBySpend(sinceHours, 10)
	if err != nil {
		return fmt.Errorf("top tools: %w", err)
	}

	limit := loadBudgetLimit()

	out := map[string]any{
		"this_month_credits":   thisMonth,
		"last_30_days_credits": last30,
		"since":                sinceStr,
		"since_credits":        within,
		"top_tools":            top,
		"limit_credits":        limit,
		"remaining_credits":    remaining(limit, thisMonth),
		"db_path":              dbPath,
	}

	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Deepline budget (local log)\n")
	fmt.Fprintf(w, "  this month: %d credit(s)\n", thisMonth)
	fmt.Fprintf(w, "  last 30d:   %d credit(s)\n", last30)
	fmt.Fprintf(w, "  since %s:  %d credit(s)\n", sinceStr, within)
	if limit > 0 {
		rem := remaining(limit, thisMonth)
		fmt.Fprintf(w, "  limit:      %d (remaining this month: %d)\n", limit, rem)
	} else {
		fmt.Fprintln(w, "  limit:      unset — run `budget set-limit <N>` to cap spend")
	}
	fmt.Fprintf(w, "\nTop tools by spend (%s):\n", sinceStr)
	if len(top) == 0 {
		fmt.Fprintln(w, "  (no calls yet)")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, bold("TOOL")+"\t"+bold("CALLS")+"\t"+bold("CREDITS"))
	for _, row := range top {
		fmt.Fprintf(tw, "%s\t%d\t%d\n", row["tool_id"], toInt(row["calls"]), toInt(row["credits"]))
	}
	return tw.Flush()
}

func newBudgetSetLimitCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set-limit <credits>",
		Short: "Set a per-month Deepline credit ceiling (local-only)",
		Long: `Store a Deepline spend ceiling locally. This CLI does not block spend at
this ceiling — it is informational. Re-run 'budget' to see remaining headroom.

The limit is stored at ~/.local/share/contact-goat-pp-cli/budget.toml.`,
		Example: `  contact-goat-pp-cli budget set-limit 500
  contact-goat-pp-cli budget set-limit 0   # clear`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 0 {
				return usageErr(fmt.Errorf("invalid limit %q: must be a non-negative integer", args[0]))
			}
			if err := saveBudgetLimit(n); err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"limit_credits": n,
					"reset_date":    monthStart().Format(time.RFC3339),
				})
			}
			if n == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "budget limit cleared")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "budget limit set to %d credits (resets %s)\n", n, monthStart().Format("2006-01-02"))
			}
			return nil
		},
	}
}

func newBudgetHistoryCmd(flags *rootFlags, limit *int) *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Last N Deepline calls (tool, credits, status, timestamp)",
		Example: `  contact-goat-pp-cli budget history
  contact-goat-pp-cli budget history --limit 100 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := defaultDBPath("contact-goat-pp-cli")
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening local db: %w", err)
			}
			defer s.Close()
			rows, err := s.GetDeeplineLogHistory(*limit)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"history": rows, "count": len(rows)})
			}
			w := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(w, "no deepline calls logged yet")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, bold("WHEN")+"\t"+bold("TOOL")+"\t"+bold("CREDITS")+"\t"+bold("STATUS"))
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r["timestamp"], r["tool_id"], toInt(r["cost_credits"]), r["status"])
			}
			return tw.Flush()
		},
	}
}

func remaining(limit, spent int) int {
	if limit <= 0 {
		return 0
	}
	r := limit - spent
	if r < 0 {
		return 0
	}
	return r
}

// hoursSinceMonthStart returns the number of hours from the start of the
// current calendar month (UTC) to now.
func hoursSinceMonthStart() int {
	h := int(time.Since(monthStart()).Hours()) + 1
	if h < 1 {
		return 1
	}
	return h
}

func monthStart() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// parseSinceHours accepts "24h", "7d", "30d", "all" (or "0"). Anything else
// defaults to 30 days.
func parseSinceHours(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "all" || s == "0" || s == "" {
		return 0
	}
	if strings.HasSuffix(s, "h") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err == nil && n > 0 {
			return n
		}
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil && n > 0 {
			return n * 24
		}
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return 24 * 30
}

// --- budget limit persistence ---

type budgetFile struct {
	LimitCredits int       `json:"limit_credits"`
	ResetDate    time.Time `json:"reset_date"`
}

func budgetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "contact-goat-pp-cli", "budget.json")
}

func loadBudgetLimit() int {
	data, err := os.ReadFile(budgetConfigPath())
	if err != nil {
		return 0
	}
	var f budgetFile
	if err := json.Unmarshal(data, &f); err != nil {
		return 0
	}
	return f.LimitCredits
}

func saveBudgetLimit(n int) error {
	path := budgetConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(budgetFile{LimitCredits: n, ResetDate: monthStart()}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
