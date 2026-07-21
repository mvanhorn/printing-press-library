// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: audit. Reads the local write journal — the accountability
// trail the Robinhood Agentic MCP server does not keep. Store-only; never
// touches the network.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"
	"github.com/spf13/cobra"
)

func newNovelAuditCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagDenied bool
	var flagTool string
	var flagPlaced bool

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "See everything the CLI (or an agent driving it) reviewed, placed, canceled",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Audit prints the local write journal: every mutating tool call this CLI
attempted (order reviews, placements, cancels, watchlist edits) and its
outcome. The server keeps no such trail — this local journal is the only
accountability record of what an agent did with the account.

Entries are newest first. Filter by time window (--since), by outcome
(--denied for guard-blocked attempts, --placed for placed orders), or by
tool name (--tool).`,
		Example: `  # Everything in the default 30-day window
  robinhood-agentic-pp-cli audit

  # Guard-blocked attempts in the last week
  robinhood-agentic-pp-cli audit --since 7d --denied

  # Orders actually placed via a specific tool
  robinhood-agentic-pp-cli audit --placed --tool place_equity_order`,
		RunE: func(cmd *cobra.Command, args []string) error {
			since, err := parseSince(flagSince)
			if err != nil {
				return usageErr(err)
			}
			if flagDenied && flagPlaced {
				return usageErr(fmt.Errorf("--denied and --placed are mutually exclusive"))
			}
			if dryRunOK(flags) {
				return nil
			}

			// Read-only open: audit only reads the local write journal, so it
			// must not take the write lock (which would block behind a
			// concurrent sync) or run a migration. A nil store means no local
			// DB exists yet — an honest empty journal.
			st, err := openStoreForRead(cmd.Context(), "robinhood-agentic-pp-cli")
			if err != nil {
				return err
			}
			var entries []store.WriteJournalEntry
			if st != nil {
				defer st.Close()
				entries, err = st.WriteJournal(buildAuditFilter(since, flagDenied, flagPlaced, flagTool))
				if err != nil {
					return err
				}
			}
			if entries == nil {
				entries = []store.WriteJournalEntry{}
			}

			out := struct {
				Since   string                    `json:"since"`
				Count   int                       `json:"count"`
				Entries []store.WriteJournalEntry `json:"entries"`
			}{
				Since:   since.UTC().Format(time.RFC3339),
				Count:   len(entries),
				Entries: entries,
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, out, flags)
			}

			if len(entries) == 0 {
				fmt.Fprintln(w, "no mutations recorded in this window")
				return nil
			}
			fmt.Fprintf(w, "%d mutation(s) since %s\n\n", len(entries), since.Local().Format("2006-01-02 15:04"))
			fmt.Fprintf(w, "%-16s  %-26s  %-10s  %-8s  %s\n", "TIMESTAMP", "TOOL", "ACTION", "SYMBOL", "OUTCOME")
			for _, e := range entries {
				fmt.Fprintf(w, "%-16s  %-26s  %-10s  %-8s  %s\n",
					e.Timestamp.Local().Format("2006-01-02 15:04"),
					orDash(e.Tool), orDash(e.Action), orDash(e.Symbol), orDash(e.Outcome))
				if strings.HasPrefix(e.Outcome, "blocked") && e.Detail != "" {
					fmt.Fprintf(w, "%-16s  └ %s\n", "", e.Detail)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "", "Window start: 30d, 12h, 2w, or 2026-07-01 (default 30d)")
	cmd.Flags().BoolVar(&flagDenied, "denied", false, "Only guard-blocked attempts (outcome prefix \"blocked\")")
	cmd.Flags().StringVar(&flagTool, "tool", "", "Only entries from this tool (e.g. place_equity_order)")
	cmd.Flags().BoolVar(&flagPlaced, "placed", false, "Only placed orders (outcome \"placed\")")
	return cmd
}

// buildAuditFilter maps the audit command's flags onto a write-journal query
// filter. Pure function, extracted for testing: --denied selects the
// "blocked" outcome prefix (blocked:kill-switch, blocked:denylist, ...),
// --placed selects the exact "placed" outcome, and --tool narrows to one tool.
func buildAuditFilter(since time.Time, denied, placed bool, tool string) store.WriteJournalFilter {
	f := store.WriteJournalFilter{Since: since}
	if denied {
		f.OutcomePfx = "blocked"
	}
	if placed {
		f.Outcome = "placed"
	}
	if tool != "" {
		f.Tool = tool
	}
	return f
}

// orDash keeps empty journal columns visibly aligned in the human table.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
