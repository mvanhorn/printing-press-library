package tab

import (
	"math"
	"testing"
)

func TestDatesAndSpan(t *testing.T) {
	for _, tt := range []struct {
		in    string
		valid bool
		want  string
	}{{"2026-02-28", true, "2026-02-28"}, {"2024-02-29", true, "2024-02-29"}, {"2026-02-29", false, ""}, {"2026-2-01", false, ""}, {"10-01", false, ""}} {
		t.Run(tt.in, func(t *testing.T) {
			d, e := ParseDate(tt.in)
			if (e == nil) != tt.valid {
				t.Fatalf("date %v", e)
			}
			if e == nil && d.Format("2006-01-02") != tt.want {
				t.Fatal(d)
			}
		})
	}
	for _, tt := range []struct {
		start, end, today, want string
		unconfirmed             bool
	}{{"2004-08-24", "2004-12-05", "2026-10-01", "archived", false}, {"2026-10-01", "2026-10-07", "2026-10-01", "in_span", false}, {"2026-10-01", "2026-10-07", "2026-10-07", "in_span", false}, {"2026-10-01", "2026-10-07", "2026-10-08", "archived", false}, {"2026-10-07", "2026-10-01", "2026-10-04", "unknown", false}, {"2026-10-01", "2026-10-07", "2026-10-08", "end_unconfirmed", true}, {"", "", "2026-10-01", "unknown", false}} {
		u := tt.unconfirmed
		if got := SpanStatus(str(tt.start), str(tt.end), &u, tt.today); got != tt.want {
			t.Fatalf("%+v: %s", tt, got)
		}
	}
	for _, tt := range []struct{ raw, want string }{{"2004-12-05T00:00:00.000Z", "2004-12-05"}, {"2026-10-01", "2026-10-01"}, {"invalid", ""}} {
		if got := sourceDate(str(tt.raw)); (tt.want == "" && got != nil) || (tt.want != "" && (got == nil || *got != tt.want)) {
			t.Fatal(got)
		}
	}
}
func TestAssessDayConservative(t *testing.T) {
	yes := true
	for _, tt := range []struct {
		name, date, want string
		eh, vh           *Hours
		end              *string
		unconfirmed      *bool
	}{{"weekly", "2026-10-05", "weekly_closure", &Hours{ClosedDays: []string{"Monday"}}, nil, str("2026-10-10"), nil}, {"explicit empty override", "2026-10-05", "no_listed_weekly_closure", &Hours{ClosedDays: []string{}}, &Hours{ClosedDays: []string{"Monday"}}, str("2026-10-10"), nil}, {"missing uses venue", "2026-10-05", "weekly_closure", nil, &Hours{ClosedDays: []string{"Monday"}}, str("2026-10-10"), nil}, {"exception", "2026-10-05", "unknown", &Hours{ClosedDays: []string{"Monday"}, SpecialCases: Name{JA: str("祝日は開館")}}, nil, str("2026-10-10"), nil}, {"hidden", "2026-10-05", "unknown", &Hours{ClosedDays: []string{}, HiddenClosedDays: &yes}, nil, str("2026-10-10"), nil}, {"outside", "2026-10-11", "outside_span", nil, nil, str("2026-10-10"), nil}, {"unknown end", "2026-10-11", "unknown", nil, nil, str("2026-10-10"), &yes}, {"missing schedule", "2026-10-05", "unknown", nil, nil, str("2026-10-10"), nil}, {"holiday", "2026-10-05", "unknown", &Hours{ClosedDays: []string{"Holidays"}}, nil, str("2026-10-10"), nil}} {
		t.Run(tt.name, func(t *testing.T) {
			e := Event{Starts: str("2026-10-01"), Ends: tt.end, EndUnconfirmed: tt.unconfirmed, Hours: tt.eh, Venue: &Venue{Hours: tt.vh}}
			d, err := AssessDay(e, tt.date)
			if err != nil || d.Status != tt.want {
				t.Fatalf("%+v %v", d, err)
			}
		})
	}
	if _, err := AssessDay(Event{}, "2026-02-30"); err == nil {
		t.Fatal("invalid on accepted")
	}
}
func TestAdmissionAndNames(t *testing.T) {
	for _, tt := range []struct{ en, ja, want string }{{"Free", "無料", "free"}, {"Adults ¥1000, children free", "一般1000円、小学生無料", "priced"}, {"TBD", "未定", "unknown"}, {"Members free", "会員無料", "unknown"}, {"", "無料", "free"}} {
		if a := admission(Name{str(tt.en), str(tt.ja)}, nil); a.Status != tt.want {
			t.Fatalf("%+v: %+v", tt, a)
		}
	}
	e := Entry{Fields: map[string]map[string]any{"eventName": {"ja-JP": "日本語のみ"}, "scheduleSpecialCases": {"en-US": "Open Tuesdays. Last admission 30 minutes before closing.", "ja-JP": "最終入場は閉館30分前。"}}}
	if names(e, "eventName").EN != nil {
		t.Fatal("fabricated English")
	}
	n := lastAdmissionText(e)
	if n.EN == nil || n.JA == nil {
		t.Fatal(n)
	}
	if safeURL("javascript:alert(1)") || safeURL("https://u:p@example.com") {
		t.Fatal("unsafe URL accepted")
	}
}
func TestDistance(t *testing.T) {
	for _, tt := range []struct {
		a, b Geo
		want float64
	}{{Geo{35, 139}, Geo{35, 139}, 0}, {Geo{0, 0}, Geo{0, 1}, 111.195}, {Geo{0, 0}, Geo{0, 180}, 20015.087}} {
		got := Distance(tt.a, tt.b)
		if math.Abs(got-tt.want) > .01 {
			t.Fatal(got)
		}
	}
}

func TestArchiveEditionYearDiffersFromScheduleYear(t *testing.T) {
	e := Entry{Fields: map[string]map[string]any{"slug": {"en-US": "2004/B7AE"}, "scheduleStartsOn": {"en-US": "2005-01-01"}, "scheduleEndsOn": {"en-US": "2005-02-01"}}}
	e.Sys.ID = "import_event_record__2004_B7AE"
	got := makeEvent(e, map[string]Venue{}, map[string]Entry{}, "2026-10-01", false)
	if got.ArchiveYear == nil || *got.ArchiveYear != 2004 || got.StartYear == nil || *got.StartYear != 2005 {
		t.Fatal(got)
	}
	e.Sys.ID = "modern-id"
	e.Fields["slug"]["en-US"] = "title/venue/2026-10-01"
	if editionYear(e) != nil {
		t.Fatal("modern start date mislabeled archive edition")
	}
}
