package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// creatorCompareRow is one creator's column in the compare table.
type creatorCompareRow struct {
	Handle          string  `json:"handle"`
	Platform        string  `json:"platform"`
	Found           bool    `json:"found"`
	FollowerCount   int64   `json:"follower_count,omitempty"`
	FollowingCount  int64   `json:"following_count,omitempty"`
	DisplayName     string  `json:"display_name,omitempty"`
	HeartCount      int64   `json:"heart_count,omitempty"`
	VideoCount      int64   `json:"video_count,omitempty"`
	EngagementRatio float64 `json:"engagement_ratio,omitempty"`
	Error           string  `json:"error,omitempty"`
}

func newCreatorCompareCmd(flags *rootFlags) *cobra.Command {
	var platform string
	cmd := &cobra.Command{
		Use:   "compare <handle>...",
		Short: "Compare two or more creators side-by-side on follower count and engagement",
		Long: `Pulls each creator's profile from the chosen platform in parallel and
emits a row per creator. Useful before sending pitch emails or shortlisting
partners. Defaults to TikTok; pass --platform to switch.`,
		Example:     "  scrape-creators-pp-cli creator compare mrbeast pewdiepie --platform youtube --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if len(args) == 0 {
					return cmd.Help()
				}
				return fmt.Errorf("compare needs at least 2 handles, got 1")
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			probe := pickProbe(platform)
			if probe == nil {
				return fmt.Errorf("unsupported platform %q (supported: tiktok, instagram, youtube, twitter, threads, bluesky, snapchat, twitch, truthsocial)", platform)
			}

			rows := make([]creatorCompareRow, len(args))
			var wg sync.WaitGroup
			for i, h := range args {
				wg.Add(1)
				go func(i int, h string) {
					defer wg.Done()
					handle := strings.TrimPrefix(h, "@")
					if probe.transform != nil {
						handle = probe.transform(handle)
					}
					raw, err := c.Get(probe.path, map[string]string{probe.param: handle})
					row := creatorCompareRow{Handle: handle, Platform: probe.platform}
					if err != nil {
						row.Error = trimError(err.Error())
						rows[i] = row
						return
					}
					row.Found = true
					var anyJSON map[string]any
					if json.Unmarshal(raw, &anyJSON) == nil {
						row.FollowerCount = lookupInt(anyJSON, probe.followerKy)
						row.DisplayName = lookupString(anyJSON, probe.nameKy)
						row.FollowingCount = lookupInt(anyJSON, []string{
							"user.stats.followingCount", "stats.followingCount",
							"following_count", "data.user.edge_follow.count",
						})
						row.HeartCount = lookupInt(anyJSON, []string{
							"user.stats.heartCount", "stats.heart", "stats.heartCount",
							"likeCount",
						})
						row.VideoCount = lookupInt(anyJSON, []string{
							"user.stats.videoCount", "stats.videoCount",
							"video_count", "videosCount",
						})
						if row.FollowerCount > 0 && row.HeartCount > 0 {
							row.EngagementRatio = float64(row.HeartCount) / float64(row.FollowerCount)
						}
					}
					rows[i] = row
				}(i, h)
				time.Sleep(40 * time.Millisecond)
			}
			wg.Wait()

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "HANDLE\tFOUND\tFOLLOWERS\tFOLLOWING\tHEARTS\tVIDEOS\tENG/FOLLOWER")
			for _, r := range rows {
				if !r.Found {
					fmt.Fprintf(tw, "%s\t-\t-\t-\t-\t-\t%s\n", r.Handle, r.Error)
					continue
				}
				fmt.Fprintf(tw, "%s\t✔\t%d\t%d\t%d\t%d\t%.4f\n",
					r.Handle, r.FollowerCount, r.FollowingCount, r.HeartCount, r.VideoCount, r.EngagementRatio)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "Platform to compare on (tiktok, instagram, youtube, twitter, threads, bluesky, snapchat, twitch, truthsocial)")
	return cmd
}

func pickProbe(platform string) *platformProbe {
	for _, p := range findProbes() {
		if p.platform == platform {
			return &p
		}
	}
	return nil
}
