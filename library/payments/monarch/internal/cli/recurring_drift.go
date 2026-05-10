// Hand-authored novel feature: detect recurring streams whose latest charge
// deviates from the trailing-6-month median by more than a threshold.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newRecurringDriftCmd(flags *rootFlags) *cobra.Command {
	var flagThreshold string
	var flagMinAmount float64
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Detect recurring streams whose latest amount drifted from their trailing median",
		Long: `Flags subscriptions/bills whose most recent charge deviates from the trailing
6-month median by more than --threshold (default 5%). Catches the silent
Netflix-going-from-$15-to-$22 bumps months before scrolling statements would.`,
		Example:     "  monarch-pp-cli recurring drift --threshold 5pct --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			pct, err := parsePercent(flagThreshold)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command: "recurring drift",
					Persona: "Devon — freelancer with 22 subscriptions",
					Plan:    fmt.Sprintf("Group transactions by recurring_stream_id, compute trailing-6mo median, flag deviations > %.1f%%", pct*100),
					Operations: []string{
						"Common_GetAggregatedRecurringItems (current streams + recent charges)",
					},
					Inputs: map[string]string{"threshold": flagThreshold},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start := daysAgo(180)
			vars := map[string]any{
				"startDate": start,
				"endDate":   today(),
			}
			data, err := fetchGraphQL(c, client.RecurringListQuery, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Items []struct {
					Stream struct {
						ID        string                 `json:"id"`
						Frequency string                 `json:"frequency"`
						Merchant  *struct{ Name string } `json:"merchant"`
					} `json:"stream"`
					Date   string  `json:"date"`
					Amount float64 `json:"amount"`
					IsPast bool    `json:"isPast"`
				} `json:"recurringItems"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing recurring: %w", err))
			}
			byStream := map[string][]float64{}
			latestByStream := map[string]struct {
				Date   string
				Amount float64
				Name   string
			}{}
			for _, it := range resp.Items {
				if !it.IsPast {
					continue
				}
				abs := math.Abs(it.Amount)
				if flagMinAmount > 0 && abs < flagMinAmount {
					continue
				}
				byStream[it.Stream.ID] = append(byStream[it.Stream.ID], abs)
				prev, ok := latestByStream[it.Stream.ID]
				if !ok || it.Date > prev.Date {
					name := ""
					if it.Stream.Merchant != nil {
						name = it.Stream.Merchant.Name
					}
					latestByStream[it.Stream.ID] = struct {
						Date   string
						Amount float64
						Name   string
					}{Date: it.Date, Amount: abs, Name: name}
				}
			}
			drifts := []map[string]any{}
			for id, amounts := range byStream {
				if len(amounts) < 3 {
					continue
				}
				sort.Float64s(amounts)
				median := amounts[len(amounts)/2]
				if median <= 0 {
					continue
				}
				latest := latestByStream[id]
				drift := (latest.Amount - median) / median
				if math.Abs(drift) >= pct {
					drifts = append(drifts, map[string]any{
						"stream_id":       id,
						"merchant":        latest.Name,
						"latest_date":     latest.Date,
						"latest_amount":   latest.Amount,
						"trailing_median": median,
						"drift_pct":       math.Round(drift*1000) / 10, // %.1f
						"sample_count":    len(amounts),
					})
				}
			}
			sort.Slice(drifts, func(i, j int) bool {
				return math.Abs(drifts[i]["drift_pct"].(float64)) > math.Abs(drifts[j]["drift_pct"].(float64))
			})
			return printJSONOrTable(flags, map[string]any{
				"threshold_pct": pct * 100,
				"window":        map[string]string{"start": start, "end": today()},
				"drifts":        drifts,
				"count":         len(drifts),
			})
		},
	}
	cmd.Flags().StringVar(&flagThreshold, "threshold", "5pct", "Minimum drift percent to flag (e.g. 5pct, 10%)")
	cmd.Flags().Float64Var(&flagMinAmount, "min-amount", 0, "Ignore streams whose absolute amount is below this floor")
	return cmd
}
