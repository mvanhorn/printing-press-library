// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
	"github.com/spf13/cobra"
)

func newNovelHooksCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "hooks [page-or-id]",
		Short: "Extract, deduplicate, and rank opening copy hooks, headlines, and CTAs weighted by collation count and longevity.",
		Long: `Use this command to extract and rank deduplicated opening copy hooks, headlines, and CTAs weighted by collation count and longevity.
Do NOT use this command to list individual winning ad records (use 'winners') or analyze destination URLs and UTMs (use 'funnels').`,
		Example: strings.Trim(`
  meta-ads-library-pp-cli hooks 15087023444 --agent
  meta-ads-library-pp-cli hooks --limit 20 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hooks")
			}

			pageFilter := ""
			if len(args) > 0 {
				pageFilter = strings.TrimSpace(args[0])
			}

			resolvedDB := dbPath
			if resolvedDB == "" {
				resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
			}
			if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: meta-ads-library-pp-cli ads search --query nike --country US\n", resolvedDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]adlibrary.HookRow, 0), flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(cmd.Context(), resolvedDB)
			if err != nil {
				return err
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "ads") {
				hintIfStale(cmd, db, "ads", flags.maxAge)
			}

			rows, err := adlibrary.ComputeHooks(cmd.Context(), db, pageFilter, limit)
			if err != nil {
				return err
			}
			if pageFilter != "" && len(rows) == 0 {
				nf := notFoundErr(fmt.Errorf("no opening hooks found for %q in local SQLite; run 'meta-ads-library-pp-cli ads search --page-name %q --country US' first", pageFilter, pageFilter))
				writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, nf, ExitCode(nf))
				return nf
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No opening hooks found in local SQLite.")
				return nil
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				hook := truncate(r.OpeningHook, 65)
				table = append(table, []string{
					fmt.Sprintf("#%d", r.Rank),
					fmt.Sprintf("%d", r.AdCount),
					fmt.Sprintf("%dx", r.TotalCollations),
					fmt.Sprintf("%dd", r.MaxDaysRunning),
					r.TopCTA,
					hook,
				})
			}
			return flags.printTable(cmd, []string{"RANK", "ADS", "COLL", "MAX DAYS", "TOP CTA", "OPENING HOOK"}, table)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum ranked opening hooks to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
