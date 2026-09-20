// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// Patch record to be written at .printing-press-patches/nepra-verify-fetch-gate.json;
// it is not in this change because the writable set for this task is source only.

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// verifyStaleness is the answer to "is this manifest still describing reality".
//
// It carries TWO ages and gates only one of them, and that asymmetry is the
// whole design. The manifest's own age is a bounded number a maintainer
// controls, so it can be a gate. Days since the newest fiscal year's coverage
// ended only ever grows, so gating it would turn the exit code into a
// constant that says "yes, time has passed" forever — a signal that fires
// always is not a signal.
type verifyStaleness struct {
	Checked bool `json:"checked"`
	// Gated says whether these measurements can change the exit code.
	Gated              bool   `json:"gated"`
	WindowDays         int    `json:"window_days"`
	ManifestMeasuredAt string `json:"manifest_measured_at"`
	ManifestAgeDays    int    `json:"manifest_age_days"`
	ManifestVerdict    string `json:"manifest_verdict"`

	NewestPublishedFY   string `json:"newest_published_fy"`
	NewestFYCoverageEnd string `json:"newest_fy_coverage_end"`
	// DaysSinceCoverageEnd is REPORTED AND NEVER GATED. See CoverageNote.
	DaysSinceCoverageEnd int    `json:"days_since_coverage_end"`
	CoverageGated        bool   `json:"coverage_gated"`
	CoverageNote         string `json:"coverage_note"`

	// NextFYProbe is null unless --check-stale was passed: it costs a
	// request, and a gate that made unrequested requests to a regulator's
	// site would be the wrong kind of thorough.
	NextFYProbe *verifyNextFYProbe `json:"next_fy_probe"`
}

// verifyNextFYProbe asks whether a new fiscal year has appeared.
//
// This is the ONLY staleness measurement that can fail the gate, and it can
// only fail one way: a 200 whose in-document band label parses to the NEXT
// fiscal year means NEPRA published a year this manifest does not know about.
// A 404 is the expected answer. A 200 that is one of the three catalogued
// decoys passes WITH A NOTE, because a decoy is not a new year — resolving
// this from the filename is exactly the mistake that would double-count
// FY2023-24 as FY2024-25.
type verifyNextFYProbe struct {
	FY            string `json:"fy"`
	URL           string `json:"url"`
	Expected      string `json:"expected"`
	Observed      string `json:"observed"`
	ObservedBytes int    `json:"observed_bytes"`
	// BandLabel is the in-document year, or "" when the body has no table.
	BandLabel string `json:"band_label"`
	Verdict   string `json:"verdict"`
	Note      string `json:"note"`
}

const verifyCoverageNote = "REPORTED, NEVER GATED. This number only grows, so gating it would make " +
	"the exit code a constant rather than a signal. The source is frozen: the Detail-of-Generation " +
	"series ends at FY2023-24, and on 19 Jan 2026 the Federal Minister for Power called the State " +
	"of Industry Report five months late and \"based on incomplete and inaccurate data\". Use " +
	"next_fy_probe, which asks the site rather than the calendar."

// verifyDaysBetween returns whole days from an ISO date to now, or 0 and false
// when the date cannot be read. An unparseable date is never silently turned
// into "0 days old", which would read as perfectly fresh.
func verifyDaysBetween(iso string, now time.Time) (int, bool) {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return 0, false
	}
	return int(now.UTC().Sub(t.UTC()).Hours() / 24), true
}

// verifyBuildStaleness computes the staleness block. asOf and now are
// parameters rather than constants so the gate's own boundary behaviour is
// testable without waiting 90 days.
func verifyBuildStaleness(asOf string, now time.Time, windowDays int, gated bool) verifyStaleness {
	s := verifyStaleness{
		Checked:             true,
		Gated:               gated,
		WindowDays:          windowDays,
		ManifestMeasuredAt:  asOf,
		ManifestVerdict:     verifyPass,
		NewestPublishedFY:   verifyNewestPublishedFY,
		NewestFYCoverageEnd: verifyNewestFYCoverageEnd,
		CoverageGated:       false,
		CoverageNote:        verifyCoverageNote,
	}
	age, ok := verifyDaysBetween(asOf, now)
	if !ok {
		s.ManifestAgeDays = -1
		s.ManifestVerdict = verifyFail
		return s
	}
	s.ManifestAgeDays = age
	if cov, ok := verifyDaysBetween(verifyNewestFYCoverageEnd, now); ok {
		s.DaysSinceCoverageEnd = cov
	}
	if age > windowDays {
		// Reported as a failure whether or not it is gated. The verdict is
		// the measurement; --check-stale decides whether the measurement
		// reaches the exit code.
		s.ManifestVerdict = verifyFail
	}
	return s
}

// verifyNextFY returns the fiscal year after the newest published one.
func verifyNextFY() (nepraparse.FiscalYear, error) {
	fy, err := nepraparse.ParseFiscalYear(verifyNewestPublishedFY)
	if err != nil {
		return nepraparse.FiscalYear{}, err
	}
	return nepraparse.FiscalYear{Start: fy.Start + 1, End: fy.End + 1}, nil
}

// verifyProbeNextFY fetches the next fiscal year's sheet and classifies it.
func verifyProbeNextFY(ctx context.Context, c *client.Client) *verifyNextFYProbe {
	next, err := verifyNextFY()
	if err != nil {
		return &verifyNextFYProbe{Verdict: verifyFail, Note: err.Error()}
	}
	path, err := verifyGenerationPath(next)
	if err != nil {
		return &verifyNextFYProbe{FY: next.Label(), Verdict: verifyFail, Note: err.Error()}
	}
	p := &verifyNextFYProbe{
		FY:       next.Label(),
		URL:      c.RequestBaseURL() + path,
		Expected: "HTTP 404 with a 9-byte body",
	}
	f := verifyFetch(ctx, c, path)
	p.ObservedBytes = len(f.Body)
	return verifyClassifyNextFY(p, next, f)
}

// verifyClassifyNextFY is the classification, split out so every branch is
// drivable from a test without a network.
func verifyClassifyNextFY(p *verifyNextFYProbe, next nepraparse.FiscalYear, f verifyFetchResult) *verifyNextFYProbe {
	if f.Err != nil {
		if f.StatusExact != nil && *f.StatusExact == 404 {
			p.Observed = "404"
			p.Verdict = verifyPass
			p.Note = "The expected answer: this year is not published, so the manifest's newest " +
				"reachable year is still current."
			return p
		}
		p.Observed = "fetch error"
		p.Verdict = verifyReported
		p.Note = "The probe could not be answered, which is not evidence either way about a new " +
			"year: " + f.Err.Error()
		return p
	}

	// HTTP 200. Three of the four possibilities are decoys.
	if len(f.Body) <= verify404StubBytes {
		p.Observed = "404-stub body under a 200"
		p.Verdict = verifyPass
		p.Note = "A 9-byte body is never data. Not a new year."
		return p
	}
	if frameset, _ := verifyFramesetMarkers(f.Body); frameset > 0 {
		p.Observed = "frameset shell"
		p.Verdict = verifyPass
		p.Note = "A <frameset> shell with no numbers in it. This body is byte-identical across " +
			"three fiscal years, so it is a wrapper and not a year. Not a new year."
		return p
	}
	w, perr := nepraparse.ParseWorkbook(f.Body, "")
	if perr != nil {
		p.Observed = "200 with no parseable workbook"
		p.Verdict = verifyReported
		p.Note = "Something answered, and it is not a workbook this parser recognises, so this " +
			"probe declines to call it either a new year or a decoy: " + perr.Error()
		return p
	}
	p.BandLabel = w.Header.BandLabel
	if w.FiscalYear == next {
		p.Observed = "a real workbook whose band label reads " + w.Header.BandLabel
		p.Verdict = verifyFail
		p.Note = fmt.Sprintf("NEPRA HAS PUBLISHED FY%s. This manifest's newest reachable year is "+
			"FY%s, so every floor and every internal in it is now incomplete. Re-measure the "+
			"manifest.", next.Label(), verifyNewestPublishedFY)
		return p
	}
	p.Observed = "a real workbook for a DIFFERENT year: " + w.Header.BandLabel
	p.Verdict = verifyPass
	p.Note = fmt.Sprintf("The path for FY%s served a workbook whose own band label says FY%s. "+
		"That is the filename-substitution decoy, not a new year — enumerating by filename here "+
		"would fabricate a year of data behind a real HTTP 200.", next.Label(), w.FiscalYear.Label())
	return p
}
