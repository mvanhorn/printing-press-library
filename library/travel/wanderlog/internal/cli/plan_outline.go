// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const allPlanInspectChecks = "counts,unformatted,lodging-coverage,closed-places,text-vs-schedule"

type planOutlineReport struct {
	TargetKey    string               `json:"target_key"`
	Title        string               `json:"title,omitempty"`
	StartDate    string               `json:"start_date,omitempty"`
	EndDate      string               `json:"end_date,omitempty"`
	SectionCount int                  `json:"section_count"`
	BlockCount   int                  `json:"block_count"`
	Sections     []planOutlineSection `json:"sections"`
	Checks       map[string]any       `json:"checks,omitempty"`
}

// PATCH(amend-2026-08-23: --check answers a question about the plan, not a
// request for the plan) planChecksReport is the projection printed when
// `plan inspect --check` runs without --with-sections. It keeps every cheap
// scalar field of planOutlineReport — they cost a few dozen bytes and are
// needed to read a check result — and drops the `sections` outline payload,
// which the caller did not ask for and which costs more than `plan outline`
// itself. Field order mirrors planOutlineReport so the two shapes read alike.
type planChecksReport struct {
	TargetKey    string         `json:"target_key"`
	Title        string         `json:"title,omitempty"`
	StartDate    string         `json:"start_date,omitempty"`
	EndDate      string         `json:"end_date,omitempty"`
	SectionCount int            `json:"section_count"`
	BlockCount   int            `json:"block_count"`
	Checks       map[string]any `json:"checks,omitempty"`
}

func checksOnlyReport(report planOutlineReport) planChecksReport {
	return planChecksReport{
		TargetKey:    report.TargetKey,
		Title:        report.Title,
		StartDate:    report.StartDate,
		EndDate:      report.EndDate,
		SectionCount: report.SectionCount,
		BlockCount:   report.BlockCount,
		Checks:       report.Checks,
	}
}

type planOutlineSection struct {
	SectionIndex int                `json:"section_index"`
	SectionID    int                `json:"section_id,omitempty"`
	Day          int                `json:"day,omitempty"`
	Date         string             `json:"date,omitempty"`
	Heading      string             `json:"heading,omitempty"`
	BlockCount   int                `json:"block_count"`
	HasHotel     bool               `json:"has_hotel"`
	Blocks       []planOutlineBlock `json:"blocks"`
}

type planOutlineBlock struct {
	BlockIndex       int    `json:"block_index"`
	BlockID          int    `json:"block_id,omitempty"`
	Type             string `json:"type,omitempty"`
	Subtype          string `json:"subtype,omitempty"`
	Name             string `json:"name,omitempty"`
	PlaceID          string `json:"place_id,omitempty"`
	Start            string `json:"start,omitempty"`
	End              string `json:"end,omitempty"`
	HasFormattedText bool   `json:"has_formatted_text"`
	Hotel            bool   `json:"hotel"`
	HotelCheckIn     string `json:"hotel_check_in,omitempty"`
	HotelCheckOut    string `json:"hotel_check_out,omitempty"`
	SpansNights      bool   `json:"spans_nights,omitempty"`
	UpvotedByCount   int    `json:"upvoted_by_count,omitempty"`
}

func newNovelPlanOutlineCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	var allSections bool
	cmd := &cobra.Command{
		Use:   "outline",
		Short: "Show a slim itinerary outline: days, section headings, and stop names",
		Example: "  wanderlog-pp-cli plan outline --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent\n" +
			"  wanderlog-pp-cli plan outline --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 2 --dry-run --agent   # --dry-run is accepted and is a no-op: this command only reads",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanOutline(cmd, flags, opts, "", allSections, true)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.day, "day", 0, "1-based day number among day sections; omit for the full plan")
	cmd.Flags().BoolVar(&allSections, "all-sections", false, "Include block lists for undated and non-day sections")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanInspectCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	var check string
	var allSections bool
	var withSections bool
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a slim itinerary outline; pass --check for counts, formatting, lodging, closures, and schedule mismatches",
		Example: "  wanderlog-pp-cli plan inspect --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --check=counts,unformatted,lodging-coverage --agent   # --check prints checks plus plan scalars; the sections outline is omitted. Use --check=NAMES: the space form runs every check\n" +
			"  wanderlog-pp-cli plan inspect --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --check=closed-places,text-vs-schedule --dry-run --agent   # --dry-run is accepted and is a no-op: this command only reads\n" +
			"  wanderlog-pp-cli plan inspect --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --check=counts --with-sections --agent   # keep the sections outline next to the checks\n" +
			"  wanderlog-pp-cli plan inspect --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent   # no --check: the full outline, same as plan outline",
		// PATCH(amend-2026-08-23: --check has NoOptDefVal, so `--check counts` parses
		// as a valueless --check plus an ignored positional and silently runs every
		// check. NoArgs turns that into a loud usage error instead of a different
		// answer than the caller asked for.)
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("check") {
				check = ""
			}
			return runPlanOutline(cmd, flags, opts, check, allSections, withSections)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.day, "day", 0, "1-based day number among day sections; omit for the full plan")
	cmd.Flags().BoolVar(&allSections, "all-sections", false, "Include block lists for undated and non-day sections")
	cmd.Flags().StringVar(&check, "check", "", "Comma-separated checks: counts,unformatted,lodging-coverage,closed-places,text-vs-schedule. Prints the checks plus the plan scalars and omits the sections outline; add --with-sections to keep it")
	if f := cmd.Flags().Lookup("check"); f != nil {
		f.NoOptDefVal = allPlanInspectChecks
	}
	cmd.Flags().BoolVar(&withSections, "with-sections", false, "With --check, also print the sections outline. No effect without --check: that already prints the full outline")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func runPlanOutline(cmd *cobra.Command, flags *rootFlags, opts planEditOptions, check string, allSections, withSections bool) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := planLiveClient(flags)
	if err != nil {
		return err
	}
	key, err := resolveEditablePlanKey(opts)
	if err != nil {
		return usageErr(err)
	}
	trip, _, err := fetchPlan(ctx, c, key, opts.clientSchemaVersion)
	if err != nil {
		return err
	}
	report, err := buildPlanOutline(trip, key, opts.day, check, allSections)
	if err != nil {
		return usageErr(err)
	}
	return printPlanOutline(cmd, flags, report, withSections)
}

func printPlanOutline(cmd *cobra.Command, flags *rootFlags, report planOutlineReport, withSections bool) error {
	if flags != nil && flags.plain {
		return printPlanOutlinePlain(cmd.OutOrStdout(), report)
	}
	// PATCH(amend-2026-08-23: --check drops the sections payload it never asked for)
	if report.Checks != nil && !withSections {
		return printJSONFiltered(cmd.OutOrStdout(), checksOnlyReport(report), flags)
	}
	return printJSONFiltered(cmd.OutOrStdout(), report, flags)
}

func printPlanOutlinePlain(w io.Writer, report planOutlineReport) error {
	for _, sec := range report.Sections {
		for _, block := range sec.Blocks {
			name := strings.ReplaceAll(strings.ReplaceAll(block.Name, "\t", " "), "\n", " ")
			if _, err := fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\t%s\t%v\t%v\n",
				sec.Day, sec.Date, block.BlockID, block.Type, name, block.Start, block.End, block.Hotel, block.HasFormattedText); err != nil {
				return err
			}
		}
	}
	return nil
}

// PATCH(amend-2026-08-23: slim itinerary projection without Quill, photos, or resources)
func buildPlanOutline(trip map[string]any, targetKey string, day int, check string, allSections bool) (planOutlineReport, error) {
	reports := sectionReports(trip)
	secs := sections(trip)
	start, end := planDateRange(trip, dayDates(trip))
	out := planOutlineReport{
		TargetKey: targetKey,
		Title:     stringField(trip, "title"),
		StartDate: start,
		EndDate:   end,
		Sections:  []planOutlineSection{},
	}
	for i, raw := range secs {
		if i >= len(reports) {
			continue
		}
		rep := reports[i]
		sec, _ := raw.(map[string]any)
		if sec == nil {
			sec = map[string]any{}
		}
		dated := datedDayPlanSection(rep, sec)
		if day > 0 && (!dated || rep.Day != day) {
			continue
		}
		blocks, _ := sec["blocks"].([]any)
		section := planOutlineSection{
			SectionIndex: i,
			SectionID:    rep.ID,
			Day:          rep.Day,
			Date:         firstNonEmpty(rep.Date, stringField(sec, "date")),
			Heading:      firstNonEmpty(rep.Title, stringField(sec, "title"), stringField(sec, "heading"), stringField(sec, "name")),
			BlockCount:   len(blocks),
			Blocks:       []planOutlineBlock{},
		}
		// PATCH(amend-2026-08-23: Day==0 marks undated/non-day sections)
		if !dated {
			section.Day = 0
		}
		for bi, rawBlock := range blocks {
			block, _ := rawBlock.(map[string]any)
			if block == nil {
				continue
			}
			ob := outlineBlock(trip, block, bi, section.Date)
			if ob.Hotel {
				section.HasHotel = true
			}
			section.Blocks = append(section.Blocks, ob)
		}
		out.Sections = append(out.Sections, section)
		out.BlockCount += section.BlockCount
	}
	if day > 0 && len(out.Sections) == 0 {
		return planOutlineReport{}, fmt.Errorf("day %d not found; run plan outline first", day)
	}
	out.SectionCount = len(out.Sections)
	if strings.TrimSpace(check) != "" {
		names, err := parsePlanInspectChecks(check)
		if err != nil {
			return planOutlineReport{}, err
		}
		out.Checks = runPlanInspectChecks(trip, out, names)
	}
	// PATCH(amend-2026-08-23: omit undated/non-day block lists unless --all-sections)
	if !allSections {
		for i := range out.Sections {
			if out.Sections[i].Day == 0 {
				out.Sections[i].Blocks = []planOutlineBlock{}
			}
		}
	}
	return out, nil
}

func datedDayPlanSection(rep planSectionReport, sec map[string]any) bool {
	mode := firstNonEmpty(rep.Mode, stringField(sec, "mode"))
	if mode != "dayPlan" && mode != "guideDayPlan" {
		return false
	}
	return strings.TrimSpace(firstNonEmpty(rep.Date, stringField(sec, "date"))) != ""
}

func outlineBlock(trip map[string]any, block map[string]any, index int, sectionDate string) planOutlineBlock {
	sum := summarizeBlock(block)
	hotel := mapField(block, "hotel")
	ob := planOutlineBlock{
		BlockIndex:       index,
		BlockID:          intAny(block["id"]),
		Type:             stringField(block, "type"),
		Name:             firstNonEmpty(stringAny(sum["place_name"]), stringField(block, "title"), stringField(block, "name"), firstLine(plainBlockText(block))),
		PlaceID:          firstNonEmpty(stringAny(sum["place_id"]), stringField(block, "placeId"), stringField(block, "place_id")),
		Start:            firstNonEmpty(stringField(block, "startTime"), stringAny(sum["startTime"])),
		End:              firstNonEmpty(stringField(block, "endTime"), stringAny(sum["endTime"])),
		HasFormattedText: blockHasFormattedText(block),
		Hotel:            hotel != nil,
	}
	if hotel != nil {
		ob.Subtype = "hotel"
		ob.HotelCheckIn = stringField(hotel, "checkIn")
		ob.HotelCheckOut = stringField(hotel, "checkOut")
		ob.SpansNights = hotelSpansNights(trip, block, sectionDate)
	}
	if ob.Type == "place" || ob.Hotel {
		ob.UpvotedByCount = len(upvotedByIDs(block))
	}
	return ob
}

func plainBlockText(block map[string]any) string {
	if text := mapField(block, "text"); text != nil {
		return plainRichText(text)
	}
	return strings.TrimSpace(stringField(block, "text"))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// PATCH(amend-2026-08-23: formatted-text means Delta bold or list:bullet, not a lone plain insert)
func blockHasFormattedText(block map[string]any) bool {
	text := mapField(block, "text")
	if text == nil {
		return false
	}
	ops, _ := text["ops"].([]any)
	for _, raw := range ops {
		op, _ := raw.(map[string]any)
		if op == nil {
			continue
		}
		attrs := mapField(op, "attributes")
		if attrs == nil {
			continue
		}
		if truthyAny(attrs["bold"]) {
			return true
		}
		if strings.EqualFold(stringAny(attrs["list"]), "bullet") {
			return true
		}
	}
	return false
}

func truthyAny(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1"
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return false
	}
}

// PATCH(amend-2026-08-23: v1 spans_nights is false when the hotel lives only on the check-in section)
func hotelSpansNights(trip map[string]any, block map[string]any, sectionDate string) bool {
	hotel := mapField(block, "hotel")
	if hotel == nil {
		return false
	}
	checkIn := stringField(hotel, "checkIn")
	checkOut := stringField(hotel, "checkOut")
	in, inOK := parseYMD(checkIn)
	out, outOK := parseYMD(checkOut)
	if !inOK || !outOK || !out.After(in.AddDate(0, 0, 1)) {
		return false
	}
	if sectionDate != "" && sectionDate != checkIn {
		return true
	}
	id := hotelCopyIdentity(block)
	copies := 0
	for _, raw := range sections(trip) {
		sec, _ := raw.(map[string]any)
		blocks, _ := sec["blocks"].([]any)
		for _, rawBlock := range blocks {
			b, _ := rawBlock.(map[string]any)
			if mapField(b, "hotel") == nil {
				continue
			}
			if hotelCopyIdentity(b) == id {
				copies++
			}
		}
	}
	return copies > 1
}

func hotelCopyIdentity(block map[string]any) string {
	place := mapField(block, "place")
	hotel := mapField(block, "hotel")
	return firstNonEmpty(stringField(place, "place_id"), stringField(place, "placeId"), stringField(block, "placeId")) + "|" +
		stringField(hotel, "checkIn") + "|" + stringField(hotel, "checkOut")
}

func parseYMD(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func parsePlanInspectChecks(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		switch part {
		case "counts", "unformatted", "lodging-coverage", "closed-places", "text-vs-schedule":
			if !seen[part] {
				seen[part] = true
				out = append(out, part)
			}
		default:
			return nil, fmt.Errorf("unknown --check %q; want counts,unformatted,lodging-coverage,closed-places,text-vs-schedule", part)
		}
	}
	if len(out) == 0 {
		out = append(out, strings.Split(allPlanInspectChecks, ",")...)
	}
	return out, nil
}

func runPlanInspectChecks(trip map[string]any, outline planOutlineReport, names []string) map[string]any {
	checks := map[string]any{}
	for _, name := range names {
		switch name {
		case "counts":
			checks["counts"] = outlineCheckCounts(outline)
		case "unformatted":
			checks["unformatted_block_ids"] = outlineUnformattedIDs(trip, outline)
		case "lodging-coverage":
			checks["days_missing_hotel"] = outlineDaysMissingHotel(outline)
		case "closed-places":
			checks["closed_places"] = outlineClosedPlaces(trip, outline)
		case "text-vs-schedule":
			checks["text_vs_schedule"] = outlineTextVsSchedule(trip, outline)
		}
	}
	return checks
}

func outlineCheckCounts(outline planOutlineReport) map[string]int {
	counts := map[string]int{"sections": outline.SectionCount, "place": 0, "note": 0, "hotel": 0}
	for _, sec := range outline.Sections {
		for _, block := range sec.Blocks {
			switch block.Type {
			case "place":
				counts["place"]++
			case "note":
				counts["note"]++
			}
			if block.Hotel {
				counts["hotel"]++
			}
		}
	}
	return counts
}

func outlineUnformattedIDs(trip map[string]any, outline planOutlineReport) []int {
	ids := []int{}
	for _, sec := range outline.Sections {
		for _, block := range sec.Blocks {
			if block.HasFormattedText {
				continue
			}
			raw := originalOutlineBlock(trip, sec, block)
			if strings.TrimSpace(plainBlockText(raw)) == "" {
				continue
			}
			ids = append(ids, block.BlockID)
		}
	}
	return ids
}

func originalOutlineBlock(trip map[string]any, sec planOutlineSection, block planOutlineBlock) map[string]any {
	secs := sections(trip)
	if sec.SectionIndex < 0 || sec.SectionIndex >= len(secs) {
		return nil
	}
	section, _ := secs[sec.SectionIndex].(map[string]any)
	blocks, _ := section["blocks"].([]any)
	if block.BlockIndex < 0 || block.BlockIndex >= len(blocks) {
		return nil
	}
	raw, _ := blocks[block.BlockIndex].(map[string]any)
	return raw
}

func outlineDaysMissingHotel(outline planOutlineReport) []int {
	days := []int{}
	seen := map[int]bool{}
	covered := map[int]bool{}
	for _, sec := range outline.Sections {
		if strings.TrimSpace(sec.Date) == "" || sec.Day <= 0 {
			continue
		}
		if sec.HasHotel {
			covered[sec.Day] = true
		}
		if !seen[sec.Day] {
			seen[sec.Day] = true
			days = append(days, sec.Day)
		}
	}
	missing := []int{}
	for _, day := range days {
		if !covered[day] {
			missing = append(missing, day)
		}
	}
	return missing
}

func outlineClosedPlaces(trip map[string]any, outline planOutlineReport) []planIssueReport {
	wantDay := map[int]bool{}
	filterDay := false
	for _, sec := range outline.Sections {
		if sec.Day > 0 {
			wantDay[sec.Day] = true
			filterDay = true
		}
	}
	issues := itineraryIssues(trip)
	out := []planIssueReport{}
	for _, issue := range issues {
		if filterDay && !wantDay[issue.Day] {
			continue
		}
		out = append(out, issue)
	}
	return out
}

var timeWindowRe = regexp.MustCompile(`(?i)(\d{1,2}(?::\d{2})?)\s*(am|pm)?\s*(?:-|–|—|to)\s*(\d{1,2}(?::\d{2})?)\s*(am|pm)?`)

// PATCH(amend-2026-08-23: heuristic text-vs-schedule mismatch for inspect --check)
func outlineTextVsSchedule(trip map[string]any, outline planOutlineReport) []map[string]any {
	out := []map[string]any{}
	for _, sec := range outline.Sections {
		for _, block := range sec.Blocks {
			if block.Start == "" && block.End == "" {
				continue
			}
			plain := plainBlockText(originalOutlineBlock(trip, sec, block))
			if strings.TrimSpace(plain) == "" {
				plain = block.Name
			}
			startMin, startOK := parseClockToMinutes(block.Start, "")
			endMin, endOK := parseClockToMinutes(block.End, "")
			matches := timeWindowRe.FindAllStringSubmatch(plain, -1)
			if len(matches) == 0 {
				continue
			}
			m := matches[0]
			textStart, textStartOK := parseClockToMinutes(m[1], m[2])
			textEnd, textEndOK := parseClockToMinutes(m[3], m[4])
			disagree := false
			if startOK && textStartOK && startMin != textStart {
				disagree = true
			}
			if endOK && textEndOK && endMin != textEnd {
				disagree = true
			}
			if !disagree {
				continue
			}
			out = append(out, map[string]any{
				"block_id":    block.BlockID,
				"start":       block.Start,
				"end":         block.End,
				"text_window": m[0],
			})
		}
	}
	return out
}

func parseClockToMinutes(raw, ampm string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	ampm = strings.ToLower(strings.TrimSpace(ampm))
	lower := strings.ToLower(raw)
	if strings.HasSuffix(lower, "am") || strings.HasSuffix(lower, "pm") {
		ampm = lower[len(lower)-2:]
		raw = strings.TrimSpace(raw[:len(raw)-2])
	}
	parts := strings.Split(raw, ":")
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	m := 0
	if len(parts) > 1 {
		m, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, false
		}
	}
	if ampm == "pm" && h < 12 {
		h += 12
	}
	if ampm == "am" && h == 12 {
		h = 0
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}
