// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: populate the local SQLite mirror (valuations, articles, cards).
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var resources string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror valuations, articles, and cards into the local store for offline use",
		Long: strings.TrimSpace(`
Populate the local SQLite mirror so offline search and the transcendence
commands (worth, portfolio, valuations drift, since) have data. By default all
resources are synced; narrow with --resources valuations,articles,cards.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli sync
  thepointsguy-pp-cli sync --resources valuations,articles
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync the local mirror")
				return nil
			}
			want := map[string]bool{}
			if strings.TrimSpace(resources) == "" {
				want[rtValuations], want[rtArticles], want[rtCards] = true, true, true
			} else {
				for _, r := range strings.Split(resources, ",") {
					want[strings.TrimSpace(r)] = true
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)
			st, err := openTPGStore()
			if err != nil {
				return err
			}
			defer st.Close()

			counts := map[string]int{}
			var firstErr error
			recordErr := func(res string, e error) {
				fmt.Fprintf(cmd.ErrOrStderr(), "sync %s: %v\n", res, e)
				if firstErr == nil {
					firstErr = e
				}
			}

			if want[rtValuations] {
				vals, _, err := c.Valuations(ctx)
				if err != nil {
					recordErr(rtValuations, err)
				} else {
					persistValuations(st, vals)
					counts[rtValuations] = len(vals)
				}
			}
			if want[rtArticles] {
				items, err := c.Latest(ctx)
				if err != nil {
					recordErr(rtArticles, err)
				} else {
					n := 0
					for _, it := range items {
						id := it.Link
						if id == "" {
							continue
						}
						data, merr := json.Marshal(it)
						if merr != nil {
							continue
						}
						if st.Upsert(rtArticles, id, data) == nil {
							n++
						}
					}
					counts[rtArticles] = n
				}
			}
			if want[rtCards] {
				slugs, err := c.CardSlugs(ctx, false)
				if err != nil {
					recordErr(rtCards, err)
				} else {
					n := 0
					for _, slug := range slugs {
						data, _ := json.Marshal(map[string]string{
							"slug": slug,
							"url":  tpg.BaseURL + "/credit-cards/" + slug + "/",
						})
						if st.Upsert(rtCards, slug, data) == nil {
							n++
						}
					}
					counts[rtCards] = n
				}
			}

			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, struct {
					Synced map[string]int `json:"synced"`
				}{counts})
			}
			for _, r := range []string{rtValuations, rtArticles, rtCards} {
				if want[r] {
					fmt.Fprintf(cmd.OutOrStdout(), "synced %d %s\n", counts[r], r)
				}
			}
			return firstErr
		},
	}
	cmd.Flags().StringVar(&resources, "resources", "", "Comma-separated resources to sync: valuations,articles,cards (default all)")
	return cmd
}
