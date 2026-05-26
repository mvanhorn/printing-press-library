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
	}
	for _, c := range cases {
		got := extractTierAxes(c.in)
		if got != c.want {
			t.Errorf("extractTierAxes(%q) =\n  %+v\nwant\n  %+v", c.in, got, c.want)
		}
	}
}
