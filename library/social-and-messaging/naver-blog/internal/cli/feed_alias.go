// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Gate-1 alias: `feed <blog_id>` is shorthand for `blogs <blog_id>`.
// MCP-hidden to avoid tool duplication.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
)

func newFeedAliasCmd(flags *rootFlags) *cobra.Command {
	var (
		flagCategoryNo int
		flagItemCount  int
		flagPage       string
		flagSort       string
		flagSponsored  bool
	)

	cmd := &cobra.Command{
		Use:   "feed <blog_id>",
		Short: "Alias for 'blogs': list recent posts from a Naver blog.",
		Long: `Shorthand for the canonical 'blogs' command. Same blog_id positional, same --category / --limit / --page flags, same output projection.

Hidden from the MCP tool surface.`,
		Example: `  naver-blog-pp-cli feed selly9401
  naver-blog-pp-cli feed selly9401 --limit 10 --page 2`,
		Annotations: map[string]string{
			"pp:endpoint":   "blogs.feed",
			"pp:method":     "GET",
			"mcp:hidden":    "true",
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
				fmt.Fprintf(os.Stderr, "fetching %d post bodies for sponsored detection...\n", len(items))
				enrichWithSponsored(ctx, c, items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().IntVar(&flagCategoryNo, "category", 0, "Same as 'blogs --category'")
	cmd.Flags().IntVar(&flagItemCount, "limit", 30, "Same as 'blogs --limit'")
	cmd.Flags().StringVar(&flagPage, "page", "1", "Same as 'blogs --page'")
	cmd.Flags().StringVar(&flagSort, "sort", "recent", "Same as 'blogs --sort'")
	cmd.Flags().BoolVar(&flagSponsored, "flag-sponsored", false, "Same as 'blogs --flag-sponsored'")
	return cmd
}
