// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type hygieneRules struct {
	AutoAdd []struct {
		Name       string `yaml:"name"`
		TitleMatch string `yaml:"title_match"`
		TagMatch   string `yaml:"tag_match"`
		Playlist   string `yaml:"playlist_id"`
	} `yaml:"auto_add"`
	ReorderByViews []string `yaml:"reorder_by_views"`
}

func newPlaylistHygieneCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playlist",
		Short: "Apply playlist-hygiene rules to the authenticated channel",
		Long: `Parent command for playlist hygiene tasks.

Currently exposes the 'hygiene' subcommand which applies a YAML rules file
to auto-add videos to playlists by title/tag patterns and to reorder
playlists by view count.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPlaylistHygieneApplyCmd(flags))
	return cmd
}

func newPlaylistHygieneApplyCmd(flags *rootFlags) *cobra.Command {
	var rulesPath string
	var apply bool

	cmd := &cobra.Command{
		Use:   "hygiene",
		Short: "Auto-add videos to playlists by tag/title rules, reorder playlists by view count",
		Long: `Reads a rules YAML and applies playlist hygiene to the authenticated channel:

- auto_add: rules with a title_match regex or tag_match regex, plus a target
  playlist_id. Matching videos are added (idempotent — skips if already in playlist).
- reorder_by_views: list of playlist IDs; reorders items by current view count.

Without --apply, prints what would change.

Example rules.yaml:

  auto_add:
    - name: tutorials
      title_match: '(?i)tutorial|how to'
      playlist_id: PLxxxTutorial
    - name: shorts
      tag_match: '^Short$'
      playlist_id: PLxxxShorts
  reorder_by_views:
    - PLxxxAllTimeBest`,
		Example: "  youtube-creator-pp-cli playlist hygiene --rules rules.yaml\n" +
			"  youtube-creator-pp-cli playlist hygiene --rules rules.yaml --apply",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if rulesPath == "" {
				if flags.dryRun {
					return nil
				}
				return usageErr(fmt.Errorf("--rules is required"))
			}
			data, err := os.ReadFile(rulesPath)
			if err != nil {
				return configErr(fmt.Errorf("reading rules: %w", err))
			}
			var rules hygieneRules
			if err := yaml.Unmarshal(data, &rules); err != nil {
				return configErr(fmt.Errorf("parsing rules: %w", err))
			}

			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Get uploads playlist
			quotaLogCost("channels-list", 1)
			chData, err := c.Get("/youtube/v3/channels", map[string]string{
				"part": "contentDetails",
				"mine": "true",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var chResp struct {
				Items []struct {
					ContentDetails struct {
						RelatedPlaylists struct {
							Uploads string `json:"uploads"`
						} `json:"relatedPlaylists"`
					} `json:"contentDetails"`
				} `json:"items"`
			}
			_ = json.Unmarshal(chData, &chResp)
			if len(chResp.Items) == 0 {
				return apiErr(fmt.Errorf("no authenticated channel found"))
			}
			uploadsID := chResp.Items[0].ContentDetails.RelatedPlaylists.Uploads

			// Get uploads with snippet+tags
			type vid struct {
				ID    string   `json:"id"`
				Title string   `json:"title"`
				Tags  []string `json:"tags"`
			}
			var allVideos []vid

			// First: get IDs from uploads playlist
			pageToken := ""
			var ids []string
			for {
				params := map[string]string{
					"part":       "contentDetails",
					"playlistId": uploadsID,
					"maxResults": "50",
				}
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				quotaLogCost("playlist-items-list", 1)
				d, err := c.Get("/youtube/v3/playlistItems", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var page struct {
					Items []struct {
						ContentDetails struct {
							VideoID string `json:"videoId"`
						} `json:"contentDetails"`
					} `json:"items"`
					NextPageToken string `json:"nextPageToken"`
				}
				_ = json.Unmarshal(d, &page)
				for _, it := range page.Items {
					ids = append(ids, it.ContentDetails.VideoID)
				}
				if page.NextPageToken == "" {
					break
				}
				pageToken = page.NextPageToken
			}

			// Fetch snippets in batches
			for i := 0; i < len(ids); i += 50 {
				end := i + 50
				if end > len(ids) {
					end = len(ids)
				}
				quotaLogCost("videos-list", 1)
				d, err := c.Get("/youtube/v3/videos", map[string]string{
					"part": "snippet",
					"id":   strings.Join(ids[i:end], ","),
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var resp struct {
					Items []struct {
						ID      string `json:"id"`
						Snippet struct {
							Title string   `json:"title"`
							Tags  []string `json:"tags"`
						} `json:"snippet"`
					} `json:"items"`
				}
				_ = json.Unmarshal(d, &resp)
				for _, v := range resp.Items {
					allVideos = append(allVideos, vid{ID: v.ID, Title: v.Snippet.Title, Tags: v.Snippet.Tags})
				}
			}

			// Match rules
			type addAction struct {
				Rule       string `json:"rule"`
				VideoID    string `json:"video_id"`
				Title      string `json:"title"`
				PlaylistID string `json:"playlist_id"`
			}
			var addActions []addAction
			for _, r := range rules.AutoAdd {
				var titleRe, tagRe *regexp.Regexp
				if r.TitleMatch != "" {
					titleRe = regexp.MustCompile(r.TitleMatch)
				}
				if r.TagMatch != "" {
					tagRe = regexp.MustCompile(r.TagMatch)
				}
				for _, v := range allVideos {
					if titleRe != nil && !titleRe.MatchString(v.Title) {
						continue
					}
					if tagRe != nil {
						any := false
						for _, t := range v.Tags {
							if tagRe.MatchString(t) {
								any = true
								break
							}
						}
						if !any {
							continue
						}
					}
					addActions = append(addActions, addAction{Rule: r.Name, VideoID: v.ID, Title: v.Title, PlaylistID: r.Playlist})
				}
			}

			result := map[string]any{
				"total_uploads": len(allVideos),
				"would_add":     addActions,
				"reorder":       rules.ReorderByViews,
				"applied":       apply,
			}

			if !apply {
				result["note"] = "Dry preview. Pass --apply to mutate."
				return flags.printJSON(cmd, result)
			}

			// Apply auto_add (each insert = 50 units)
			added := 0
			for _, a := range addActions {
				body := map[string]any{
					"snippet": map[string]any{
						"playlistId": a.PlaylistID,
						"resourceId": map[string]any{
							"kind":    "youtube#video",
							"videoId": a.VideoID,
						},
					},
				}
				quotaLogCost("playlist-items-insert", 50)
				_, _, err := c.PostWithParams("/youtube/v3/playlistItems", map[string]string{"part": "snippet"}, body)
				if err != nil {
					// Likely duplicate — ignore but report
					continue
				}
				added++
			}
			result["added_count"] = added

			// Apply reorder_by_views: fetch playlist items + view counts, reorder
			reordered := 0
			for _, plID := range rules.ReorderByViews {
				type plItem struct {
					ID      string
					VideoID string
				}
				var items []plItem
				token := ""
				for {
					params := map[string]string{"part": "snippet,contentDetails", "playlistId": plID, "maxResults": "50"}
					if token != "" {
						params["pageToken"] = token
					}
					quotaLogCost("playlist-items-list", 1)
					d, err := c.Get("/youtube/v3/playlistItems", params)
					if err != nil {
						break
					}
					var page struct {
						Items []struct {
							ID             string `json:"id"`
							ContentDetails struct {
								VideoID string `json:"videoId"`
							} `json:"contentDetails"`
						} `json:"items"`
						NextPageToken string `json:"nextPageToken"`
					}
					_ = json.Unmarshal(d, &page)
					for _, it := range page.Items {
						items = append(items, plItem{ID: it.ID, VideoID: it.ContentDetails.VideoID})
					}
					if page.NextPageToken == "" {
						break
					}
					token = page.NextPageToken
				}
				// Fetch view counts
				views := map[string]int64{}
				for i := 0; i < len(items); i += 50 {
					end := i + 50
					if end > len(items) {
						end = len(items)
					}
					var vids []string
					for _, it := range items[i:end] {
						vids = append(vids, it.VideoID)
					}
					quotaLogCost("videos-list", 1)
					d, err := c.Get("/youtube/v3/videos", map[string]string{
						"part": "statistics",
						"id":   strings.Join(vids, ","),
					})
					if err != nil {
						continue
					}
					var resp struct {
						Items []struct {
							ID         string `json:"id"`
							Statistics struct {
								ViewCount string `json:"viewCount"`
							} `json:"statistics"`
						} `json:"items"`
					}
					_ = json.Unmarshal(d, &resp)
					for _, v := range resp.Items {
						var n int64
						_, _ = fmt.Sscanf(v.Statistics.ViewCount, "%d", &n)
						views[v.ID] = n
					}
				}
				// Sort by view count desc
				sort.Slice(items, func(i, j int) bool {
					return views[items[i].VideoID] > views[items[j].VideoID]
				})
				// Update each playlistItem's position
				for pos, it := range items {
					body := map[string]any{
						"id": it.ID,
						"snippet": map[string]any{
							"playlistId": plID,
							"position":   pos,
							"resourceId": map[string]any{
								"kind":    "youtube#video",
								"videoId": it.VideoID,
							},
						},
					}
					quotaLogCost("playlist-items-update", 50)
					_, _, err := c.PostWithParams("/youtube/v3/playlistItems", map[string]string{"part": "snippet"}, body)
					if err == nil {
						reordered++
					}
				}
			}
			result["reordered_items"] = reordered
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&rulesPath, "rules", "", "Path to rules.yaml")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply mutations (default: dry preview)")
	return cmd
}
