// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: snapshot a topic across platforms to see where it leads.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"
)

type trendPlatformRow struct {
	Platform    string `json:"platform"`
	ResultCount int    `json:"result_count"`
	Rank        int    `json:"rank"`
}

func newNovelTrendsTriangulateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "triangulate <query>",
		Short:       "Snapshot a hashtag or topic across platforms in one call to see which platform it's biggest on.",
		Example:     "  scrape-creators-pp-cli trends triangulate \"stanley cup\"",
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

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fan out to every search endpoint concurrently, each bounded by
			// its own sub-request ctx; collect into per-source slots so result
			// assembly is deterministic regardless of finish order.
			type probe struct {
				data json.RawMessage
				err  error
			}
			probes := make([]probe, len(searchSources))
			var wg sync.WaitGroup
			for i, s := range searchSources {
				wg.Add(1)
				go func(i int, s searchSource) {
					defer wg.Done()
					sctx, scancel := subRequestCtx(ctx)
					defer scancel()
					data, gerr := c.Get(sctx, s.path, map[string]string{s.queryParam: query})
					probes[i] = probe{data: data, err: gerr}
				}(i, s)
			}
			wg.Wait()

			rows := make([]trendPlatformRow, 0, len(searchSources))
			failures := make([]fetchFailure, 0)

			for i, s := range searchSources {
				pr := probes[i]
				if pr.err != nil {
					failures = append(failures, fetchFailure{Source: s.name, Error: sanitizeFetchErr(pr.err)})
					continue
				}
				rows = append(rows, trendPlatformRow{Platform: s.name, ResultCount: len(resultArray(pr.data, s.resultKey))})
			}

			sort.Slice(rows, func(i, j int) bool {
				if rows[i].ResultCount != rows[j].ResultCount {
					return rows[i].ResultCount > rows[j].ResultCount
				}
				return rows[i].Platform < rows[j].Platform
			})
			for i := range rows {
				rows[i].Rank = i + 1
			}
			leading := ""
			if len(rows) > 0 && rows[0].ResultCount > 0 {
				leading = rows[0].Platform
			}

			warnFetchFailures(cmd, "trends triangulate", failures)

			if novelWantsMachine(cmd.OutOrStdout(), flags) {
				envelope := map[string]any{
					"query":            query,
					"platforms":        rows,
					"leading_platform": leading,
					"fetch_failures":   failures,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Topic %q across %d platforms (leading: %s)\n\n", query, len(rows), leading)
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "RANK\tPLATFORM\tRESULTS")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%s\t%d\n", r.Rank, r.Platform, r.ResultCount)
			}
			return tw.Flush()
		},
	}
	return cmd
}
