// Hand-authored: novel feature `grep` — FTS5 search over the local
// actions ledger. Created during Phase 3 build; survives regen because
// it lives in its own file.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"

	"github.com/spf13/cobra"
)

func newGrepCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var computer string
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:   "grep <query>",
		Short: "FTS5 search over historical bash commands, Python exec code, and click coordinates from the local actions store.",
		Long: `Full-text search over the local actions ledger.

The query is passed straight to SQLite's FTS5 MATCH operator, so SQLite
boolean syntax works: 'pip install', 'python AND error', '"rm -rf"'.
Filters narrow by kind (bash/exec/click/screenshot/key) and computer.`,
		Example: strings.Trim(`
  # Find every bash command containing "pip install" in the last 30 days
  orgo-pp-cli grep "pip install" --type bash --since 30d

  # Search across every kind for the last 24 hours
  orgo-pp-cli grep "rm -rf" --since 24h

  # Limit to a single computer
  orgo-pp-cli grep "screenshot" --computer abc123 --type screenshot
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			query := args[0]
			filter := store.ActionFilter{
				ComputerID: computer,
				Search:     ftsQuote(query),
				Limit:      limit,
			}
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
				filter.Since = ts
			}
			if kind != "" && kind != "all" {
				filter.Kinds = []string{kind}
			}

			dbPath := defaultDBPath("orgo-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			actions, err := db.QueryActions(cmd.Context(), filter)
			if err != nil {
				return fmt.Errorf("searching actions: %w", err)
			}

			// Reverse to chronological for parity with `audit`.
			reversed := make([]store.Action, len(actions))
			for i, a := range actions {
				reversed[len(actions)-1-i] = a
			}

			if len(reversed) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No matches in the local actions ledger. Widen --since, drop --type, or check that you've run actions through the CLI first.")
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				return nil
			}

			rows := make([]map[string]any, 0, len(reversed))
			for _, a := range reversed {
				rows = append(rows, map[string]any{
					"ts":               a.Timestamp.UTC().Format(time.RFC3339),
					"computer_id":      truncate(a.ComputerID, 8),
					"computer_id_full": a.ComputerID,
					"kind":             a.Kind,
					"summary":          truncate(a.Command, 80),
					"command":          a.Command,
					"args":             a.ArgsJSON,
				})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, bold("TS")+"\t"+bold("COMPUTER_ID")+"\t"+bold("KIND")+"\t"+bold("SUMMARY"))
			for _, r := range rows {
				fmt.Fprintf(tw, "%v\t%v\t%v\t%v\n", r["ts"], r["computer_id"], r["kind"], r["summary"])
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&kind, "type", "all", "Restrict to a single kind: bash|exec|click|screenshot|key|all")
	cmd.Flags().StringVar(&computer, "computer", "", "Filter by computer ID")
	cmd.Flags().StringVar(&since, "since", "30d", "Relative time window (e.g. 30m, 24h, 7d, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum number of matches to return")

	return cmd
}

// ftsQuote wraps a user query as an FTS5 literal phrase unless it already
// looks like a hand-crafted FTS5 boolean expression (starts with a quote
// or uses AND/OR/NOT/NEAR/+/-/^). Phrase wrapping with internal double-
// quote escaping is robust against apostrophes, hyphens, asterisks, and
// the rest of FTS5's reserved set — those characters are common in
// shell history but make FTS5's tokenizer fail with "syntax error".
func ftsQuote(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return q
	}
	// Already a hand-written FTS5 expression: leave it alone.
	if strings.HasPrefix(q, "\"") || strings.HasPrefix(q, "(") {
		return q
	}
	// FTS5 phrase quoting: wrap in double-quotes; double internal quotes.
	return "\"" + strings.ReplaceAll(q, "\"", "\"\"") + "\""
}
