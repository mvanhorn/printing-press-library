package walkerplus

import (
	"testing"
	"time"
)

func TestWeekdayRolesAndGrammarSuffix(t *testing.T) {
	for _, tc := range []struct {
		name, schedule   string
		closed, positive []time.Weekday
		monday, sunday   string
	}{
		{"closure", "毎週月曜日休館", []time.Weekday{time.Monday}, nil, "excluded", "possible"},
		{"labelled_closure", "休館日：毎週月曜日", []time.Weekday{time.Monday}, nil, "excluded", "possible"},
		{"positive", "毎週月曜日開催", nil, []time.Weekday{time.Monday}, "confirmed", "excluded"},
		{"mixed", "毎週土曜開催・月曜休館", []time.Weekday{time.Monday}, []time.Weekday{time.Saturday}, "excluded", "excluded"},
		{"multiple_closures", "休館日：月曜日・火曜日", []time.Weekday{time.Monday, time.Tuesday}, nil, "excluded", "possible"},
		{"coordinated_closures", "毎週月曜日と木曜日は休み", []time.Weekday{time.Monday, time.Thursday}, nil, "excluded", "possible"},
		{"spaced_coordinated_closures", "毎週月曜日 と 木曜日 は休み", []time.Weekday{time.Monday, time.Thursday}, nil, "excluded", "possible"},
		{"labelled_coordinated_closures", "休館日：毎週月曜日および木曜日", []time.Weekday{time.Monday, time.Thursday}, nil, "excluded", "possible"},
		{"coordinated_positive", "毎週月曜日及び木曜日開催", nil, []time.Weekday{time.Monday, time.Thursday}, "confirmed", "excluded"},
		{"coordinated_mixed", "毎週土曜開催。毎週月曜日と木曜日は休み", []time.Weekday{time.Monday, time.Thursday}, []time.Weekday{time.Saturday}, "excluded", "excluded"},
		{"weekend_closures", "休館日：土日", []time.Weekday{time.Saturday, time.Sunday}, nil, "possible", "excluded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 "+tc.schedule)
			if len(e.Schedule.closed) != len(tc.closed) || len(e.Schedule.weekdays) != len(tc.positive) {
				t.Fatalf("wrong weekday roles: %+v", e.Schedule)
			}
			for _, day := range tc.closed {
				if !containsWeekday(e.Schedule.closed, day) {
					t.Fatalf("missing closure %v: %+v", day, e.Schedule)
				}
			}
			for _, day := range tc.positive {
				if !containsWeekday(e.Schedule.weekdays, day) {
					t.Fatalf("missing positive recurrence %v: %+v", day, e.Schedule)
				}
			}
			for _, day := range []struct{ date, want string }{{"2026-10-12", tc.monday}, {"2026-10-18", tc.sunday}} {
				if got := matchEvent(e, Query{From: day.date, To: day.date, Timing: "overlap"}, true); got.State != day.want {
					t.Fatalf("%s: got %+v, want %s", day.date, got, day.want)
				}
			}
		})
	}
}

func TestCoordinatedClosureKeepsOtherDaysPossible(t *testing.T) {
	for _, raw := range []string{"毎週月曜日と木曜日は休み", "休館日：毎週月曜日及び木曜日", "毎週月曜日並びに木曜日は休館"} {
		e := scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 "+raw)
		if e.Schedule.Recurrence != nil || len(e.Schedule.closed) != 2 || !containsWeekday(e.Schedule.closed, time.Monday) || !containsWeekday(e.Schedule.closed, time.Thursday) {
			t.Fatalf("negative coordination leaked into positive recurrence: %+v", e.Schedule)
		}
		for _, day := range []struct{ date, want string }{{"2026-10-12", "excluded"}, {"2026-10-15", "excluded"}, {"2026-10-13", "possible"}, {"2026-10-18", "possible"}} {
			got := matchEvent(e, Query{From: day.date, To: day.date, Timing: "overlap"}, true)
			if got.State != day.want || len(got.ConfirmedDays) != 0 {
				t.Fatalf("%s: got %+v, want %s", day.date, got, day.want)
			}
		}
	}
	e := scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 毎日開催。毎週月曜日と木曜日は休み")
	for _, day := range []struct{ date, want string }{{"2026-10-12", "excluded"}, {"2026-10-15", "excluded"}, {"2026-10-13", "confirmed"}, {"2026-10-18", "confirmed"}} {
		if got := matchEvent(e, Query{From: day.date, To: day.date, Timing: "overlap"}, true); got.State != day.want {
			t.Fatalf("daily plus coordinated closure on %s: %+v", day.date, got)
		}
	}
}

func TestNonExhaustiveWeekdayListDoesNotBecomeExclusive(t *testing.T) {
	for _, raw := range []string{
		"毎週月曜日や木曜日開催",
		"毎週月曜日 や 木曜日開催",
		"月曜日や木曜日のみ開催",
		"休館日：毎週月曜日や木曜日",
		"毎週月曜日や木曜日は休み",
		"毎日開催。月曜や木曜は休み",
	} {
		e := scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 "+raw)
		if len(e.Schedule.weekdays) > 0 || len(e.Schedule.closed) > 0 || !containsDate(e.Schedule.Unresolved, "non-exhaustive weekday list is unresolved") {
			t.Fatalf("partial weekday rule from %q: %+v", raw, e.Schedule)
		}
		for _, date := range []string{"2026-10-12", "2026-10-15", "2026-10-18"} {
			if got := matchEvent(e, Query{From: date, To: date, Timing: "overlap"}, true); got.State != "possible" || len(got.ConfirmedDays) > 0 {
				t.Fatalf("%q on %s produced exclusive activity: %+v", raw, date, got)
			}
		}
	}
}

func TestUnsupportedMonthlyAndHolidaySchedulesStayPossible(t *testing.T) {
	for _, raw := range []string{"毎月第1日曜日開催", "第1月曜日開催", "毎月開催", "休館日：毎週月曜日(祝日の場合は翌平日)"} {
		e := scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 "+raw)
		if len(e.Schedule.Unresolved) == 0 || e.Schedule.Recurrence != nil {
			t.Fatalf("unsupported/closure grammar became positive recurrence: %+v", e.Schedule)
		}
		for _, date := range []string{"2026-10-12", "2026-10-13", "2026-10-18"} {
			got := matchEvent(e, Query{From: date, To: date, Timing: "overlap"}, true)
			if got.State != "possible" || len(got.ConfirmedDays) != 0 {
				t.Fatalf("%s: unsupported schedule confirmed/excluded %s: %+v", raw, date, got)
			}
		}
	}
	e := scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 毎日開催。毎週月曜日休館")
	if got := matchEvent(e, Query{From: "2026-10-13", To: "2026-10-13", Timing: "overlap"}, true); got.State != "confirmed" {
		t.Fatalf("explicit daily evidence lost: %+v", got)
	}
	if got := matchEvent(e, Query{From: "2026-10-12", To: "2026-10-12", Timing: "overlap"}, true); got.State != "excluded" {
		t.Fatalf("daily evidence overrode closure: %+v", got)
	}
}

func TestDailyTokenAndWeekdayExclusions(t *testing.T) {
	for _, tc := range []struct {
		name, raw, recurrence             string
		monday, tuesday, saturday, sunday string
	}{
		{"short_sunday", "毎日曜開催", "weekly", "excluded", "excluded", "excluded", "confirmed"},
		{"long_sunday", "毎日曜日開催", "weekly", "excluded", "excluded", "excluded", "confirmed"},
		{"daily_control", "毎日開催", "daily", "confirmed", "confirmed", "confirmed", "confirmed"},
		{"excluded_monday", "毎日開催（月曜を除く）", "daily", "excluded", "confirmed", "confirmed", "confirmed"},
		{"excluded_weekend", "毎日開催。土日を除く", "daily", "confirmed", "confirmed", "excluded", "excluded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 "+tc.raw)
			if value(e.Schedule.Recurrence) != tc.recurrence {
				t.Fatalf("wrong recurrence for %s: %+v", tc.raw, e.Schedule)
			}
			for _, day := range []struct{ date, want string }{{"2026-10-12", tc.monday}, {"2026-10-13", tc.tuesday}, {"2026-10-17", tc.saturday}, {"2026-10-18", tc.sunday}} {
				got := matchEvent(e, Query{From: day.date, To: day.date, Timing: "overlap"}, true)
				if got.State != day.want {
					t.Fatalf("%s: got %+v, want %s", day.date, got, day.want)
				}
			}
		})
	}
	e := scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 毎日開催。10月18日(日)を除く")
	if len(e.Schedule.closed) != 0 {
		t.Fatalf("calendar exclusion became a weekly closure: %+v", e.Schedule)
	}
	for _, day := range []struct{ date, want string }{{"2026-10-11", "confirmed"}, {"2026-10-18", "excluded"}} {
		if got := matchEvent(e, Query{From: day.date, To: day.date, Timing: "overlap"}, true); got.State != day.want {
			t.Fatalf("date exclusion affected other Sundays: %+v", got)
		}
	}
}
