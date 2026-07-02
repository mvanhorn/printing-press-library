// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: browse article URLs by content category via sitemaps.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newBrowseCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "browse <category>",
		Short: "Browse article URLs for a content category",
		Long: strings.TrimSpace(`
List article URLs for a content category from The Points Guy's sitemaps.
Categories include: news, deals, credit-cards, airline, aviation, hotel,
cruise, disney, loyalty-programs, other. Read one with 'read <url>'.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli browse deals --limit 20
  thepointsguy-pp-cli browse loyalty-programs --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "news"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would browse a category")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a category is required, e.g. news, deals, credit-cards"))
			}
			if limit <= 0 {
				limit = 50
			}
			category := strings.ToLower(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)
			urls, err := c.ArticleSitemapURLs(ctx, category, limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, struct {
					Category string   `json:"category"`
					Count    int      `json:"count"`
					URLs     []string `json:"urls"`
				}{category, len(urls), urls})
			}
			for _, u := range urls {
				fmt.Fprintln(cmd.OutOrStdout(), u)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum article URLs to return")
	return cmd
}
