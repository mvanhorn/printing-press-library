// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"time"
)

func newNovelScheduleTonightCmd(flags *rootFlags) *cobra.Command {
	var zone string
	cmd := &cobra.Command{Use: "tonight", Short: "See only episodes from your current anime list that air in your local day.", Example: "--timezone America/New_York --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		loc := time.Local
		var err error
		if zone != "" {
			loc, err = time.LoadLocation(zone)
			if err != nil {
				return usageErr(fmt.Errorf("--timezone must be an IANA location: %w", err))
			}
		}
		now := personalNow().In(loc)
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		end := start.AddDate(0, 0, 1)
		if flags.dryRun {
			return flags.printJSON(cmd, map[string]any{"timezone": loc.String(), "start": start, "end": end, "dry_run": true})
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		user, err := viewerID(cmd.Context(), c)
		if err != nil {
			return err
		}
		entries, err := allListEntries(cmd.Context(), c, user, "CURRENT")
		if err != nil {
			return err
		}
		ids := make([]int, 0, len(entries))
		followed := map[int]bool{}
		for _, e := range entries {
			ids = append(ids, e.Media.ID)
			followed[e.Media.ID] = true
		}
		type airing struct {
			Episode  int           `json:"episode"`
			AiringAt int64         `json:"airingAt"`
			Media    personalMedia `json:"media"`
		}
		var out []airing
		const q = `query($ids:[Int],$from:Int,$to:Int,$page:Int!){Page(page:$page,perPage:50){pageInfo{hasNextPage} airingSchedules(mediaId_in:$ids,airingAt_greater:$from,airingAt_lesser:$to){episode airingAt media{id title{userPreferred}}}}}`
		for page := 1; ; page++ {
			var r struct {
				Page struct {
					PageInfo struct {
						HasNextPage bool `json:"hasNextPage"`
					} `json:"pageInfo"`
					AiringSchedules []airing `json:"airingSchedules"`
				} `json:"Page"`
			}
			if err := anilistGraphQL(cmd.Context(), c, q, map[string]any{"ids": ids, "from": start.Unix() - 1, "to": end.Unix(), "page": page}, &r); err != nil {
				return err
			}
			for _, a := range r.Page.AiringSchedules {
				if followed[a.Media.ID] && a.AiringAt >= start.Unix() && a.AiringAt < end.Unix() {
					out = append(out, a)
				}
			}
			if !r.Page.PageInfo.HasNextPage {
				break
			}
		}
		return flags.printJSON(cmd, out)
	}}
	cmd.Flags().StringVar(&zone, "timezone", "", "IANA timezone; defaults to the process local timezone")
	return cmd
}
