// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlanDaysCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var selection, modes, selected string
	cmd := &cobra.Command{Use: "days", Short: "Read selected complete days from one snapshot, sharing constraints, reservations and places once", Args: cobra.NoArgs,
		Example:     "  wanderlog-pp-cli plan days --target-key naertjcoixqrgrfc --days 1,3-5 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
			days, err := parsePlanDays(selection)
			if err != nil {
				return usageErr(err)
			}
			want, mode, err := planReadModes(modes, selected)
			if err != nil {
				return usageErr(err)
			}
			trip, key, err := readTravelPlan(cmd, flags, opts)
			if err != nil {
				return err
			}
			result, err := buildPlanDays(trip, key, days, want, mode)
			if err != nil {
				return usageErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&selection, "days", "", "Required comma-separated 1-based days or inclusive ranges, e.g. 1,3-5; order retained")
	cmd.Flags().StringVar(&modes, "modes", "driving,walking", "Saved estimate modes to include: driving,walking,bicycling,transit")
	cmd.Flags().StringVar(&selected, "travel-mode", "", "Explicit mode for schedule slack; omitted means unknown")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func parsePlanDays(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("--days is required, e.g. 1,3-5")
	}
	out := []int{}
	seen := map[int]bool{}
	for _, part := range strings.Split(value, ",") {
		bounds := strings.Split(strings.TrimSpace(part), "-")
		if len(bounds) > 2 {
			return nil, fmt.Errorf("invalid day range %q", part)
		}
		start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
		if err != nil || start < 1 {
			return nil, fmt.Errorf("day must be positive: %q", part)
		}
		end := start
		if len(bounds) == 2 {
			end, err = strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil || end < start {
				return nil, fmt.Errorf("invalid ascending day range %q", part)
			}
		}
		// Bound expansion before allocation; real itineraries cannot need millions
		// of sections and a mistyped upper bound must not exhaust memory.
		if end-start > 999 || len(out)+(end-start+1) > 1000 {
			return nil, fmt.Errorf("--days exceeds 1000 selected days")
		}
		for day := start; day <= end; day++ {
			if seen[day] {
				return nil, fmt.Errorf("duplicate selected day %d", day)
			}
			seen[day] = true
			out = append(out, day)
			if day == end {
				break
			}
		}
	}
	return out, nil
}

func planReadModes(modes, selected string) ([]string, string, error) {
	want, err := travelModes(modes)
	if err != nil {
		return nil, "", err
	}
	sort.Strings(want)
	selected = strings.ToLower(strings.TrimSpace(selected))
	if selected != "" {
		found := false
		for _, mode := range want {
			found = found || mode == selected
		}
		if !found {
			return nil, "", fmt.Errorf("--travel-mode must be included in --modes")
		}
	}
	return want, selected, nil
}

type planSourceBlock struct {
	Key     string
	Raw     map[string]any
	Section planSectionReport
	Index   int
}

func indexedPlanSourceBlocks(trip map[string]any) ([]planSourceBlock, bool) {
	counts := map[int]int{}
	for _, raw := range sections(trip) {
		section, _ := raw.(map[string]any)
		rows, _ := section["blocks"].([]any)
		for _, row := range rows {
			block, _ := row.(map[string]any)
			counts[intAny(block["id"])]++
		}
	}
	reports := sectionReports(trip)
	out := []planSourceBlock{}
	safe := true
	for si, raw := range sections(trip) {
		section, _ := raw.(map[string]any)
		rows, _ := section["blocks"].([]any)
		for bi, row := range rows {
			block, ok := row.(map[string]any)
			if !ok {
				safe = false
				continue
			}
			id := intAny(block["id"])
			key := strconv.Itoa(id)
			if id <= 0 || counts[id] != 1 {
				safe = false
				key = fmt.Sprintf("position:%d:%d", si, bi)
			}
			out = append(out, planSourceBlock{Key: key, Raw: block, Section: reports[si], Index: bi})
		}
	}
	return out, safe
}

func planningSectionContext(raw map[string]any, report planSectionReport) map[string]any {
	day := report.Day
	if !planningDayMode(report.Mode) {
		day = 0
	}
	out := map[string]any{"mode": report.Mode, "index": report.Index, "id": report.ID, "day": day, "date": report.Date, "heading": firstNonEmpty(report.Title, stringField(raw, "heading"), stringField(raw, "name")), "text": plainBlockText(raw)}
	if links := readableBlockLinks(raw); len(links) > 0 {
		out["links"] = links
	}
	for _, key := range []string{"startTime", "endTime", "timezone", "notes", "constraints", "options"} {
		if value, ok := raw[key]; ok {
			out[key] = value
		}
	}
	return out
}
func planningTripContext(trip map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"title", "startDate", "endDate", "timezone", "timeZone", "notes", "description", "constraints"} {
		if value, ok := trip[key]; ok {
			out[key] = value
		}
	}
	itinerary := mapField(trip, "itinerary")
	for _, key := range []string{"options", "constraints", "notes"} {
		if value, ok := itinerary[key]; ok {
			out["itinerary_"+key] = value
		}
	}
	return out
}
func planningDayMode(mode string) bool { return mode == "dayPlan" || mode == "guideDayPlan" }

// buildPlanDays uses original source positions for identity when IDs collide.
// Shared dictionaries contain content once; every day/context retains order and
// source references. Equal place IDs with unequal details stay separate through
// dayPlaceReference's content hash.
func buildPlanDays(trip map[string]any, key string, requested []int, modes []string, selected string) (map[string]any, error) {
	seen := map[int]bool{}
	resolved := []resolvedSection{}
	for _, day := range requested {
		if day < 1 || seen[day] {
			return nil, fmt.Errorf("requested days must be positive and unique: %d", day)
		}
		seen[day] = true
		sec, err := resolveSection(trip, day, -1, 0)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, sec)
	}
	source, safe := indexedPlanSourceBlocks(trip)
	blocks, places := map[string]any{}, map[string]any{}
	include := func(item planSourceBlock) {
		if blocks[item.Key] != nil {
			return
		}
		block := dayPlanningBlock(trip, item.Raw, places)
		block["source_section_index"] = item.Section.Index
		block["block_index"] = item.Index
		blocks[item.Key] = block
	}
	reports := sectionReports(trip)
	contexts := []map[string]any{}
	// All non-day sections remain explicit, including candidate place notes,
	// undated bookings and dated list constraints that are not itinerary days.
	for si, raw := range sections(trip) {
		if planningDayMode(reports[si].Mode) {
			continue
		}
		section, _ := raw.(map[string]any)
		context := planningSectionContext(section, reports[si])
		keys := []string{}
		for _, item := range source {
			if item.Section.Index == si {
				include(item)
				keys = append(keys, item.Key)
			}
		}
		context["block_keys"] = keys
		contexts = append(contexts, context)
	}
	days := []map[string]any{}
	warnings := []string{}
	if !safe {
		warnings = append(warnings, "Missing or duplicate block IDs use source-position keys. Do not use these keys as editable stable IDs.")
	}
	for _, sec := range resolved {
		order, reservations := []string{}, []string{}
		dayWarnings := []string{}
		for _, item := range source {
			if item.Section.Index == sec.Index {
				include(item)
				order = append(order, item.Key)
			}
			if reservationKindForBlock(item.Raw) == "" {
				continue
			}
			relevant, unknown := dayReservationRelevant(item.Raw, sec.Report.Date)
			if !relevant {
				continue
			}
			if unknown && item.Section.Date != "" && item.Section.Date != sec.Report.Date {
				continue
			}
			include(item)
			reservations = append(reservations, item.Key)
			if unknown {
				dayWarnings = appendUniquePlanningWarning(dayWarnings, "Undated reservation constraints are included; their exact day assignment is unknown.")
			}
		}
		travel := buildTravelLegs(trip, sec, modes, selected, false)
		for _, field := range []string{"command", "section", "stops", "target_key"} {
			delete(travel, field)
		}
		if selected == "" {
			dayWarnings = append(dayWarnings, "No travel mode selected; schedule feasibility remains unknown.")
		}
		if sec.Report.Date == "" {
			dayWarnings = append(dayWarnings, "Day has no date; calendar closure and lodging coverage are unknown.")
		}
		for _, leg := range travel["legs"].([]map[string]any) {
			for _, estimate := range leg["estimates"].([]map[string]any) {
				if estimate["available"] != true {
					dayWarnings = appendUniquePlanningWarning(dayWarnings, "Some saved travel estimates are unavailable; partial totals do not establish a complete day estimate.")
				}
			}
		}
		days = append(days, map[string]any{"section": planningSectionContext(sec.Raw, sec.Report), "order_keys": order, "reservation_keys": reservations, "travel": travel, "checks": dayChecksForSection(trip, sec), "warnings": dayWarnings})
	}
	// Other dated days are intentionally selected out, never mistaken for global
	// context; source day count makes that boundary explicit.
	total := 0
	for _, report := range reports {
		if planningDayMode(report.Mode) {
			total++
		}
	}
	return map[string]any{"command": "plan days", "target_key": key, "days": days, "blocks": blocks, "places": places, "context": contexts, "trip_context": planningTripContext(trip), "selected_days": requested, "total_days": total, "stable_ids": safe, "warnings": warnings, "source": "saved_plan_snapshot", "freshness": "unknown"}, nil
}
func appendUniquePlanningWarning(warnings []string, value string) []string {
	for _, old := range warnings {
		if old == value {
			return warnings
		}
	}
	return append(warnings, value)
}
