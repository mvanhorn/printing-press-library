// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Gate-1 alias: `post <url>` is shorthand for `posts <url>` /
// `posts <blog_id> <log_no>`. Delegates to the same fetch path used
// by the canonical `posts` command (fetchSinglePost) so behavior is
// identical. Marked MCP-hidden to avoid surfacing two tools for the
// same operation.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newPostAliasCmd(flags *rootFlags) *cobra.Command {
	var flagSponsored bool
	var flagIncludeComments bool

	cmd := &cobra.Command{
		Use:   "post <url>",
		Short: "Alias for 'posts': fetch a single Naver Blog post.",
		Long: `Shorthand for the canonical 'posts' command. Same argument shapes (one URL or two positional blog_id/log_no), same output, same --flag-sponsored and --include-comments options.

Hidden from the MCP tool surface to avoid duplicating the 'posts' agent tool — both invoke the same fetch pipeline. Prefer 'posts' in scripts so the canonical name stays load-bearing.`,
		Example: `  naver-blog-pp-cli post https://m.blog.naver.com/selly9401/224234460263
  naver-blog-pp-cli post selly9401 224234460263 --flag-sponsored
  naver-blog-pp-cli post perfect62 224286416663 --include-comments --json`,
		Annotations: map[string]string{
			"pp:endpoint":   "posts.get",
			"pp:method":     "GET",
			"mcp:hidden":    "true",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before any arg validation so verify dry-run
			// probes succeed without forcing a sample URL or blog_id/log_no.
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			blogID, logNo, err := parsePostArgs(args)
			if err != nil {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": err.Error(),
						"usage": fmt.Sprintf("%s <blog_id> <log_no> | <url>", cmd.CommandPath()),
					}, flags)
				}
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
			out, err := fetchSinglePostWithOptions(ctx, c, blogID, logNo, postFetchOptions{
				FlagSponsored:   flagSponsored,
				IncludeComments: flagIncludeComments,
			})
			if err != nil {
				return classifyCommentAPIError(err, flags)
			}
			recordPostEngagement(ctx, blogID, logNo, out)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&flagSponsored, "flag-sponsored", false, "Same as 'posts --flag-sponsored'")
	cmd.Flags().BoolVar(&flagIncludeComments, "include-comments", false, "Same as 'posts --include-comments'")
	return cmd
}
