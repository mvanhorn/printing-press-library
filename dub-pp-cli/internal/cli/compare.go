package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagGroupBy string
	var flagEvent string
	var flagDomain string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare analytics between two time periods",
		Long:  "Compare click, lead, and sale analytics between the current period and the previous period of equal length.",
		Example: `  dub-pp-cli compare --days 30
  dub-pp-cli compare --days 7 --group-by countries
  dub-pp-cli compare --days 30 --event sales --domain dub.sh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			currentStart := now.AddDate(0, 0, -flagDays)
			prevStart := currentStart.AddDate(0, 0, -flagDays)

			q1 := map[string]string{
				"groupBy": flagGroupBy,
				"start":   currentStart.Format("2006-01-02"),
				"end":     now.Format("2006-01-02"),
			}
			if flagEvent != "" {
				q1["event"] = flagEvent
			}
			if flagDomain != "" {
				q1["domain"] = flagDomain
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "GET /analytics (current: %s to %s)\n", q1["start"], q1["end"])
				fmt.Fprintf(cmd.OutOrStdout(), "GET /analytics (previous: %s to %s)\n", prevStart.Format("2006-01-02"), currentStart.Format("2006-01-02"))
				return nil
			}

			currentResp, err := c.Get("/analytics", q1)
			if err != nil {
				return fmt.Errorf("fetching current period: %w", err)
			}

			q2 := map[string]string{
				"groupBy": flagGroupBy,
				"start":   prevStart.Format("2006-01-02"),
				"end":     currentStart.Format("2006-01-02"),
			}
			if flagEvent != "" {
				q2["event"] = flagEvent
			}
			if flagDomain != "" {
				q2["domain"] = flagDomain
			}

			prevResp, err := c.Get("/analytics", q2)
			if err != nil {
				return fmt.Errorf("fetching previous period: %w", err)
			}

			if flags.asJSON {
				result := map[string]json.RawMessage{
					"current":  currentResp,
					"previous": prevResp,
				}
				return flags.printJSON(cmd, result)
			}

			var currentCount, prevCount float64
			json.Unmarshal(currentResp, &currentCount)
			json.Unmarshal(prevResp, &prevCount)

			delta := currentCount - prevCount
			var pctChange string
			if prevCount > 0 {
				pct := (delta / prevCount) * 100
				pctChange = fmt.Sprintf("%+.1f%%", pct)
			} else if currentCount > 0 {
				pctChange = "+∞"
			} else {
				pctChange = "0%"
			}

			headers := []string{"PERIOD", "VALUE", "DELTA", "CHANGE"}
			rows := [][]string{
				{
					fmt.Sprintf("%s to %s", prevStart.Format("2006-01-02"), currentStart.Format("2006-01-02")),
					fmt.Sprintf("%.0f", prevCount),
					"",
					"(baseline)",
				},
				{
					fmt.Sprintf("%s to %s", currentStart.Format("2006-01-02"), now.Format("2006-01-02")),
					fmt.Sprintf("%.0f", currentCount),
					fmt.Sprintf("%+.0f", delta),
					pctChange,
				},
			}

			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of days per period")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "count", "Analytics groupBy dimension")
	cmd.Flags().StringVar(&flagEvent, "event", "clicks", "Event type: clicks, leads, sales, composite")
	cmd.Flags().StringVar(&flagDomain, "domain", "", "Filter by domain")

	return cmd
}
