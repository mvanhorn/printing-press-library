// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: side-by-side creator comparison on followers and engagement.

package cli

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

type creatorCompareRow struct {
	Handle         string  `json:"handle"`
	Platform       string  `json:"platform"`
	FollowerCount  int64   `json:"follower_count"`
	AvgEngagement  int64   `json:"avg_engagement"`
	EngagementRate float64 `json:"engagement_rate"`
	ContentCount   int     `json:"content_count"`
}

func newNovelCreatorCompareCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var sampleSize int

	cmd := &cobra.Command{
		Use:         "compare <handle> <handle>...",
		Short:       "Compare two or more creators side-by-side on followers, engagement rate, and content volume.",
		Example:     "  scrape-creators-pp-cli creator compare mkbhd mrwhosetheboss --select handle,engagement_rate,follower_count",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least two handles are required"))
			}
			if platform == "" {
				platform = "tiktok"
			}
			cp, ok := creatorPlatformByName(platform)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unsupported --platform %q", platform))
			}
			cs, hasContent := contentSources[platform]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Compare each handle concurrently; each goroutine writes its own
			// slot so rows preserve the input handle order on collection.
			type handleResult struct {
				row      creatorCompareRow
				hasRow   bool
				failures []fetchFailure
			}
			outcomes := make([]handleResult, len(args))
			var wg sync.WaitGroup
			for i, handle := range args {
				wg.Add(1)
				go func(i int, handle string) {
					defer wg.Done()
					pctx, pcancel := subRequestCtx(ctx)
					defer pcancel()
					profile, perr := c.Get(pctx, cp.profilePath, map[string]string{cp.handleParam: handle})
					if perr != nil {
						outcomes[i] = handleResult{failures: []fetchFailure{{Source: handle, Error: sanitizeFetchErr(perr)}}}
						return
					}
					followers, _ := extractFollowerCount(profile)
					row := creatorCompareRow{Handle: handle, Platform: platform, FollowerCount: followers}
					var fails []fetchFailure

					if hasContent {
						vctx, vcancel := subRequestCtx(ctx)
						defer vcancel()
						vids, verr := c.Get(vctx, cs.path, map[string]string{cs.handleParam: handle})
						if verr != nil {
							// Profile succeeded; note the content gap but keep the row.
							fails = append(fails, fetchFailure{Source: handle + ":content", Error: sanitizeFetchErr(verr)})
						} else {
							items := resultArray(vids, cs.arrayKey)
							if sampleSize > 0 && len(items) > sampleSize {
								items = items[:sampleSize]
							}
							var total int64
							for _, it := range items {
								total += extractContentMetrics(it).engagement()
							}
							row.ContentCount = len(items)
							if len(items) > 0 {
								row.AvgEngagement = total / int64(len(items))
							}
						}
					}
					if followers > 0 {
						row.EngagementRate = float64(row.AvgEngagement) / float64(followers)
					}
					outcomes[i] = handleResult{row: row, hasRow: true, failures: fails}
				}(i, handle)
			}
			wg.Wait()

			rows := make([]creatorCompareRow, 0, len(args))
			failures := make([]fetchFailure, 0)
			for _, o := range outcomes {
				if o.hasRow {
					rows = append(rows, o.row)
				}
				failures = append(failures, o.failures...)
			}

			warnFetchFailures(cmd, "creator compare", failures)

			if novelWantsMachine(cmd.OutOrStdout(), flags) {
				envelope := map[string]any{
					"platform":       platform,
					"creators":       rows,
					"fetch_failures": failures,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			w := cmd.OutOrStdout()
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "HANDLE\tPLATFORM\tFOLLOWERS\tAVG_ENGAGEMENT\tENGAGEMENT_RATE\tCONTENT")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%.4f%%\t%d\n",
					r.Handle, r.Platform, r.FollowerCount, r.AvgEngagement, r.EngagementRate*100, r.ContentCount)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "creator platform to compare on (tiktok, instagram, youtube, ...)")
	cmd.Flags().IntVar(&sampleSize, "sample", 20, "max recent content items to average engagement over")
	return cmd
}
