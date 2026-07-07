// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestDistinctiveBiomarkerName(t *testing.T) {
	tests := []struct {
		q    string
		want bool
	}{
		{"", false},
		{"ApoB", true},     // internal uppercase
		{"hsCRP", true},    // internal uppercase
		{"ALT", true},      // internal uppercase (abbreviation)
		{"Vitamin D", true}, // has a space
		{"Omega-3", true},  // has a digit / hyphen
		{"Glucose", true},  // length >= 5
		{"Lead", false},    // short all-lowercase common word
		{"Iron", false},    // short all-lowercase common word
	}
	for _, tc := range tests {
		if got := distinctiveBiomarkerName(tc.q); got != tc.want {
			t.Errorf("distinctiveBiomarkerName(%q) = %v, want %v", tc.q, got, tc.want)
		}
	}
}

func TestWholeWordContains(t *testing.T) {
	tests := []struct {
		text, query string
		want        bool
	}{
		{"Iron is low this round", "iron", true},
		{"leading cause of fatigue", "lead", false}, // substring, not whole word
		{"ApoB trending up", "apob", true},          // case-insensitive
		{"nothing relevant here", "glucose", false},
		{"anything", "", false}, // empty query never matches
	}
	for _, tc := range tests {
		if got := wholeWordContains(tc.text, tc.query); got != tc.want {
			t.Errorf("wholeWordContains(%q, %q) = %v, want %v", tc.text, tc.query, got, tc.want)
		}
	}
}

func TestParseNoteRecords(t *testing.T) {
	bare := []byte(`[{"date":"2024-01-01","notes":[{"note":"Iron low","category":{"categoryName":"Nutrients"}}]}]`)
	if got := parseNoteRecords(bare); len(got) != 1 || len(got[0].Notes) != 1 || got[0].Notes[0].Note != "Iron low" {
		t.Errorf("bare array parse = %+v, want one record with one note", got)
	}

	wrappedResults := []byte(`{"results":[{"date":"2024-02-02","notes":[]}]}`)
	if got := parseNoteRecords(wrappedResults); len(got) != 1 || got[0].Date != "2024-02-02" {
		t.Errorf("results envelope parse = %+v, want one record", got)
	}

	wrappedData := []byte(`{"data":[{"date":"2024-03-03","notes":[]},{"date":"2024-04-04","notes":[]}]}`)
	if got := parseNoteRecords(wrappedData); len(got) != 2 {
		t.Errorf("data envelope parse = %+v, want two records", got)
	}

	if got := parseNoteRecords([]byte(`not json`)); got != nil {
		t.Errorf("garbage parse = %+v, want nil", got)
	}
}
