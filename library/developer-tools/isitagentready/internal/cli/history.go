// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newNovelHistoryCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var checkID string

	cmd := &cobra.Command{
		Use:   "history <url>",
		Short: "Show a site's readiness level across past scans and which checks flipped",
		Long: "Read a site's stored scan history (oldest to newest) and print its readiness level over\n" +
			"time, flagging which checks flipped pass/fail between consecutive scans. Reads local data\n" +
			"only; run 'check <url>' more than once to build a timeline. Narrow flips with --check.",
		Example: "  isitagentready-pp-cli history https://example.com\n" +
			"  isitagentready-pp-cli history https://example.com --check mcpServerCard --agent",
		// An "invalid" URL still has stored siteError scans to show (exit 0), so
		// there is no bad-input error path to probe; skip it (see check.go).
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show the scan history for a URL")
				return nil
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("history reads local scan history only; it has no live equivalent (remove --data-source live)"))
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a URL argument is required, e.g. history https://example.com"))
			}
			url := args[0]

			recs, err := loadStore()
			if err != nil {
				return err
			}
			hist := store.HistoryFor(recs, url)
			if len(hist) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no scans yet for %s; run: isitagentready-pp-cli check %s\n", url, url)
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			entries := store.BuildHistory(hist, checkID)
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			return renderHistory(cmd, url, entries)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Show only the most recent N scans (0 = all)")
	cmd.Flags().StringVar(&checkID, "check", "", "Only report flips for a single check id")
	return cmd
}

func renderHistory(cmd *cobra.Command, url string, entries []store.HistoryEntry) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s — %d scan(s)\n", bold(url), len(entries))
	for _, e := range entries {
		when := e.ScannedAt
		if t := cliutil.ParseStoredTime(e.ScannedAt); !t.IsZero() {
			when = t.Format("2006-01-02 15:04")
		}
		if e.SiteError {
			fmt.Fprintf(out, "  %s  level %d — %s  %s\n", when, e.Level, e.LevelName, red("(site error)"))
		} else {
			fmt.Fprintf(out, "  %s  level %d — %s\n", when, e.Level, e.LevelName)
		}
		for _, f := range e.Flips {
			mark := yellow(f.Change)
			if f.Change == "regressed" {
				mark = red(f.Change)
			} else if f.Change == "improved" || f.Change == "added" {
				mark = green(f.Change)
			}
			fmt.Fprintf(out, "      %s %s (%s → %s)\n", mark, f.Check, f.From, f.To)
		}
	}
	return nil
}
