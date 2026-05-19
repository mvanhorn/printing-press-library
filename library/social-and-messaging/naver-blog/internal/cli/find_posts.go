// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written implementation of `find posts`. Drives Naver's mobile
// integrated-view search (m.search.naver.com) and extracts every
// m.blog.naver.com/<id>/<n> hit on the page via serpparse.

package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/serpparse"
)

const naverSERPBaseURL = "https://m.search.naver.com/search.naver"

func newFindPostsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWhere string
		flagQuery string
		flagMonth string
		flagLimit int
	)

	cmd := &cobra.Command{
		Use:   "posts",
		Short: "Find Naver Blog posts by keyword via Naver integrated search.",
		Long: `Drive Naver's mobile integrated-view search (m.search.naver.com/search.naver) and extract every m.blog.naver.com/<blog_id>/<log_no> hit on the SERP. Returns each hit's rank, URL, blog_id, log_no, title, and snippet.

The SERP does not surface a per-result upload date, so --month is applied downstream by 'posts get' after fetching the publish date for each hit. If you pass --month here without a follow-up resolve step, no filtering happens — the flag is recorded for downstream pipelines.

--query is required. Korean queries should be passed UTF-8; the CLI URL-encodes automatically.`,
		Example: `  naver-blog-pp-cli find posts --query "칠리 협찬"
  naver-blog-pp-cli find posts --query 마라탕 --limit 10`,
		Annotations: map[string]string{
			"pp:endpoint":   "find.posts",
			"pp:method":     "GET",
			"pp:path":       naverSERPBaseURL,
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before required-flag validation so verify
			// dry-run probes succeed without forcing a sample query.
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagQuery) == "" {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "query is required",
						"usage": fmt.Sprintf("%s --query <text>", cmd.CommandPath()),
					}, flags)
				}
				return usageErr(fmt.Errorf("required flag --query not set"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			results, err := runSERPSearch(ctx, c, flagQuery, flagWhere, flagLimit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			_ = flagMonth // applied downstream; documented in --help
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagWhere, "where", "m_view", "Naver search vertical (default 'm_view' = mobile integrated view)")
	cmd.Flags().StringVar(&flagQuery, "query", "", "Search query (Korean preferred; UTF-8). Required.")
	cmd.Flags().StringVar(&flagMonth, "month", "", "Optional YYYY-MM filter applied downstream by 'posts get'")
	cmd.Flags().IntVar(&flagLimit, "limit", 22, "Max results to return (1-22; one SERP page)")
	return cmd
}

// runSERPSearch fetches one SERP page and runs serpparse.ParseSERP
// over the bytes. Applies --limit by truncating the result slice.
func runSERPSearch(ctx context.Context, c *client.Client, query, where string, limit int) ([]serpparse.SearchResult, error) {
	q := url.Values{}
	q.Set("where", where)
	q.Set("query", query)
	absURL := naverSERPBaseURL + "?" + q.Encode()
	html, err := getHTMLBytes(c, absURL)
	if err != nil {
		return nil, err
	}
	results, err := serpparse.ParseSERP(html, query)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	_ = ctx
	return results, nil
}
