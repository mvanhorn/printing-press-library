// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/gtrends"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/store"
)

// pp:data-source live
func newNovelTrendsTrendingCmd(flags *rootFlags) *cobra.Command {
	var flagGeo string
	var flagHl string

	cmd := &cobra.Command{
		Use:         "trending",
		Short:       "Current trending search topics (optionally filtered by geo). See also: trending at.",
		Example:     "  google-trends-pp-cli trends trending --geo US --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			hl := flagHl
			if hl == "" {
				hl = gtrends.DefaultHL
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			topics, err := gtrends.TrendingNow(ctx, c, flagGeo, hl)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			date := time.Now().UTC().Format("2006-01-02")
			syncedAt := time.Now().UTC().Format(time.RFC3339)
			out := make([]gtTrendingTopicRecord, 0, len(topics))

			db, dbErr := store.OpenWithContext(ctx, defaultDBPath("google-trends-pp-cli"))
			if dbErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open local store to cache trending results: %v\n", dbErr)
			} else {
				defer db.Close()
			}

			for _, t := range topics {
				row := gtTrendingTopicRecord{Date: date, Geo: flagGeo, Term: t.Term, Rank: t.Rank, SyncedAt: syncedAt}
				out = append(out, row)
				if db != nil {
					body, _ := json.Marshal(row)
					if err := db.Upsert("gt_trending_topic", sha256ID(date, flagGeo, t.Term), body); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to cache trending topic %q: %v\n", t.Term, err)
					}
				}
			}

			return printLiveResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagGeo, "geo", "", "Geo code to scope trending topics (e.g. US); default: worldwide")
	cmd.Flags().StringVar(&flagHl, "hl", "en-US", "UI/response language")
	cmd.AddCommand(newNovelTrendsTrendingAtCmd(flags))
	return cmd
}
