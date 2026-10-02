// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/internal/ferry"
	"github.com/spf13/cobra"
)

func newNovelSailingsCmd(flags *rootFlags) *cobra.Command {
	const sailingsOnly = true
	var route, direction, date, cabinID string
	var limit int
	party := ferry.Party{}
	name := "sailings"
	cmd := &cobra.Command{Use: "sailings", Short: "Discover the source's dated sailings with exact Japan departure and overnight arrival", Long: "Run only the official anonymous trip-details/fare lookup. The response preserves source season, discount, ship and cabin/availability evidence for the entered party. No cabin selection, waitlist registration, reservation, hold, personal data or payment step is accessible. Availability remains a snapshot. Rooms absent for this party may be ineligible or unavailable; absence proves neither cause.", Example: "  sunflower-ferry-pp-cli " + name + " --route osaka-beppu --date 2026-10-15 --adults 1 --mode foot --agent", Annotations: ferryAnnotations("live", "--route=osaka-beppu;--date=2026-10-15;--adults=1;--mode=foot"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, name)
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
		if _, e = ferry.ParseDate(date); e != nil {
			return usageErr(fmt.Errorf("--date is required: %w", e))
		}
		if e = party.Validate(); e != nil {
			return usageErr(e)
		}
		if limit < 1 || limit > 40 {
			return usageErr(fmt.Errorf("--limit must be 1..40"))
		}
		if len(cabinID) > 120 {
			return usageErr(fmt.Errorf("cabin identifier is too long"))
		}
		ctx, cancel := ferryCtx(cmd, flags)
		defer cancel()
		c := ferry.New(flags.rateLimit)
		data, e := c.Quote(ctx, r, line, date, party)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		if sailingsOnly {
			for i := range data.Sailings {
				data.Sailings[i].CabinFares = nil
			}
			return ferryWrite(cmd, flags, data, c.Meta("en"), "Sailings are observed for this source lookup and input party; not all-party inventory. Recommended terminal arrival uses the English 60-minute guidance; confirm booking-specific requirements.")
		}
		matched := 0
		returned := 0
		for i := range data.Sailings {
			rows := make([]ferry.CabinFare, 0)
			for _, f := range data.Sailings[i].CabinFares {
				if cabinID == "" || f.ID == cabinID {
					matched++
					if len(rows) < limit {
						rows = append(rows, f)
						returned++
					}
				}
			}
			data.Sailings[i].CabinFares = rows
		}
		view := map[string]any{"quote": data, "matched_cabin_rows": matched, "returned_cabin_rows": returned, "limit_per_sailing": limit, "truncated": matched > returned}
		if matched == 0 {
			view["note"] = "No matching source cabin fare for this entered party and cabin filter; no fare or availability inferred."
		}
		return ferryWrite(cmd, flags, view, c.Meta("en"), "Source displayed prices are not an itemized final payable quote. No reservation or hold is made. Meal, fuel/tax/fee breakdown and special-ticket eligibility require confirmation.")
	}}
	cmd.Flags().StringVar(&route, "route", "osaka-beppu", "Official route slug or source direction code (see routes list)")
	cmd.Flags().StringVar(&direction, "direction", "outbound", "Route direction: outbound or inbound; source line IDs already fix direction")
	cmd.Flags().StringVar(&date, "date", "", "Exact Japan boarding date YYYY-MM-DD within the source booking window")
	cmd.Flags().IntVar(&party.Adults, "adults", 1, "Junior-high-or-older adults (school stage matters; at least 1)")
	cmd.Flags().IntVar(&party.Children, "children", 0, "Elementary-school children, including elementary students aged 12")
	cmd.Flags().IntVar(&party.Toddlers, "toddlers", 0, "Preschool children aged one year or older")
	cmd.Flags().IntVar(&party.Infants, "infants", 0, "Infants less than one year old")
	cmd.Flags().StringVar(&party.Mode, "mode", "foot", "Travel mode: foot, one car, or two-wheeled bike category")
	cmd.Flags().StringVar(&party.CarCategory, "car-category", "", "Car length category lt3m, lt4m, lt5m or lt6m (strict upper bound)")
	cmd.Flags().StringVar(&party.BikeCategory, "bike-category", "", "Two-wheeled category over750cc, le750cc, le125cc or bicycle")
	cmd.Flags().IntVar(&party.Bikes, "bikes", 0, "Number of two-wheeled vehicles in the selected category, at most adults")
	cmd.Flags().IntVar(&party.PetCages, "pet-cages", 0, "Medium pet cages, between zero and two; contact operator for other sizes")
	cmd.Flags().IntVar(&limit, "limit", 40, "Maximum cabin rows returned per sailing, between 1 and 40")
	if !sailingsOnly {
		cmd.Flags().StringVar(&cabinID, "cabin-id", "", "Exact stable source-derived cabin ID from a previous quote; filters after lookup")
	}
	return cmd
}
