// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"sort"
	"time"
)

func newNovelProgressCatchUpCmd(flags *rootFlags) *cobra.Command {
	var asOf string
	cmd := &cobra.Command{Use: "catch-up", Short: "Find followed shows with aired episodes beyond your recorded progress.", Example: "--as-of 2026-07-21T20:00:00Z --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		t := personalNow()
		var err error
		if asOf != "" {
			t, err = time.Parse(time.RFC3339, asOf)
			if err != nil {
				return usageErr(fmt.Errorf("--as-of must be RFC3339: %w", err))
			}
		}
		if flags.dryRun {
			return flags.printJSON(cmd, map[string]any{"as_of": t, "dry_run": true})
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		u, err := viewerID(cmd.Context(), c)
		if err != nil {
			return err
		}
		entries, err := allListEntries(cmd.Context(), c, u, "CURRENT")
		if err != nil {
			return err
		}
		ids := make([]int, 0, len(entries))
		byID := map[int]personalEntry{}
		for _, e := range entries {
			ids = append(ids, e.Media.ID)
			byID[e.Media.ID] = e
		}
		const q = `query($ids:[Int],$until:Int,$page:Int!){Page(page:$page,perPage:50){pageInfo{hasNextPage} airingSchedules(mediaId_in:$ids,airingAt_lesser:$until){episode airingAt media{id}}}}`
		highest := map[int]int{}
		for page := 1; ; page++ {
			var r struct {
				Page struct {
					PageInfo struct {
						HasNextPage bool `json:"hasNextPage"`
					} `json:"pageInfo"`
					AiringSchedules []struct {
						Episode  int   `json:"episode"`
						AiringAt int64 `json:"airingAt"`
						Media    struct {
							ID int `json:"id"`
						} `json:"media"`
					} `json:"airingSchedules"`
				} `json:"Page"`
			}
			if err := anilistGraphQL(cmd.Context(), c, q, map[string]any{"ids": ids, "until": t.Unix() + 1, "page": page}, &r); err != nil {
				return err
			}
			for _, a := range r.Page.AiringSchedules {
				if _, followed := byID[a.Media.ID]; !followed || a.AiringAt > t.Unix() {
					continue
				}
				if a.Episode > highest[a.Media.ID] {
					highest[a.Media.ID] = a.Episode
				}
			}
			if !r.Page.PageInfo.HasNextPage {
				break
			}
		}
		highestIDs := make([]int, 0, len(highest))
		for id := range highest {
			highestIDs = append(highestIDs, id)
		}
		sort.Ints(highestIDs)
		out := []map[string]any{}
		for _, id := range highestIDs {
			air := highest[id]
			e := byID[id]
			if air > e.Progress {
				out = append(out, map[string]any{"media_id": id, "title": e.Media.Title.UserPreferred, "progress": e.Progress, "highest_aired_episode": air, "gap": air - e.Progress, "as_of": t.Format(time.RFC3339)})
			}
		}
		return flags.printJSON(cmd, out)
	}}
	cmd.Flags().StringVar(&asOf, "as-of", "", "RFC3339 instant; defaults to invocation time")
	return cmd
}
