// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (do not re-stub on regen).

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelIssuesDupesCmd(flags *rootFlags) *cobra.Command {
	var flagLimit, flagMaxPages int
	var flagRepo string
	var flagRefresh bool

	cmd := &cobra.Command{
		Use:   "dupes <term>",
		Short: "Find open issues whose title and body match a term, ranked by relevance, so you spot duplicates before re-triaging.",
		Long: `Find likely-duplicate issues by full-text-matching a term against synced issue
titles and bodies, ranked by relevance (local SQLite FTS5, offline after the
first populate — never spends the GitHub search rate limit).

Pass --repo owner/repo the first time to populate the local mirror; later runs
reuse it offline. --refresh re-fetches.`,
		Example:     "  github-pp-cli issues dupes \"panic on nil map\" --repo cli/cli --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			term := args[0]

			st, err := openNovelStoreRW(cmd)
			if err != nil {
				return err
			}
			defer st.Close()
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if err := nvEnsurePopulated(ctx, cmd, flags, st, flagRepo, flagRefresh, flagMaxPages, "issues"); err != nil {
				return err
			}

			matches, err := st.Search(term, nvCount(st, "issues"), "issues")
			if err != nil {
				return fmt.Errorf("searching synced issues: %w", err)
			}
			rows := nvOpenIssueRows(matches, flagLimit)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(rows) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "no synced issues match %q (try --repo owner/repo to populate)\n", term)
				}
				tr := make([][]string, 0, len(rows))
				for _, r := range rows {
					tr = append(tr, []string{"#" + strconv.FormatInt(r.Number, 10), r.State, r.Title})
				}
				return flags.printTable(cmd, []string{"ISSUE", "STATE", "TITLE"}, tr)
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum matching issues to return")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "owner/repo to populate the local mirror from (inferred from prior syncs when omitted)")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Re-fetch issues even if the local mirror already has them")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 3, "Maximum pages (100/page) to fetch when populating")
	return cmd
}

func nvOpenIssueRows(matches []json.RawMessage, limit int) []issueRow {
	if limit <= 0 {
		limit = 50
	}
	rows := make([]issueRow, 0, min(limit, len(matches)))
	for _, raw := range matches {
		row := nvIssueRow(raw)
		if row.State != "open" {
			continue
		}
		rows = append(rows, row)
		if len(rows) == limit {
			break
		}
	}
	return rows
}
