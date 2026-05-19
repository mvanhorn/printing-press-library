// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Gate-1 alias: `hashtag <tags>` is shorthand for
// `find hashtag --tags <tags>`. Tags are passed as a positional
// (comma-separated) rather than via --tags. MCP-hidden to avoid tool
// duplication with the canonical command.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/serpparse"
)

func newHashtagAliasCmd(flags *rootFlags) *cobra.Command {
	var (
		flagRequireAll bool
		flagMonth      string
		flagLimit      int
	)

	cmd := &cobra.Command{
		Use:   "hashtag <tag1>[,<tag2>,...]",
		Short: "Alias for 'find hashtag': search Naver Blog by one or more hashtags.",
		Long: `Shorthand for the canonical 'find hashtag' command with the tag list as a positional argument rather than --tags. Same union (default) / intersection (--require-all) semantics.

Hidden from the MCP tool surface.`,
		Example: `  naver-blog-pp-cli hashtag 협찬,체험단
  naver-blog-pp-cli hashtag 칠리,여성청결제 --require-all`,
		Annotations: map[string]string{
			"pp:endpoint":   "find.hashtag",
			"pp:method":     "GET",
			"mcp:hidden":    "true",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			tagsRaw := strings.Join(args, ",")
			tags := splitTags(tagsRaw)
			if len(tags) == 0 {
				return usageErr(fmt.Errorf("no valid tags after parsing %q", tagsRaw))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			var results []serpparse.SearchResult
			if flagRequireAll {
				results, err = runHashtagIntersection(ctx, c, tags, "m_view", flagLimit)
			} else {
				results, err = runHashtagUnion(ctx, c, tags, "m_view", flagLimit)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			_ = flagMonth
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().BoolVar(&flagRequireAll, "require-all", false, "Same as 'find hashtag --require-all'")
	cmd.Flags().StringVar(&flagMonth, "month", "", "Same as 'find hashtag --month'")
	cmd.Flags().IntVar(&flagLimit, "limit", 22, "Same as 'find hashtag --limit'")
	return cmd
}
