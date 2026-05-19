// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `blogs-info` command. Fetches profile-level metadata from
// Naver's mobile blog metadata endpoint.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/blogapi"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
)

func newBlogsInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blogs-info <blog_id>",
		Short: "Get Naver Blog profile metadata and reach signals.",
		Long: `Get rich profile metadata for a Naver Blog ID through Naver's mobile blog metadata endpoint.

Use this to find subscriber count, daily/total visitor stats, Power Blog status,
official directory category, and profile/cover image URLs.

Accepts any of:
  - bare slug:            selly9401
  - mobile homepage URL:  https://m.blog.naver.com/selly9401
  - desktop homepage URL: https://blog.naver.com/selly9401
  - any post URL:         https://blog.naver.com/selly9401/224234460263
  - PostList.naver URL:   https://blog.naver.com/PostList.naver?blogId=selly9401`,
		Example: `  naver-blog-pp-cli blogs-info selly9401 --agent
  naver-blog-pp-cli blogs-info perfect62 --select blog_id,subscriber_count,power_blog,directory_name`,
		Annotations: map[string]string{
			"pp:endpoint":   "blogs.info",
			"pp:method":     "GET",
			"pp:path":       "/api/blogs/{blog_id}",
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
			// Accept either a bare blog_id slug ("selly9401") or any
			// Naver Blog URL shape (mobile/desktop homepage, post URL,
			// PostList.naver, PostView.naver). ExtractBlogID does the
			// canonicalization so the user doesn't have to.
			blogID, ok := naverurl.ExtractBlogID(args[0])
			if !ok {
				return usageErr(fmt.Errorf("blog_id is required (accepts slug or any Naver Blog URL)"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			info, err := blogapi.GetBlogInfoLimited(ctx, c.HTTPClient, blogID, c.Limiter())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), info, flags)
		},
	}
	return cmd
}
