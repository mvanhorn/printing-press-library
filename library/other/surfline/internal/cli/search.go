// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: offline spot lookup over the local store. Matches
// names/IDs of spots you have captured with `journal log`, with no network. For
// discovering new spots online use `spots find`.
//
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Resolve spot names to spotIds offline, over spots you've captured with `journal log`.",
		Long: "Searches the local store for spots whose name or ID matches the query, with no network.\n" +
			"The index is built from spots you capture with 'journal log'.\n\n" +
			"Use this command for offline name lookup. To discover new spots online use 'spots find'.",
		Example: strings.Trim(`
  surfline-pp-cli search "Pleasure Point"
  surfline-pp-cli search Trestles --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			query := strings.Join(args, " ")
			resolved := dbPath
			if resolved == "" {
				resolved = defaultDBPath(surflineDBName)
			}
			if _, statErr := os.Stat(resolved); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local store yet; capture spots with 'surfline-pp-cli journal log <spotId>', or look up online: surfline-pp-cli spots find %q\n",
					query)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openSurflineStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			matches, err := searchJournaledSpots(ctx, db, query, limit)
			if err != nil {
				return fmt.Errorf("searching local store: %w", err)
			}
			if matches == nil {
				matches = []spotMatch{}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(),
					"no captured spots match %q; look up online: surfline-pp-cli spots find %q\n", query, query)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SPOT\tSPOT_ID\tSNAPSHOTS\tLAST_LOGGED")
			for _, m := range matches {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n",
					truncate(firstNonEmpty(m.SpotName, "(unnamed)"), 28), m.SpotID, m.Snapshots, localTime(m.LastLogged, 0, "2006-01-02 15:04"))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/surfline-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}
