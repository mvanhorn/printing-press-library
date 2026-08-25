// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command — NOT generator output.
// pp:data-source local
//
// quote surfaces the verbatim pull-quote (and speaker) for matching snips — the
// distinct unit Readwise concatenates into transcript prose and loses. It ranks
// snips by full-text relevance, then returns only those that carry a quote.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/snipd"
)

type quoteRow struct {
	SnipID       string `json:"snip_id"`
	Quote        string `json:"quote"`
	Speaker      string `json:"speaker,omitempty"`
	Title        string `json:"title"`
	Show         string `json:"show"`
	EpisodeTitle string `json:"episode_title"`
	Start        string `json:"start,omitempty"`
	URL          string `json:"url"`
}

func newNovelQuoteCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var snipID string

	cmd := &cobra.Command{
		Use:   "quote <query>",
		Short: "Get the verbatim pull-quote and speaker for matching snips — the distinct unit other tools concatenate into transcript prose and lose.",
		Long: "Return the verbatim pull-quote and speaker for snips matching a query, or\n" +
			"for a specific snip via --snip. Use this when you need a citable line, not a\n" +
			"paraphrase or the whole transcript. Only snips that carry a quote are returned.",
		Example:     "  snipd-pp-cli quote \"thought partner\" --agent\n  snipd-pp-cli quote --snip 79b70845-023e-40f3-a68f-6e03ae9c0bd6",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" && snipID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query or --snip <id> is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			st, dbResolved, err := openCorpusForRead(ctx, dbPath)
			if err != nil {
				return err
			}
			if st == nil {
				return emitNoCorpus(cmd, flags, dbResolved)
			}
			defer st.Close()

			var snips []snipd.Snip
			if snipID != "" {
				got, err := querySnips(ctx, st, "id = ?", "1", []any{snipID}, 1)
				if err != nil {
					return err
				}
				snips = got
			} else {
				raws, err := st.Search(query, limit*3, "snips") // over-fetch: some hits have no quote
				if err != nil {
					return err
				}
				snips = unmarshalSnips(raws)
			}

			rows := make([]quoteRow, 0, len(snips))
			for _, s := range snips {
				if strings.TrimSpace(s.Quote) == "" {
					continue
				}
				rows = append(rows, quoteRow{
					SnipID: s.SnipID, Quote: s.Quote, Speaker: s.Speaker,
					Title: s.Title, Show: s.Show, EpisodeTitle: s.EpisodeTitle,
					Start: s.Start, URL: s.URL,
				})
				if limit > 0 && len(rows) >= limit {
					break
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if rows == nil {
					rows = []quoteRow{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			w := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(w, "no matching snips with a quote")
				return nil
			}
			for _, r := range rows {
				fmt.Fprintf(w, "“%s”\n", r.Quote)
				attr := r.Speaker
				if attr == "" {
					attr = r.EpisodeTitle
				}
				fmt.Fprintf(w, "  — %s · %s\n  %s\n\n", attr, r.Show, r.URL)
			}
			fmt.Fprintf(w, "%d quote(s)\n", len(rows))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path (default: per-user data dir)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum quotes to return")
	cmd.Flags().StringVar(&snipID, "snip", "", "Return the quote for a specific snip id")
	return cmd
}
