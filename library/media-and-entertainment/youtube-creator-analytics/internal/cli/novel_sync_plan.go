package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/store"
)

// quota costs per Data v3 op (units).
// https://developers.google.com/youtube/v3/determine_quota_cost
const (
	costSearchList     = 100
	costVideoListByID  = 1
	costChannelList    = 1
	costPlaylistItems  = 1
	costCommentThreads = 1
)

type syncPlanResult struct {
	DailyQuota     int            `json:"daily_quota_default"`
	Channel        string         `json:"channel,omitempty"`
	CachedVideos   int            `json:"cached_videos"`
	Operations     map[string]int `json:"estimated_units_per_op"`
	Estimate       planEstimate   `json:"estimate"`
	Recommendation string         `json:"recommendation"`
}

type planEstimate struct {
	UnitsForVideoRefresh int `json:"units_for_video_refresh"`
	UnitsForCommentSync  int `json:"units_for_comment_sync"`
	UnitsTotal           int `json:"units_total"`
	HeadroomDaily        int `json:"headroom_daily"`
}

// newSyncPlanCmd computes a quota-aware sync plan from what's already cached.
func newSyncPlanCmd(flags *rootFlags) *cobra.Command {
	var dbPath, channel string
	var commentsPerVideo int
	cmd := &cobra.Command{
		Use:         "sync-plan",
		Short:       "Quota-aware sync planner: estimates Data v3 units for refresh + comment sync",
		Example:     "  youtube-creator-analytics-pp-cli sync-plan --channel UC123 --comments-per-video 1 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("youtube-creator-analytics-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()
			ch, _ := resolveChannel(db, channel)
			vids, _ := loadVideos(db, ch, 0)
			n := len(vids)
			est := planEstimate{
				UnitsForVideoRefresh: (n + 49) / 50 * costVideoListByID,
				UnitsForCommentSync:  n * commentsPerVideo * costCommentThreads,
			}
			est.UnitsTotal = est.UnitsForVideoRefresh + est.UnitsForCommentSync
			est.HeadroomDaily = 10000 - est.UnitsTotal
			rec := "ok"
			if est.UnitsTotal > 5000 {
				rec = "split across days or request quota increase"
			} else if est.UnitsTotal > 10000 {
				rec = "exceeds daily quota; must split or batch"
			}
			return flags.printJSON(cmd, syncPlanResult{
				DailyQuota:   10000,
				Channel:      ch,
				CachedVideos: n,
				Operations: map[string]int{
					"search.list":    costSearchList,
					"videos.list":    costVideoListByID,
					"channels.list":  costChannelList,
					"playlistItems":  costPlaylistItems,
					"commentThreads": costCommentThreads,
				},
				Estimate:       est,
				Recommendation: rec,
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&channel, "channel", "", "Channel ID")
	cmd.Flags().IntVar(&commentsPerVideo, "comments-per-video", 1, "Avg comment-thread pages to sync per video")
	return cmd
}
