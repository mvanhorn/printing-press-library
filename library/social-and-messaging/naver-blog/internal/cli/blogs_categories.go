// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `categories` command. Lists the categories a blog uses
// by sampling its recent post-list and grouping by (categoryNo,
// categoryName). The post-list endpoint already carries each item's
// categoryNo + categoryName, so we don't need a separate categories
// endpoint — we just summarize a sample.
//
// Wired as top-level `categories` (rather than `blogs categories`)
// because `blogs` is already a leaf that fetches the feed.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
)

// categoryRow is one element of the output array.
type categoryRow struct {
	CategoryNo        int    `json:"category_no"`
	CategoryName      string `json:"category_name"`
	PostCountObserved int    `json:"post_count_observed"`
}

func newCategoriesCmd(flags *rootFlags) *cobra.Command {
	var flagSampleSize int

	cmd := &cobra.Command{
		Use:   "categories <blog_id>",
		Short: "List the numbered categories a Naver blog uses, with observed post counts.",
		Long: `Sample a blog's recent post list and group the results by (categoryNo, categoryName). Returns one row per observed category, sorted by descending post count.

This is a sampling rollup — the count reflects posts the API returned in the sampled page(s), NOT every post the blog has ever filed under each category. The default sample size (--sample 30) matches Naver's per-page cap.

Use the returned category_no values with 'blogs --category N' to filter the feed to a specific category.`,
		Example: `  naver-blog-pp-cli categories selly9401
  naver-blog-pp-cli categories selly9401 --sample 90`,
		Annotations: map[string]string{
			"pp:endpoint":   "blogs.categories",
			"pp:method":     "GET",
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
			if flagSampleSize <= 0 {
				flagSampleSize = 30
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			rows, err := listBlogCategories(ctx, c, blogID, flagSampleSize)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagSampleSize, "sample", 30, "How many recent posts to sample (Naver caps a single page at ~30; >30 paginates)")
	return cmd
}

// listBlogCategories walks one or more pages of the post-list endpoint
// and rolls up (categoryNo, categoryName) -> count. We use the raw
// per-item map[string]any from fetchBlogFeed's projection input
// shape because blogFeedItem drops categoryNo. Adding it to the
// projection would break the existing `blogs` shape; reading the raw
// pages here is the lower-risk path.
func listBlogCategories(ctx context.Context, c *client.Client, blogID string, sample int) ([]categoryRow, error) {
	totals := make(map[int]*categoryRow)
	itemsPerPage := 30
	pagesNeeded := (sample + itemsPerPage - 1) / itemsPerPage
	if pagesNeeded <= 0 {
		pagesNeeded = 1
	}
	for pageNo := 1; pageNo <= pagesNeeded; pageNo++ {
		raws, err := fetchRawFeedPage(ctx, c, blogID, itemsPerPage, fmt.Sprintf("%d", pageNo))
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", pageNo, err)
		}
		if len(raws) == 0 {
			break
		}
		for _, item := range raws {
			catNo := intFieldOrZero(item, "categoryNo", "category_no")
			catName := stringFieldOrEmpty(item, "categoryName", "category_name")
			if catNo == 0 && catName == "" {
				continue
			}
			if existing, ok := totals[catNo]; ok {
				existing.PostCountObserved++
				if existing.CategoryName == "" {
					existing.CategoryName = catName
				}
				continue
			}
			totals[catNo] = &categoryRow{
				CategoryNo:        catNo,
				CategoryName:      catName,
				PostCountObserved: 1,
			}
		}
		if len(raws) < itemsPerPage {
			break
		}
	}
	out := make([]categoryRow, 0, len(totals))
	for _, r := range totals {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PostCountObserved != out[j].PostCountObserved {
			return out[i].PostCountObserved > out[j].PostCountObserved
		}
		return out[i].CategoryNo < out[j].CategoryNo
	})
	return out, nil
}

// fetchRawFeedPage hits the post-list endpoint and returns the raw
// items array without the typed blogFeedItem projection (so we can see
// fields like categoryNo that the projection drops).
func fetchRawFeedPage(ctx context.Context, c *client.Client, blogID string, itemCount int, page string) ([]map[string]any, error) {
	path := "/api/blogs/" + blogID + "/post-list"
	params := map[string]string{
		"categoryNo": "0",
		"itemCount":  fmt.Sprintf("%d", itemCount),
		"page":       page,
	}
	headers := map[string]string{
		"Referer": "https://m.blog.naver.com/" + blogID,
	}
	raw, err := c.GetWithHeaders(path, params, headers)
	if err != nil {
		return nil, err
	}
	var env struct {
		IsSuccess bool            `json:"isSuccess"`
		Result    json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decoding feed envelope: %w", err)
	}
	var result struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(env.Result, &result); err != nil {
		// Some buckets return result as a bare array.
		if err2 := json.Unmarshal(env.Result, &result.Items); err2 != nil {
			return nil, fmt.Errorf("decoding feed result: %w", err)
		}
	}
	_ = ctx
	return result.Items, nil
}

// intFieldOrZero pulls a JSON-decoded numeric field (or string-encoded
// number) from a map[string]any. Returns 0 on absence or parse failure.
func intFieldOrZero(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return int(t)
			case json.Number:
				if n, err := t.Int64(); err == nil {
					return int(n)
				}
			}
		}
	}
	return 0
}

// stringFieldOrEmpty pulls a string field; empty when absent or not a
// string.
func stringFieldOrEmpty(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
