// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelResearchFetchManyCmd(flags *rootFlags) *cobra.Command {
	var urls []string
	var maxChars, concurrency int
	var live, authenticated bool
	cmd := &cobra.Command{
		Use:         "fetch-many",
		Short:       "Fetch a bounded URL list with concurrency limits and explicit partial failures.",
		Example:     "  keenable-pp-cli research fetch-many --url https://docs.keenable.ai/api-reference --url https://docs.keenable.ai/mcp-server --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "research fetch-many")
			}
			clean := make([]string, 0, len(urls))
			for _, rawURL := range urls {
				if strings.TrimSpace(rawURL) != "" {
					clean = append(clean, rawURL)
				}
			}
			if len(clean) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one --url is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			pages, failures := fetchMany(ctx, flags, clean, maxChars, concurrency, live, authenticated)
			snap := researchSnapshot{ID: newResearchSnapshotID(strings.Join(clean, "\n")), Query: "fetch-many", CreatedAt: "now", AuthMode: map[bool]string{true: "authenticated", false: "public"}[authenticated], FetchedCount: len(pages)}
			s, err := openResearchStore(ctx)
			if err != nil {
				return fmt.Errorf("open research store: %w", err)
			}
			if err := persistResearchSnapshot(s, snap, nil, pages); err != nil {
				_ = s.Close()
				return fmt.Errorf("persist research snapshot: %w", err)
			}
			if err := s.Close(); err != nil {
				return fmt.Errorf("close research store: %w", err)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed; %d pages saved\n", len(failures), len(clean), len(pages))
			}
			view := map[string]any{"snapshot": snap.ID, "pages": pages, "fetch_failures": failures, "requested": len(clean), "succeeded": len(pages)}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "live"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %d/%d pages in snapshot %s\n", len(pages), len(clean), snap.ID)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&urls, "url", nil, "URL to fetch; repeat for multiple pages")
	cmd.Flags().IntVar(&maxChars, "max-chars", 50000, "Maximum characters per fetched page")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Maximum concurrent fetches (1-8)")
	cmd.Flags().BoolVar(&live, "live", false, "Fetch live pages instead of indexed copies")
	cmd.Flags().BoolVar(&authenticated, "authenticated", false, "Use authenticated fetch endpoints")
	return cmd
}
