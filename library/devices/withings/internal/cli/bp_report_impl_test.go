// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored synthetic-data test for `bp-report`.

package cli

import (
	"testing"
	"time"
)

func TestComputeBPReport_RowCarriesBPAndNote(t *testing.T) {
	s, _ := newTestStore(t)

	// A blood-pressure measurement group: systolic (type 10) 128, diastolic
	// (type 9) 82, pulse (type 11) 64 — all unit 0 (whole mmHg / bpm).
	day := daysAgoYMD(5)
	epoch := daysAgoEpoch(5)
	upsertJSON(t, s, "measure", "bp1", measureGrp(10, epoch,
		measureValue{Value: 128, Type: 10, Unit: 0},
		measureValue{Value: 82, Type: 9, Unit: 0},
		measureValue{Value: 64, Type: 11, Unit: 0}))

	// Persist an annotation for that date via the store API the command uses.
	if err := s.EnsureBPNotesTable(); err != nil {
		t.Fatalf("EnsureBPNotesTable: %v", err)
	}
	if err := s.UpsertBPNote(day, "started 5mg lisinopril"); err != nil {
		t.Fatalf("UpsertBPNote: %v", err)
	}

	rows, err := computeBPReport(s, time.Now().Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("computeBPReport: %v", err)
	}

	var found *bpRow
	for i := range rows {
		if rows[i].Date == day {
			found = &rows[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no row for %s; rows=%+v", day, rows)
	}
	if found.Systolic != 128 {
		t.Errorf("systolic = %d, want 128", found.Systolic)
	}
	if found.Diastolic != 82 {
		t.Errorf("diastolic = %d, want 82", found.Diastolic)
	}
	if found.Pulse != 64 {
		t.Errorf("pulse = %d, want 64", found.Pulse)
	}
	if found.Note != "started 5mg lisinopril" {
		t.Errorf("note = %q, want the annotation text", found.Note)
	}
	// No ECG inserted, so AFib defaults to negative.
	if found.Afib != "negative" {
		t.Errorf("afib = %q, want negative", found.Afib)
	}
}

func TestComputeBPReport_AfibFromHeart(t *testing.T) {
	s, _ := newTestStore(t)
	day := daysAgoYMD(3)
	epoch := daysAgoEpoch(3)
	upsertJSON(t, s, "measure", "bp2", measureGrp(11, epoch,
		measureValue{Value: 140, Type: 10, Unit: 0},
		measureValue{Value: 90, Type: 9, Unit: 0}))
	// A heart recording on the same day with a positive AFib classification.
	upsertJSON(t, s, "heart", "h1", map[string]any{
		"timestamp": epoch,
		"data":      map[string]any{"ecg": map[string]any{"afib": 1}},
	})

	if err := s.EnsureBPNotesTable(); err != nil {
		t.Fatalf("EnsureBPNotesTable: %v", err)
	}
	rows, err := computeBPReport(s, time.Now().Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("computeBPReport: %v", err)
	}
	for _, r := range rows {
		if r.Date == day {
			if r.Afib != "afib" {
				t.Errorf("afib = %q, want afib", r.Afib)
			}
			return
		}
	}
	t.Fatalf("no row for %s", day)
}
