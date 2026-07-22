// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: cross-platform creator presence matrix.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"
)

type creatorPresenceRow struct {
	Platform      string `json:"platform"`
	Exists        bool   `json:"exists"`
	FollowerCount int64  `json:"follower_count"`
}

func newNovelCreatorFindCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "find <handle>",
		Short:       "Given one handle, see which creator platforms the handle exists on with follower counts side-by-side.",
		Example:     "  scrape-creators-pp-cli creator find mrbeast",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("handle is required"))
			}
			handle := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fan out to every platform concurrently; each goroutine writes its
			// own slot so collection stays deterministic regardless of finish
			// order. One slow platform is bounded by its own sub-request ctx.
			type probe struct {
				data json.RawMessage
				err  error
			}
			probes := make([]probe, len(creatorPlatforms))
			var wg sync.WaitGroup
			for i, p := range creatorPlatforms {
				wg.Add(1)
				go func(i int, p creatorPlatform) {
					defer wg.Done()
					sctx, scancel := subRequestCtx(ctx)
					defer scancel()
					data, gerr := c.Get(sctx, p.profilePath, map[string]string{p.handleParam: handle})
					probes[i] = probe{data: data, err: gerr}
				}(i, p)
			}
			wg.Wait()

			rows := make([]creatorPresenceRow, 0, len(creatorPlatforms))
			failures := make([]fetchFailure, 0)
			presentCount := 0
			var totalFollowers int64

			for i, p := range creatorPlatforms {
				pr := probes[i]
				if pr.err != nil {
					// A 404 is a real "not present" result, not a fetch failure.
					if isNotFoundErr(pr.err) {
						rows = append(rows, creatorPresenceRow{Platform: p.name, Exists: false})
						continue
					}
					failures = append(failures, fetchFailure{Source: p.name, Error: sanitizeFetchErr(pr.err)})
					continue
				}
				followers, found := extractFollowerCount(pr.data)
				// A platform counts as present when we extracted a follower count,
				// or the body is a non-trivial, non-error profile payload. A 200
				// that is actually a {"success": false} error envelope must not
				// read as "exists".
				exists := found || (len(pr.data) > 2 && !isErrorEnvelope(pr.data))
				rows = append(rows, creatorPresenceRow{Platform: p.name, Exists: exists, FollowerCount: followers})
				if exists {
					presentCount++
					totalFollowers += followers
				}
			}

			sort.Slice(rows, func(i, j int) bool {
				if rows[i].FollowerCount != rows[j].FollowerCount {
					return rows[i].FollowerCount > rows[j].FollowerCount
				}
				return rows[i].Platform < rows[j].Platform
			})

			warnFetchFailures(cmd, "creator find", failures)

			if novelWantsMachine(cmd.OutOrStdout(), flags) {
				envelope := map[string]any{
					"handle":          handle,
					"platforms":       rows,
					"present_count":   presentCount,
					"total_followers": totalFollowers,
					"fetch_failures":  failures,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Creator presence for %q (%d platforms, %d present)\n\n", handle, len(rows), presentCount)
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "PLATFORM\tEXISTS\tFOLLOWERS")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%v\t%d\n", r.Platform, r.Exists, r.FollowerCount)
			}
			return tw.Flush()
		},
	}
	return cmd
}
