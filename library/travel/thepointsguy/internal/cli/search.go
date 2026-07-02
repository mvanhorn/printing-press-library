// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: full-text search over The Points Guy content (Algolia),
// with an offline FTS fallback over synced articles.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var suggestions bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search The Points Guy articles, news, guides, and card reviews",
		Long: strings.TrimSpace(`
Full-text search over The Points Guy content via its live Algolia index. With
--data-source local, or when the live index is unreachable, this falls back to
an offline search over articles you have synced into the local store.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli search "amex platinum" --limit 5
  thepointsguy-pp-cli search "lounge access" --agent --select hits.title,hits.url
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search The Points Guy")
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			if limit <= 0 {
				limit = 10
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			index := tpg.IndexContent
			if suggestions {
				index = tpg.IndexSuggestions
			}

			if flags.dataSource != "local" {
				c := newTPGClient(flags)
				res, err := c.Search(ctx, index, query, limit)
				if err == nil {
					return emitSearch(cmd, flags, res)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "live search unavailable (%v); trying local store\n", err)
			}
			return searchLocal(cmd, flags, query, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results to return")
	cmd.Flags().BoolVar(&suggestions, "suggestions", false, "Search the query-suggestions index instead of content")
	return cmd
}

func emitSearch(cmd *cobra.Command, flags *rootFlags, res *tpg.SearchResult) error {
	if flags.asJSON || flags.agent {
		return emitJSON(cmd, flags, res)
	}
	w := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintf(w, "TITLE\tCATEGORY\tURL\n")
	for _, h := range res.Hits {
		fmt.Fprintf(w, "%s\t%s\t%s\n", truncRunes(h.Title, 60), h.Category, h.URL)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "\n%d total matches for %q\n", res.NbHits, res.Query)
	return nil
}

func searchLocal(cmd *cobra.Command, flags *rootFlags, query string, limit int) error {
	st, err := openTPGStore()
	if err != nil {
		return err
	}
	defer st.Close()
	raws, err := st.Search(query, limit, rtArticles)
	if err != nil {
		return err
	}
	if len(raws) == 0 {
		missingMirrorHint(cmd, flags, rtArticles)
		return nil
	}
	hits := make([]tpg.SearchHit, 0, len(raws))
	for _, r := range raws {
		var h tpg.SearchHit
		if json.Unmarshal(r, &h) == nil {
			hits = append(hits, h)
		}
	}
	return emitSearch(cmd, flags, &tpg.SearchResult{Query: query, NbHits: len(hits), Hits: hits})
}
