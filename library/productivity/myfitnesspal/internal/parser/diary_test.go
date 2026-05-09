// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
package parser

import (
	"os"
	"strings"
	"testing"
)

func TestParseDiarySynthetic(t *testing.T) {
	t.Parallel()
	f, err := os.Open("testdata/diary_synthetic.html")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	d, err := ParseDiary(f, "2024-01-15", "synth_user")
	if err != nil {
		t.Fatalf("ParseDiary: %v", err)
	}

	if d.Date != "2024-01-15" {
		t.Errorf("Date = %q, want 2024-01-15", d.Date)
	}
	if got := len(d.Meals); got != 2 {
		t.Fatalf("len(Meals) = %d, want 2", got)
	}

	// Breakfast
	if d.Meals[0].Name != "breakfast" {
		t.Errorf("Meals[0].Name = %q, want breakfast", d.Meals[0].Name)
	}
	if got := len(d.Meals[0].Entries); got != 2 {
		t.Errorf("Breakfast entries = %d, want 2", got)
	}
	if d.Meals[0].Entries[0].Name != "Banana, Raw" {
		t.Errorf("Banana name mismatch: %q", d.Meals[0].Entries[0].Name)
	}
	if got := d.Meals[0].Entries[0].Nutrients["calories"]; got != 100 {
		t.Errorf("Banana calories = %v, want 100", got)
	}
	if got := d.Meals[0].Entries[0].Nutrients["carbohydrates"]; got != 27 {
		t.Errorf("Banana carbohydrates = %v, want 27 (carbs->carbohydrates abbreviation should resolve)", got)
	}

	// Lunch — second entry has no <a> wrapper, just text.
	if d.Meals[1].Entries[1].Name != "Salad, Mixed Greens" {
		t.Errorf("Salad name fallback mismatch: %q", d.Meals[1].Entries[1].Name)
	}

	// Totals
	if d.Totals == nil {
		t.Fatal("Totals nil")
	}
	if got := d.Totals["calories"]; got != 400 {
		t.Errorf("Total calories = %v, want 400", got)
	}

	// Goals
	if d.Goals == nil {
		t.Fatal("Goals nil")
	}
	if got := d.Goals["calories"]; got != 2000 {
		t.Errorf("Goal calories = %v, want 2000", got)
	}
	if got := d.Goals["protein"]; got != 150 {
		t.Errorf("Goal protein = %v, want 150", got)
	}

	// Completion
	if d.Complete {
		t.Error("Complete = true, want false (fixture has incomplete message)")
	}

	// Field normalization
	want := []string{"name", "calories", "carbohydrates", "fat", "protein", "sodium", "sugar"}
	if got := strings.Join(d.Fields, ","); got != strings.Join(want, ",") {
		t.Errorf("Fields = %v, want %v", d.Fields, want)
	}
}

func TestParseDiarySessionExpired(t *testing.T) {
	t.Parallel()
	html := `<html><body><h1>Sign In</h1><a href="/account/forgot">Forgot your password</a></body></html>`
	_, err := ParseDiary(strings.NewReader(html), "2024-01-15", "")
	if err == nil {
		t.Fatal("expected session-expired error, got nil")
	}
	if !strings.Contains(err.Error(), "session expired") {
		t.Errorf("err = %v, want session-expired message", err)
	}
}

func TestParseDiaryEmptyDay(t *testing.T) {
	t.Parallel()
	html := `<html><body><table id="food"><tbody></tbody></table></body></html>`
	d, err := ParseDiary(strings.NewReader(html), "2024-01-15", "")
	if err != nil {
		t.Fatalf("ParseDiary: %v", err)
	}
	if len(d.Meals) != 0 {
		t.Errorf("Meals = %d, want 0", len(d.Meals))
	}
	if len(d.RawErrors) == 0 {
		t.Error("expected RawErrors to be populated for zero-meal page")
	}
}
