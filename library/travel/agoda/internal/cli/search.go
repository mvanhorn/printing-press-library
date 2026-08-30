// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/store"
)

type corpusResult struct {
	Query      string      `json:"query"`
	Matches    int         `json:"matches"`
	CorpusSize int         `json:"corpus_size"`
	Results    []corpusRow `json:"results"`
	Note       string      `json:"note,omitempty"`
}

// newAgodaSearchCmd provides offline search over properties this CLI has seen.
//
// Agoda has no bulk catalog endpoint, so there is nothing to sync up front. The
// corpus is instead built from live searches the user already ran, which makes
// this a genuine local-state feature rather than a cache of one fetch.
func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search locally cached properties offline, with no network call",
		Long: `Full-text search over every property this CLI has already seen.

Agoda exposes no bulk catalog endpoint, so the local corpus is built up as a
side effect of ordinary searching: each 'hotels search', 'hotels rank', and
'hotels fees' run records the properties it saw. Over time this becomes a
cross-destination corpus that answers questions without touching the network.

Matching is FTS5 over property name and address, ranked by BM25 then review
volume.`,
		Example: "  agoda-pp-cli search \"shinjuku\" --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "query=hotel",
			// An empty corpus is a normal first-run state, not a failure.
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("agoda-pp-cli")
			}
			out := corpusResult{Query: query, Results: make([]corpusRow, 0)}

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local property corpus at %s\nrun a live search first: agoda-pp-cli hotels search <destination>\n",
					dbPath)
				out.Note = "local corpus is empty; run 'agoda-pp-cli hotels search <destination>' to populate it"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				return nil
			}
			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer st.Close()
			if err := store.EnsureAgodaSchema(ctx, st.DB()); err != nil {
				return err
			}
			_ = st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM properties`).Scan(&out.CorpusSize)

			rows, err := queryCorpus(ctx, st.DB(), ftsQuery(query), limit)
			if err != nil {
				return fmt.Errorf("searching local corpus: %w", err)
			}
			out.Results = rows
			out.Matches = len(rows)
			if len(rows) == 0 {
				out.Note = fmt.Sprintf(
					"no local match for %q across %d cached properties; run more live searches to widen the corpus",
					query, out.CorpusSize)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAgodaJSON(cmd.OutOrStdout(), out, flags, "local")
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out.Note)
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PROPERTY\tSTARS\tSCORE\tREVIEWS\tID")
			for _, r := range rows {
				name := r.Name
				if len(name) > 44 {
					name = name[:41] + "..."
				}
				fmt.Fprintf(w, "%s\t%.1f\t%.1f\t%d\t%d\n", name, r.StarRating, r.ReviewScore, r.ReviewCount, r.PropertyID)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d match(es) from %d locally cached properties.\n", out.Matches, out.CorpusSize)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local property corpus database")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matches to return")
	return cmd
}

// ftsQuery makes a free-text query safe and forgiving for FTS5.
//
// Bare user input can contain FTS5 operators that either error or silently
// change the meaning of the query, so each term is quoted and given a prefix
// wildcard.
func ftsQuery(q string) string {
	fields := strings.Fields(q)
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Map(func(r rune) rune {
			if r == '"' || r == '*' || r == '(' || r == ')' || r == ':' || r == '^' {
				return -1
			}
			return r
		}, f)
		if f == "" {
			continue
		}
		parts = append(parts, "\""+f+"\"*")
	}
	if len(parts) == 0 {
		return "\"\""
	}
	return strings.Join(parts, " ")
}
