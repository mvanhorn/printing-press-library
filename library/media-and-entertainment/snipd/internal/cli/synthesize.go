// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command — NOT generator output.
// pp:data-source local
//
// synthesize gathers a deduped, snippet-only evidence set (title + note + quote +
// deep-link) scoped by query and optionally by show — sized in KB for an agent to
// reason over. Transcripts are omitted by default (fetch on demand with
// --transcript); this is the shape an outliner substrate overflowed on.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelSynthesizeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var show string
	var withTranscript bool

	cmd := &cobra.Command{
		Use:   "synthesize <query>",
		Short: "Gather a deduped, snippet-only evidence set (title + note + quote + deep-link) scoped by query, show, or topic.",
		Long: "Assemble a compact evidence feed for a concept across your snips — ranked by\n" +
			"full-text relevance, deduped, and projected to snippet fields (no transcript\n" +
			"unless --transcript). Use this to hand an agent exactly what it needs to\n" +
			"synthesize a cross-show answer without dragging whole transcripts through\n" +
			"context. Plain stemmed terms work best; the index is porter-stemmed.",
		Example:     "  snipd-pp-cli synthesize \"AI and jobs\" --limit 20 --agent\n  snipd-pp-cli synthesize \"personas\" --show \"UX Research Geeks\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
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

			// Over-fetch when a show filter is applied so the post-filter set can
			// still reach --limit.
			fetch := limit
			if show != "" {
				fetch = limit * 4
			}
			raws, err := st.Search(query, fetch, "snips")
			if err != nil {
				return err
			}
			snips := unmarshalSnips(raws)

			seen := map[string]bool{}
			rows := make([]compactSnip, 0, len(snips))
			for _, s := range snips {
				if show != "" && !strings.EqualFold(s.Show, show) {
					continue
				}
				key := s.SnipID
				if key == "" {
					key = s.URL
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				rows = append(rows, toCompact(s, withTranscript))
				if limit > 0 && len(rows) >= limit {
					break
				}
			}
			return printCompact(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path (default: per-user data dir)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum snips in the evidence set")
	cmd.Flags().StringVar(&show, "show", "", "Restrict to a single show")
	cmd.Flags().BoolVar(&withTranscript, "transcript", false, "Include full transcripts (larger output)")
	return cmd
}
