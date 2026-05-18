// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

func mortgage(cfn int64, date, mortgagor, lender string) *store.Recording {
	return &store.Recording{
		CFNMasterID: cfn, DocTypeCode: "MOR", RecordingDate: date,
		FirstParty: mortgagor, SecondParty: lender,
	}
}

func satisfaction(cfn int64, date, lender, mortgagor string) *store.Recording {
	return &store.Recording{
		CFNMasterID: cfn, DocTypeCode: "SAT", RecordingDate: date,
		FirstParty: lender, SecondParty: mortgagor,
	}
}

func TestFindMatchingRelease(t *testing.T) {
	t.Run("exact-case match released", func(t *testing.T) {
		lien := mortgage(100, "2020-01-01", "DOE JOHN", "BIG BANK NA")
		releases := map[string][]*store.Recording{
			"SAT": {satisfaction(200, "2024-06-01", "BIG BANK NA", "DOE JOHN")},
		}
		got := findMatchingRelease(lien, releases, map[int64]bool{})
		if got != 200 {
			t.Errorf("expected release 200 to match, got %d", got)
		}
	})

	t.Run("mixed-case match released (regression: clerk casing drift)", func(t *testing.T) {
		lien := mortgage(101, "2020-01-01", "DOE JOHN", "BIG BANK NA")
		releases := map[string][]*store.Recording{
			"SAT": {satisfaction(201, "2024-06-01", "Big Bank Na", "Doe John")},
		}
		got := findMatchingRelease(lien, releases, map[int64]bool{})
		if got != 201 {
			t.Errorf("expected release 201 to match across case drift, got %d", got)
		}
	})

	t.Run("each release consumed once", func(t *testing.T) {
		lien1 := mortgage(102, "2020-01-01", "DOE JOHN", "BIG BANK NA")
		lien2 := mortgage(103, "2021-01-01", "DOE JOHN", "BIG BANK NA")
		releases := map[string][]*store.Recording{
			"SAT": {satisfaction(202, "2024-06-01", "BIG BANK NA", "DOE JOHN")},
		}
		used := map[int64]bool{}
		got1 := findMatchingRelease(lien1, releases, used)
		if got1 != 202 {
			t.Fatalf("lien1: expected 202, got %d", got1)
		}
		used[got1] = true
		got2 := findMatchingRelease(lien2, releases, used)
		if got2 != 0 {
			t.Errorf("lien2: release should be consumed; expected 0, got %d", got2)
		}
	})

	t.Run("only one party matches → not released (conservative)", func(t *testing.T) {
		lien := mortgage(104, "2020-01-01", "DOE JOHN", "BIG BANK NA")
		releases := map[string][]*store.Recording{
			"SAT": {satisfaction(203, "2024-06-01", "BIG BANK NA", "SMITH JANE")},
		}
		got := findMatchingRelease(lien, releases, map[int64]bool{})
		if got != 0 {
			t.Errorf("expected no match when only one party aligns, got %d", got)
		}
	})

	t.Run("release dated before lien → not released", func(t *testing.T) {
		lien := mortgage(105, "2024-06-01", "DOE JOHN", "BIG BANK NA")
		releases := map[string][]*store.Recording{
			"SAT": {satisfaction(204, "2020-01-01", "BIG BANK NA", "DOE JOHN")},
		}
		got := findMatchingRelease(lien, releases, map[int64]bool{})
		if got != 0 {
			t.Errorf("expected no match when release predates lien, got %d", got)
		}
	})
}
