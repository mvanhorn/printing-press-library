// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type staleRow struct {
	Campaign string `json:"campaign"`
	Name     string `json:"name"`
	Status   string `json:"status,omitempty"`
}

func newNovelStaleCampaignsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var limit int

	cmd := &cobra.Command{
		Use:   "stale-campaigns",
		Short: "Find non-archived campaigns with zero delivery in the last N days",
		Long: "Cross-reference your non-archived campaigns against campaigns that actually delivered (spent) in the " +
			"last N days. Campaigns with no delivery are surfaced with their status so you can spot live-but-not-" +
			"delivering campaigns. Read-only.",
		Example:     "  stackadapt-pp-cli stale-campaigns --days 14 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "stale-campaigns", "would find non-delivering campaigns")
			}
			from, to := dateRange(days)
			recs, err := fetchCampaignDelivery(cmd.Context(), flags, map[string]any{"archived": false}, "TOTAL", from, to)
			if err != nil {
				return err
			}
			delivering := map[string]bool{}
			for _, r := range recs {
				if r.Metrics["cost"] > 0 || r.Metrics["impressionsBigint"] > 0 {
					delivering[r.CampaignID] = true
				}
			}

			data, err := runQuery(cmd.Context(), flags,
				fmt.Sprintf(`query{ campaigns(first:%d, filterBy:{archived:false}){ nodes { id name campaignStatus { state status } } } }`, limit), nil)
			if err != nil {
				return err
			}
			nodes, _, err := nodesAt(data, "campaigns")
			if err != nil {
				return err
			}
			stale := make([]staleRow, 0)
			for _, n := range nodes {
				var c struct {
					ID             json.RawMessage `json:"id"`
					Name           string          `json:"name"`
					CampaignStatus struct {
						State  string `json:"state"`
						Status string `json:"status"`
					} `json:"campaignStatus"`
				}
				_ = json.Unmarshal(n, &c)
				id := trimQuotes(c.ID)
				if delivering[id] {
					continue
				}
				status := c.CampaignStatus.Status
				if status == "" {
					status = c.CampaignStatus.State
				}
				stale = append(stale, staleRow{Campaign: id, Name: c.Name, Status: status})
			}
			sort.SliceStable(stale, func(i, j int) bool { return stale[i].Name < stale[j].Name })
			return emitView(cmd, flags, map[string]any{
				"window":          fmt.Sprintf("%s..%s", from, to),
				"stale_count":     len(stale),
				"stale_campaigns": stale,
			})
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "Days of delivery history to consider")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max non-archived campaigns to scan (raise for accounts with >500 campaigns)")
	return cmd
}
