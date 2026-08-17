// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/registrydb"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search the offline registry by owner, co-owner, manufacturer, or model",
		Long: `Full-text search over the local registry database: registrant names,
co-owner names, manufacturers, and models. Matching is prefix-based per word,
so partial names work ("netj" finds NETJETS entries). Requires a prior sync.`,
		Example:     "  faa-registry-pp-cli search \"NETJETS\"\n  faa-registry-pp-cli search \"GULFSTREAM 650\" --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			emitRegistryStaleHint(cmd, db, flags)
			res, err := db.Search(cmd.Context(), strings.Join(args, " "), limit)
			if err != nil {
				return err
			}
			if res == nil {
				res = []registrydb.SearchResult{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	return cmd
}
