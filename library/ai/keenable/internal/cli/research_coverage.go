// pp:data-source local

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelResearchCoverageCmd(flags *rootFlags) *cobra.Command {
	var snapshotID string
	cmd := &cobra.Command{
		Use:         "coverage",
		Short:       "Measure domain diversity, rank share, timestamp coverage, and missing metadata in a saved run.",
		Example:     "  keenable-pp-cli research coverage --snapshot latest --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "research coverage")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := openResearchStore(ctx)
			if err != nil {
				return fmt.Errorf("opening local research store: %w", err)
			}
			defer s.Close()
			snap, err := loadResearchSnapshot(s, snapshotID)
			if err != nil {
				return err
			}
			results, err := loadSnapshotResults(s, snap.ID)
			if err != nil {
				return err
			}
			domains := map[string]int{}
			missing := map[string]int{"title": 0, "description": 0, "snippet": 0, "published_at": 0, "acquired_at": 0}
			for _, result := range results {
				domains[researchDomain(result.URL)]++
				if result.Title == "" {
					missing["title"]++
				}
				if result.Description == "" {
					missing["description"]++
				}
				if result.Snippet == "" {
					missing["snippet"]++
				}
				if result.PublishedAt == "" {
					missing["published_at"]++
				}
				if result.AcquiredAt == "" {
					missing["acquired_at"]++
				}
			}
			type domainCount struct {
				Domain  string  `json:"domain"`
				Results int     `json:"results"`
				Share   float64 `json:"share"`
			}
			domainList := make([]domainCount, 0, len(domains))
			for domain, count := range domains {
				share := 0.0
				if len(results) > 0 {
					share = float64(count) / float64(len(results))
				}
				domainList = append(domainList, domainCount{domain, count, share})
			}
			sort.Slice(domainList, func(i, j int) bool { return domainList[i].Results > domainList[j].Results })
			view := map[string]any{"snapshot": snap.ID, "result_count": len(results), "unique_domains": len(domains), "domains": domainList, "missing_metadata": missing}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "snapshot %s: %d results across %d domains\n", snap.ID, len(results), len(domains))
			for _, item := range domainList {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-32s %d (%.0f%%)\n", item.Domain, item.Results, item.Share*100)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshotID, "snapshot", "latest", "Snapshot ID or latest")
	return cmd
}
