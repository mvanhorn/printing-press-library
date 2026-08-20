package history

import (
	"testing"
	"time"
)

func tsOn(date string, loc *time.Location) float64 {
	t, err := time.ParseInLocation("2006-01-02 15:04", date, loc)
	if err != nil {
		panic(err)
	}
	return float64(t.Unix())
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestAddAndListSessions(t *testing.T) {
	loc := time.Local
	s := newTestStore(t)
	want := Session{ID: "a", StartTS: tsOn("2026-06-01 08:00", loc), DurationS: 600, DistanceM: 1000, Steps: 1500, MaxSpeedKmh: 3.0}
	if err := s.AddSession(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a" || got[0].DistanceM != 1000 {
		t.Fatalf("Sessions = %+v", got)
	}
}

func TestSessionsEmptyStore(t *testing.T) {
	s := newTestStore(t)
	got, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestTotalsOnAndSessionsOn(t *testing.T) {
	loc := time.Local
	s := newTestStore(t)
	_ = s.AddSession(Session{ID: "a", StartTS: tsOn("2026-06-01 08:00", loc), DurationS: 600, DistanceM: 1000, Steps: 1500})
	_ = s.AddSession(Session{ID: "b", StartTS: tsOn("2026-06-01 18:00", loc), DurationS: 300, DistanceM: 500, Steps: 700})
	_ = s.AddSession(Session{ID: "c", StartTS: tsOn("2026-06-02 09:00", loc), DurationS: 100, DistanceM: 200, Steps: 300})

	totals, err := s.TotalsOn("2026-06-01", loc)
	if err != nil {
		t.Fatal(err)
	}
	if totals.DistanceM != 1500 || totals.Steps != 2200 || totals.DurationS != 900 || totals.Sessions != 2 {
		t.Fatalf("TotalsOn 06-01 = %+v", totals)
	}
	day1, err := s.SessionsOn("2026-06-01", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(day1) != 2 {
		t.Fatalf("SessionsOn 06-01 = %d, want 2", len(day1))
	}
}

func TestDailySeriesIncludesZeroDays(t *testing.T) {
	loc := time.Local
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, loc)
	s := newTestStore(t)
	// Walk only on 06-01; 06-02 and 06-03 should appear as zero days.
	_ = s.AddSession(Session{ID: "a", StartTS: tsOn("2026-06-01 08:00", loc), DistanceM: 1000, Steps: 1500, DurationS: 600})

	series, err := s.DailySeries(3, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 3 {
		t.Fatalf("series len = %d, want 3", len(series))
	}
	if series[0].Date != "2026-06-01" || series[0].DistanceM != 1000 {
		t.Errorf("series[0] = %+v", series[0])
	}
	if series[1].DistanceM != 0 || series[1].Sessions != 0 {
		t.Errorf("series[1] (zero day) = %+v", series[1])
	}
	if series[2].Date != "2026-06-03" {
		t.Errorf("series[2].Date = %s, want 2026-06-03", series[2].Date)
	}
}

func TestStreak(t *testing.T) {
	loc := time.Local
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, loc)
	s := newTestStore(t)
	// Consecutive 06-01, 06-02, 06-03 -> streak 3.
	for _, d := range []string{"2026-06-01 08:00", "2026-06-02 08:00", "2026-06-03 08:00"} {
		_ = s.AddSession(Session{ID: d, StartTS: tsOn(d, loc), DistanceM: 500})
	}
	streak, err := s.Streak(now)
	if err != nil {
		t.Fatal(err)
	}
	if streak != 3 {
		t.Fatalf("Streak = %d, want 3", streak)
	}
}

func TestStreakIgnoresZeroDistance(t *testing.T) {
	loc := time.Local
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, loc)
	s := newTestStore(t)
	// A zero-distance session today must not count as an active day.
	_ = s.AddSession(Session{ID: "today-zero", StartTS: tsOn("2026-06-03 08:00", loc), DistanceM: 0})
	_ = s.AddSession(Session{ID: "yesterday", StartTS: tsOn("2026-06-02 08:00", loc), DistanceM: 500})
	streak, err := s.Streak(now)
	if err != nil {
		t.Fatal(err)
	}
	if streak != 1 { // only yesterday counts; today has no real walk
		t.Fatalf("Streak = %d, want 1", streak)
	}
}
