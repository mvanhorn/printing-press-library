// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelResearchSnapshotCmd(flags *rootFlags) *cobra.Command {
	var query, site, acquiredAfter, acquiredBefore, publishedAfter, publishedBefore, queryTime string
	var maxResults, snippetMax, fetchTop, maxChars int
	var live, authenticated bool
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Save an exact search and fetch run as an immutable local evidence snapshot.",
		Example:     "  keenable-pp-cli research snapshot --query 'AI agent evaluation methods' --max-results 8 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "research snapshot")
			}
			if strings.TrimSpace(query) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			snap, results, pages, err := saveLiveSnapshot(ctx, flags, researchSearchRequest{Query: query, Site: site, AcquiredAfter: acquiredAfter, AcquiredBefore: acquiredBefore, PublishedAfter: publishedAfter, PublishedBefore: publishedBefore, QueryTime: queryTime, MaxResults: maxResults, SnippetMax: snippetMax, Authenticated: authenticated}, fetchTop, maxChars, live, "")
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			view := map[string]any{"snapshot": snap, "results": results, "pages": pages}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "live"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved snapshot %s: %d results, %d pages\n", snap.ID, len(results), len(pages))
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Natural-language search query to snapshot")
	cmd.Flags().StringVar(&site, "site", "", "Restrict results to a domain")
	cmd.Flags().StringVar(&acquiredAfter, "acquired-after", "", "Keep pages acquired after this date or relative delta")
	cmd.Flags().StringVar(&acquiredBefore, "acquired-before", "", "Keep pages acquired before this date or relative delta")
	cmd.Flags().StringVar(&publishedAfter, "published-after", "", "Keep pages published after this date or relative delta")
	cmd.Flags().StringVar(&publishedBefore, "published-before", "", "Keep pages published before this date or relative delta")
	cmd.Flags().StringVar(&queryTime, "query-time", "", "Search the index as it stood at this instant")
	cmd.Flags().IntVar(&maxResults, "max-results", 10, "Maximum search results to save (1-50)")
	cmd.Flags().IntVar(&snippetMax, "snippet-max-length", 0, "Maximum snippet length (180-10000)")
	cmd.Flags().IntVar(&fetchTop, "fetch-top", 3, "Fetch content for the first N results")
	cmd.Flags().IntVar(&maxChars, "max-chars", 50000, "Maximum characters per fetched page")
	cmd.Flags().BoolVar(&live, "live", false, "Fetch selected pages live instead of indexed copies")
	cmd.Flags().BoolVar(&authenticated, "authenticated", false, "Use the authenticated endpoint instead of the public tier")
	return cmd
}
