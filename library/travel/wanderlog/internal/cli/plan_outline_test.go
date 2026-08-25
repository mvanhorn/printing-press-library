// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func testOutlineTrip() map[string]any {
	return map[string]any{
		"title":     "Tiny outline trip",
		"startDate": "2026-09-01",
		"endDate":   "2026-09-03",
		"itinerary": map[string]any{
			"sections": []any{
				map[string]any{
					"id":    1,
					"mode":  "dayPlan",
					"date":  "2026-09-01",
					"title": "Arrival",
					"blocks": []any{
						map[string]any{
							"id":   10,
							"type": "place",
							"place": map[string]any{
								"name":     "Hotel Sun",
								"place_id": "ChIJHotel",
							},
							"hotel": map[string]any{
								"checkIn":  "2026-09-01",
								"checkOut": "2026-09-03",
							},
						},
						map[string]any{
							"id":   11,
							"type": "note",
							"text": map[string]any{"ops": []any{
								map[string]any{"insert": "Pack snacks\n"},
							}},
						},
						map[string]any{
							"id":   12,
							"type": "place",
							"place": map[string]any{
								"name":     "Shuri Castle",
								"place_id": "ChIJCastle",
							},
							"startTime": "10:00",
							"endTime":   "12:00",
							"text": map[string]any{"ops": []any{
								map[string]any{"insert": "Buy tickets"},
								map[string]any{"insert": "\n", "attributes": map[string]any{"list": "bullet"}},
							}},
						},
						map[string]any{
							"id":   13,
							"type": "place",
							"place": map[string]any{
								"name":     "Cafe mismatch",
								"place_id": "ChIJCafe",
								"opening_hours": map[string]any{"periods": []any{
									map[string]any{
										"open":  map[string]any{"day": 1, "time": "0900"},
										"close": map[string]any{"day": 1, "time": "1700"},
									},
								}},
							},
							"startTime": "09:00",
							"endTime":   "10:00",
							"text": map[string]any{"ops": []any{
								map[string]any{"insert": "Open 11:00-13:00\n"},
							}},
						},
					},
				},
				map[string]any{
					"id":     2,
					"mode":   "dayPlan",
					"date":   "2026-09-02",
					"title":  "Explore",
					"blocks": []any{},
				},
				map[string]any{
					"id":    3,
					"mode":  "dayPlan",
					"date":  "2026-09-03",
					"title": "Hotel copy night",
					"blocks": []any{
						map[string]any{
							"id":   30,
							"type": "place",
							"place": map[string]any{
								"name":     "Hotel Sun",
								"place_id": "ChIJHotel",
							},
							"hotel": map[string]any{
								"checkIn":  "2026-09-01",
								"checkOut": "2026-09-03",
							},
						},
					},
				},
			},
		},
	}
}

func TestBuildPlanOutlineTable(t *testing.T) {
	trip := testOutlineTrip()
	tests := []struct {
		name            string
		day             int
		check           string
		wantErr         bool
		wantSections    int
		wantBlocks      int
		wantHotelName   string
		wantNoteFmt     bool
		wantPlaceFmt    bool
		wantSpansCI     bool
		wantSpansCopy   bool
		wantMissingDay  int
		wantUnformatted []int
		wantClosed      int
		wantMismatch    int
		wantCounts      map[string]int
	}{
		{
			name:          "full outline",
			wantSections:  3,
			wantBlocks:    5,
			wantHotelName: "Hotel Sun",
			wantNoteFmt:   false,
			wantPlaceFmt:  true,
			wantSpansCI:   true,
			wantSpansCopy: true,
		},
		{
			name:         "day 2 empty",
			day:          2,
			wantSections: 1,
			wantBlocks:   0,
		},
		{
			name:    "missing day",
			day:     9,
			wantErr: true,
		},
		{
			name:            "inspect checks",
			check:           allPlanInspectChecks,
			wantSections:    3,
			wantBlocks:      5,
			wantMissingDay:  2,
			wantUnformatted: []int{11, 13},
			wantClosed:      1,
			wantMismatch:    1,
			wantCounts:      map[string]int{"sections": 3, "place": 4, "note": 1, "hotel": 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPlanOutline(trip, "abcdefghijklmnop", tt.day, tt.check, false)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildPlanOutline: %v", err)
			}
			if got.TargetKey != "abcdefghijklmnop" || got.Title != "Tiny outline trip" {
				t.Fatalf("identity = key %q title %q", got.TargetKey, got.Title)
			}
			if got.StartDate != "2026-09-01" || got.EndDate != "2026-09-03" {
				t.Fatalf("dates = %q %q", got.StartDate, got.EndDate)
			}
			if got.SectionCount != tt.wantSections || got.BlockCount != tt.wantBlocks {
				t.Fatalf("counts sections=%d blocks=%d want %d %d", got.SectionCount, got.BlockCount, tt.wantSections, tt.wantBlocks)
			}
			if tt.wantHotelName != "" {
				if len(got.Sections) == 0 || len(got.Sections[0].Blocks) == 0 {
					t.Fatalf("missing first block: %#v", got.Sections)
				}
				hotel := got.Sections[0].Blocks[0]
				if !hotel.Hotel || hotel.Subtype != "hotel" || hotel.Name != tt.wantHotelName || hotel.PlaceID != "ChIJHotel" {
					t.Fatalf("hotel block = %#v", hotel)
				}
				if hotel.HotelCheckIn != "2026-09-01" || hotel.HotelCheckOut != "2026-09-03" {
					t.Fatalf("hotel dates = %#v", hotel)
				}
				if hotel.SpansNights != tt.wantSpansCI {
					t.Fatalf("check-in spans_nights = %v want %v", hotel.SpansNights, tt.wantSpansCI)
				}
				note := got.Sections[0].Blocks[1]
				if note.Type != "note" || note.HasFormattedText != tt.wantNoteFmt || note.Name != "Pack snacks" {
					t.Fatalf("note block = %#v", note)
				}
				place := got.Sections[0].Blocks[2]
				if place.Type != "place" || place.HasFormattedText != tt.wantPlaceFmt || place.Start != "10:00" || place.End != "12:00" {
					t.Fatalf("place block = %#v", place)
				}
			}
			if tt.wantSpansCopy && len(got.Sections) >= 3 && len(got.Sections[2].Blocks) > 0 {
				copyHotel := got.Sections[2].Blocks[0]
				if !copyHotel.SpansNights {
					t.Fatalf("copy hotel spans_nights = false: %#v", copyHotel)
				}
			}
			if tt.check != "" {
				if got.Checks == nil {
					t.Fatalf("expected checks")
				}
				missing, _ := got.Checks["days_missing_hotel"].([]int)
				if tt.wantMissingDay != 0 && (len(missing) != 1 || missing[0] != tt.wantMissingDay) {
					t.Fatalf("days_missing_hotel = %#v", got.Checks["days_missing_hotel"])
				}
				unformatted, _ := got.Checks["unformatted_block_ids"].([]int)
				if !intSliceEqual(unformatted, tt.wantUnformatted) {
					t.Fatalf("unformatted_block_ids = %#v want %#v", unformatted, tt.wantUnformatted)
				}
				closed, _ := got.Checks["closed_places"].([]planIssueReport)
				if len(closed) != tt.wantClosed {
					t.Fatalf("closed_places = %#v", got.Checks["closed_places"])
				}
				mismatch, _ := got.Checks["text_vs_schedule"].([]map[string]any)
				if len(mismatch) != tt.wantMismatch {
					t.Fatalf("text_vs_schedule = %#v", got.Checks["text_vs_schedule"])
				}
				if tt.wantCounts != nil {
					counts, _ := got.Checks["counts"].(map[string]int)
					for k, v := range tt.wantCounts {
						if counts[k] != v {
							t.Fatalf("counts[%s]=%d want %d (%#v)", k, counts[k], v, counts)
						}
					}
				}
			}
		})
	}
}

func TestOutlineHotelOnlyOnCheckInDoesNotSpan(t *testing.T) {
	trip := map[string]any{
		"title":     "Single hotel night copy",
		"startDate": "2026-09-01",
		"endDate":   "2026-09-03",
		"itinerary": map[string]any{"sections": []any{
			map[string]any{
				"id":   1,
				"mode": "dayPlan",
				"date": "2026-09-01",
				"blocks": []any{map[string]any{
					"id":    10,
					"type":  "place",
					"place": map[string]any{"name": "Inn", "place_id": "inn"},
					"hotel": map[string]any{"checkIn": "2026-09-01", "checkOut": "2026-09-03"},
				}},
			},
			map[string]any{"id": 2, "mode": "dayPlan", "date": "2026-09-02", "blocks": []any{}},
		}},
	}
	got, err := buildPlanOutline(trip, "abcdefghijklmnop", 0, "", false)
	if err != nil {
		t.Fatalf("buildPlanOutline: %v", err)
	}
	hotel := got.Sections[0].Blocks[0]
	if hotel.SpansNights {
		t.Fatalf("expected spans_nights false when hotel is only on check-in: %#v", hotel)
	}
}

func TestBlockHasFormattedTextTable(t *testing.T) {
	tests := []struct {
		name string
		text map[string]any
		want bool
	}{
		{name: "plain insert", text: map[string]any{"ops": []any{map[string]any{"insert": "hello\n"}}}, want: false},
		{name: "bold", text: map[string]any{"ops": []any{map[string]any{"insert": "hello", "attributes": map[string]any{"bold": true}}}}, want: true},
		{name: "bullet", text: map[string]any{"ops": []any{
			map[string]any{"insert": "item"},
			map[string]any{"insert": "\n", "attributes": map[string]any{"list": "bullet"}},
		}}, want: true},
		{name: "nil", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := map[string]any{"type": "note"}
			if tt.text != nil {
				block["text"] = tt.text
			}
			if got := blockHasFormattedText(block); got != tt.want {
				t.Fatalf("blockHasFormattedText = %v want %v", got, tt.want)
			}
		})
	}
}

func TestPrintPlanOutlinePlainTSV(t *testing.T) {
	trip := testOutlineTrip()
	got, err := buildPlanOutline(trip, "abcdefghijklmnop", 1, "", false)
	if err != nil {
		t.Fatalf("buildPlanOutline: %v", err)
	}
	var buf bytes.Buffer
	if err := printPlanOutlinePlain(&buf, got); err != nil {
		t.Fatalf("print: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "1\t2026-09-01\t10\tplace\tHotel Sun\t\t\ttrue\tfalse") {
		t.Fatalf("row0 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "\t11\tnote\tPack snacks\t") || !strings.HasSuffix(lines[1], "\tfalse\tfalse") {
		t.Fatalf("row1 = %q", lines[1])
	}
	if !strings.Contains(lines[2], "\t12\tplace\tShuri Castle\t10:00\t12:00\tfalse\ttrue") {
		t.Fatalf("row2 = %q", lines[2])
	}
}

func TestParsePlanInspectChecks(t *testing.T) {
	got, err := parsePlanInspectChecks("counts, unknown")
	if err == nil {
		t.Fatalf("expected unknown check error, got %v", got)
	}
	got, err = parsePlanInspectChecks("counts,unformatted")
	if err != nil || len(got) != 2 || got[0] != "counts" || got[1] != "unformatted" {
		t.Fatalf("got %#v err %v", got, err)
	}
}

func TestShouldStubFatTrip(t *testing.T) {
	tests := []struct {
		name  string
		flags *rootFlags
		full  bool
		n     int
		want  bool
	}{
		{name: "agent over limit", flags: &rootFlags{agent: true}, n: fatTripByteLimit + 1, want: true},
		{name: "compact over limit", flags: &rootFlags{compact: true}, n: fatTripByteLimit + 1, want: true},
		{name: "agent at limit", flags: &rootFlags{agent: true}, n: fatTripByteLimit, want: false},
		{name: "full dumps", flags: &rootFlags{agent: true}, full: true, n: fatTripByteLimit + 1, want: false},
		{name: "select dumps", flags: &rootFlags{agent: true, selectFields: "title"}, n: fatTripByteLimit + 1, want: false},
		{name: "json only", flags: &rootFlags{asJSON: true}, n: fatTripByteLimit + 1, want: false},
		{name: "nil flags", n: fatTripByteLimit + 1, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldStubFatTrip(tt.flags, tt.full, tt.n); got != tt.want {
				t.Fatalf("shouldStubFatTrip = %v want %v", got, tt.want)
			}
		})
	}
}

func TestFatTripStubMessage(t *testing.T) {
	trip := testOutlineTrip()
	env := map[string]any{"success": true, "tripPlan": trip}
	body, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw, err := fatTripStub(body, "abcdefghijklmnop")
	if err != nil {
		t.Fatalf("fatTripStub: %v", err)
	}
	var got fatTripStubReport
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Title != "Tiny outline trip" || got.StartDate != "2026-09-01" || got.EndDate != "2026-09-03" || got.SectionCount != 3 {
		t.Fatalf("stub = %#v", got)
	}
	if got.TargetKey != "abcdefghijklmnop" || !strings.Contains(got.Message, "use plan outline --target-key abcdefghijklmnop --agent") || !strings.Contains(got.Message, "--full") {
		t.Fatalf("message = %q", got.Message)
	}
	if len(raw) > 2048 {
		t.Fatalf("stub size %d", len(raw))
	}
}

func TestInspectWithoutCheckMatchesOutline(t *testing.T) {
	trip := testOutlineTrip()
	a, err := buildPlanOutline(trip, "abcdefghijklmnop", 0, "", false)
	if err != nil {
		t.Fatalf("outline: %v", err)
	}
	b, err := buildPlanOutline(trip, "abcdefghijklmnop", 0, "", false)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if b.Checks != nil {
		t.Fatalf("inspect without --check should omit checks: %#v", b.Checks)
	}
	if a.SectionCount != b.SectionCount || a.BlockCount != b.BlockCount {
		t.Fatalf("mismatch %#v vs %#v", a, b)
	}
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func testOutlineTripWithUndated() map[string]any {
	trip := testOutlineTrip()
	it, _ := trip["itinerary"].(map[string]any)
	secs, _ := it["sections"].([]any)
	secs = append(secs, map[string]any{
		"id":    99,
		"mode":  "normal",
		"title": "Ideas",
		"blocks": []any{
			map[string]any{
				"id":   90,
				"type": "place",
				"place": map[string]any{
					"name":     "Maybe later",
					"place_id": "ChIJMaybe",
				},
				"upvotedBy": []any{"u1", "u2"},
			},
			map[string]any{
				"id":   91,
				"type": "note",
				"text": map[string]any{"ops": []any{map[string]any{"insert": "Parking\n"}}},
			},
		},
	})
	it["sections"] = secs
	return trip
}

func TestBuildPlanOutlineOmitsUndatedBlocksUnlessAllSections(t *testing.T) {
	trip := testOutlineTripWithUndated()
	got, err := buildPlanOutline(trip, "abcdefghijklmnop", 0, "counts", false)
	if err != nil {
		t.Fatalf("buildPlanOutline: %v", err)
	}
	if got.SectionCount != 4 {
		t.Fatalf("section_count = %d want 4", got.SectionCount)
	}
	undated := got.Sections[len(got.Sections)-1]
	if undated.Day != 0 || undated.Heading != "Ideas" || undated.BlockCount != 2 {
		t.Fatalf("undated section = %#v", undated)
	}
	if len(undated.Blocks) != 0 {
		t.Fatalf("default undated blocks = %#v", undated.Blocks)
	}
	counts, _ := got.Checks["counts"].(map[string]int)
	if counts["place"] != 5 {
		t.Fatalf("counts after strip should still include undated places: %#v", counts)
	}

	full, err := buildPlanOutline(trip, "abcdefghijklmnop", 0, "", true)
	if err != nil {
		t.Fatalf("all-sections: %v", err)
	}
	undated = full.Sections[len(full.Sections)-1]
	if len(undated.Blocks) != 2 {
		t.Fatalf("all-sections blocks = %#v", undated.Blocks)
	}
	if undated.Blocks[0].UpvotedByCount != 2 || undated.Blocks[0].Name != "Maybe later" {
		t.Fatalf("place block = %#v", undated.Blocks[0])
	}
}

func TestOutlinePlaceUpvotedByCount(t *testing.T) {
	trip := testOutlineTrip()
	secs := sections(trip)
	sec, _ := secs[0].(map[string]any)
	blocks, _ := sec["blocks"].([]any)
	place, _ := blocks[2].(map[string]any)
	place["upvotedBy"] = []any{map[string]any{"id": "alice"}, float64(7)}
	got, err := buildPlanOutline(trip, "abcdefghijklmnop", 1, "", false)
	if err != nil {
		t.Fatalf("buildPlanOutline: %v", err)
	}
	placeBlock := got.Sections[0].Blocks[2]
	if placeBlock.UpvotedByCount != 2 {
		t.Fatalf("upvoted_by_count = %d want 2 (%#v)", placeBlock.UpvotedByCount, placeBlock)
	}
}

// planOutlineJSON runs printPlanOutline into a buffer the way the command does.
func planOutlineJSON(t *testing.T, report planOutlineReport, withSections bool) ([]byte, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "inspect"}
	cmd.SetOut(&buf)
	if err := printPlanOutline(cmd, &rootFlags{asJSON: true}, report, withSections); err != nil {
		t.Fatalf("printPlanOutline: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", buf.String(), err)
	}
	return buf.Bytes(), got
}

func TestPrintPlanInspectCheckOmitsSections(t *testing.T) {
	trip := testOutlineTrip()
	report, err := buildPlanOutline(trip, "abcdefghijklmnop", 0, "counts", false)
	if err != nil {
		t.Fatalf("buildPlanOutline: %v", err)
	}
	raw, got := planOutlineJSON(t, report, false)
	if _, ok := got["sections"]; ok {
		t.Fatalf("--check output should omit sections: %s", raw)
	}
	if _, ok := got["checks"]; !ok {
		t.Fatalf("--check output should carry checks: %s", raw)
	}
	for _, key := range []string{"target_key", "title", "start_date", "end_date", "section_count", "block_count"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("--check output should keep %q: %s", key, raw)
		}
	}
	if got["target_key"] != "abcdefghijklmnop" || got["title"] != "Tiny outline trip" {
		t.Fatalf("scalars = %#v", got)
	}

	withRaw, withGot := planOutlineJSON(t, report, true)
	if _, ok := withGot["sections"]; !ok {
		t.Fatalf("--with-sections should keep sections: %s", withRaw)
	}
	if _, ok := withGot["checks"]; !ok {
		t.Fatalf("--with-sections should keep checks: %s", withRaw)
	}
	if len(raw) >= len(withRaw) {
		t.Fatalf("checks-only payload %d bytes is not smaller than %d", len(raw), len(withRaw))
	}
}

// The no-check path is plan inspect's other, legitimate use: it must stay
// byte-identical to plan outline, sections and all.
func TestPrintPlanInspectWithoutCheckKeepsSections(t *testing.T) {
	trip := testOutlineTrip()
	report, err := buildPlanOutline(trip, "abcdefghijklmnop", 0, "", false)
	if err != nil {
		t.Fatalf("buildPlanOutline: %v", err)
	}
	inspect, got := planOutlineJSON(t, report, false)
	if _, ok := got["sections"]; !ok {
		t.Fatalf("inspect without --check must keep sections: %s", inspect)
	}
	outline, _ := planOutlineJSON(t, report, true)
	if !bytes.Equal(inspect, outline) {
		t.Fatalf("inspect without --check differs from outline:\n%s\n%s", inspect, outline)
	}
}

func TestPlanInspectHasWithSectionsFlag(t *testing.T) {
	cmd := newNovelPlanInspectCmd(&rootFlags{})
	f := cmd.Flags().Lookup("with-sections")
	if f == nil {
		t.Fatal("plan inspect missing --with-sections")
	}
	if f.DefValue != "false" {
		t.Fatalf("--with-sections default = %q want false", f.DefValue)
	}
	if newNovelPlanOutlineCmd(&rootFlags{}).Flags().Lookup("with-sections") != nil {
		t.Fatal("plan outline should not carry --with-sections: it has no --check")
	}
	if check := cmd.Flags().Lookup("check"); check == nil || !strings.Contains(check.Usage, "--with-sections") {
		t.Fatal("--check help should point at --with-sections")
	}
}

func TestPrintPlanInspectAllChecksOmitSections(t *testing.T) {
	trip := testOutlineTrip()
	for _, name := range append(strings.Split(allPlanInspectChecks, ","), allPlanInspectChecks) {
		t.Run(name, func(t *testing.T) {
			report, err := buildPlanOutline(trip, "abcdefghijklmnop", 0, name, false)
			if err != nil {
				t.Fatalf("buildPlanOutline: %v", err)
			}
			raw, got := planOutlineJSON(t, report, false)
			if _, ok := got["sections"]; ok {
				t.Fatalf("--check %s should omit sections: %s", name, raw)
			}
			checks, _ := got["checks"].(map[string]any)
			if len(checks) == 0 {
				t.Fatalf("--check %s produced no checks: %s", name, raw)
			}
		})
	}
}
