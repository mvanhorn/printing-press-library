// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
// This command computes results from locally stored history (resource_snapshots)
// built up as the user browses; it does not read a single upstream resource type.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type authorEntry struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func newNovelAuthorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "author <org>",
		Short:       "See everything one GitHub org has published across servers, skills, and clients in one view.",
		Example:     "  mcpmarket-pp-cli author mendableai --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "org=mendableai", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "author")
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("org is required"))
			}
			org := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("mcpmarket-pp-cli")
			db, err := storeOpenForNovel(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			results := make([]authorEntry, 0)
			for _, resourceType := range []string{"server", "skill", "mcpclient"} {
				rows, err := db.List(resourceType, 0)
				if err != nil {
					continue
				}
				for _, data := range rows {
					var entity struct {
						Name   string `json:"name"`
						URL    string `json:"url"`
						Author struct {
							Name string `json:"name"`
							URL  string `json:"url"`
						} `json:"author"`
					}
					if err := json.Unmarshal(data, &entity); err != nil {
						continue
					}
					if !strings.EqualFold(entity.Author.Name, org) {
						continue
					}
					results = append(results, authorEntry{
						Type: resourceType,
						ID:   slugFromMCPMarketURL(entity.URL),
						Name: entity.Name,
						URL:  entity.URL,
					})
				}
			}
			if len(results) == 0 {
				// A local-only lookup can't tell "invalid org" from "valid org,
				// nothing browsed into the local mirror yet" — both look like
				// zero matches. Treat as a valid empty result (exit 0), not an
				// error; see the pp:no-error-path-probe annotation above.
				note := fmt.Sprintf("no locally-mirrored entries found for author %q. This searches only what has been browsed into the local store — run `mcpmarket-pp-cli server list`, `skill list`, or `server search` first to widen coverage.", org)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"author": org, "entries": results, "note": note}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"author": org, "entries": results}, flags)
		},
	}
	return cmd
}
