// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// driftWeek is one week of campaign rates plus its delta against the prior
// (older) week. Deltas are zero for the oldest week in the window.
type driftWeek struct {
	WeekStart   string  `json:"week_start"`
	WeekEnd     string  `json:"week_end"`
	Sent        int     `json:"sent"`
	OpenRate    float64 `json:"open_rate"`
	ReplyRate   float64 `json:"reply_rate"`
	BounceRate  float64 `json:"bounce_rate"`
	OpenDelta   float64 `json:"open_rate_delta"`
	ReplyDelta  float64 `json:"reply_rate_delta"`
	BounceDelta float64 `json:"bounce_rate_delta"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var campaignID string
	var weeks int

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Week-over-week open/reply/bounce drift for a campaign",
		Long: strings.Trim(`
Track how a campaign's open, reply, and bounce rates move week over week. The
analytics-by-date endpoint is queried once per seven-day window and the
week-over-week deltas are computed here — the SmartLead API has no trend
endpoint. A negative reply-rate delta is a campaign decaying.`, "\n"),
		Example: strings.Trim(`
  smartlead-pp-cli drift --campaign 3344703
  smartlead-pp-cli drift --campaign 3344703 --weeks 6 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if campaignID == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if weeks < 2 {
				return usageErr(fmt.Errorf("--weeks must be at least 2 to compute drift"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := nowUTC()
			// Collect oldest -> newest so each week can delta against the prior.
			ordered := make([]driftWeek, 0, weeks)
			for i := weeks - 1; i >= 0; i-- {
				end := now.AddDate(0, 0, -7*i)
				start := end.AddDate(0, 0, -7)
				w := driftWeek{
					WeekStart: start.Format("2006-01-02"),
					WeekEnd:   end.Format("2006-01-02"),
				}
				raw, gerr := c.Get("/campaigns/"+campaignID+"/analytics-by-date", map[string]string{
					"start_date": w.WeekStart,
					"end_date":   w.WeekEnd,
				})
				if gerr != nil {
					return classifyAPIError(gerr, flags)
				}
				var obj map[string]any
				if json.Unmarshal(raw, &obj) != nil {
					return apiErr(fmt.Errorf("unexpected analytics response for %s..%s", w.WeekStart, w.WeekEnd))
				}
				sent := asInt(obj["sent_count"])
				opens := asInt(obj["unique_open_count"])
				if opens == 0 {
					opens = asInt(obj["open_count"])
				}
				w.Sent = sent
				w.OpenRate = rate(opens, sent)
				w.ReplyRate = rate(asInt(obj["reply_count"]), sent)
				w.BounceRate = rate(asInt(obj["bounce_count"]), sent)
				ordered = append(ordered, w)
			}

			for i := 1; i < len(ordered); i++ {
				prev := ordered[i-1]
				ordered[i].OpenDelta = round4(ordered[i].OpenRate - prev.OpenRate)
				ordered[i].ReplyDelta = round4(ordered[i].ReplyRate - prev.ReplyRate)
				ordered[i].BounceDelta = round4(ordered[i].BounceRate - prev.BounceRate)
			}

			// Present newest first.
			out := make([]driftWeek, len(ordered))
			for i, w := range ordered {
				out[len(ordered)-1-i] = w
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "WEEK ENDING\tSENT\tOPEN%\tREPLY%\tBOUNCE%\tREPLY Δ")
			for _, w := range out {
				fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\t%.1f\t%+.1f\n",
					w.WeekEnd, w.Sent, w.OpenRate*100, w.ReplyRate*100,
					w.BounceRate*100, w.ReplyDelta*100)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&campaignID, "campaign", "", "Campaign ID to analyze (required)")
	cmd.Flags().IntVar(&weeks, "weeks", 4, "Number of trailing weeks to compare")
	return cmd
}

// round4 rounds a rate delta to 4 decimal places.
func round4(v float64) float64 {
	if v < 0 {
		return -float64(int(-v*10000+0.5)) / 10000
	}
	return float64(int(v*10000+0.5)) / 10000
}
