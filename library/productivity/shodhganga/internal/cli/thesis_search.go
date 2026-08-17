// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: rich DSpace-backed thesis search (replaces the generated
// html_extract baseline). Not generated — generate --force preserves this file.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/dspace"
)

func newThesisSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagQuery string
		flagRpp   int
		flagStart int
	)
	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search theses by keyword across all of Shodhganga",
		Example: "  shodhganga-pp-cli thesis search --query \"black hole physics\" --limit 10",
		Annotations: map[string]string{
			"pp:endpoint": "thesis.search", "pp:method": "GET", "pp:path": "/simple-search",
			"mcp:read-only": "true", "pp:happy-args": "--query=physics;--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagQuery == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required"))
			}
			// verify runs in mock mode with no network; return a valid empty result.
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), &dspace.SearchResult{Hits: []dspace.SearchHit{}}, flags)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newDSpaceClient(flags)
			if err != nil {
				return err
			}
			res, err := c.Search(ctx, flagQuery, flagRpp, flagStart)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d results (showing %d) for %q:\n", res.Total, len(res.Hits), flagQuery)
			for _, h := range res.Hits {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", h.Handle, h.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Search keywords")
	cmd.Flags().IntVar(&flagRpp, "limit", 10, "Results per page")
	cmd.Flags().IntVar(&flagStart, "start", 0, "Result offset for paging")
	return cmd
}
