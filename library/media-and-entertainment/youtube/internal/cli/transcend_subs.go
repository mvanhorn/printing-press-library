package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/quota"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store"

	"github.com/spf13/cobra"
)

// newSubscriptionsCmd: parent for `subscriptions sweep`.
// (The endpoint-mirror subscriptions list/insert/delete are emitted by the
// generator under the youtube_subscriptions-* commands.)
func newSubscriptionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "OAuth-gated subscription helpers (sweep)",
	}
	cmd.AddCommand(newSubscriptionsSweepCmd(flags))
	return cmd
}

func newSubscriptionsSweepCmd(flags *rootFlags) *cobra.Command {
	var since string
	var maxPerChannel int
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Rebuild your subscription feed: subs.list → uploads in window → chronological feed",
		Long: `OAuth-gated. Lists every channel you're subscribed to, joins each channel's
uploads (from the local store; populates via sync-channel) within --since, and
emits a chronological feed.

Requires an OAuth bearer (` + "`youtube-pp-cli auth login`" + `). API key alone
is not enough — subscriptions.list requires user identity.`,
		Example: "  youtube-pp-cli subscriptions sweep --since 7d --json --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, _ := config.Load(flags.configPath)
			if cfg == nil || cfg.AuthHeader() == "" {
				return fmt.Errorf("oauth required: subscriptions sweep needs an OAuth bearer; run `youtube-pp-cli auth login` first")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cutoff, err := parseWindow(since)
			if err != nil {
				return err
			}
			dbPath := defaultDBPath("youtube-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureYouTubeExtras(cmd.Context()); err != nil {
				return err
			}
			// Walk subscriptions.list with mine=true
			pageToken := ""
			channelIDs := []string{}
			channelTitles := map[string]string{}
			for {
				params := map[string]string{
					"part":       "snippet",
					"mine":       "true",
					"maxResults": "50",
				}
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				raw, err := c.Get("/youtube/v3/subscriptions", params)
				if err != nil {
					return fmt.Errorf("subscriptions.list: %w", err)
				}
				_ = db.LogQuota(cmd.Context(), hashKeyFromConfig(cfg), "subscriptions sweep", "subscriptions.list", quota.Cost("subscriptions", "list"), 200, "")
				var resp struct {
					NextPageToken string `json:"nextPageToken"`
					Items         []struct {
						Snippet struct {
							Title      string `json:"title"`
							ResourceID struct {
								ChannelID string `json:"channelId"`
							} `json:"resourceId"`
						} `json:"snippet"`
					} `json:"items"`
				}
				if err := json.Unmarshal(raw, &resp); err != nil {
					return err
				}
				for _, it := range resp.Items {
					channelIDs = append(channelIDs, it.Snippet.ResourceID.ChannelID)
					channelTitles[it.Snippet.ResourceID.ChannelID] = it.Snippet.Title
				}
				if resp.NextPageToken == "" {
					break
				}
				pageToken = resp.NextPageToken
			}

			// Pull recent uploads for each subscribed channel from local store.
			feed := []map[string]any{}
			for _, ch := range channelIDs {
				rows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT video_id, title, published_at, view_count
					   FROM yt_videos
					   WHERE channel_id = ? AND published_at >= ?
					   ORDER BY published_at DESC LIMIT ?`,
					ch, cutoff.UTC().Format(time.RFC3339), maxPerChannel)
				if err != nil {
					continue
				}
				for rows.Next() {
					var vid, title, pub string
					var views int
					if err := rows.Scan(&vid, &title, &pub, &views); err != nil {
						continue
					}
					feed = append(feed, map[string]any{
						"video_id":      vid,
						"title":         title,
						"channel_id":    ch,
						"channel_title": channelTitles[ch],
						"published_at":  pub,
						"view_count":    views,
					})
				}
				rows.Close()
			}
			sort.SliceStable(feed, func(i, j int) bool {
				return fmt.Sprintf("%v", feed[i]["published_at"]) > fmt.Sprintf("%v", feed[j]["published_at"])
			})
			env := map[string]any{
				"subscribed_channels": len(channelIDs),
				"feed":                feed,
				"feed_size":           len(feed),
				"since":               cutoff.UTC().Format(time.RFC3339),
				"hint":                "feed only contains channels whose uploads have been synced via `youtube-pp-cli sync-channel`",
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Only include uploads after this window (e.g. 7d, 30d)")
	cmd.Flags().IntVar(&maxPerChannel, "max-per-channel", 5, "Cap uploads taken from each channel")
	return cmd
}

var _ = strings.TrimSpace
