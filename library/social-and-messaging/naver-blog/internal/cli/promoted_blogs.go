// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written implementation of the `blogs` command. Lists posts
// from a single blog via the /api/blogs/<blog_id>/post-list JSON
// endpoint, unwraps the {"isSuccess":true,"result":{"items":[...]}}
// envelope, and emits each post in a stable shape.
//
// Optional --flag-sponsored fans out per-item to mobile-post fetches
// for KFTC disclosure detection. Off by default because the cost is
// linear in --limit (N extra HTTP calls).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/postparse"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/sponsored"
)

// blogFeedItem mirrors one entry in the result.items array. We only
// project the high-gravity fields and drop the noisy ones (encoded
// HTML preview etc.) to keep the agent envelope readable.
type blogFeedItem struct {
	BlogID           string   `json:"blog_id"`
	LogNo            string   `json:"log_no"`
	URL              string   `json:"url"`
	Title            string   `json:"title"`
	BriefContents    string   `json:"brief_contents,omitempty"`
	MemoLog          bool     `json:"memo_log"`
	PlaceName        string   `json:"place_name,omitempty"`
	MarketPost       bool     `json:"market_post"`
	VideoPlayTime    int      `json:"video_play_time,omitempty"`
	IsVRThumbnail    bool     `json:"is_vr_thumbnail"`
	IsVideoThumbnail bool     `json:"is_video_thumbnail"`
	CategoryNo       int      `json:"category_no"`
	CategoryName     string   `json:"category_name,omitempty"`
	SympathyCount    int      `json:"sympathy_cnt"`
	CommentCount     int      `json:"comment_cnt"`
	ShareCount       int      `json:"share_cnt"`
	ReadCount        *int     `json:"read_count"`
	OpenGraphLink    string   `json:"open_graph_link,omitempty"`
	ScrapType        int      `json:"scrap_type"`
	AddDate          int64    `json:"add_date,omitempty"`
	ThumbnailURL     string   `json:"thumbnail_url,omitempty"`
	Sponsored        *bool    `json:"sponsored,omitempty"`
	SponsoredMarkers []string `json:"sponsored_markers,omitempty"`
}

// rawFeedResponse describes the API's wrapper shape. The press's
// spec set response_path to result.items so a generated extractor
// would already cope, but this hand-written command parses it
// directly to keep the typed projection clean.
type rawFeedResponse struct {
	IsSuccess bool            `json:"isSuccess"`
	Result    json.RawMessage `json:"result"`
}

type rawFeedResult struct {
	Items []rawFeedItem `json:"items"`
	Page  int           `json:"page,omitempty"`
}

type rawFeedItem struct {
	LogNo                   json.Number `json:"logNo"`
	BlogNo                  json.Number `json:"blogNo"`
	DomainIDOrBlogID        string      `json:"domainIdOrBlogId"`
	TitleWithInspectMessage string      `json:"titleWithInspectMessage"`
	Title                   string      `json:"title"`
	BriefContents           string      `json:"briefContents"`
	MemoLog                 bool        `json:"memoLog"`
	PlaceName               string      `json:"placeName"`
	MarketPost              bool        `json:"marketPost"`
	VideoPlayTime           int         `json:"videoPlayTime"`
	IsVRThumbnail           bool        `json:"isVRThumbnail"`
	IsVideoThumbnail        bool        `json:"isVideoThumbnail"`
	CategoryNo              int         `json:"categoryNo"`
	SympathyCount           int         `json:"sympathyCnt"`
	CommentCount            int         `json:"commentCnt"`
	ShareCount              int         `json:"shareCnt"`
	ReadCount               *int        `json:"readCount"`
	OpenGraphLink           string      `json:"openGraphLink"`
	ScrapType               int         `json:"scrapType"`
	AddDate                 json.Number `json:"addDate"`
	CategoryName            string      `json:"categoryName"`
	ThumbnailURL            string      `json:"thumbnailUrl"`
}

func newBlogsPromotedCmd(flags *rootFlags) *cobra.Command {
	var (
		flagCategoryNo int
		flagItemCount  int
		flagPage       string
		flagSort       string
		flagSponsored  bool
	)

	cmd := &cobra.Command{
		Use:   "blogs <blog_id>",
		Short: "List recent posts from a Naver blog with engagement metrics already populated.",
		Long: `List recent posts from a single Naver blog. Calls /api/blogs/<blog_id>/post-list and returns the unwrapped result.items array, projected to the stable shape used across this CLI: blog_id, log_no, url, title, brief_contents, memo_log, place_name, market_post, video_play_time, is_vr_thumbnail, is_video_thumbnail, category_no, category_name, sympathy_cnt, comment_cnt, share_cnt, read_count, open_graph_link, scrap_type, add_date, thumbnail_url.

Pagination via --page (1-indexed). --category filters server-side by Naver category number (run 'blogs categories' to enumerate categories for a blog — added in Pass 2). --sort accepts recent (default) or best — but Naver's mobile post-list endpoint appears to ignore the parameter in practice (the response order is identical for recent and best on every blog we've tested). Treat --sort best as best-effort and don't rely on different ranking; if you need engagement-ordered output, sort the JSON locally on sympathy_cnt or comment_cnt.

Pass --flag-sponsored to additionally fetch each post's body and detect KFTC sponsorship-disclosure markers. WARNING: this adds N extra HTTP calls (one per item in --limit), so enable only when you actually need the flag.`,
		Example: `  naver-blog-pp-cli blogs selly9401
  naver-blog-pp-cli blogs selly9401 --limit 10 --page 2
  naver-blog-pp-cli blogs selly9401 --category 11 --flag-sponsored
  naver-blog-pp-cli blogs selly9401 --sort best`,
		Annotations: map[string]string{
			"pp:endpoint":   "blogs.feed",
			"pp:method":     "GET",
			"pp:path":       "/api/blogs/{blog_id}/post-list",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before any arg validation so verify dry-run
			// probes succeed without forcing a sample blog_id.
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			// Accept slug or any Naver Blog URL (homepage, post, PostList, etc.)
			blogID, ok := naverurl.ExtractBlogID(args[0])
			if !ok {
				return usageErr(fmt.Errorf("blog_id is required (accepts slug or any Naver Blog URL)"))
			}
			sortMode, err := normalizeBlogSort(flagSort)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			items, err := fetchBlogFeed(ctx, c, blogID, flagCategoryNo, flagItemCount, flagPage, sortMode)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flagSponsored {
				fmt.Fprintf(os.Stderr, "fetching %d post bodies for sponsored detection (this adds %d HTTP calls)...\n", len(items), len(items))
				enrichWithSponsored(ctx, c, items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().IntVar(&flagCategoryNo, "category", 0, "Filter by Naver category number (0 = all)")
	cmd.Flags().IntVar(&flagItemCount, "limit", 30, "Max posts per page (Naver caps at ~30)")
	cmd.Flags().StringVar(&flagPage, "page", "1", "Page number (1-indexed)")
	cmd.Flags().StringVar(&flagSort, "sort", "recent", "Sort feed by recent or best (best-effort; Naver semantics are undocumented)")
	cmd.Flags().BoolVar(&flagSponsored, "flag-sponsored", false, "Detect KFTC sponsorship markers per post (adds N HTTP calls; default off)")
	return cmd
}

// fetchBlogFeed calls the post-list endpoint and projects the JSON
// response into typed feedItem values.
func fetchBlogFeed(ctx context.Context, c *client.Client, blogID string, categoryNo int, itemCount int, page string, sortMode string) ([]blogFeedItem, error) {
	path := "/api/blogs/" + blogID + "/post-list"
	// categoryNo MUST be present in the URL or Naver returns 404
	// (data_is_not_exist). 0 means "all categories" and is the default.
	// Verified via direct curl A/B with and without the param.
	params := map[string]string{
		"categoryNo": fmt.Sprintf("%d", categoryNo),
	}
	if sortMode != "" {
		params["sort"] = sortMode
	}
	if itemCount != 0 {
		params["itemCount"] = fmt.Sprintf("%d", itemCount)
	}
	if page != "" {
		params["page"] = page
	}
	// Naver's feed JSON endpoint requires a Referer matching the blog's mobile
	// home page; without it Naver returns 403 even though the endpoint is
	// otherwise anonymous. Verified via direct curl A/B with and without Referer.
	headers := map[string]string{
		"Referer": "https://m.blog.naver.com/" + blogID,
	}
	raw, err := c.GetWithHeaders(path, params, headers)
	if err != nil {
		return nil, err
	}
	var envelope rawFeedResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decoding feed envelope: %w", err)
	}
	var result rawFeedResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		// Some buckets return result as an array directly. Try that
		// shape before giving up.
		var items []rawFeedItem
		if err2 := json.Unmarshal(envelope.Result, &items); err2 != nil {
			return nil, fmt.Errorf("decoding feed result: %w", err)
		}
		result.Items = items
	}
	out := make([]blogFeedItem, 0, len(result.Items))
	for _, raw := range result.Items {
		item := projectFeedItem(blogID, raw)
		out = append(out, item)
	}
	return out, nil
}

func normalizeBlogSort(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "recent", nil
	}
	switch raw {
	case "recent", "best":
		return raw, nil
	default:
		return "", fmt.Errorf("invalid --sort %q: must be recent or best", raw)
	}
}

func projectFeedItem(blogID string, raw rawFeedItem) blogFeedItem {
	logNo := raw.LogNo.String()
	title := raw.TitleWithInspectMessage
	if title == "" {
		title = raw.Title
	}
	return blogFeedItem{
		BlogID:           blogID,
		LogNo:            logNo,
		URL:              naverurl.MobileURL(blogID, logNo),
		Title:            title,
		BriefContents:    raw.BriefContents,
		MemoLog:          raw.MemoLog,
		PlaceName:        raw.PlaceName,
		MarketPost:       raw.MarketPost,
		VideoPlayTime:    raw.VideoPlayTime,
		IsVRThumbnail:    raw.IsVRThumbnail,
		IsVideoThumbnail: raw.IsVideoThumbnail,
		CategoryNo:       raw.CategoryNo,
		CategoryName:     raw.CategoryName,
		SympathyCount:    raw.SympathyCount,
		CommentCount:     raw.CommentCount,
		ShareCount:       raw.ShareCount,
		ReadCount:        raw.ReadCount,
		OpenGraphLink:    raw.OpenGraphLink,
		ScrapType:        raw.ScrapType,
		AddDate:          numberInt64(raw.AddDate),
		ThumbnailURL:     raw.ThumbnailURL,
	}
}

func numberInt64(n json.Number) int64 {
	if n == "" {
		return 0
	}
	v, err := n.Int64()
	if err != nil {
		return 0
	}
	return v
}

// enrichWithSponsored fetches each item's mobile-post body and writes
// the sponsored fields in-place. Errors per item are logged and the
// item is left without the flag (caller can re-run targeted at the
// single failed log_no).
func enrichWithSponsored(ctx context.Context, c *client.Client, items []blogFeedItem) {
	for i := range items {
		mobileURL := naverurl.MobileURL(items[i].BlogID, items[i].LogNo)
		htmlBytes, err := getHTMLBytes(c, mobileURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: sponsored fetch %s/%s: %v\n", items[i].BlogID, items[i].LogNo, err)
			continue
		}
		meta, err := postparse.ParseMobilePost(htmlBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: sponsored parse %s/%s: %v\n", items[i].BlogID, items[i].LogNo, err)
			continue
		}
		isSponsored, markers := sponsored.DetectSponsored(meta.BodyText)
		items[i].Sponsored = &isSponsored
		items[i].SponsoredMarkers = markers
		_ = ctx
	}
}
