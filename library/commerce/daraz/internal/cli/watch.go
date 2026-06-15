// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Safe to edit.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type watchOut struct {
	Query         string `json:"query"`
	ItemsRecorded int    `json:"itemsRecorded"`
	TotalResults  int    `json:"totalResults"`
	Message       string `json:"message"`
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var maxScan int

	cmd := &cobra.Command{
		Use:         "watch <query>",
		Short:       "Record current prices for every item matching a search into the local store so price history builds over time.",
		Long:        "Record current prices for every item matching a search into the local store so price history builds over time.\n\nRun periodically on a query you care about; then 'price-history <itemId>' shows the trend for any captured item, and 'since <query>' shows what changed.",
		Example: "  daraz-pp-cli watch \"gaming laptop\"",
		// watch is write-primary: its purpose is to record prices into the
		// local store, so it is intentionally NOT marked mcp:read-only.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required, e.g. watch \"gaming laptop\""))
			}
			query := strings.Join(args, " ")
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			items, total, _, err := scanSearch(ctx, c, query, "", "", maxScan, 0)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			s, err := openDarazStore(ctx, flags)
			if err != nil {
				return err
			}
			defer s.Close()
			recordProducts(ctx, s, items)
			recordSearchSnapshot(ctx, s, query, items)
			out := watchOut{
				Query:         query,
				ItemsRecorded: len(items),
				TotalResults:  total,
				Message:       fmt.Sprintf("Recorded %d items for %q. Use 'price-history <itemId>' for any item, or 'since %q' later to see changes.", len(items), query, query),
			}
			return emitDaraz(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&maxScan, "max-scan-pages", 2, "maximum search pages to record (40 items per page)")
	return cmd
}
