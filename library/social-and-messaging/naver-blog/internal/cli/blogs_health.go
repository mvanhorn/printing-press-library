// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `blogs-health` command. Given a file of blog IDs,
// returns per-blog rollups computed off the recent post feed: count
// of posts in the window, median sympathy (likes), median comments,
// optional sponsored ratio, days since the last post.
//
// Wired as a top-level `blogs-health` (rather than `blogs health`)
// for the same reason posts-diff is top-level: the `blogs` command is
// a leaf and the task brief warns against making it a parent.

package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/postparse"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/sponsored"
)

// blogHealthRow is one row of the output array. SponsoredRatio is a
// pointer so the "not computed" case (no --include-sponsored) emits
// JSON null, distinct from a real 0.0 ratio.
type blogHealthRow struct {
	BlogID            string   `json:"blog_id"`
	PostsInWindow     int      `json:"posts_in_window"`
	MedianLikes       float64  `json:"median_likes"`
	MedianComments    float64  `json:"median_comments"`
	SponsoredRatio    *float64 `json:"sponsored_ratio"`
	DaysSinceLastPost *float64 `json:"days_since_last_post"`
	Errors            []string `json:"errors,omitempty"`
}

func newBlogsHealthCmd(flags *rootFlags) *cobra.Command {
	var (
		flagIDsFile          string
		flagWindow           string
		flagIncludeSponsored bool
	)

	cmd := &cobra.Command{
		Use:   "blogs-health",
		Short: "Per-blog activity rollup for a list of Naver Blog IDs.",
		Long: `Reads a list of Naver Blog IDs from --ids-file (one per line; '#' starts a comment) and returns a rollup row per blog:
  - posts_in_window     count of posts published in the last --window
  - median_likes        median sympathyCnt across those posts
  - median_comments     median commentCnt across those posts
  - sponsored_ratio     fraction whose body contains KFTC disclosure markers
                        (NULL unless --include-sponsored is set; we won't
                        silently report 0 just because we didn't check)
  - days_since_last_post  hours-since-last / 24 (NULL when the blog has
                          no posts in the most-recent fetch page)

--include-sponsored fetches the mobile HTML for each post in window and
runs sponsored.DetectSponsored — adds N HTTP calls per blog (where N =
posts in window). Off by default to keep this command cheap.`,
		Example: `  naver-blog-pp-cli blogs-health --ids-file influencers.txt
  naver-blog-pp-cli blogs-health --ids-file influencers.txt --window 14d --include-sponsored`,
		Annotations: map[string]string{
			"pp:endpoint":   "blogs.health",
			"pp:method":     "GET",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before required-flag validation so verify
			// dry-run probes succeed without forcing a sample ids file.
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagIDsFile) == "" {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "ids-file is required",
						"usage": fmt.Sprintf("%s --ids-file <path>", cmd.CommandPath()),
					}, flags)
				}
				return usageErr(fmt.Errorf("required flag --ids-file not set"))
			}
			window, err := parseSinceWindow(flagWindow)
			if err != nil {
				return usageErr(err)
			}
			ids, err := readBlogIDsFile(flagIDsFile)
			if err != nil {
				return usageErr(err)
			}
			if len(ids) == 0 {
				return usageErr(fmt.Errorf("no blog IDs found in %q", flagIDsFile))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			rows, errs := cliutil.FanoutRun(
				ctx,
				ids,
				func(id string) string { return id },
				func(ctx context.Context, id string) (blogHealthRow, error) {
					return computeBlogHealth(ctx, c, id, window, flagIncludeSponsored)
				},
				cliutil.WithConcurrency(5),
			)
			cliutil.FanoutReportErrors(os.Stderr, errs)
			out := make([]blogHealthRow, 0, len(rows)+len(errs))
			for _, r := range rows {
				out = append(out, r.Value)
			}
			// Surface error-bearing rows so consumers still see an
			// envelope for every requested id.
			for _, e := range errs {
				out = append(out, blogHealthRow{
					BlogID: e.Source,
					Errors: []string{e.Err.Error()},
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagIDsFile, "ids-file", "", "Path to a file with one blog_id per line. '#' starts a comment. Required.")
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Window for posts_in_window / days_since_last_post (Go duration; supports Nd suffix)")
	cmd.Flags().BoolVar(&flagIncludeSponsored, "include-sponsored", false, "Fetch each post's HTML and compute the sponsored ratio. Default off (sponsored_ratio will be null).")
	return cmd
}

// readBlogIDsFile reads one blog_id per line. Lines starting with '#'
// and blank lines are skipped. Returns the IDs in file order.
func readBlogIDsFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening --ids-file: %w", err)
	}
	defer f.Close()
	out := make([]string, 0)
	seen := make(map[string]bool)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scanning --ids-file: %w", err)
	}
	return out, nil
}

// computeBlogHealth pulls one page of the blog feed and rolls up the
// stats for posts within window.
func computeBlogHealth(ctx context.Context, c *client.Client, blogID string, window time.Duration, includeSponsored bool) (blogHealthRow, error) {
	row := blogHealthRow{BlogID: blogID}
	items, err := fetchBlogFeed(ctx, c, blogID, 0, 30, "1", "recent")
	if err != nil {
		return row, fmt.Errorf("fetch feed: %w", err)
	}
	cutoffMs := time.Now().Add(-window).UnixMilli()
	var inWindow []blogFeedItem
	var latestMs int64
	for _, item := range items {
		if item.AddDate > 0 && item.AddDate > latestMs {
			latestMs = item.AddDate
		}
		if item.AddDate >= cutoffMs {
			inWindow = append(inWindow, item)
		}
	}
	row.PostsInWindow = len(inWindow)
	if latestMs > 0 {
		hoursSince := time.Since(time.UnixMilli(latestMs)).Hours()
		days := hoursSince / 24
		row.DaysSinceLastPost = &days
	}
	if len(inWindow) == 0 {
		return row, nil
	}
	likes := make([]float64, 0, len(inWindow))
	comments := make([]float64, 0, len(inWindow))
	for _, item := range inWindow {
		likes = append(likes, float64(item.SympathyCount))
		comments = append(comments, float64(item.CommentCount))
	}
	row.MedianLikes = median(likes)
	row.MedianComments = median(comments)
	if includeSponsored {
		ratio, sponsorErrs := computeSponsoredRatio(ctx, c, inWindow)
		row.SponsoredRatio = &ratio
		if len(sponsorErrs) > 0 {
			row.Errors = append(row.Errors, sponsorErrs...)
		}
	}
	return row, nil
}

// computeSponsoredRatio fetches each in-window post's HTML and
// returns the fraction whose body matches sponsored.DetectSponsored.
// Returns 0 when no post HTML was fetchable; sponsorErrs collects
// per-post fetch/parse failures for the caller to surface in row.Errors.
func computeSponsoredRatio(ctx context.Context, c *client.Client, items []blogFeedItem) (float64, []string) {
	if len(items) == 0 {
		return 0, nil
	}
	sponsoredHits := 0
	checked := 0
	var errs []string
	for _, it := range items {
		htmlBytes, err := getHTMLBytes(c, it.URL)
		if err != nil {
			errs = append(errs, fmt.Sprintf("sponsored fetch %s: %v", it.LogNo, err))
			continue
		}
		meta, err := postparse.ParseMobilePost(htmlBytes)
		if err != nil {
			errs = append(errs, fmt.Sprintf("sponsored parse %s: %v", it.LogNo, err))
			continue
		}
		isSponsored, _ := sponsored.DetectSponsored(meta.BodyText)
		checked++
		if isSponsored {
			sponsoredHits++
		}
		_ = ctx
	}
	if checked == 0 {
		return 0, errs
	}
	return float64(sponsoredHits) / float64(checked), errs
}

// median sorts a copy of in and returns the middle element (or the
// mean of the two middle elements when len is even). Returns 0 on
// empty input.
func median(in []float64) float64 {
	if len(in) == 0 {
		return 0
	}
	sorted := append([]float64(nil), in...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
