package crestronfw

import (
	"context"
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"
)

type fakeSearcher struct {
	rows []crestronparse.SearchResult
	err  error
}

func (f fakeSearcher) SearchFirmware(context.Context, string, int) ([]crestronparse.SearchResult, error) {
	return f.rows, f.err
}

// The seven-model family release is the case that motivates this package: a
// fleet lookup for TSW-1070 must find the release titled under the whole family.
func TestCoveringReleasesFamilyBreadth(t *testing.T) {
	rows := []crestronparse.SearchResult{
		{Title: "TSW-570/TSW-770/TSW-1070/TSS-770/TSS-1070/TS-770/TS-1070 3.0.1234", Date: "Jun 16, 2026"},
	}
	rels := ReleasesFrom(rows)
	for _, model := range []string{"TSW-570", "TSW-770", "TSW-1070", "TSS-770", "TSS-1070", "TS-770", "TS-1070"} {
		hits := CoveringReleases(rels, model)
		if len(hits) != 1 {
			t.Errorf("%s: got %d covering releases, want 1", model, len(hits))
			continue
		}
		if hits[0].Version != "3.0.1234" {
			t.Errorf("%s: version = %q, want 3.0.1234", model, hits[0].Version)
		}
	}
}

func TestCoveringReleasesNewestFirst(t *testing.T) {
	rels := ReleasesFrom([]crestronparse.SearchResult{
		{Title: "CP4N 2.8006.00110.01", Date: "Jul 15, 2025"},
		{Title: "CP4N 2.8006.00322.01", Date: "Jun 30, 2026"},
		{Title: "CP4N 2.8006.00284.01", Date: "Mar 17, 2026"},
	})
	hits := CoveringReleases(rels, "CP4N")
	if len(hits) != 3 {
		t.Fatalf("got %d releases, want 3", len(hits))
	}
	if hits[0].Version != "2.8006.00322.01" {
		t.Errorf("newest = %q, want 2.8006.00322.01", hits[0].Version)
	}
}

// A model must not match an unrelated release just because both are Crestron.
func TestCoveringReleasesNegative(t *testing.T) {
	rels := ReleasesFrom([]crestronparse.SearchResult{
		{Title: "CP4N 2.8006.00322.01", Date: "Jun 30, 2026"},
		{Title: "DM-NVX-DIR2 5.3.276", Date: "Jan 22, 2026"},
	})
	if hits := CoveringReleases(rels, "TSW-1070"); len(hits) != 0 {
		t.Fatalf("TSW-1070 matched unrelated releases: %+v", hits)
	}
}

func TestCoveringReleasesToleratesPunctuation(t *testing.T) {
	rels := ReleasesFrom([]crestronparse.SearchResult{
		{Title: "DM-NVX-384(C)_DM-NVX-385(C) 7.4.0255.22319", Date: "May 06, 2026"},
	})
	for _, model := range []string{"DM-NVX-384", "DM-NVX-384(C)", "dm nvx 384c"} {
		if hits := CoveringReleases(rels, model); len(hits) == 0 {
			t.Errorf("%q did not match its own release", model)
		}
	}
}

func TestParseFleetFile(t *testing.T) {
	got := ParseFleetFile(`
# my fleet
DM-NVX-384  7.3.0125
CP4N,2.8006.00110.01
TSW-1070=3.0.1000
HD-MD4X2          # no version supplied

`)
	if len(got) != 4 {
		t.Fatalf("parsed %d entries, want 4: %+v", len(got), got)
	}
	want := map[string]string{
		"DM-NVX-384": "7.3.0125",
		"CP4N":       "2.8006.00110.01",
		"TSW-1070":   "3.0.1000",
		"HD-MD4X2":   "",
	}
	for _, s := range got {
		w, ok := want[s.Model]
		if !ok {
			t.Errorf("unexpected model %q", s.Model)
			continue
		}
		if s.Installed != w {
			t.Errorf("%s installed = %q, want %q", s.Model, s.Installed, w)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"7.4.0255", "7.3.0125", 1},
		{"7.3.0125", "7.4.0255", -1},
		{"2.8006.00322.01", "2.8006.00322.01", 0},
		{"3.0.1000", "3.0.999", 1},
		{"5.3", "5.3.276", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestResolveStates(t *testing.T) {
	rows := []crestronparse.SearchResult{
		{Title: "DM-NVX-384(C)_DM-NVX-385(C) 7.4.0255.22319", Date: "May 06, 2026",
			URL: "/Software-Firmware/Firmware/DigitalMedia/DM-NVX-384(C)/7-4-0255-22319"},
	}
	s := fakeSearcher{rows: rows}
	ctx := context.Background()

	out := Resolve(ctx, s, Status{Model: "DM-NVX-384", Installed: "7.3.0125"}, 25)
	if out.State != StateOutdated {
		t.Errorf("state = %q, want %q", out.State, StateOutdated)
	}
	if out.Latest != "7.4.0255.22319" {
		t.Errorf("latest = %q", out.Latest)
	}
	// The covering release names the family, not just the requested model.
	if out.CoveredBy == "" {
		t.Error("expected covered_by to name the release that governs this model")
	}

	out = Resolve(ctx, s, Status{Model: "DM-NVX-384", Installed: "7.4.0255.22319"}, 25)
	if out.State != StateCurrent {
		t.Errorf("state = %q, want %q", out.State, StateCurrent)
	}

	out = Resolve(ctx, s, Status{Model: "DM-NVX-384"}, 25)
	if out.State != StateUnknown {
		t.Errorf("state = %q, want %q when no installed version is supplied", out.State, StateUnknown)
	}
	if out.Latest == "" {
		t.Error("latest version should still be reported without an installed version")
	}

	out = Resolve(ctx, s, Status{Model: "ZZ-NOT-A-MODEL"}, 25)
	if out.State != StateNoRelease {
		t.Errorf("state = %q, want %q", out.State, StateNoRelease)
	}
}

// A failed fetch must be reported, never silently rendered as "no release" —
// that would read as "your fleet is fine" when the lookup actually broke.
func TestResolveSurfacesSearchErrors(t *testing.T) {
	out := Resolve(context.Background(),
		fakeSearcher{err: errors.New("429 rate limited")},
		Status{Model: "CP4N", Installed: "1.0"}, 25)
	if out.State != StateError {
		t.Fatalf("state = %q, want %q", out.State, StateError)
	}
	if out.Note == "" {
		t.Error("expected the error to be surfaced in the note")
	}
	if out.State == StateCurrent || out.State == StateNoRelease {
		t.Error("a failed search must never look like a clean verdict")
	}
}
