package cli

import "testing"

func TestExtractTierAxes(t *testing.T) {
	cases := []struct {
		in   string
		want tierAxes
	}{
		{"general admission", tierAxes{AccessClass: "ga", Matched: true}},
		{"vip experience", tierAxes{AccessClass: "vip", Matched: true}},
		{"super early birds: must enter by 11pm", tierAxes{SalesStage: "super_early_bird", EntryWindowType: "deadline", EntryWindowTime: "23:00", Matched: true}},
		{"early birds: must enter by 5pm", tierAxes{SalesStage: "early_bird", EntryWindowType: "deadline", EntryWindowTime: "17:00", Matched: true}},
		{"ga anytime entry", tierAxes{AccessClass: "ga", EntryWindowType: "anytime", Matched: true}},
		{"tickets available at the door", tierAxes{EntryWindowType: "door", Matched: true}},
		{"group ticket (4)", tierAxes{GroupSize: 4, Matched: true}},
		{"you+2", tierAxes{GroupSize: 3, Matched: true}},
		{"comp", tierAxes{CompFlag: true, Matched: true}},
		{"zzz unknown label", tierAxes{Matched: false}},
		// sales_stage=final_release must match.
		{"final release", tierAxes{SalesStage: "final_release", Matched: true}},
		{"final tickets", tierAxes{SalesStage: "final_release", Matched: true}},
		// Bare "final" at end-of-string must match.
		{"vip final", tierAxes{AccessClass: "vip", SalesStage: "final_release", Matched: true}},
		// False positives that must NOT match final_release.
		{"final countdown", tierAxes{Matched: false}},
		{"finalists", tierAxes{Matched: false}},

		// --- Task A: entry-window deadline synonyms ---
		// "enter before <time>" -> deadline 22:00
		{"enter before 10pm", tierAxes{EntryWindowType: "deadline", EntryWindowTime: "22:00", Matched: true}},
		// "must be in by <time>" -> deadline 23:00
		{"must be in by 11pm", tierAxes{EntryWindowType: "deadline", EntryWindowTime: "23:00", Matched: true}},
		// "entry by <time>" -> deadline 21:00
		{"entry by 9pm", tierAxes{EntryWindowType: "deadline", EntryWindowTime: "21:00", Matched: true}},
		// "arrive before <time>" -> deadline
		{"arrive before 8pm", tierAxes{EntryWindowType: "deadline", EntryWindowTime: "20:00", Matched: true}},
		// "be in by <time>" -> deadline (no "must")
		{"be in by 10pm", tierAxes{EntryWindowType: "deadline", EntryWindowTime: "22:00", Matched: true}},
		// Existing patterns still work.
		{"must arrive by 9pm", tierAxes{EntryWindowType: "deadline", EntryWindowTime: "21:00", Matched: true}},

		// --- Task A: group_size from table/booth WITH a number ---
		// "table for N" -> group_size N
		{"table for 6", tierAxes{GroupSize: 6, Matched: true}},
		// "booth of N" -> group_size N
		{"booth of 4", tierAxes{GroupSize: 4, Matched: true}},
		// "table of N" -> group_size N
		{"table of 8", tierAxes{GroupSize: 8, Matched: true}},
		// "booth for N" -> group_size N
		{"booth for 2", tierAxes{GroupSize: 2, Matched: true}},
		// "vip booth" (no number) -> group_size 0 (NOT matched by this rule)
		{"vip booth", tierAxes{AccessClass: "vip", Matched: true}},
		// bare "table" -> no group_size
		{"table", tierAxes{Matched: false}},

		// --- Task A: comp from guestlist/guest ---
		{"guestlist", tierAxes{CompFlag: true, Matched: true}},
		{"guest list", tierAxes{CompFlag: true, Matched: true}},
		// "guest" standalone
		{"vip guest", tierAxes{AccessClass: "vip", CompFlag: true, Matched: true}},
		// "guests welcome" — "guest" substring matched via \bguest\b; comp_flag=true is acceptable per spec
		// (spec says guest of honour vip -> comp true is fine; we only guard other axes)
		// "complimentary" still works
		{"complimentary ticket", tierAxes{CompFlag: true, Matched: true}},
	}
	for _, c := range cases {
		got := extractTierAxes(c.in)
		if got != c.want {
			t.Errorf("extractTierAxes(%q) =\n  %+v\nwant\n  %+v", c.in, got, c.want)
		}
	}
}

// TestEntryWindowDeadlineSynonyms verifies the entry-window deadline synonym
// patterns added in Task A produce the correct 24-hour time values.
func TestEntryWindowDeadlineSynonyms(t *testing.T) {
	cases := []struct {
		in       string
		wantTime string
	}{
		{"entry by 9pm", "21:00"},
		{"enter before 10pm", "22:00"},
		{"must be in by 11pm", "23:00"},
		{"must enter by 12pm", "12:00"},
		{"arrive before 12am", "00:00"},
		{"must arrive by 8:30pm", "20:30"},
		{"enter by 6:00am", "06:00"},
	}
	for _, c := range cases {
		got := extractTierAxes(c.in)
		if got.EntryWindowType != "deadline" {
			t.Errorf("entry_window_type(%q) = %q, want deadline", c.in, got.EntryWindowType)
		}
		if got.EntryWindowTime != c.wantTime {
			t.Errorf("entry_window_time(%q) = %q, want %q", c.in, got.EntryWindowTime, c.wantTime)
		}
	}
}

// TestTableBoothGroupSize verifies that table/booth with a number sets group_size,
// and that bare table/booth (no number) does NOT.
func TestTableBoothGroupSize(t *testing.T) {
	positives := []struct {
		in   string
		want int
	}{
		{"table for 6", 6},
		{"booth of 4", 4},
		{"table of 8", 8},
		{"booth for 2", 2},
		{"table 3", 3},
		{"vip table for 10", 10},
	}
	for _, c := range positives {
		got := extractTierAxes(c.in)
		if got.GroupSize != c.want {
			t.Errorf("group_size(%q) = %d, want %d", c.in, got.GroupSize, c.want)
		}
	}
	negatives := []string{"vip booth", "table", "booth", "lounge table area"}
	for _, in := range negatives {
		got := extractTierAxes(in)
		if got.GroupSize != 0 {
			t.Errorf("group_size(%q) = %d, want 0 (no number present)", in, got.GroupSize)
		}
	}
}

// TestGuestlistComp verifies that guestlist/guest list/\bguest\b set comp_flag=true.
func TestGuestlistComp(t *testing.T) {
	positives := []string{
		"guestlist",
		"guest list",
		"vip guest",
		"guest of honour",
	}
	for _, in := range positives {
		got := extractTierAxes(in)
		if !got.CompFlag {
			t.Errorf("comp_flag(%q) = false, want true", in)
		}
	}
}

// TestParse12hEdgeCases verifies 12am → "00:00" and 12pm → "12:00".
func TestParse12hEdgeCases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"must enter by 12am", "00:00"},
		{"must enter by 12pm", "12:00"},
	}
	for _, c := range cases {
		got := extractTierAxes(c.in)
		if got.EntryWindowTime != c.want {
			t.Errorf("parse12h(%q): EntryWindowTime = %q, want %q", c.in, got.EntryWindowTime, c.want)
		}
		if got.EntryWindowType != "deadline" {
			t.Errorf("parse12h(%q): EntryWindowType = %q, want %q", c.in, got.EntryWindowType, "deadline")
		}
	}
}
