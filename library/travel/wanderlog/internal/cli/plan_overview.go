// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlanOverviewCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var modes, selected string
	cmd := &cobra.Command{Use: "overview", Short: "Orient across every trip day with coordinates, schedules, travel gaps, lodging changes and global constraints", Args: cobra.NoArgs,
		Example:     "  wanderlog-pp-cli plan overview --target-key naertjcoixqrgrfc --agent",
		Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
			want, mode, err := planReadModes(modes, selected)
			if err != nil {
				return usageErr(err)
			}
			trip, key, err := readTravelPlan(cmd, flags, opts)
			if err != nil {
				return err
			}
			result, err := buildPlanOverview(trip, key, want, mode)
			if err != nil {
				return usageErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&modes, "modes", "driving,walking", "Saved estimate modes to include: driving,walking,bicycling,transit")
	cmd.Flags().StringVar(&selected, "travel-mode", "", "Explicit mode for schedule slack; omitted means unknown")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func buildPlanOverview(trip map[string]any, key string, modes []string, selected string) (map[string]any, error) {
	requested := []int{}
	for _, report := range sectionReports(trip) {
		if planningDayMode(report.Mode) {
			requested = append(requested, report.Day)
		}
	}
	all, err := buildPlanDays(trip, key, requested, modes, selected)
	if err != nil {
		return nil, err
	}
	blocks := mapField(all, "blocks")
	places := mapField(all, "places")
	fullBlocks := map[string]any{}
	// Global list notes/candidates retain complete content. They often carry
	// booking cutoffs or group constraints that cannot safely be summarized away.
	for _, context := range all["context"].([]map[string]any) {
		for _, ref := range context["block_keys"].([]string) {
			fullBlocks[ref] = blocks[ref]
		}
	}
	days := []map[string]any{}
	previousLodging := []string{}
	previousLodgingKnown := false
	detailsOmitted := false
	for _, day := range all["days"].([]map[string]any) {
		section := day["section"].(map[string]any)
		stops := []map[string]any{}
		for _, ref := range day["order_keys"].([]string) {
			block := blocks[ref].(map[string]any)
			stop := map[string]any{"block_key": ref, "id": block["id"], "type": block["type"], "name": block["name"]}
			for _, field := range []string{"startTime", "endTime", "durationMinutes", "timezone", "reservation_kind", "warnings", "issues"} {
				if value, ok := block[field]; ok {
					stop[field] = value
				}
			}
			if placeRef := stringField(block, "place_ref"); placeRef != "" {
				place := mapField(places, placeRef)
				if location, ok := place["location"]; ok {
					stop["coordinates"] = location
				} else {
					stop["coordinates"] = nil
					stop["coordinates_status"] = "unavailable"
				}
				for _, field := range []string{"place_id", "placeId", "business_status"} {
					if value, ok := place[field]; ok {
						stop[field] = value
					}
				}
			}
			if stringField(block, "reservation_kind") != "" || stringField(block, "type") == "note" || stringField(block, "type") == "checklist" {
				fullBlocks[ref] = block
				stop["details_in_blocks"] = true
			} else if fullBlocks[ref] != nil {
				stop["details_in_blocks"] = true
			} else {
				stop["details_omitted"] = true
				stop["has_note"] = strings.TrimSpace(stringField(block, "text")) != ""
				detailsOmitted = true
			}
			stops = append(stops, stop)
		}
		for _, ref := range day["reservation_keys"].([]string) {
			fullBlocks[ref] = blocks[ref]
		}
		lodging := overviewLodging(blocks, day["reservation_keys"].([]string), stringField(section, "date"))
		current := lodging["night_keys"].([]string)
		if len(days) == 0 || !previousLodgingKnown || lodging["coverage_known"] != true {
			lodging["changed_from_previous_day"] = nil
		} else {
			lodging["changed_from_previous_day"] = !reflect.DeepEqual(current, previousLodging)
		}
		previousLodging = append([]string{}, current...)
		previousLodgingKnown = lodging["coverage_known"] == true
		days = append(days, map[string]any{"section": section, "stops": stops, "reservation_keys": day["reservation_keys"], "lodging": lodging, "travel": day["travel"], "checks": day["checks"], "warnings": day["warnings"]})
	}
	// Full metadata is useful only for the shared constraints/booking records;
	// ordinary stop orientation needs coordinates and operational status above.
	usedPlaces := map[string]bool{}
	for _, block := range fullBlocks {
		collectPlanningPlaceRefs(block, usedPlaces)
	}
	overviewPlaces := map[string]any{}
	for ref := range usedPlaces {
		if value, ok := places[ref]; ok {
			overviewPlaces[ref] = value
		}
	}
	out := map[string]any{"command": "plan overview", "target_key": key, "trip_context": all["trip_context"], "days": days, "total_days": len(days), "context": all["context"], "blocks": fullBlocks, "places": overviewPlaces, "stable_ids": all["stable_ids"], "warnings": all["warnings"], "source": "saved_plan_snapshot", "freshness": "unknown", "block_details_omitted": detailsOmitted, "transfers_between_days": overviewDayTransfers(trip, modes, selected)}
	if detailsOmitted {
		out["detail_command"] = "plan days --days N[,M] for complete stop notes, contacts, opening hours and planning details"
	}
	if len(days) == 0 {
		out["warnings"] = appendUniquePlanningWarning(out["warnings"].([]string), "No itinerary day sections exist; all available section constraints and candidate places are in context.")
	}
	return out, nil
}

func overviewLodging(blocks map[string]any, keys []string, date string) map[string]any {
	nights, arrivals, departures, unknown := []string{}, []string{}, []string{}, []string{}
	for _, key := range keys {
		block := mapField(blocks, key)
		hotel := mapField(block, "hotel")
		if hotel == nil {
			continue
		}
		checkIn, checkOut := stringField(hotel, "checkIn"), stringField(hotel, "checkOut")
		_, dateOK := parseYMD(date)
		_, inOK := parseYMD(checkIn)
		_, outOK := parseYMD(checkOut)
		if !dateOK || !inOK || !outOK || checkOut <= checkIn {
			unknown = append(unknown, key)
			continue
		}
		if date == checkIn {
			arrivals = append(arrivals, key)
		}
		if date == checkOut {
			departures = append(departures, key)
		}
		if date >= checkIn && date < checkOut {
			nights = append(nights, key)
		}
	}
	sort.Strings(nights)
	_, dateOK := parseYMD(date)
	status := "recorded"
	if len(nights) == 0 {
		status = "no_recorded_lodging"
	}
	if !dateOK || len(unknown) > 0 {
		status = "unknown"
	}
	return map[string]any{"status": status, "night_keys": nights, "check_in_keys": arrivals, "check_out_keys": departures, "undated_keys": unknown, "coverage_known": dateOK && len(unknown) == 0}
}
func collectPlanningPlaceRefs(value any, refs map[string]bool) {
	switch item := value.(type) {
	case map[string]any:
		for key, value := range item {
			if strings.HasSuffix(key, "_ref") {
				if ref, ok := value.(string); ok {
					refs[ref] = true
				}
			}
			collectPlanningPlaceRefs(value, refs)
		}
	case []any:
		for _, value := range item {
			collectPlanningPlaceRefs(value, refs)
		}
	}
}

// These links expose spatial gaps between itinerary days. They do not invent
// an overnight route via lodging and never compare clock-only schedules across
// dates. Missing endpoints and missing saved estimates remain explicit.
func overviewDayTransfers(trip map[string]any, modes []string, selected string) []map[string]any {
	source, _ := indexedPlanSourceBlocks(trip)
	reports := []planSectionReport{}
	for _, report := range sectionReports(trip) {
		if planningDayMode(report.Mode) {
			reports = append(reports, report)
		}
	}
	saved, _ := planResource(trip, "distancesBetweenPlaces").(map[string]any)
	transfers := []map[string]any{}
	for i := 1; i < len(reports); i++ {
		fromSection, toSection := reports[i-1], reports[i]
		var from, to *planSourceBlock
		for j := range source {
			block := source[j].Raw
			if travelStopID(block) == "" && stringField(block, "type") != "place" && mapField(block, "place") == nil && mapField(block, "hotel") == nil {
				continue
			}
			if source[j].Section.Index == fromSection.Index {
				from = &source[j]
			}
			if source[j].Section.Index == toSection.Index && to == nil {
				to = &source[j]
			}
		}
		transfer := map[string]any{"from_day": fromSection.Day, "to_day": toSection.Day, "from_date": fromSection.Date, "to_date": toSection.Date, "source": "saved_api_estimate", "freshness": "unknown", "schedule_status": "not_checked_cross_day", "selected_mode": selected, "via_lodging_inferred": false, "link_kind": "last_first_routable_place"}
		estimates := []map[string]any{}
		if from == nil || to == nil {
			transfer["endpoints_status"] = "unavailable"
			for _, mode := range modes {
				estimates = append(estimates, map[string]any{"mode": mode, "available": false, "reason": "missing_day_endpoint"})
			}
		} else {
			transfer["endpoints_status"] = "available"
			transfer["from_block_key"] = from.Key
			transfer["to_block_key"] = to.Key
			transfer["from_place_id"] = travelStopID(from.Raw)
			transfer["to_place_id"] = travelStopID(to.Raw)
			// Transport bookings after/before these routable places make the
			// actual overnight boundary unknown; do not imply an attraction
			// route is the journey taken around an intervening flight or train.
			transportKeys := []string{}
			unknownDateKeys := []string{}
			for _, item := range source {
				if !overviewTransportReservation(item.Raw) {
					continue
				}
				boundary := item.Section.Index == fromSection.Index && item.Index >= from.Index || item.Section.Index == toSection.Index && item.Index <= to.Index
				if !planningDayMode(item.Section.Mode) {
					relevant, unknown := overviewGlobalTransportRelevant(item.Raw, fromSection.Date, toSection.Date)
					boundary = relevant && (unknown || !overviewTransportOutsideBoundary(item.Raw, fromSection.Date, toSection.Date, from.Raw, to.Raw))
					if relevant && unknown {
						unknownDateKeys = append(unknownDateKeys, item.Key)
					}
				}
				if boundary {
					transportKeys = append(transportKeys, item.Key)
				}
			}
			if len(unknownDateKeys) > 0 {
				transfer["transport_date_unknown_keys"] = unknownDateKeys
			}
			if len(transportKeys) > 0 {
				transfer["endpoints_status"] = "unknown_transport_boundary"
				transfer["intervening_transport_keys"] = transportKeys
				transfer["warnings"] = []string{"Transport reservations may intervene at the day boundary; global bookings with overlapping or unknown dates prevent assuming a direct transfer. Airport, station, port and rental routes are not inferred."}
			}
			for _, mode := range modes {
				if len(transportKeys) > 0 {
					estimates = append(estimates, map[string]any{"mode": mode, "available": false, "reason": "intervening_transport_reservation"})
				} else {
					estimates = append(estimates, savedTravelEstimate(saved, travelStopID(from.Raw), travelStopID(to.Raw), mode, from.Raw, to.Raw))
				}
			}
		}
		transfer["estimates"] = estimates
		transfers = append(transfers, transfer)
	}
	return transfers
}

func overviewTransportReservation(block map[string]any) bool {
	switch reservationKindForBlock(block) {
	case "flight", "rentalCar", "train", "bus", "ferry", "cruise":
		return true
	}
	return false
}

// Global bookings have no itinerary ordering. Conservatively treat any overlap
// with the two day dates as a possible boundary journey, including bookings
// spanning both dates. Missing/invalid dates cannot certify a direct transfer.
func overviewGlobalTransportRelevant(block map[string]any, fromDate, toDate string) (bool, bool) {
	if _, valid := parseYMD(fromDate); !valid {
		return true, true
	}
	if _, valid := parseYMD(toDate); !valid {
		return true, true
	}
	if fromDate > toDate {
		fromDate, toDate = toDate, fromDate
	}
	dates := []string{}
	unknown := false
	add := func(value any) {
		text, ok := value.(string)
		if !ok || len(text) < 10 {
			unknown = true
			return
		}
		if _, valid := parseYMD(text[:10]); !valid {
			unknown = true
			return
		}
		dates = append(dates, text[:10])
	}
	for _, key := range []string{"date", "startDate", "endDate"} {
		if value, present := block[key]; present {
			add(value)
		}
	}
	for _, key := range []string{"depart", "arrive", "pickUp", "dropOff"} {
		if value, present := block[key]; present {
			stop, ok := value.(map[string]any)
			if !ok {
				unknown = true
			} else {
				add(stop["date"])
			}
		}
	}
	for _, pair := range [][2]string{{"depart", "arrive"}, {"pickUp", "dropOff"}, {"startDate", "endDate"}} {
		_, hasStart := block[pair[0]]
		_, hasEnd := block[pair[1]]
		if hasStart != hasEnd {
			unknown = true
		}
	}
	if len(dates) == 0 || unknown {
		return true, true
	}
	sort.Strings(dates)
	return dates[0] <= toDate && dates[len(dates)-1] >= fromDate, false
}

// Explicit booking times can prove a same-day journey ended before the last
// visit or starts after the next day's first visit. Without both date and clock
// evidence, retain the conservative date-overlap guard.
func overviewTransportOutsideBoundary(booking map[string]any, fromDate, toDate string, from, to map[string]any) bool {
	fromClock := stringField(from, "endTime")
	if fromClock == "" {
		fromClock = stringField(from, "startTime")
	}
	fromMinutes, fromOK := parseClockToMinutes(fromClock, "")
	toMinutes, toOK := parseClockToMinutes(stringField(to, "startTime"), "")
	for _, key := range []string{"arrive", "dropOff"} {
		stop := mapField(booking, key)
		minutes, ok := parseClockToMinutes(stringField(stop, "time"), "")
		if fromOK && ok && stringField(stop, "date") == fromDate && minutes <= fromMinutes {
			return true
		}
	}
	for _, key := range []string{"depart", "pickUp"} {
		stop := mapField(booking, key)
		minutes, ok := parseClockToMinutes(stringField(stop, "time"), "")
		if toOK && ok && stringField(stop, "date") == toDate && minutes >= toMinutes {
			return true
		}
	}
	return false
}
