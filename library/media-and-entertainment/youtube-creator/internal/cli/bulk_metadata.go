// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newBulkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Bulk operations across the full video catalog",
		Long: `Quota-cheap bulk operations: enumerates uploads via channels.list +
playlistItems.list (1 unit per page) rather than search.list (100 units),
then applies the requested mutation to matching videos.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBulkMetadataCmd(flags))
	return cmd
}

func newBulkMetadataCmd(flags *rootFlags) *cobra.Command {
	var titleContains, descContains, categoryEq, publishedAfter string
	var appendDesc, prependDesc string
	var setCategory, setTags string
	var apply bool
	var maxVideos int

	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Filter videos and apply metadata mutations (dry-run by default)",
		Long: `Enumerate uploads via the cheap playlistItems.list path, filter by simple
predicates (--title-contains, --description-contains, --category, --published-after),
then apply mutations (--append-description from file, --prepend-description,
--set-category, --set-tags).

Without --apply, prints what would change (dry-run diff).`,
		Example: "  youtube-creator-pp-cli bulk metadata --title-contains 'tutorial' --append-description ./footer.md\n" +
			"  youtube-creator-pp-cli bulk metadata --published-after 2026-01-01 --set-category 22 --apply",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: get uploads playlist ID via channels.list?mine=true&part=contentDetails
			quotaLogCost("channels-list", 1)
			channelData, err := c.Get("/youtube/v3/channels", map[string]string{
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
			if err := json.Unmarshal(channelData, &chResp); err != nil {
				return fmt.Errorf("decoding channels.list: %w", err)
			}
			if len(chResp.Items) == 0 {
				return apiErr(fmt.Errorf("no authenticated channel found"))
			}
			uploadsID := chResp.Items[0].ContentDetails.RelatedPlaylists.Uploads

			// Step 2: page through playlistItems.list to get all video IDs
			var videoIDs []string
			pageToken := ""
			for {
				params := map[string]string{
					"part":       "snippet,contentDetails",
					"playlistId": uploadsID,
					"maxResults": "50",
				}
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				quotaLogCost("playlist-items-list", 1)
				data, err := c.Get("/youtube/v3/playlistItems", params)
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
				if err := json.Unmarshal(data, &page); err != nil {
					return fmt.Errorf("decoding: %w", err)
				}
				for _, it := range page.Items {
					if id := it.ContentDetails.VideoID; id != "" {
						videoIDs = append(videoIDs, id)
					}
				}
				if page.NextPageToken == "" || (maxVideos > 0 && len(videoIDs) >= maxVideos) {
					break
				}
				pageToken = page.NextPageToken
			}

			// Step 3: batch videos.list to get full snippets (50 per call)
			type videoMeta struct {
				ID      string          `json:"id"`
				Snippet json.RawMessage `json:"snippet"`
			}
			var videos []videoMeta
			for i := 0; i < len(videoIDs); i += 50 {
				end := i + 50
				if end > len(videoIDs) {
					end = len(videoIDs)
				}
				ids := strings.Join(videoIDs[i:end], ",")
				quotaLogCost("videos-list", 1)
				data, err := c.Get("/youtube/v3/videos", map[string]string{
					"part": "snippet",
					"id":   ids,
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var resp struct {
					Items []videoMeta `json:"items"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					return err
				}
				videos = append(videos, resp.Items...)
			}

			// Step 4: filter
			var matched []videoMeta
			cutoff := time.Time{}
			if publishedAfter != "" {
				t, err := time.Parse("2006-01-02", publishedAfter)
				if err != nil {
					return usageErr(fmt.Errorf("--published-after must be YYYY-MM-DD: %w", err))
				}
				cutoff = t
			}
			for _, v := range videos {
				var snip struct {
					Title       string    `json:"title"`
					Description string    `json:"description"`
					CategoryID  string    `json:"categoryId"`
					PublishedAt time.Time `json:"publishedAt"`
				}
				_ = json.Unmarshal(v.Snippet, &snip)
				if titleContains != "" && !strings.Contains(strings.ToLower(snip.Title), strings.ToLower(titleContains)) {
					continue
				}
				if descContains != "" && !strings.Contains(snip.Description, descContains) {
					continue
				}
				if categoryEq != "" && snip.CategoryID != categoryEq {
					continue
				}
				if !cutoff.IsZero() && snip.PublishedAt.Before(cutoff) {
					continue
				}
				matched = append(matched, v)
			}

			// Step 5: read append/prepend file content
			var appendText, prependText string
			if appendDesc != "" {
				data, err := os.ReadFile(appendDesc)
				if err != nil {
					return configErr(fmt.Errorf("reading --append-description file: %w", err))
				}
				appendText = string(data)
			}
			if prependDesc != "" {
				data, err := os.ReadFile(prependDesc)
				if err != nil {
					return configErr(fmt.Errorf("reading --prepend-description file: %w", err))
				}
				prependText = string(data)
			}

			// Step 6: compute diffs / apply mutations
			type change struct {
				VideoID   string   `json:"video_id"`
				Title     string   `json:"title"`
				Mutations []string `json:"mutations"`
			}
			var changes []change
			for _, v := range matched {
				var snip map[string]any
				_ = json.Unmarshal(v.Snippet, &snip)
				var muts []string
				if appendText != "" {
					muts = append(muts, "append description (+"+fmt.Sprintf("%d", len(appendText))+" chars)")
				}
				if prependText != "" {
					muts = append(muts, "prepend description")
				}
				if setCategory != "" {
					muts = append(muts, "set categoryId="+setCategory)
				}
				if setTags != "" {
					muts = append(muts, "set tags=["+setTags+"]")
				}
				title, _ := snip["title"].(string)
				changes = append(changes, change{VideoID: v.ID, Title: title, Mutations: muts})
			}

			result := map[string]any{
				"matched_count":     len(matched),
				"total_videos":      len(videos),
				"quota_cost_so_far": 1 + (len(videoIDs)/50 + 1) + (len(videoIDs)/50 + 1),
				"applied":           apply,
				"changes":           changes,
			}

			if !apply {
				result["note"] = "Dry preview. Pass --apply to mutate. Each apply costs 50 quota units per video."
				return flags.printJSON(cmd, result)
			}

			// Apply: PUT videos.update with id + part=snippet
			applied := 0
			for _, v := range matched {
				var snip map[string]any
				_ = json.Unmarshal(v.Snippet, &snip)
				if appendText != "" {
					d, _ := snip["description"].(string)
					snip["description"] = d + "\n\n" + appendText
				}
				if prependText != "" {
					d, _ := snip["description"].(string)
					snip["description"] = prependText + "\n\n" + d
				}
				if setCategory != "" {
					snip["categoryId"] = setCategory
				}
				if setTags != "" {
					snip["tags"] = strings.Split(setTags, ",")
				}
				body := map[string]any{
					"id":      v.ID,
					"snippet": snip,
				}
				quotaLogCost("videos-update", 50)
				_, _, err := c.PostWithParams("/youtube/v3/videos", map[string]string{"part": "snippet"}, body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				applied++
			}
			result["applied_count"] = applied
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&titleContains, "title-contains", "", "Filter: title substring (case-insensitive)")
	cmd.Flags().StringVar(&descContains, "description-contains", "", "Filter: description substring (case-sensitive)")
	cmd.Flags().StringVar(&categoryEq, "category", "", "Filter: exact categoryId")
	cmd.Flags().StringVar(&publishedAfter, "published-after", "", "Filter: publishedAt >= YYYY-MM-DD")
	cmd.Flags().StringVar(&appendDesc, "append-description", "", "Append the content of this file to each matching video's description")
	cmd.Flags().StringVar(&prependDesc, "prepend-description", "", "Prepend the content of this file to each matching video's description")
	cmd.Flags().StringVar(&setCategory, "set-category", "", "Replace categoryId on matching videos")
	cmd.Flags().StringVar(&setTags, "set-tags", "", "Comma-separated tag list to set on matching videos (replaces existing tags)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply mutations (default: dry preview)")
	cmd.Flags().IntVar(&maxVideos, "max", 0, "Limit videos enumerated (0 = all)")
	return cmd
}
