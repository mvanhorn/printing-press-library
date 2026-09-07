// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cdcparse

import "testing"

// Markup below is verbatim from a real admin-ajax fragment.
const realFragment = `
<div class="posts-container"><div class="download_list">
  <div class="row">
    <div class="col-8 col-xs-8 col-md-10">
      <h4>Credit of Bonus &#8211; Jubilee Life Insurance Company Limited</h4>
      <div class="meta">03 September</div>
    </div>
    <a download href="https://www.cdcpakistan.com/assets/uploads/2026/09/Credit-of-Bonus.pdf" class="col-xs-2 icon_download">Download<span></span></a>
  </div>
</div>
<div class="download_list">
  <div class="row">
    <div class="col-8 col-xs-8 col-md-10">
      <h4>CDC Newsletter</h4>
      <div class="meta">17 December</div>
    </div>
    <a download href="http://www.cdcpakistan.com/assets/uploads/publications/CDC_Newsletter_Oct_Dec_2014.pdf" class="col-xs-2 icon_download">Download</a>
  </div>
</div></div>`

func TestParseDownloads(t *testing.T) {
	docs, err := ParseDownloads(realFragment, "circulars", 2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("docs = %d, want 2", len(docs))
	}
	d := docs[0]
	if d.Title != "Credit of Bonus – Jubilee Life Insurance Company Limited" {
		t.Errorf("HTML entity not decoded: %q", d.Title)
	}
	if d.MetaDayMonth != "03 September" {
		t.Errorf("meta = %q; CDC omits the year and it must be preserved as-is", d.MetaDayMonth)
	}
	if d.UploadYear != "2026" || d.UploadMonth != "09" {
		t.Errorf("upload path not parsed: %q/%q", d.UploadYear, d.UploadMonth)
	}
	if d.LegacyPath {
		t.Errorf("dated path wrongly marked legacy")
	}
	// The legacy /assets/uploads/publications/ shape carries NO date, which is
	// true of ~14% of the corpus, so it must be flagged not guessed.
	if !docs[1].LegacyPath {
		t.Errorf("undated legacy path not flagged")
	}
	if docs[1].UploadYear != "" {
		t.Errorf("legacy path must not invent an upload year, got %q", docs[1].UploadYear)
	}
}

// A challenge must never be reported as an empty bucket. This is the defect that
// made a whole 300-bucket sweep record zeros against a dead clearance cookie.
func TestChallengeIsNotAnEmptyBucket(t *testing.T) {
	bodies := []string{
		`<!DOCTYPE html><html><head><title>Just a moment...</title></head><body></body></html>`,
		`<html><script>window._cf_chl_opt={};</script></html>`,
		`<html><head><meta name="cf-mitigated" content="challenge"></head></html>`,
	}
	for _, b := range bodies {
		if !IsChallenge(b) {
			t.Errorf("IsChallenge missed a challenge body: %.60s", b)
		}
		if _, err := ParseDownloads(b, "notices", 2024); err != ErrChallenge {
			t.Errorf("ParseDownloads should return ErrChallenge, got %v", err)
		}
		if _, err := ParseStatistics(b); err != ErrChallenge {
			t.Errorf("ParseStatistics should return ErrChallenge, got %v", err)
		}
	}
	// A genuinely empty fragment is NOT a challenge.
	if IsChallenge(`<div class="posts-container"></div>`) {
		t.Errorf("empty fragment misreported as a challenge")
	}
}

// Regression: "Securities-Unlisted" contains the substring "listed" and used to
// collapse onto securities_listed, producing a duplicate key and silently losing
// the 59,217 unlisted count.
func TestNormaliseLabel_UnlistedDoesNotCollapseOntoListed(t *testing.T) {
	listed, ok1 := NormaliseLabel("Number of Securities-Listed (Equity & Debt)")
	unlisted, ok2 := NormaliseLabel("Number of Securities-Unlisted (Equity & Debt)")
	if !ok1 || !ok2 {
		t.Fatalf("both labels should be known: %v %v", ok1, ok2)
	}
	if listed == unlisted {
		t.Fatalf("listed and unlisted collapsed onto the same key %q", listed)
	}
	if listed != "securities_listed" || unlisted != "securities_unlisted" {
		t.Errorf("keys wrong: listed=%q unlisted=%q", listed, unlisted)
	}
}

// Regression: "Total Number of Securities under Share Registrar / Transfer Agent
// Services" contains "number of securities" and used to be keyed as a securities
// count instead of a registrar count.
func TestNormaliseLabel_ShareRegistrarBeatsGenericSecuritiesCount(t *testing.T) {
	got, ok := NormaliseLabel("Total Number of Securities under Share Registrar / Transfer Agent Services")
	if !ok {
		t.Fatal("label should be known")
	}
	if got != "securities_under_share_registrar" {
		t.Errorf("key = %q, want securities_under_share_registrar", got)
	}
}

// Drift must surface as data, not as a silent gap: an unrecognised row is kept
// with Known=false. CDC has already renamed rows and deleted one.
func TestParseStatistics_UnknownLabelKeptAndFlagged(t *testing.T) {
	body := `<table>
	  <tr><td>Information</td><td>Facts (As of July-2026)</td></tr>
	  <tr><td>Total Number of sub Accounts (Individual)</td><td>665,934</td></tr>
	  <tr><td>Some Entirely New Metric CDC Invented</td><td>42</td></tr>
	</table>`
	snap, err := ParseStatistics(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.AsOf != "July-2026" {
		t.Errorf("as_of = %q, want July-2026", snap.AsOf)
	}
	if len(snap.Metrics) != 2 {
		t.Fatalf("metrics = %d, want 2 (unknown rows are KEPT)", len(snap.Metrics))
	}
	if len(snap.UnknownLabels) != 1 {
		t.Errorf("unknown labels = %v, want exactly 1", snap.UnknownLabels)
	}
	var sawUnknown bool
	for _, m := range snap.Metrics {
		if !m.Known {
			sawUnknown = true
			if m.Raw != "42" {
				t.Errorf("unknown row lost its value: %q", m.Raw)
			}
		}
	}
	if !sawUnknown {
		t.Error("unknown row was dropped instead of kept with Known=false")
	}
}

func TestClassifyTitle_OrderingMatters(t *testing.T) {
	cases := []struct {
		title string
		kind  EventKind
		state EligibilityState
	}{
		// "extension of suspension" must beat bare "suspension"
		{"Notice of Extension of Suspension of CDS Eligibility of X", KindEligibility, StateExtension},
		// "removal of suspension" must beat bare "suspension"
		{"Notice of Removal of Suspension of CDS Eligibility of X", KindEligibility, StateRemovalOfSuspension},
		// "removal of intention" must beat bare "intention"
		{"Notice of Removal of Intention to Suspend CDS Eligibility of X", KindEligibility, StateRemovalOfIntention},
		{"Notice of Intention to Suspend CDS Eligibility of X", KindEligibility, StateIntentionToSuspend},
		{"Notice of Suspension of the CDS Eligibility of Ordinary Shares of Y", KindEligibility, StateSuspended},
		{"Notice of Declaration of CDS Eligibility of Ordinary Shares of Z", KindEligibility, StateDeclared},
		{"Notice of Revocation of CDS Eligibility of Privately Placed Sukuk", KindEligibility, StateRevoked},
		{"Notice of Termination of Admission of Infinite Securities Limited", KindEligibility, StateTerminated},
		{"Change of Security Name and Symbol – ICI Pakistan to Lucky Core", KindIdentity, ""},
		{"Credit of Bonus – Jubilee Life Insurance", KindCorpAction, ""},
		{"SUB-DIVISION OF SHARES – ZUMA", KindCorpAction, ""},
		{"Remote Education via NMS (Participants)", KindOperational, ""},
		{"Something CDC Has Never Published Before", KindUnclassified, ""},
	}
	for _, c := range cases {
		got := ClassifyTitle(c.title)
		if got.Kind != c.kind {
			t.Errorf("%q kind = %q, want %q", c.title, got.Kind, c.kind)
		}
		if got.State != c.state {
			t.Errorf("%q state = %q, want %q", c.title, got.State, c.state)
		}
	}
}

func TestReportPartAndPenetrationDetection(t *testing.T) {
	a := "https://www.cdcpakistan.com/assets/uploads/2025/12/Share-Percentage-in-CDS-with-respect-to-paid-up-Capital-as-of-30-November-2025-A.pdf"
	b := "https://www.cdcpakistan.com/assets/uploads/2025/12/Share-Percentage-in-CDS-with-respect-to-paid-up-Capital-as-of-30-November-2025-B.pdf"
	if ReportPart(a) != "A" || ReportPart(b) != "B" {
		t.Errorf("part detection failed: %q %q", ReportPart(a), ReportPart(b))
	}
	if ReportPart("https://x/Security-List-Report-30-April-2026-1.pdf") != "" {
		t.Error("non-part file should return empty part")
	}
	if !IsPenetrationReport("Share Percentage in CDS with respect to paid up Capital as of 30 November 2025") {
		t.Error("penetration report not detected")
	}
	if IsPenetrationReport("Credit of Bonus – Jubilee Life") {
		t.Error("false positive on a corp-action title")
	}
}

func TestParseYearParam(t *testing.T) {
	if _, err := ParseYearParam("2024"); err != nil {
		t.Errorf("2024 should be valid: %v", err)
	}
	for _, bad := range []string{"", "abc", "1200", "3000"} {
		if _, err := ParseYearParam(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
