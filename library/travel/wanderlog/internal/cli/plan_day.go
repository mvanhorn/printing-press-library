// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const planDayStateVersion = 2

type planDayQuery struct {
	Day                 int      `json:"day"`
	Modes               []string `json:"modes"`
	SelectedMode        string   `json:"selected_mode"`
	ClientSchemaVersion int      `json:"client_schema_version"`
}

type planDayState struct {
	SchemaVersion int            `json:"schema_version"`
	TargetKey     string         `json:"target_key"`
	Query         planDayQuery   `json:"query"`
	Digest        string         `json:"digest"`
	Snapshot      map[string]any `json:"snapshot"`
}

func newNovelPlanDayCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var modes, selected, since, save string
	cmd := &cobra.Command{Use: "day", Short: "Read one complete planning day, with shared place details, reservations and saved travel estimates", Args: cobra.NoArgs,
		Long: "Fetch one plan snapshot and return the selected day's complete notes, schedules, booking constraints, saved travel estimates and checks. No travel mode is assumed. --save-state writes private local content with mode 0600; --since compares against that content digest, not a server cursor. Missing, invalid, incompatible or ambiguous state returns a full snapshot with a reason. Schema 2 deltas omit unchanged fields: the consumer must load the complete persisted baseline matching base_digest, inherit omitted values (including warnings), apply replacements, and verify digest. CLI access to a state file does not restore model context; omit --since when that baseline is unavailable to the consumer.",
		Example: "  wanderlog-pp-cli plan day --target-key naertjcoixqrgrfc --day 1 --agent\n" +
			"  wanderlog-pp-cli plan day --target-key YOUR_TRIP_KEY --day 1 --travel-mode walking --save-state day-state.json --agent\n" +
			"  wanderlog-pp-cli plan day --target-key YOUR_TRIP_KEY --day 1 --travel-mode walking --since day-state.json --save-state day-state.json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
			if (since != "" || save != "") && (flags.selectFields != "" || flags.quiet || flags.csv || flags.plain) {
				return usageErr(fmt.Errorf("--since/--save-state require complete JSON output; omit --select, --quiet, --csv and --plain"))
			}
			if opts.day < 1 {
				return usageErr(fmt.Errorf("--day must be a positive day number; use plan outline"))
			}
			want, err := travelModes(modes)
			if err != nil {
				return usageErr(err)
			}
			sort.Strings(want)
			selected = strings.ToLower(strings.TrimSpace(selected))
			if selected != "" {
				found := false
				for _, mode := range want {
					found = found || mode == selected
				}
				if !found {
					return usageErr(fmt.Errorf("--travel-mode must be included in --modes"))
				}
			}
			trip, key, err := readTravelPlan(cmd, flags, opts)
			if err != nil {
				return err
			}
			snapshot, err := buildPlanDay(trip, key, opts.day, want, selected)
			if err != nil {
				return usageErr(err)
			}
			state, err := makePlanDayState(key, planDayQuery{Day: opts.day, Modes: want, SelectedMode: selected, ClientSchemaVersion: opts.clientSchemaVersion}, snapshot)
			if err != nil {
				return err
			}
			var previous *planDayState
			fallback := ""
			if since != "" {
				previous, fallback = readPlanDayState(since)
			}
			report := planDayResponse(state, previous, fallback)
			if save != "" {
				if err := savePlanDayState(save, state); err != nil {
					return fmt.Errorf("save day state: %w", err)
				}
				report["state_saved"] = true
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.day, "day", 0, "Required 1-based itinerary day")
	cmd.Flags().StringVar(&modes, "modes", "driving,walking", "Saved estimate modes to include: driving,walking,bicycling,transit")
	cmd.Flags().StringVar(&selected, "travel-mode", "", "Explicit mode for schedule slack; omitted means unknown")
	cmd.Flags().StringVar(&since, "since", "", "Compare against a previous local day-state file; incompatible state returns full output")
	cmd.Flags().StringVar(&save, "save-state", "", "Atomically save private day content and digest for the next --since call")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func buildPlanDay(trip map[string]any, key string, day int, modes []string, selected string) (map[string]any, error) {
	sec, err := resolveSection(trip, day, -1, 0)
	if err != nil {
		return nil, err
	}
	places := map[string]any{}
	blocks := map[string]any{}
	order, reservations := []int{}, []int{}
	orderKeys := []string{}
	warnings := []string{}
	idsSafe := true
	add := func(raw map[string]any, section planSectionReport, context string) string {
		id := intAny(raw["id"])
		storageKey := strconv.Itoa(id)
		if id <= 0 || blocks[storageKey] != nil {
			idsSafe = false
			storageKey = fmt.Sprintf("position:%d:%d", section.Index, len(blocks))
		}
		block := dayPlanningBlock(trip, raw, places)
		block["scope"] = context
		if context != "day" {
			block["source_section_id"] = section.ID
		}
		blocks[storageKey] = block
		if context == "day" {
			order = append(order, id)
			orderKeys = append(orderKeys, storageKey)
		} else if context == "reservation" || context == "undated_reservation" {
			reservations = append(reservations, id)
		}
		return storageKey
	}
	for _, raw := range sec.Blocks {
		if block, ok := raw.(map[string]any); ok {
			add(block, sec.Report, "day")
		}
	}
	// Reservations often live in a non-day section. Preserve date-matching and
	// undated booking constraints so a slim day view does not hide a flight.
	reports := sectionReports(trip)
	for i, raw := range sections(trip) {
		if i == sec.Index {
			continue
		}
		section, _ := raw.(map[string]any)
		rows, _ := section["blocks"].([]any)
		for _, value := range rows {
			block, ok := value.(map[string]any)
			if !ok || reservationKindForBlock(block) == "" {
				continue
			}
			relevant, unknown := dayReservationRelevant(block, sec.Report.Date)
			if !relevant {
				continue
			}
			if unknown && reports[i].Date != "" && reports[i].Date != sec.Report.Date {
				continue
			}
			scope := "reservation"
			if unknown {
				scope = "undated_reservation"
			}
			add(block, reports[i], scope)
		}
	}

	globalContext := []map[string]any{}
	omittedCandidates := false
	for i, raw := range sections(trip) {
		if i == sec.Index || reports[i].Mode == "dayPlan" || reports[i].Mode == "guideDayPlan" || reports[i].Date != "" {
			continue
		}
		section, _ := raw.(map[string]any)
		context := map[string]any{"section_id": reports[i].ID, "heading": firstNonEmpty(reports[i].Title, stringField(section, "heading"), stringField(section, "name")), "text": plainBlockText(section)}
		if links := readableBlockLinks(section); len(links) > 0 {
			context["links"] = links
		}
		keys := []string{}
		rows, _ := section["blocks"].([]any)
		for _, rawBlock := range rows {
			block, ok := rawBlock.(map[string]any)
			if !ok {
				continue
			}
			switch stringField(block, "type") {
			case "note", "checklist":
				keys = append(keys, add(block, reports[i], "global"))
			default:
				if reservationKindForBlock(block) == "" && (travelStopID(block) != "" || mapField(block, "place") != nil) {
					omittedCandidates = true
				}
			}
		}
		if len(keys) > 0 || context["text"] != "" || context["links"] != nil {
			context["block_keys"] = keys
			globalContext = append(globalContext, context)
		}
	}
	if omittedCandidates {
		warnings = append(warnings, "Undated candidate places are omitted; use plan outline --all-sections to review them.")
	}
	travel := buildTravelLegs(trip, sec, modes, selected, false)
	for _, field := range []string{"command", "section", "stops", "source", "freshness", "target_key"} {
		delete(travel, field)
	}
	if selected == "" {
		warnings = append(warnings, "No travel mode selected; schedule feasibility remains unknown.")
	}
	for _, leg := range travel["legs"].([]map[string]any) {
		delete(leg, "from_place_id")
		delete(leg, "to_place_id")
	}
	for _, leg := range travel["legs"].([]map[string]any) {
		for _, estimate := range leg["estimates"].([]map[string]any) {
			if estimate["available"] != true {
				warnings = append(warnings, "Some travel estimates are unavailable; partial totals are not a complete day estimate.")
				goto estimatesChecked
			}
		}
	}
estimatesChecked:
	if !idsSafe {
		warnings = append(warnings, "Missing or duplicate block IDs prevent safe incremental comparison; use the full snapshot.")
	}
	for _, block := range blocks {
		if mapFieldValue(block, "scope") == "undated_reservation" {
			warnings = append(warnings, "Undated reservation constraints are included for review; their day assignment is unknown.")
			break
		}
	}
	checks := dayChecksForSection(trip, sec)
	if strings.TrimSpace(sec.Report.Date) == "" {
		warnings = append(warnings, "The selected day has no date; calendar closure and lodging-coverage checks are unknown.")
	}
	section := map[string]any{"id": sec.Report.ID, "day": day, "date": sec.Report.Date, "heading": firstNonEmpty(sec.Report.Title, stringField(sec.Raw, "heading"), stringField(sec.Raw, "name")), "text": plainBlockText(sec.Raw)}
	if links := readableBlockLinks(sec.Raw); len(links) > 0 {
		section["links"] = links
	}
	return map[string]any{"section": section, "context": globalContext, "blocks": blocks, "order": order, "order_keys": orderKeys, "reservation_ids": reservations, "places": places, "travel": travel, "checks": checks, "warnings": warnings, "stable_ids": idsSafe}, nil
}

func mapFieldValue(v any, key string) string { m, _ := v.(map[string]any); return stringField(m, key) }

func dayPlanningBlock(trip, raw map[string]any, places map[string]any) map[string]any {
	out := readablePlanBlock(raw, false, false)
	delete(out, "place")
	if place := mapField(raw, "place"); place != nil {
		out["place_ref"] = dayPlaceReference(trip, place, places)
	}
	if reservationKindForBlock(raw) != "" {
		// Retain additional booking fields without copying collaborators/UI data.
		for key, value := range raw {
			switch key {
			case "place", "text", "addedBy", "upvotedBy", "collapsed", "isCollapsed":
				continue
			}
			if _, exists := out[key]; !exists {
				out[key] = value
			}
		}
	}
	// Endpoints may embed a complete Google place alongside a flight's airport
	// code, local dates and times. Deduplicate only the embedded place, retaining
	// all other booking fields and unknown future constraints.
	for _, key := range []string{"depart", "arrive", "pickUp", "dropOff", "hotel"} {
		if value, ok := out[key]; ok {
			out[key] = dayReservationPlaces(trip, value, places)
		}
	}
	return out
}

func dayPlaceReference(trip, place map[string]any, places map[string]any) string {
	details := readablePlanBlock(map[string]any{"place": place}, false, false)["place"].(map[string]any)
	delete(details, "name") // Display labels belong to individual blocks.
	planning := travelPlanningFields(trip, place)
	delete(planning, "freshness")
	// Embedded fields already live in details; attach only additional resources.
	for key, value := range planning {
		if _, exists := details[key]; !exists {
			details[key] = value
		}
	}
	data, _ := json.Marshal(details)
	sum := sha256.Sum256(data)
	reference := firstNonEmpty(stringField(place, "place_id"), stringField(place, "placeId"), "unidentified") + ":" + hex.EncodeToString(sum[:8])
	places[reference] = details
	return reference
}

func dayReservationPlaces(trip map[string]any, value any, places map[string]any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range v {
			if key == "googlePlace" || key == "place" {
				if place, ok := child.(map[string]any); ok && (stringField(place, "place_id") != "" || stringField(place, "placeId") != "") {
					out[key+"_ref"] = dayPlaceReference(trip, place, places)
					if name := stringField(place, "name"); name != "" {
						out[key+"_name"] = name
					}
					continue
				}
			}
			out[key] = dayReservationPlaces(trip, child, places)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = dayReservationPlaces(trip, child, places)
		}
		return out
	default:
		return value
	}
}

func dayReservationRelevant(block map[string]any, date string) (bool, bool) {
	if date == "" {
		return true, true
	}
	dates := []string{}
	add := func(value any) {
		if text, ok := value.(string); ok && len(text) >= 10 {
			if _, valid := parseYMD(text[:10]); valid {
				dates = append(dates, text[:10])
			}
		}
	}
	for _, key := range []string{"date", "startDate", "endDate"} {
		add(block[key])
	}
	for _, key := range []string{"depart", "arrive", "pickUp", "dropOff"} {
		add(mapField(block, key)["date"])
	}
	hotel := mapField(block, "hotel")
	add(hotel["checkIn"])
	add(hotel["checkOut"])
	if len(dates) == 0 {
		return true, true
	}
	sort.Strings(dates)
	return date >= dates[0] && date <= dates[len(dates)-1], false
}

func planDayDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
func makePlanDayState(key string, query planDayQuery, snapshot map[string]any) (planDayState, error) {
	digest, err := planDayDigest(snapshot)
	return planDayState{SchemaVersion: planDayStateVersion, TargetKey: key, Query: query, Digest: digest, Snapshot: snapshot}, err
}

func readPlanDayState(path string) (*planDayState, string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "previous_state_unreadable"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 32<<20 {
		return nil, "previous_state_invalid"
	}
	var state planDayState
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&state) != nil {
		return nil, "previous_state_invalid"
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, "previous_state_invalid"
	}
	digest, err := planDayDigest(state.Snapshot)
	if err != nil || state.SchemaVersion != planDayStateVersion || !validDaySnapshotShape(state.Snapshot) || digest != state.Digest {
		return nil, "previous_state_invalid"
	}
	return &state, ""
}

func planDayResponse(current planDayState, previous *planDayState, fallback string) map[string]any {
	out := map[string]any{"command": "plan day", "schema_version": planDayStateVersion, "target_key": current.TargetKey, "query": current.Query, "digest": current.Digest, "mode": "full", "provenance": map[string]any{"snapshot_source": "Wanderlog plan API", "travel_and_place_data": "saved API estimates and metadata", "freshness": "unknown"}}
	if previous != nil {
		oldQuery, _ := planDayDigest(previous.Query)
		newQuery, _ := planDayDigest(current.Query)
		previousDigest, digestErr := planDayDigest(previous.Snapshot)
		if digestErr != nil || previousDigest != previous.Digest || !validDaySnapshotShape(previous.Snapshot) {
			fallback = "previous_state_invalid"
		} else if previous.SchemaVersion != planDayStateVersion || previous.TargetKey != current.TargetKey || oldQuery != newQuery {
			fallback = "previous_state_target_or_query_mismatch"
		} else if !dayStateIDsSafe(current.Snapshot) || !dayStateIDsSafe(previous.Snapshot) {
			fallback = "ambiguous_block_ids"
		} else {
			oldBlocks := mapField(previous.Snapshot, "blocks")
			newBlocks := mapField(current.Snapshot, "blocks")
			changed := map[string]any{}
			deleted := []int{}
			for id, block := range newBlocks {
				a, _ := planDayDigest(block)
				b, _ := planDayDigest(oldBlocks[id])
				if a != b {
					changed[id] = block
				}
			}
			for id := range oldBlocks {
				if _, exists := newBlocks[id]; !exists {
					value, _ := strconv.Atoi(id)
					deleted = append(deleted, value)
				}
			}
			sort.Ints(deleted)
			components := map[string]any{}
			for _, key := range []string{"section", "context", "places", "travel", "checks"} {
				a, _ := planDayDigest(current.Snapshot[key])
				b, _ := planDayDigest(previous.Snapshot[key])
				if a != b {
					components[key] = current.Snapshot[key]
				}
			}
			out["mode"] = "delta"
			out["base_digest"] = previous.Digest
			out["inheritance"] = "Requires the matching saved baseline in consumer context; otherwise reread without --since. Omitted fields inherit; present snapshot fields/components replace, changed_blocks merge, deleted_block_ids remove. Verify digest."
			delete(out, "provenance")
			if len(changed) > 0 {
				out["changed_blocks"] = changed
			}
			if len(deleted) > 0 {
				out["deleted_block_ids"] = deleted
			}
			for _, key := range []string{"order", "order_keys", "reservation_ids", "warnings"} {
				a, _ := planDayDigest(current.Snapshot[key])
				b, _ := planDayDigest(previous.Snapshot[key])
				if a != b {
					out[key] = current.Snapshot[key]
				}
			}
			if len(components) > 0 {
				out["changed_components"] = components
			}
			return out
		}
	}
	out["snapshot"] = current.Snapshot
	if fallback != "" {
		out["state_warning"] = fallback
	}
	return out
}

// The delta representation replaces a fixed set of snapshot components. Reject
// unknown or missing components rather than silently inheriting unhandled data.
func validDaySnapshotShape(snapshot map[string]any) bool {
	types := map[string]byte{"section": '{', "context": '[', "blocks": '{', "order": '[', "order_keys": '[', "reservation_ids": '[', "places": '{', "travel": '{', "checks": '{', "warnings": '[', "stable_ids": 'b'}
	if len(snapshot) != len(types) {
		return false
	}
	for key, kind := range types {
		value, exists := snapshot[key]
		if !exists {
			return false
		}
		if kind == 'b' {
			if _, ok := value.(bool); !ok {
				return false
			}
			continue
		}
		data, err := json.Marshal(value)
		if err != nil || len(data) == 0 || data[0] != kind {
			return false
		}
	}
	return true
}

func dayStateIDsSafe(snapshot map[string]any) bool {
	if snapshot["stable_ids"] != true {
		return false
	}
	blocks := mapField(snapshot, "blocks")
	if blocks == nil {
		return false
	}
	for key, value := range blocks {
		id, err := strconv.Atoi(key)
		block, _ := value.(map[string]any)
		if err != nil || id <= 0 || intAny(block["id"]) != id {
			return false
		}
	}
	return true
}

func savePlanDayState(path string, state planDayState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(directory, ".wanderlog-day-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// The selected section is authoritative. buildPlanOutline's dated-day filter
// intentionally omits undated guide days, but plan day still needs their notes
// and non-calendar checks. Keep original indices for raw-block lookup.
func dayChecksForSection(trip map[string]any, sec resolvedSection) map[string]any {
	section := planOutlineSection{SectionIndex: sec.Index, SectionID: sec.Report.ID, Day: sec.Day, Date: sec.Report.Date, BlockCount: len(sec.Blocks), Blocks: []planOutlineBlock{}}
	for index, raw := range sec.Blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		item := outlineBlock(trip, block, index, sec.Report.Date)
		section.HasHotel = section.HasHotel || item.Hotel
		section.Blocks = append(section.Blocks, item)
	}
	outline := planOutlineReport{Sections: []planOutlineSection{section}, SectionCount: 1, BlockCount: len(sec.Blocks)}
	checks := runPlanInspectChecks(trip, outline, strings.Split(allPlanInspectChecks, ","))
	// The legacy closure helper filters by Day; section indices disambiguate
	// undated lists that can carry the preceding day's ordinal in old snapshots.
	closed := []planIssueReport{}
	for _, issue := range itineraryIssues(trip) {
		if issue.SectionIndex == sec.Index {
			closed = append(closed, issue)
		}
	}
	checks["closed_places"] = closed
	if strings.TrimSpace(sec.Report.Date) == "" {
		checks["calendar_checks_status"] = "unknown_date"
	}
	return checks
}
