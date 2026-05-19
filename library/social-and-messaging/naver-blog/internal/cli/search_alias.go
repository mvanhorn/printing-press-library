// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Gate-1 alias: `search <query>` is shorthand for `find posts --query <query>`.
// The query is a positional rather than a flag for ergonomic parity
// with "search-with-a-string" UX. MCP-hidden to avoid tool duplication.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSearchAliasCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWhere string
		flagMonth string
		flagLimit int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Alias for 'find posts': keyword search against Naver mobile SERP.",
		Long: `Shorthand for the canonical 'find posts' command with the query as a positional argument rather than --query. Same SERP path, same per-hit output shape (rank, url, blog_id, log_no, title, snippet).

Hidden from the MCP tool surface.`,
		Example: `  naver-blog-pp-cli search "칠리 협찬"
  naver-blog-pp-cli search 마라탕 --limit 10`,
		Annotations: map[string]string{
			"pp:endpoint":   "find.posts",
			"pp:method":     "GET",
			"mcp:hidden":    "true",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			if strings.TrimSpace(query) == "" {
				return usageErr(fmt.Errorf("query must be non-empty"))
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
			results, err := runSERPSearch(ctx, c, query, flagWhere, flagLimit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			_ = flagMonth
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagWhere, "where", "m_view", "Same as 'find posts --where'")
	cmd.Flags().StringVar(&flagMonth, "month", "", "Same as 'find posts --month'")
	cmd.Flags().IntVar(&flagLimit, "limit", 22, "Same as 'find posts --limit'")
	return cmd
}
