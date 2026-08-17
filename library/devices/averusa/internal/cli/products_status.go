// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: products status — flag models AVer lists as discontinued,
// filterable by category. Current/discontinued status is unstructured HTML on
// the /support/ page; only sync-time parsing into the corpus makes it queryable.
// pp:data-source local
// Local-store command: reads the harvested product catalog only.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelProductsStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var flagCategory string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Flag which models AVer lists as discontinued, filterable by category.",
		Long: strings.Trim(`
List products that AVer marks as discontinued (from the /support/ download
center's "Discontinued Devices" sections), optionally filtered by category.
Use this before specing a model into a bid so a retired unit never ships.

The discontinued roster includes models no longer in the live catalog; those
appear with category "discontinued".
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli products status
  averusa-pp-cli products status --category conference-camera --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--category=conference-camera",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "products status")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("--data-source live has no live equivalent for products status; it reads the corpus after `harvest`"))
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("products status takes no positional args; use --category"))
			}
			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				return notFoundErr(fmt.Errorf("no corpus for products status"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			prods, err := st.ListAVERUSAProducts(flagCategory, 500)
			if err != nil {
				return err
			}
			var disc []struct {
				Slug     string `json:"slug"`
				Name     string `json:"name"`
				Category string `json:"category"`
			}
			for _, p := range prods {
				if p.Discontinued {
					disc = append(disc, struct {
						Slug     string `json:"slug"`
						Name     string `json:"name"`
						Category string `json:"category"`
					}{p.Slug, p.Name, p.Category})
				}
			}
			if len(disc) == 0 {
				return notFoundErr(fmt.Errorf("no discontinued products%s in the corpus", map[bool]string{true: " for category " + flagCategory, false: ""}[flagCategory != ""]))
			}
			if flags.asJSON {
				return flags.printJSON(cmd, disc)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d discontinued product(s):\n", len(disc))
			for _, d := range disc {
				fmt.Fprintf(w, "  %-14s %-24s %s\n", d.Slug, d.Name, d.Category)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	cmd.Flags().StringVar(&flagCategory, "category", "", "only products in this category (e.g. conference-camera)")
	return cmd
}
