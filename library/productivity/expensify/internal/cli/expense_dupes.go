// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `expense dupes` — heuristic duplicate detection on the local store.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

// parseDayWindow accepts a bare integer (days) or a short duration suffix
// like "3d", "2w", "1m" and returns a day count. Examples:
//
//	"3"  -> 3
//	"3d" -> 3
//	"2w" -> 14
//	"1m" -> 30
func parseDayWindow(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty window value")
	}
	mult := 1
	switch {
	case strings.HasSuffix(s, "d"):
		s = strings.TrimSuffix(s, "d")
	case strings.HasSuffix(s, "w"):
		s = strings.TrimSuffix(s, "w")
		mult = 7
	case strings.HasSuffix(s, "m"):
		s = strings.TrimSuffix(s, "m")
		mult = 30
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid window %q (use N, Nd, Nw, or Nm)", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("window must be non-negative")
	}
	return n * mult, nil
}

func newExpenseDupesCmd(flags *rootFlags) *cobra.Command {
	var windowSpec string
	cmd := &cobra.Command{
		Use:   "dupes",
		Short: "Find suspected duplicate expenses in the local store",
		Long: `Clusters expenses sharing the same merchant and amount when their dates are
within --window days. Useful before submitting a report.

--window accepts a bare integer (days) or a short suffix: 3, 3d, 2w, 1m.`,
		Example: "  expensify-pp-cli expense dupes\n  expensify-pp-cli expense dupes --window 3d\n  expensify-pp-cli expense dupes --window 2w --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			window, err := parseDayWindow(windowSpec)
			if err != nil {
				return configErr(err)
			}
			st, err := store.Open("")
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			groups, err := st.Dupes(window)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"report":      "dupes",
					"window_days": window,
					"count":       len(groups),
					"groups":      groups,
				})
			}
			w := cmd.OutOrStdout()
			if len(groups) == 0 {
				fmt.Fprintf(w, "dupes: clean — no suspected duplicates within %d-day window.\n", window)
				return nil
			}
			fmt.Fprintf(w, "dupes within %d-day window:\n", window)
			for i, g := range groups {
				fmt.Fprintf(w, "Group %d: %s (%.2f) — %d expenses\n", i+1, g.Merchant, float64(g.Amount)/100, len(g.Expenses))
				for _, e := range g.Expenses {
					fmt.Fprintf(w, "  %s  %s  %s\n", e.Date, e.TransactionID, e.Category)
				}
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "dupes: %d group(s) flagged within %d-day window.\n", len(groups), window)
			return nil
		},
	}
	cmd.Flags().StringVar(&windowSpec, "window", "3", "Day window for merchant+amount matches (N, Nd, Nw, or Nm)")
	return cmd
}
