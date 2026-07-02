// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: read an article's content by URL or path.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newReadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <url-or-path>",
		Short: "Read a Points Guy article's title, byline, and body",
		Long: strings.TrimSpace(`
Fetch an article by full URL or site path and extract its title, author,
publish date, summary, and body text. Get URLs from 'search', 'latest', or
'browse'.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli read https://thepointsguy.com/loyalty-programs/monthly-valuations/
  thepointsguy-pp-cli read /loyalty-programs/monthly-valuations/ --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "/loyalty-programs/monthly-valuations/"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read an article")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an article URL or path is required (from search/latest/browse)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)
			meta, err := c.PageMetadata(ctx, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, meta)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s\n", meta.Title)
			if meta.Author != "" || meta.Published != "" {
				fmt.Fprintf(out, "  %s %s\n", meta.Author, meta.Published)
			}
			fmt.Fprintf(out, "  %s\n\n", meta.URL)
			if meta.Body != "" {
				fmt.Fprintln(out, meta.Body)
			} else if meta.Description != "" {
				fmt.Fprintln(out, meta.Description)
			}
			return nil
		},
	}
	return cmd
}
