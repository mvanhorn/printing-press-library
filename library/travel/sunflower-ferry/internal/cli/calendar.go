// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/internal/ferry"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func newNovelCalendarCmd(flags *rootFlags) *cobra.Command {
	var route, direction, from, to, band string
	var limit int
	cmd := &cobra.Command{Use: "calendar", Short: "Compare published seasonal fare bands across a bounded Japan date window", Long: "Read explicit source seasonal dates. E marks special daytime cruises and must not be treated as an ordinary overnight departure. Missing dates remain unknown; a fare band establishes neither an actual sailing nor inventory.", Example: "  sunflower-ferry-pp-cli calendar --route osaka-beppu --from 2026-10-01 --to 2026-10-31 --agent", Annotations: ferryAnnotations("live", "--route=osaka-beppu;--from=2026-10-15;--to=2026-10-17"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "calendar")
		}
		if e := validateDataSourceStrategy(flags, "live"); e != nil {
			return usageErr(e)
		}
		if e := ferryNoArgs(args); e != nil {
			return e
		}
		r, line, e := ferryResolve(route, direction)
		if e != nil {
			return e
		}
		start := from
		if start == "" {
			start = time.Now().In(ferry.JST).Format("2006-01-02")
		}
		a, e := ferry.ParseDate(start)
		if e != nil {
			return usageErr(e)
		}
		end := to
		if end == "" {
			end = a.AddDate(0, 0, 29).Format("2006-01-02")
		}
		b, e := ferry.ParseDate(end)
		if e != nil {
			return usageErr(e)
		}
		if b.Before(a) || b.Sub(a) > 92*24*time.Hour {
			return usageErr(fmt.Errorf("date window must be ordered and at most 93 inclusive days"))
		}
		if limit < 1 || limit > 93 {
			return usageErr(fmt.Errorf("--limit must be 1..93"))
		}
		if band != "" && !strings.Contains("ABCDE", band) || len(band) > 1 {
			return usageErr(fmt.Errorf("--band must be A, B, C, D or E"))
		}
		ctx, cancel := ferryCtx(cmd, flags)
		defer cancel()
		c := ferry.New(flags.rateLimit)
		data, e := c.Calendar(ctx, r, line, a, b)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		dates := data.Dates
		var selected = make([]ferry.SeasonDate, 0)
		for _, d := range dates {
			if band == "" || d.Band == band {
				selected = append(selected, d)
			}
		}
		matched := len(selected)
		if len(selected) > limit {
			selected = selected[:limit]
		}
		data.Dates = selected
		view := map[string]any{"calendar": data, "scanned_dates": int(b.Sub(a).Hours()/24) + 1, "matched_dates": matched, "returned_dates": len(selected), "limit": limit, "truncated": matched > limit}
		return ferryWrite(cmd, flags, view, c.Meta("en; official calendar script"), "Bands are published date classifications, not prices or inventory. Use quote for entered party/vehicle fares; E can be a same-day cruise.")
	}}
	cmd.Flags().StringVar(&route, "route", "osaka-beppu", "Official route slug or source direction code (see routes list)")
	cmd.Flags().StringVar(&direction, "direction", "outbound", "Route direction: outbound or inbound; source line IDs already fix direction")
	cmd.Flags().StringVar(&from, "from", "", "First inclusive Japan boarding date YYYY-MM-DD (default today JST)")
	cmd.Flags().StringVar(&to, "to", "", "Last inclusive date YYYY-MM-DD (default 30-day window)")
	cmd.Flags().StringVar(&band, "band", "", "Keep only this exact published seasonal band A–E")
	cmd.Flags().IntVar(&limit, "limit", 93, "Maximum returned seasonal dates, between 1 and 93")
	return cmd
}
