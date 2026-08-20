package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/drudge"
	"github.com/spf13/cobra"
)

// newHeadlinesCmd returns the command that ranks all current Drudge headlines.
func newHeadlinesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var slot string
	cmd := &cobra.Command{
		Use:         "headlines",
		Short:       "All current headlines ranked by composite editorial weight (slot + red + image).",
		Long:        "Fetch the live Drudge page, persist a local snapshot, and rank all current headlines by composite editorial weight: splash > red anywhere > column tops > column body > top-left rail.",
		Example:     "  drudgereport-pp-cli headlines --limit 10 --json --select title,slot,is_red,url",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be non-negative"))
			}

			slotFilter, err := parseDrudgeSlot(slot)
			if err != nil {
				return err
			}

			_, stories, _, err := fetchDrudge(cmd.Context())
			if err != nil {
				return err
			}

			filtered := make([]drudge.Story, 0)
			for _, story := range stories {
				if slotFilter == "" || story.Slot == slotFilter {
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
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of headlines to show (0 = no cap)")
	cmd.Flags().StringVar(&slot, "slot", "", "Filter by slot: splash, top-left, column1, column2")
	return cmd
}

func parseDrudgeSlot(raw string) (drudge.Slot, error) {
	switch raw {
	case "":
		return "", nil
	case string(drudge.SlotSplash):
		return drudge.SlotSplash, nil
	case string(drudge.SlotTopLeft):
		return drudge.SlotTopLeft, nil
	case string(drudge.SlotColumn1):
		return drudge.SlotColumn1, nil
	case string(drudge.SlotColumn2):
		return drudge.SlotColumn2, nil
	default:
		return "", usageErr(fmt.Errorf("invalid --slot %q: must be splash, top-left, column1, or column2", raw))
	}
}
