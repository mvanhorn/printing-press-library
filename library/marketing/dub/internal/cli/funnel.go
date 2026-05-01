// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type funnelStep struct {
	Stage string  `json:"stage"`
	Count int     `json:"count"`
	Pct   float64 `json:"pct_of_clicks"`
}

func newFunnelCmd(flags *rootFlags) *cobra.Command {
	var linkID string
	var interval string

	cmd := &cobra.Command{
		Use:   "funnel",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Click → lead → sale conversion rates for a link or workspace",
		Long: `Show clicks, leads, and sales counts plus the conversion percentage at each
stage. Useful for finding where prospects drop off in the funnel.

Live API call: queries /events three times (event=click, lead, sale) for the
specified scope and interval.`,
		Example: `  # Workspace-wide funnel for the last 30 days
  dub-pp-cli funnel --interval 30d --json

  # Funnel for a specific link
  dub-pp-cli funnel --link link_abc --interval 7d --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			fetch := func(event string) (int, error) {
				params := map[string]string{
					"event":    event,
					"groupBy":  "count",
					"interval": interval,
				}
				if linkID != "" {
					params["linkId"] = linkID
				}
				resp, err := c.Get("/analytics", params)
				if err != nil {
					return 0, classifyAPIError(err)
				}
				return parseAnalyticsCount(resp, event), nil
			}

			clicks, err := fetch("clicks")
			if err != nil {
				return err
			}
			leads, err := fetch("leads")
			if err != nil {
				return err
			}
			sales, err := fetch("sales")
			if err != nil {
				return err
			}

			out := []funnelStep{
				{Stage: "clicks", Count: clicks, Pct: 100},
				{Stage: "leads", Count: leads, Pct: pctOf(leads, clicks)},
				{Stage: "sales", Count: sales, Pct: pctOf(sales, clicks)},
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			headers := []string{"STAGE", "COUNT", "% OF CLICKS"}
			rows := [][]string{
				{"clicks", fmt.Sprintf("%d", clicks), "100.0%"},
				{"leads", fmt.Sprintf("%d", leads), fmt.Sprintf("%.1f%%", pctOf(leads, clicks))},
				{"sales", fmt.Sprintf("%d", sales), fmt.Sprintf("%.1f%%", pctOf(sales, clicks))},
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&linkID, "link", "", "Restrict to a specific link ID (workspace-wide if omitted)")
	cmd.Flags().StringVar(&interval, "interval", "30d", "Time interval (e.g. 24h, 7d, 30d, all)")
	return cmd
}

// parseAnalyticsCount handles both shapes Dub's /analytics returns:
//   - {"clicks": 123} (single-row count)
//   - [{"clicks": 123}] (one-element array)
//   - [{"count": 123}]
func parseAnalyticsCount(raw json.RawMessage, event string) int {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		if v, ok := arr[0]["count"]; ok {
			return toInt(v)
		}
		if v, ok := arr[0][event]; ok {
			return toInt(v)
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		if v, ok := obj["count"]; ok {
			return toInt(v)
		}
		if v, ok := obj[event]; ok {
			return toInt(v)
		}
	}
	return 0
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

func pctOf(n, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}
