// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestFilterByWindowRounds(t *testing.T) {
	rows := []resultRow{
		{DrawDate: "2021-01-01", Value: 1},
		{DrawDate: "2022-01-01", Value: 2},
		{DrawDate: "2023-01-01", Value: 3},
		{DrawDate: "2024-01-01", Value: 4},
	}
	if got := filterByWindow(rows, "2rounds"); len(got) != 2 || got[0].Value != 3 {
		t.Errorf("2rounds = %+v, want last two (values 3,4)", got)
	}
	if got := filterByWindow(rows, ""); len(got) != 4 {
		t.Errorf("empty window should pass all rows, got %d", len(got))
	}
	if got := filterByWindow(rows, "9rounds"); len(got) != 4 {
		t.Errorf("window larger than series should pass all rows, got %d", len(got))
	}
}

func TestFilterByWindowAgeExcludesOld(t *testing.T) {
	// A draw from the year 2000 is always older than one year, so "1y" must drop
	// it. An unparseable date is conservatively kept.
	rows := []resultRow{
		{DrawDate: "2000-01-01", Value: 1},
		{DrawDate: "not-a-date", Value: 2},
	}
	got := filterByWindow(rows, "1y")
	if len(got) != 1 || got[0].Value != 2 {
		t.Errorf("1y = %+v, want only the unparseable-date row kept", got)
	}
}

func TestSparkline(t *testing.T) {
	if got := sparkline(nil); got != "" {
		t.Errorf("empty sparkline = %q, want empty", got)
	}
	if got := sparkline([]float64{5}); got != "▁" {
		t.Errorf("single-value sparkline = %q, want lowest bar", got)
	}
	if got := sparkline([]float64{0, 1, 2, 3, 4, 5, 6, 7}); got != "▁▂▃▄▅▆▇█" {
		t.Errorf("ramp sparkline = %q, want full ramp", got)
	}
	if got := sparkline([]float64{1, 2, 3}); len([]rune(got)) != 3 {
		t.Errorf("sparkline length = %d, want one rune per value", len([]rune(got)))
	}
}

func TestFilterByBiomarker(t *testing.T) {
	rows := []resultRow{
		{BiomarkerName: "ApoB", BiomarkerID: "id-apob"},
		{BiomarkerName: "Glucose", BiomarkerID: "id-glucose"},
	}
	if got := filterByBiomarker(rows, "apob"); len(got) != 1 || got[0].BiomarkerName != "ApoB" {
		t.Errorf("case-insensitive name match = %+v, want ApoB", got)
	}
	if got := filterByBiomarker(rows, "id-glucose"); len(got) != 1 || got[0].BiomarkerName != "Glucose" {
		t.Errorf("exact ID match = %+v, want Glucose", got)
	}
	if got := filterByBiomarker(rows, ""); len(got) != 2 {
		t.Errorf("empty query should return all rows, got %d", len(got))
	}
}
