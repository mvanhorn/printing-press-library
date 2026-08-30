// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelResearchReplayCmd(flags *rootFlags) *cobra.Command {
	var snapshotID string
	var fetchTop, maxChars int
	var live bool
	cmd := &cobra.Command{
		Use:         "replay",
		Short:       "Rerun a saved research recipe and report how current evidence changed.",
		Example:     "  keenable-pp-cli research replay --snapshot latest --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "research replay")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := openResearchStore(ctx)
			if err != nil {
				return fmt.Errorf("opening local research store: %w", err)
			}
			defer s.Close()
			baseline, err := loadResearchSnapshot(s, snapshotID)
			if err != nil {
				return err
			}
			if baseline.Query == "" {
				view := map[string]any{"baseline": "latest", "replay": "", "added": 0, "removed": 0, "result_count": 0, "fetched_count": 0, "note": "no saved snapshot yet; run research snapshot first"}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view["note"])
				return nil
			}
			before, err := loadSnapshotResults(s, baseline.ID)
			if err != nil {
				return err
			}
			snap, current, pages, err := saveLiveSnapshot(ctx, flags, researchSearchRequest{Query: baseline.Query, Site: baseline.Site, AcquiredAfter: baseline.AcquiredAfter, AcquiredBefore: baseline.AcquiredBefore, PublishedAfter: baseline.PublishedAfter, PublishedBefore: baseline.PublishedBefore, QueryTime: baseline.QueryTime, MaxResults: baseline.ResultCount, Authenticated: baseline.AuthMode == "authenticated"}, fetchTop, maxChars, live, "")
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			oldURLs, newURLs := map[string]bool{}, map[string]bool{}
			for _, item := range before {
				oldURLs[item.URL] = true
			}
			for _, item := range current {
				newURLs[item.URL] = true
			}
			added, removed := 0, 0
			for rawURL := range newURLs {
				if !oldURLs[rawURL] {
					added++
				}
			}
			for rawURL := range oldURLs {
				if !newURLs[rawURL] {
					removed++
				}
			}
			view := map[string]any{"baseline": baseline.ID, "replay": snap.ID, "added": added, "removed": removed, "result_count": len(current), "fetched_count": len(pages)}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "live"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "replayed %s as %s: %d results, %d pages, +%d/- %d URLs\n", baseline.ID, snap.ID, len(current), len(pages), added, removed)
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshotID, "snapshot", "latest", "Snapshot ID to replay")
	cmd.Flags().IntVar(&fetchTop, "fetch-top", 3, "Fetch content for the first N results")
	cmd.Flags().IntVar(&maxChars, "max-chars", 50000, "Maximum characters per fetched page")
	cmd.Flags().BoolVar(&live, "live", false, "Fetch selected pages live instead of indexed copies")
	return cmd
}
