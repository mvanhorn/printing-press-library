package scraper

import (
	"testing"
	"time"
)

func TestEstimated1RM(t *testing.T) {
	cases := []struct {
		weight float64
		reps   int
		want   float64
	}{
		{135, 1, 135},
		{100, 10, 133.33333333333334},
		{225, 5, 262.5},
		{0, 10, 0},
		{100, 0, 0},
	}
	for _, c := range cases {
		got := Estimated1RM(c.weight, c.reps)
		if c.want == 0 && got != 0 {
			t.Errorf("Estimated1RM(%v, %v) = %v, want 0", c.weight, c.reps, got)
		} else if c.want != 0 && (got < c.want*0.99 || got > c.want*1.01) {
			t.Errorf("Estimated1RM(%v, %v) = %v, want ~%v", c.weight, c.reps, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"May 17, 2026", time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)},
		{"January 1, 2026", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-05-17", time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
		{"not a date", time.Time{}},
	}
	for _, c := range cases {
		got := parseDate(c.input)
		if !got.Equal(c.want) {
			t.Errorf("parseDate(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestExercisesJSON(t *testing.T) {
	w := &Workout{
		Exercises: []Exercise{
			{
				Name: "Bench Press",
				Slug: "bench-press",
				Sets: []Set{
					{Number: 1, WeightLbs: 135, Reps: 10},
					{Number: 2, WeightLbs: 135, Reps: 8},
				},
			},
		},
	}
	got := w.ExercisesJSON()
	if got == "" || got == "[]" {
		t.Errorf("ExercisesJSON() returned empty for non-empty workout")
	}
	if !contains(got, "Bench Press") {
		t.Errorf("ExercisesJSON() missing exercise name: %s", got)
	}
	if !contains(got, "135") {
		t.Errorf("ExercisesJSON() missing weight: %s", got)
	}
}

func TestParseProfilePage(t *testing.T) {
	html := `<html><body>
<div class="user-workout">
  <a class="stretched-link" href="/workouts/12345/">Full Body + Core ›</a>
  <span class="started-at">May 17, 2026</span>
</div>
<div class="user-workout">
  <a class="stretched-link" href="/workouts/67890/">Pull Day ›</a>
  <span class="started-at">May 15, 2026</span>
</div>
<a href="/users/123/?page=2">Next ›</a>
</body></html>`

	summaries, hasNext, err := ParseProfilePage(html)
	if err != nil {
		t.Fatalf("ParseProfilePage error: %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("got %d summaries, want 2", len(summaries))
	}
	if summaries[0].ID != "12345" {
		t.Errorf("got ID %q, want 12345", summaries[0].ID)
	}
	if !contains(summaries[0].Title, "Full Body") {
		t.Errorf("got title %q, want to contain Full Body", summaries[0].Title)
	}
	if !hasNext {
		t.Errorf("hasNext = false, want true")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
