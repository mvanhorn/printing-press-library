// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMessagesStatsCmd(f *rootFlags) *cobra.Command {
	var topHandles int
	var includeTapbacks bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Summary counts and breakdowns across your iMessage history",
		Example: `  icloud-pp-cli messages stats
  icloud-pp-cli messages stats --top-handles 5
  icloud-pp-cli messages stats --agent | jq .`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openMessagesDB(f.messagesDBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			totals, err := statsTotals(db, includeTapbacks)
			if err != nil {
				return err
			}
			byYear, err := statsByYear(db, includeTapbacks)
			if err != nil {
				return err
			}
			byHandle, err := statsByHandle(db, topHandles, includeTapbacks)
			if err != nil {
				return err
			}

			if f.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printMessagesStatsJSON(cmd, f, totals, byYear, byHandle)
			}
			return printMessagesStatsTable(cmd, f, totals, byYear, byHandle)
		},
	}

	cmd.Flags().IntVar(&topHandles, "top-handles", 10, "Number of top handles to include in the breakdown")
	cmd.Flags().BoolVar(&includeTapbacks, "include-tapbacks", false, "Count tapback rows in totals (default: excluded)")

	return cmd
}

type messagesStatsJSON struct {
	TotalMessages int64           `json:"total_messages"`
	TotalChats    int64           `json:"total_chats"`
	TotalHandles  int64           `json:"total_handles"`
	ByYear        []YearStats     `json:"by_year,omitempty"`
	TopHandles    []HandleStats   `json:"top_handles,omitempty"`
}

func printMessagesStatsJSON(cmd *cobra.Command, f *rootFlags, t MessagesTotals, byYear []YearStats, byHandle []HandleStats) error {
	row := messagesStatsJSON{
		TotalMessages: t.TotalMessages,
		TotalChats:    t.TotalChats,
		TotalHandles:  t.TotalHandles,
	}
	if !f.compact {
		row.ByYear = byYear
		row.TopHandles = byHandle
	}
	return printJSON(cmd.OutOrStdout(), row)
}

func printMessagesStatsTable(cmd *cobra.Command, f *rootFlags, t MessagesTotals, byYear []YearStats, byHandle []HandleStats) error {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "%s  %d messages · %d chats · %d handles\n",
		bold(f, out, "Messages corpus"),
		t.TotalMessages, t.TotalChats, t.TotalHandles,
	)
	fmt.Fprintln(out)

	if len(byYear) > 0 {
		fmt.Fprintln(out, bold(f, out, "By year"))
		w := newTabWriter(out)
		for _, y := range byYear {
			fmt.Fprintf(w, "  %s\t%d messages\n", y.Year, y.MessageCount)
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Fprintln(out)
	}

	if len(byHandle) > 0 {
		fmt.Fprintln(out, bold(f, out, "Top handles"))
		w := newTabWriter(out)
		for _, h := range byHandle {
			fmt.Fprintf(w, "  %s\t%d messages\n", h.Handle, h.MessageCount)
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return nil
}
