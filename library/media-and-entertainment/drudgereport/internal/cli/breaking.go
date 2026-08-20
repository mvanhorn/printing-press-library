package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/drudge"
	"github.com/spf13/cobra"
)

// newBreakingCmd returns the command that lists red Drudge headlines.
func newBreakingCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "breaking",
		Short:       "Every headline currently set in red by Drudge's editor, ordered by slot importance.",
		Long:        "Fetch the live Drudge page, persist a local snapshot, and list every headline currently set in red by Drudge's editor, ordered by slot importance.",
		Example:     "  drudgereport-pp-cli breaking --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be non-negative"))
			}

			_, stories, _, err := fetchDrudge(cmd.Context())
			if err != nil {
				return err
			}

			filtered := make([]drudge.Story, 0)
			for _, story := range stories {
				if story.IsRed {
					filtered = append(filtered, story)
				}
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				left := drudge.SlotRank(filtered[i].Slot, filtered[i].SlotIndex, filtered[i].IsRed, filtered[i].HasImage)
				right := drudge.SlotRank(filtered[j].Slot, filtered[j].SlotIndex, filtered[j].IsRed, filtered[j].HasImage)
				return left > right
			})
			if limit > 0 && limit < len(filtered) {
				filtered = filtered[:limit]
			}

			results := make([]map[string]any, 0)
			for _, story := range filtered {
				results = append(results, drudgeStoryResult(story, nil))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, story := range results {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s  (%s)\n", story["slot"], story["title"], story["url"])
				}
				return nil
			}

			raw, err := json.Marshal(results)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of red headlines to show (0 = no cap)")
	return cmd
}
