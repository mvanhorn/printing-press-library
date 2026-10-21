// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type topPinRow struct {
	PinID   string         `json:"pin_id"`
	Metrics map[string]any `json:"metrics"`
}

type funnelRow struct {
	PinID             string  `json:"pin_id"`
	Impressions       float64 `json:"impressions"`
	Saves             float64 `json:"saves"`
	OutboundClicks    float64 `json:"outbound_clicks"`
	PinClicks         float64 `json:"pin_clicks"`
	SaveRate          float64 `json:"save_rate"`
	OutboundClickRate float64 `json:"outbound_click_rate"`
	SavedNotClicked   bool    `json:"saved_not_clicked"`
}

func computeFunnel(pins []topPinRow) []funnelRow {
	rows := make([]funnelRow, 0, len(pins))
	valid := make([]int, 0, len(pins))
	for _, p := range pins {
		value := func(key string) float64 {
			if n, ok := p.Metrics[key].(float64); ok {
				return n
			}
			return 0
		}
		r := funnelRow{PinID: p.PinID, Impressions: value("IMPRESSION"), Saves: value("SAVE"), OutboundClicks: value("OUTBOUND_CLICK"), PinClicks: value("PIN_CLICK")}
		if r.Impressions > 0 {
			r.SaveRate = math.Round(r.Saves/r.Impressions*10000) / 10000
			r.OutboundClickRate = math.Round(r.OutboundClicks/r.Impressions*10000) / 10000
			valid = append(valid, len(rows))
		}
		rows = append(rows, r)
	}
	if len(valid) < 4 {
		return rows
	}
	saves, clicks := make([]float64, 0, len(valid)), make([]float64, 0, len(valid))
	for _, i := range valid {
		saves = append(saves, rows[i].SaveRate)
		clicks = append(clicks, rows[i].OutboundClickRate)
	}
	sort.Float64s(saves)
	sort.Float64s(clicks)
	q75 := saves[int(math.Ceil(.75*float64(len(saves))))-1]
	q25 := clicks[int(math.Ceil(.25*float64(len(clicks))))-1]
	for _, i := range valid {
		// Strict on saves so a flat all-zero distribution flags nothing.
		rows[i].SavedNotClicked = rows[i].SaveRate > 0 && rows[i].SaveRate >= q75 && rows[i].OutboundClickRate <= q25 && rows[i].OutboundClickRate < rows[i].SaveRate
	}
	return rows
}

func newNovelPinsFunnelCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	var sortBy string
	allowed := []string{"IMPRESSION", "SAVE", "OUTBOUND_CLICK", "PIN_CLICK", "ENGAGEMENT"}
	cmd := &cobra.Command{Use: "funnel", Short: "Rank Pins by save and outbound click rates", Long: "Use this command for per-Pin rates and rankings.\nDo NOT use this command for account totals over time; use 'perf compare' instead.", Example: strings.Trim(`
  pinterest-pp-cli pins funnel --days=30 --limit=10
`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--days=30;--limit=10"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "pins funnel")
		}
		if days < 1 || days > 90 {
			return usageErr(fmt.Errorf("--days must be between 1 and 90"))
		}
		if limit < 1 || limit > 50 {
			return usageErr(fmt.Errorf("--limit must be between 1 and 50"))
		}
		sortBy = strings.ToUpper(sortBy)
		if !containsString(allowed, sortBy) {
			return usageErr(fmt.Errorf("--sort-by must be one of %s", strings.Join(allowed, ", ")))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		end := nowUTC().AddDate(0, 0, -1)
		start := end.AddDate(0, 0, -days+1)
		path := appendArrayQueryParam("/user_account/analytics/top_pins", "metric_types", "IMPRESSION,SAVE,OUTBOUND_CLICK,PIN_CLICK", "form", false)
		raw, err := c.Get(ctx, path, map[string]string{"start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02"), "sort_by": sortBy, "num_of_pins": fmt.Sprint(limit)})
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		var response struct {
			Pins []topPinRow `json:"pins"`
		}
		if err = json.Unmarshal(raw, &response); err != nil {
			return fmt.Errorf("parsing top Pins: %w", err)
		}
		rows := computeFunnel(response.Pins)
		result := map[string]any{"start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02"), "sort_by": sortBy, "pins": rows}
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}}
	cmd.Flags().IntVar(&days, "days", 30, "Number of recent analytics days to include (1-90)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of ranked Pins to return (1-50)")
	cmd.Flags().StringVar(&sortBy, "sort-by", "IMPRESSION", "Metric used to sort the returned Pin rankings")
	return cmd
}
