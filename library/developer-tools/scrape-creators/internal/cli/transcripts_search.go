// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Novel command: FTS5 full-text search across locally synced transcripts.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

type transcriptHit struct {
	Platform string          `json:"platform"`
	Creator  string          `json:"creator,omitempty"`
	Snippet  string          `json:"snippet,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

func newNovelTranscriptsSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "FTS5 full-text search across every transcript you've synced — TikTok, YouTube, Instagram, and more.",
		Example:     "  scrape-creators-pp-cli transcripts search \"affiliate link\" --select creator,platform,snippet",
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
				return usageErr(fmt.Errorf("query is required"))
			}
			query := args[0]
			if limit <= 0 {
				limit = 20
			}
			if dbPath == "" {
				dbPath = defaultDBPath("scrape-creators-pp-cli")
			}

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: scrape-creators-pp-cli sync --resources <transcript-resource> --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			// Search is restricted to transcript resource types so a query
			// never returns profile or video rows. Store.Search filters by a
			// single resource_type, so fan out across the nine and merge.
			hits := make([]transcriptHit, 0)
			for _, rt := range transcriptResourceTypes {
				if len(hits) >= limit {
					break
				}
				rows, serr := db.Search(query, limit, rt)
				if serr != nil {
					return serr
				}
				for _, raw := range rows {
					if len(hits) >= limit {
						break
					}
					hits = append(hits, transcriptHit{
						Platform: platformFromResourceType(rt),
						Creator:  extractString(raw, creatorNameKeys),
						Snippet:  truncate(extractString(raw, transcriptTextKeys), 160),
						Data:     raw,
					})
				}
			}

			if novelWantsMachine(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}

			w := cmd.OutOrStdout()
			if len(hits) == 0 {
				fmt.Fprintf(w, "No transcript matches for %q.\n", query)
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "PLATFORM\tCREATOR\tSNIPPET")
			for _, h := range hits {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", h.Platform, h.Creator, h.Snippet)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum transcript matches to return")
	return cmd
}
