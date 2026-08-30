// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelResearchLocalSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "local-search [query]",
		Short:       "Search saved titles, snippets, URLs, and Markdown without spending an upstream request.",
		Example:     "  keenable-pp-cli research local-search 'retrieval evaluation' --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "query=retrieval evaluation"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "research local-search")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("query is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			path := researchDBPath()
			if _, err := os.Stat(path); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local research mirror at %s\nrun: keenable-pp-cli research snapshot --query %q\n", path, args[0])
				return printJSONFiltered(cmd.OutOrStdout(), make([]any, 0), flags)
			}
			s, err := openResearchStore(ctx)
			if err != nil {
				return fmt.Errorf("opening local research store: %w", err)
			}
			defer s.Close()
			results, err := s.Search(args[0], limit, "research_results")
			if err != nil {
				return fmt.Errorf("searching saved results: %w", err)
			}
			pages, err := s.Search(args[0], limit, "research_pages")
			if err != nil {
				return fmt.Errorf("searching saved pages: %w", err)
			}
			view := map[string]any{"query": args[0], "results": results, "pages": pages, "source": "local"}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "local matches for %q: %d results, %d pages\n", args[0], len(results), len(pages))
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matches from each saved collection")
	return cmd
}
