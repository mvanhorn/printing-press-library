// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	hw "github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/hostelworld"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
	"time"
)

func newNovelHostelsSearchCmd(flags *rootFlags) *cobra.Command {
	var o stayOptions
	var city, currency string
	var page int
	cmd := &cobra.Command{Use: "search", Short: "Find a bounded dated city shortlist; summary prices are from-prices", Example: "  " + "hostelworld-pp-cli hostels search --city-id 452 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "false", "mcp:local-write": "true", "pp:live-happy-path": "true", "pp:data-source": "live", "pp:happy-args": "--city-id=452;--check-in=2026-10-04;--check-out=2026-10-07;--guests=2;--limit=5"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 || !hw.ValidID(city) {
			return usageErr(fmt.Errorf("--city-id must be a resolved numeric city ID; run destinations search first"))
		}
		q, err := o.query()
		if err != nil {
			return usageErr(err)
		}
		if o.limit > 50 {
			return usageErr(fmt.Errorf("city search --limit must be 1–50"))
		}
		if page < 1 || page > 20 {
			return usageErr(fmt.Errorf("--page must be 1–20"))
		}
		if len(currency) != 3 || strings.IndexFunc(currency, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0 {
			return usageErr(fmt.Errorf("--currency must be a three-letter uppercase source currency code"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		params := q.Params()
		params["currency"] = currency
		params["per-page"] = strconv.Itoa(o.limit)
		params["page"] = strconv.Itoa(page)
		params["show-rooms"] = "1"
		v, err := sourceObject(ctx, c, "/legacy-hwapi-service/2.2/cities/"+city+"/properties/", params)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		rows, ok := v["properties"].([]any)
		if !ok {
			return fmt.Errorf("city search response schema changed: properties missing")
		}
		location, _ := v["location"].(map[string]any)
		locationCity, _ := location["city"].(map[string]any)
		cityName := hw.Text(locationCity["name"])
		out := []any{}
		for _, x := range rows {
			r, _ := x.(map[string]any)
			if o.kind == "dorm" && r["lowestAverageDormPricePerNight"] == nil {
				continue
			}
			if o.kind == "private" && r["lowestAveragePrivatePricePerNight"] == nil {
				continue
			}
			id := hw.Text(r["id"])
			if o.free {
				available, _ := r["freeCancellationAvailable"].(bool)
				deadline, e := time.Parse(time.RFC3339, hw.Text(r["freeCancellationAvailableUntil"]))
				if !available || e != nil || !deadline.After(time.Now()) {
					continue
				}
			}
			if !hw.ValidID(id) {
				return fmt.Errorf("city search response schema changed: property ID missing")
			}
			out = append(out, map[string]any{"id": id, "name": r["name"], "type": r["type"], "source_rating": r["overallRating"], "distance": r["distance"], "dorm_from_per_bed_per_night": r["lowestAverageDormPricePerNight"], "private_from_per_room_per_night": r["lowestAveragePrivatePricePerNight"], "cancellation_scope": "property_summary_only; fetch hostels offers to establish rate terms", "free_cancellation_available": r["freeCancellationAvailable"], "free_cancellation_deadline": r["freeCancellationAvailableUntil"], "stay_rule_violations": r["stayRuleViolations"], "booking_url": hw.BookingURL(id, hw.Text(r["name"]), cityName, q)})
			if len(out) >= o.limit {
				break
			}
		}
		return outputPlanning(cmd, flags, map[string]any{"query": q, "city_id": city, "location": v["location"], "results": out, "pagination": v["pagination"], "returned": len(out), "scanned_pages": 1, "kind": o.kind, "filter_scope": "matching non-null source from-prices on one bounded source page; not complete city coverage", "cancellation_scope": "property_summary_only; no selected-rate refund guarantee", "source_order_preserved": true, "observed_at": time.Now().UTC().Format(time.RFC3339), "price_status": "dated discovery from-prices; inspect actual plans with hostels offers before comparing party costs"}, o.save)
	}}
	decoratePlanning(cmd, flags)
	stayFlags(cmd, &o)
	cmd.Flags().Lookup("free-cancellation").Usage = "Keep property-summary free-cancellation signals with an unexpired deadline; fetch offers to establish rate terms"
	cmd.Flags().Lookup("limit").Usage = "Maximum discovery results from one source page, 1–50; default 30"
	cmd.Flags().StringVar(&city, "city-id", "", "Exact city ID returned by destinations search")
	cmd.Flags().StringVar(&currency, "currency", "JPY", "Requested source currency for discovery prices")
	cmd.Flags().IntVar(&page, "page", 1, "One source page to fetch, 1–20; pagination is never followed automatically")

	return cmd
}
