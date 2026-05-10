// Hand-authored: convenience for `accounts networth-snapshots`. Same data,
// shorter path so users naturally reach for `networth history`.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newNetworthHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagWeeks int
	cmd := &cobra.Command{
		Use:         "history",
		Short:       "Net-worth history (asset/liability snapshots over time)",
		Example:     "  monarch-pp-cli networth history --weeks 12 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command:    "networth history",
					Plan:       fmt.Sprintf("Fetch aggregate snapshots for the last %d weeks", flagWeeks),
					Operations: []string{"Common_GetAggregateSnapshots"},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start := daysAgo(flagWeeks * 7)
			vars := map[string]any{
				"filters": map[string]any{
					"startDate": start,
					"endDate":   today(),
				},
			}
			data, err := fetchGraphQL(c, client.NetworthSnapshotsQuery, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Snapshots json.RawMessage `json:"aggregateSnapshots"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing snapshots: %w", err))
			}
			return printJSONOrTable(flags, map[string]any{
				"window":    map[string]string{"start": start, "end": today()},
				"snapshots": resp.Snapshots,
			})
		},
	}
	cmd.Flags().IntVar(&flagWeeks, "weeks", 12, "Number of weeks of history")
	return cmd
}
