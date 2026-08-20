// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Behavioral tests for the seven hand-authored Webflow audit commands. The
// scaffold tests beside each command cover wiring; these cover the logic that
// makes each command worth shipping, including the absence-of-correctness
// cases (empty in, empty out — never fabricated findings).

package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func rowOf(t *testing.T, id, scope string, v any) rawRow {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return rawRow{ID: id, Scope: scope, Data: b}
}

func boolp(b bool) *bool { return &b }

func page(id, slug, title, seoTitle, seoDesc string) wfPage {
	p := wfPage{ID: id, Slug: slug, Title: title, LastUpdated: "2026-07-01T00:00:00Z"}
	if seoTitle != "" || seoDesc != "" {
		p.SEO = &wfPageSEO{Title: seoTitle, Description: seoDesc}
	}
	return p
}

var stdThresholds = seoThresholds{titleMax: 60, descMax: 160, descMin: 50}

func findingIssues(v seoAuditView) map[string]int {
	out := map[string]int{}
	for _, f := range v.Findings {
		out[f.Issue]++
	}
	return out
}

func TestAuditPagesFlagsMissingMetadata(t *testing.T) {
	goodDesc := "A description that is comfortably long enough to clear the thin-description floor."
	tests := []struct {
		name      string
		pages     []wfPage
		wantIssue string
		wantCount int
	}{
		{
			name:      "missing title",
			pages:     []wfPage{page("p1", "/about", "About", "", goodDesc)},
			wantIssue: "missing-seo-title",
			wantCount: 1,
		},
		{
			name:      "missing description",
			pages:     []wfPage{page("p1", "/about", "About", "About Us", "")},
			wantIssue: "missing-seo-description",
			wantCount: 1,
		},
		{
			name: "over-length title",
			pages: []wfPage{page("p1", "/about", "About",
				"This SEO title is deliberately far longer than sixty characters so it trips the limit", goodDesc)},
			wantIssue: "long-seo-title",
			wantCount: 1,
		},
		{
			name:      "thin description",
			pages:     []wfPage{page("p1", "/about", "About", "About Us", "Too short.")},
			wantIssue: "thin-seo-description",
			wantCount: 1,
		},
		{
			name: "fully populated page produces no findings",
			pages: []wfPage{func() wfPage {
				p := page("p1", "/about", "About", "About Us", goodDesc)
				p.OpenGraph = &wfPageOG{Title: "About Us", Description: goodDesc}
				return p
			}()},
			wantIssue: "",
			wantCount: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditPages(tc.pages, "site1", stdThresholds)
			if tc.wantIssue == "" {
				if len(got.Findings) != 0 {
					t.Fatalf("expected no findings, got %d: %+v", len(got.Findings), got.Findings)
				}
				return
			}
			if n := findingIssues(got)[tc.wantIssue]; n != tc.wantCount {
				t.Fatalf("issue %q count = %d, want %d (all findings: %+v)",
					tc.wantIssue, n, tc.wantCount, got.Findings)
			}
		})
	}
}

func TestAuditPagesDetectsDuplicatesAcrossPages(t *testing.T) {
	desc := "A description that is comfortably long enough to clear the thin-description floor."
	pages := []wfPage{
		page("p1", "/a", "A", "Shared Title", desc),
		page("p2", "/b", "B", "Shared Title", desc),
		page("p3", "/c", "C", "Unique Title", "Another description that also clears the thin floor comfortably."),
	}
	got := auditPages(pages, "site1", stdThresholds)
	issues := findingIssues(got)
	// Both pages sharing the title are reported, so the consultant can fix either.
	if issues["duplicate-seo-title"] != 2 {
		t.Fatalf("duplicate-seo-title = %d, want 2 (findings: %+v)", issues["duplicate-seo-title"], got.Findings)
	}
	if issues["duplicate-seo-description"] != 2 {
		t.Fatalf("duplicate-seo-description = %d, want 2", issues["duplicate-seo-description"])
	}
	if got.PagesAudited != 3 {
		t.Fatalf("PagesAudited = %d, want 3", got.PagesAudited)
	}
}

func TestAuditPagesSkipsDraftsUnlessAsked(t *testing.T) {
	draft := page("p1", "/draft", "Draft", "", "")
	draft.Draft = boolp(true)
	pages := []wfPage{draft}

	if got := auditPages(pages, "s", stdThresholds); got.PagesAudited != 0 || len(got.Findings) != 0 {
		t.Fatalf("draft page audited by default: audited=%d findings=%d", got.PagesAudited, len(got.Findings))
	}
	th := stdThresholds
	th.includeDrafts = true
	if got := auditPages(pages, "s", th); got.PagesAudited != 1 || len(got.Findings) == 0 {
		t.Fatalf("--include-drafts did not audit the draft: audited=%d findings=%d", got.PagesAudited, len(got.Findings))
	}
}

// Absence-of-correctness: no pages must yield no findings, never invented ones.
func TestAuditPagesEmptyInputProducesNoFindings(t *testing.T) {
	got := auditPages(nil, "site1", stdThresholds)
	if len(got.Findings) != 0 || got.PagesAudited != 0 {
		t.Fatalf("empty input produced findings: %+v", got)
	}
	if got.Findings == nil {
		t.Fatal("Findings must marshal as [] not null")
	}
}

func TestHasUnpublishedChanges(t *testing.T) {
	tests := []struct {
		name string
		item wfItem
		want bool
	}{
		{"never published", wfItem{LastUpdated: "2026-07-01T00:00:00Z"}, true},
		{"edited after publish", wfItem{LastUpdated: "2026-07-02T00:00:00Z", LastPublished: "2026-07-01T00:00:00Z"}, true},
		{"published after edit", wfItem{LastUpdated: "2026-07-01T00:00:00Z", LastPublished: "2026-07-02T00:00:00Z"}, false},
		{"same timestamp", wfItem{LastUpdated: "2026-07-01T00:00:00Z", LastPublished: "2026-07-01T00:00:00Z"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasUnpublishedChanges(tc.item); got != tc.want {
				t.Fatalf("hasUnpublishedChanges = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestComputeDriftSeparatesNeverPublishedFromEdited(t *testing.T) {
	rows := []rawRow{
		rowOf(t, "i1", "c1", wfItem{ID: "i1", LastUpdated: "2026-07-01T00:00:00Z",
			FieldData: map[string]any{"name": "Never published"}}),
		rowOf(t, "i2", "c1", wfItem{ID: "i2", LastUpdated: "2026-07-02T00:00:00Z", LastPublished: "2026-07-01T00:00:00Z",
			FieldData: map[string]any{"name": "Edited since"}}),
		rowOf(t, "i3", "c1", wfItem{ID: "i3", LastUpdated: "2026-07-01T00:00:00Z", LastPublished: "2026-07-02T00:00:00Z",
			FieldData: map[string]any{"name": "Clean"}}),
	}
	got := computeDrift(rows, "c1", 100, false)
	if got.ItemsScanned != 3 {
		t.Fatalf("ItemsScanned = %d, want 3", got.ItemsScanned)
	}
	if got.ItemsDrifted != 2 {
		t.Fatalf("ItemsDrifted = %d, want 2", got.ItemsDrifted)
	}
	if got.NeverPublished != 1 || got.EditedSince != 1 {
		t.Fatalf("NeverPublished=%d EditedSince=%d, want 1/1", got.NeverPublished, got.EditedSince)
	}
	// Negative check: the clean item must not appear in the drift list.
	for _, it := range got.Items {
		if it.Name == "Clean" {
			t.Fatalf("a fully published item was reported as drifted: %+v", it)
		}
	}
}

// Absence-of-correctness: a collection with nothing pending returns an empty
// list rather than fabricating drift.
func TestComputeDriftAllPublishedReturnsEmpty(t *testing.T) {
	rows := []rawRow{
		rowOf(t, "i1", "c1", wfItem{ID: "i1", LastUpdated: "2026-07-01T00:00:00Z", LastPublished: "2026-07-02T00:00:00Z"}),
	}
	got := computeDrift(rows, "c1", 100, false)
	if got.ItemsDrifted != 0 || len(got.Items) != 0 {
		t.Fatalf("expected no drift, got %+v", got)
	}
	if got.Items == nil {
		t.Fatal("Items must marshal as [] not null")
	}
}

func TestBuildPublishPreviewCountsChangedPagesAndItems(t *testing.T) {
	site := wfSite{ID: "s1", DisplayName: "Client Site", LastPublished: "2026-07-01T00:00:00Z"}
	pages := []rawRow{
		rowOf(t, "p1", "s1", wfPage{ID: "p1", Slug: "/changed", LastUpdated: "2026-07-05T00:00:00Z"}),
		rowOf(t, "p2", "s1", wfPage{ID: "p2", Slug: "/stable", LastUpdated: "2026-06-01T00:00:00Z"}),
		rowOf(t, "p3", "s1", wfPage{ID: "p3", Slug: "/draft", LastUpdated: "2026-07-05T00:00:00Z", Draft: boolp(true)}),
	}
	colls := []rawRow{rowOf(t, "c1", "s1", wfCollection{ID: "c1", DisplayName: "Blog"})}
	items := []rawRow{
		rowOf(t, "i1", "c1", wfItem{ID: "i1", LastUpdated: "2026-07-04T00:00:00Z"}),
		rowOf(t, "i2", "c1", wfItem{ID: "i2", LastUpdated: "2026-06-01T00:00:00Z", LastPublished: "2026-06-02T00:00:00Z"}),
	}

	got := buildPublishPreview(site, pages, colls, items, 3, 50)
	if len(got.PagesChanged) != 1 || got.PagesChanged[0].Slug != "/changed" {
		t.Fatalf("PagesChanged = %+v, want only /changed", got.PagesChanged)
	}
	if len(got.DraftPages) != 1 {
		t.Fatalf("DraftPages = %d, want 1", len(got.DraftPages))
	}
	if got.TotalUnpublished != 1 {
		t.Fatalf("TotalUnpublished = %d, want 1", got.TotalUnpublished)
	}
	if got.RedirectCount != 3 {
		t.Fatalf("RedirectCount = %d, want 3", got.RedirectCount)
	}
	if !got.WouldChangeAnythg {
		t.Fatal("WouldChangeAnythg = false, want true")
	}
}

// Absence-of-correctness: a fully published site reports nothing pending.
func TestBuildPublishPreviewCleanSiteReportsNothingPending(t *testing.T) {
	site := wfSite{ID: "s1", DisplayName: "Clean", LastPublished: "2026-07-10T00:00:00Z"}
	pages := []rawRow{rowOf(t, "p1", "s1", wfPage{ID: "p1", Slug: "/x", LastUpdated: "2026-07-01T00:00:00Z"})}
	got := buildPublishPreview(site, pages, nil, nil, 0, 50)
	if got.WouldChangeAnythg {
		t.Fatalf("clean site reported pending changes: %+v", got)
	}
	if len(got.PagesChanged) != 0 || got.TotalUnpublished != 0 {
		t.Fatalf("clean site produced changes: %+v", got)
	}
}

func TestAuditRedirectsFindsShadowLoopAndDuplicate(t *testing.T) {
	pages := []rawRow{
		rowOf(t, "p1", "s1", wfPage{ID: "p1", Slug: "/live-page", PublishedPath: "/live-page"}),
		rowOf(t, "p2", "s1", wfPage{ID: "p2", Slug: "/target", PublishedPath: "/target"}),
	}
	redirects := []rawRow{
		rowOf(t, "r1", "s1", wfRedirect{ID: "r1", FromURL: "/live-page", ToURL: "/target"}),
		rowOf(t, "r2", "s1", wfRedirect{ID: "r2", FromURL: "/old", ToURL: "/nowhere"}),
		rowOf(t, "r3", "s1", wfRedirect{ID: "r3", FromURL: "/old", ToURL: "/target"}),
		rowOf(t, "r4", "s1", wfRedirect{ID: "r4", FromURL: "/loop-a", ToURL: "/loop-b"}),
		rowOf(t, "r5", "s1", wfRedirect{ID: "r5", FromURL: "/loop-b", ToURL: "/loop-a"}),
	}
	got := auditRedirects(redirects, pages, nil, "s1")
	issues := map[string]int{}
	for _, f := range got.Findings {
		issues[f.Issue]++
	}
	if issues["shadows-live-page"] != 1 {
		t.Fatalf("shadows-live-page = %d, want 1 (findings: %+v)", issues["shadows-live-page"], got.Findings)
	}
	// /nowhere, /loop-a, and /loop-b all point at paths with no page behind
	// them, so three rules are legitimately flagged.
	if issues["unknown-target"] != 3 {
		t.Fatalf("unknown-target = %d, want 3", issues["unknown-target"])
	}
	var sawNowhere bool
	for _, f := range got.Findings {
		if f.Issue == "unknown-target" && f.FromURL == "/old" && f.ToURL == "/nowhere" {
			sawNowhere = true
		}
	}
	if !sawNowhere {
		t.Fatalf("the /old -> /nowhere rule was not flagged (findings: %+v)", got.Findings)
	}
	if issues["duplicate-source"] != 1 {
		t.Fatalf("duplicate-source = %d, want 1", issues["duplicate-source"])
	}
	if issues["redirect-loop"] == 0 {
		t.Fatalf("redirect-loop not detected (findings: %+v)", got.Findings)
	}
	if got.RulesAudited != 5 {
		t.Fatalf("RulesAudited = %d, want 5", got.RulesAudited)
	}
}

// Without synced pages the audit must NOT claim every target is unknown.
func TestAuditRedirectsSkipsTargetCheckWithoutPages(t *testing.T) {
	redirects := []rawRow{rowOf(t, "r1", "s1", wfRedirect{ID: "r1", FromURL: "/a", ToURL: "/b"})}
	got := auditRedirects(redirects, nil, nil, "s1")
	if got.TargetsChecked {
		t.Fatal("TargetsChecked = true with no synced pages")
	}
	for _, f := range got.Findings {
		if f.Issue == "unknown-target" {
			t.Fatalf("flagged an unknown target with no page universe: %+v", f)
		}
	}
	if got.Note == "" {
		t.Fatal("expected a note explaining that target checking was skipped")
	}
}

func TestAuditRedirectsCleanTableProducesNoFindings(t *testing.T) {
	pages := []rawRow{rowOf(t, "p1", "s1", wfPage{ID: "p1", Slug: "/target", PublishedPath: "/target"})}
	redirects := []rawRow{rowOf(t, "r1", "s1", wfRedirect{ID: "r1", FromURL: "/old", ToURL: "/target"})}
	got := auditRedirects(redirects, pages, nil, "s1")
	if len(got.Findings) != 0 {
		t.Fatalf("clean redirect table produced findings: %+v", got.Findings)
	}
}

func TestBuildCompletenessComputesFillRates(t *testing.T) {
	schema := []rawRow{rowOf(t, "c1", "c1", wfCollection{
		ID: "c1", DisplayName: "Blog",
		Fields: []wfField{
			{Slug: "name", DisplayName: "Name", Type: "PlainText", IsRequired: boolp(true)},
			{Slug: "summary", DisplayName: "Summary", Type: "PlainText"},
			{Slug: "unused", DisplayName: "Unused", Type: "PlainText"},
		},
	})}
	items := []rawRow{
		rowOf(t, "i1", "c1", wfItem{ID: "i1", FieldData: map[string]any{"name": "One", "summary": "S"}}),
		rowOf(t, "i2", "c1", wfItem{ID: "i2", FieldData: map[string]any{"name": "Two"}}),
		rowOf(t, "i3", "c1", wfItem{ID: "i3", FieldData: map[string]any{"name": ""}}),
	}
	got := buildCompleteness("c1", schema, items, false)
	if got.ItemsScanned != 3 {
		t.Fatalf("ItemsScanned = %d, want 3", got.ItemsScanned)
	}
	byslug := map[string]fieldCoverage{}
	for _, f := range got.Fields {
		byslug[f.Slug] = f
	}
	if byslug["name"].Filled != 2 || byslug["name"].Empty != 1 {
		t.Fatalf("name coverage = %+v, want 2 filled / 1 empty", byslug["name"])
	}
	if byslug["name"].Issue == "" {
		t.Fatal("required field with an empty value was not flagged")
	}
	if got.RequiredGaps != 1 {
		t.Fatalf("RequiredGaps = %d, want 1", got.RequiredGaps)
	}
	if byslug["unused"].Filled != 0 || got.DeadFields != 1 {
		t.Fatalf("dead field not detected: %+v (DeadFields=%d)", byslug["unused"], got.DeadFields)
	}
	if r := byslug["summary"].FillRate; r < 0.32 || r > 0.34 {
		t.Fatalf("summary FillRate = %v, want ~0.333", r)
	}
}

func TestBuildCompletenessWithoutSchemaInfersFieldsAndSaysSo(t *testing.T) {
	items := []rawRow{rowOf(t, "i1", "c1", wfItem{ID: "i1", FieldData: map[string]any{"name": "One"}})}
	got := buildCompleteness("c1", nil, items, false)
	if len(got.Fields) != 1 || got.Fields[0].Slug != "name" {
		t.Fatalf("expected inferred field 'name', got %+v", got.Fields)
	}
	if got.RequiredGaps != 0 {
		t.Fatalf("RequiredGaps = %d without schema, want 0", got.RequiredGaps)
	}
	if got.Note == "" {
		t.Fatal("expected a note that the schema was not synced")
	}
}

func TestIsFilledValue(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want bool
	}{
		{"nil", nil, false},
		{"empty string", "", false},
		{"whitespace", "   ", false},
		{"text", "hello", true},
		{"empty slice", []any{}, false},
		{"slice", []any{1}, true},
		{"empty map", map[string]any{}, false},
		{"map", map[string]any{"a": 1}, true},
		{"false bool is a real answer", false, true},
		{"number", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFilledValue(tc.in); got != tc.want {
				t.Fatalf("isFilledValue(%#v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSelectBulkTargetsMatchesAndCaps(t *testing.T) {
	rows := []rawRow{
		rowOf(t, "i1", "c1", wfItem{ID: "i1", FieldData: map[string]any{"name": "A", "category": "news"}}),
		rowOf(t, "i2", "c1", wfItem{ID: "i2", FieldData: map[string]any{"name": "B", "category": "news"}}),
		rowOf(t, "i3", "c1", wfItem{ID: "i3", FieldData: map[string]any{"name": "C", "category": "guides"}}),
		rowOf(t, "i4", "c1", wfItem{ID: "i4", IsArchived: boolp(true), FieldData: map[string]any{"name": "D", "category": "news"}}),
	}
	set := map[string]string{"category": "updates"}

	got := selectBulkTargets(rows, "c1", map[string]string{"category": "news"}, set, 100, nil)
	if got.ItemsScanned != 4 {
		t.Fatalf("ItemsScanned = %d, want 4", got.ItemsScanned)
	}
	if got.Matched != 2 {
		t.Fatalf("Matched = %d, want 2 (archived item must be excluded)", got.Matched)
	}
	// Negative check: the non-matching item must not be selected.
	for _, ch := range got.Changes {
		if ch.Name == "C" || ch.Name == "D" {
			t.Fatalf("selected an item that should not match: %+v", ch)
		}
	}

	capped := selectBulkTargets(rows, "c1", map[string]string{"category": "news"}, set, 1, nil)
	if len(capped.Changes) != 1 {
		t.Fatalf("--limit 1 produced %d changes", len(capped.Changes))
	}
	if capped.Note == "" {
		t.Fatal("expected a note when the limit truncated the change set")
	}
}

func TestSelectBulkTargetsNoMatchExplainsItself(t *testing.T) {
	rows := []rawRow{rowOf(t, "i1", "c1", wfItem{ID: "i1", FieldData: map[string]any{"category": "news"}})}
	got := selectBulkTargets(rows, "c1", map[string]string{"category": "nope"}, map[string]string{"x": "y"}, 10, nil)
	if got.Matched != 0 || len(got.Changes) != 0 {
		t.Fatalf("expected no matches, got %+v", got)
	}
	if got.Note == "" {
		t.Fatal("a zero-match run must explain why rather than printing an empty list")
	}
}

func TestParseKeyValuePairs(t *testing.T) {
	got, err := parseKeyValuePairs("set", []string{"a=1", "b=two=three", "c="})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["a"] != "1" || got["b"] != "two=three" || got["c"] != "" {
		t.Fatalf("parsed = %+v", got)
	}
	if _, err := parseKeyValuePairs("set", []string{"novalue"}); err == nil {
		t.Fatal("expected an error for a pair with no '='")
	}
	if _, err := parseKeyValuePairs("match", []string{"=1"}); err == nil {
		t.Fatal("expected an error for an empty key")
	}
}

func TestBuildOverviewAggregatesPerSite(t *testing.T) {
	sites := []rawRow{
		rowOf(t, "s1", "s1", wfSite{ID: "s1", DisplayName: "Alpha", LastPublished: "2026-07-01T00:00:00Z"}),
		rowOf(t, "s2", "s2", wfSite{ID: "s2", DisplayName: "Beta", LastPublished: "2026-07-01T00:00:00Z"}),
	}
	pages := []rawRow{
		rowOf(t, "p1", "s1", wfPage{ID: "p1", Slug: "/a"}),
		rowOf(t, "p2", "s1", wfPage{ID: "p2", Slug: "/b"}),
		rowOf(t, "p3", "s2", wfPage{ID: "p3", Slug: "/c"}),
	}
	colls := []rawRow{rowOf(t, "c1", "s1", wfCollection{ID: "c1", DisplayName: "Blog"})}
	items := []rawRow{
		rowOf(t, "i1", "c1", wfItem{ID: "i1", LastUpdated: "2026-07-02T00:00:00Z"}),
	}
	got := buildOverview(sites, pages, colls, items, nil)
	if got.SitesTotal != 2 {
		t.Fatalf("SitesTotal = %d, want 2", got.SitesTotal)
	}
	bySite := map[string]overviewSite{}
	for _, s := range got.Sites {
		bySite[s.SiteID] = s
	}
	if bySite["s1"].Pages != 2 || bySite["s2"].Pages != 1 {
		t.Fatalf("page counts wrong: %+v", got.Sites)
	}
	if bySite["s1"].Collections == nil || *bySite["s1"].Collections != 1 {
		t.Fatalf("s1 collections wrong: %+v", bySite["s1"].Collections)
	}
	if bySite["s2"].Collections == nil || *bySite["s2"].Collections != 0 {
		t.Fatalf("s2 collections wrong: %+v", bySite["s2"].Collections)
	}
	if bySite["s1"].UnpublishedItems == nil || *bySite["s1"].UnpublishedItems != 1 {
		t.Fatalf("s1 UnpublishedItems = %v, want 1", bySite["s1"].UnpublishedItems)
	}
	// The item belongs to s1's collection, so s2 must not inherit it.
	if bySite["s2"].UnpublishedItems == nil || *bySite["s2"].UnpublishedItems != 0 {
		t.Fatalf("s2 wrongly counted another site's items: %v", bySite["s2"].UnpublishedItems)
	}
	// Every page here is missing SEO metadata, so findings must be non-zero.
	if got.FindingsTotal == 0 {
		t.Fatal("FindingsTotal = 0 though every fixture page lacks SEO metadata")
	}
}

func TestBuildOverviewEmptyMirror(t *testing.T) {
	got := buildOverview(nil, nil, nil, nil, nil)
	if got.SitesTotal != 0 || len(got.Sites) != 0 {
		t.Fatalf("expected an empty rollup, got %+v", got)
	}
	if got.Sites == nil {
		t.Fatal("Sites must marshal as [] not null")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"/About/", "/about"},
		{"about", "/about"},
		{"/a?b=c", "/a"},
		{"/a#frag", "/a"},
		{"/", "/"},
		{"  /Mixed/Case/  ", "/mixed/case"},
	}
	for _, tc := range tests {
		if got := normalizePath(tc.in); got != tc.want {
			t.Fatalf("normalizePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseWFTime(t *testing.T) {
	if _, ok := parseWFTime(""); ok {
		t.Fatal("empty string parsed as a time")
	}
	if _, ok := parseWFTime("null"); ok {
		t.Fatal("literal null parsed as a time")
	}
	if _, ok := parseWFTime("not a date"); ok {
		t.Fatal("garbage parsed as a time")
	}
	got, ok := parseWFTime("2026-07-01T12:00:00Z")
	if !ok || !got.Equal(time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("parseWFTime RFC3339 = %v, %v", got, ok)
	}
}

func TestItemMatches(t *testing.T) {
	it := wfItem{FieldData: map[string]any{"category": "news", "featured": true, "count": float64(3)}}
	tests := []struct {
		name  string
		match map[string]string
		want  bool
	}{
		{"empty match selects everything", map[string]string{}, true},
		{"exact string", map[string]string{"category": "news"}, true},
		{"wrong value", map[string]string{"category": "guides"}, false},
		{"missing key", map[string]string{"nope": "x"}, false},
		{"bool stringified", map[string]string{"featured": "true"}, true},
		{"all pairs must match", map[string]string{"category": "news", "featured": "false"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := itemMatches(it, tc.match); got != tc.want {
				t.Fatalf("itemMatches = %v, want %v", got, tc.want)
			}
		})
	}
}

// Columns the mirror cannot fill must be null, not 0 — otherwise `overview`
// silently contradicts `publish preview` on the same mirror.
func TestBuildOverviewReportsUnavailableColumnsAsNull(t *testing.T) {
	sites := []rawRow{rowOf(t, "s1", "s1", wfSite{ID: "s1", DisplayName: "Alpha"})}
	pages := []rawRow{rowOf(t, "p1", "s1", wfPage{ID: "p1", Slug: "/a"})}
	got := buildOverview(sites, pages, nil, nil, nil)
	if len(got.Sites) != 1 {
		t.Fatalf("want 1 site, got %d", len(got.Sites))
	}
	s := got.Sites[0]
	if s.Collections != nil || s.UnpublishedItems != nil || s.Redirects != nil {
		t.Fatalf("unavailable columns must be nil, got collections=%v unpublished=%v redirects=%v",
			s.Collections, s.UnpublishedItems, s.Redirects)
	}
	if got.UnpublishedTotal != nil {
		t.Fatalf("UnpublishedTotal must be nil when no site could compute it, got %v", *got.UnpublishedTotal)
	}
	if got.Note == "" {
		t.Fatal("expected a note explaining why those columns are null")
	}
	if s.Pages != 1 {
		t.Fatalf("Pages should still be counted from the mirror, got %d", s.Pages)
	}
}

// Length checks must count characters, not bytes.
func TestAuditPagesCountsRunesNotBytes(t *testing.T) {
	// 30 CJK runes = 90 UTF-8 bytes. Under a 60-character limit it is fine.
	title := strings.Repeat("\u65e5", 30)
	desc := strings.Repeat("\u3042", 80) // 80 runes, under the 160 limit
	p := page("p1", "/jp", "JP", title, desc)
	p.OpenGraph = &wfPageOG{Title: title, Description: desc}
	got := auditPages([]wfPage{p}, "s1", stdThresholds)
	for _, f := range got.Findings {
		if f.Issue == "long-seo-title" || f.Issue == "long-seo-description" {
			t.Fatalf("byte-counted a multibyte value as over-length: %+v", f)
		}
	}
}

// Webflow slugs are unique per folder, so same-named pages in different folders
// are not duplicates.
func TestAuditPagesDuplicateSlugRespectsFolders(t *testing.T) {
	desc := "A description that is comfortably long enough to clear the thin-description floor."
	a := page("p1", "index", "Blog index", "Blog", desc)
	a.PublishedPath = "/blog/index"
	b := page("p2", "index", "Docs index", "Docs", "Another description that also clears the thin floor comfortably.")
	b.PublishedPath = "/docs/index"
	got := auditPages([]wfPage{a, b}, "s1", stdThresholds)
	for _, f := range got.Findings {
		if f.Issue == "duplicate-slug" {
			t.Fatalf("same slug in different folders flagged as duplicate: %+v", f)
		}
	}
}

// When pages really do collide, the finding must identify which page.
func TestAuditPagesDuplicateFindingsCarryPageID(t *testing.T) {
	desc := "A description that is comfortably long enough to clear the thin-description floor."
	a := page("p1", "/a", "A", "Shared", desc)
	b := page("p2", "/b", "B", "Shared", desc)
	got := auditPages([]wfPage{a, b}, "s1", stdThresholds)
	seen := map[string]bool{}
	for _, f := range got.Findings {
		if f.Issue == "duplicate-seo-title" {
			if f.PageID == "" {
				t.Fatalf("duplicate finding has no page id: %+v", f)
			}
			seen[f.PageID] = true
		}
	}
	if !seen["p1"] || !seen["p2"] {
		t.Fatalf("both colliding pages must be identified, got %v", seen)
	}
}

// --limit must not hide how many pages actually changed.
func TestBuildPublishPreviewKeepsTotalWhenTruncating(t *testing.T) {
	site := wfSite{ID: "s1", DisplayName: "S", LastPublished: "2026-07-01T00:00:00Z"}
	pages := make([]rawRow, 0, 4)
	for _, slug := range []string{"/a", "/b", "/c", "/d"} {
		pages = append(pages, rowOf(t, slug, "s1", wfPage{ID: slug, Slug: slug, LastUpdated: "2026-07-05T00:00:00Z"}))
	}
	got := buildPublishPreview(site, pages, nil, nil, 0, 2)
	if len(got.PagesChanged) != 2 {
		t.Fatalf("expected 2 listed pages, got %d", len(got.PagesChanged))
	}
	if got.PagesChangedTotal != 4 {
		t.Fatalf("PagesChangedTotal = %d, want 4", got.PagesChangedTotal)
	}
}

// Each selected item must own its field map, so a future per-item tweak cannot
// corrupt every other row.
func TestSelectBulkTargetsDoesNotAliasSetMap(t *testing.T) {
	rows := []rawRow{
		rowOf(t, "i1", "c1", wfItem{ID: "i1", FieldData: map[string]any{"name": "A"}}),
		rowOf(t, "i2", "c1", wfItem{ID: "i2", FieldData: map[string]any{"name": "B"}}),
	}
	set := map[string]string{"category": "updates"}
	got := selectBulkTargets(rows, "c1", nil, set, 10, nil)
	if len(got.Changes) != 2 {
		t.Fatalf("want 2 changes, got %d", len(got.Changes))
	}
	got.Changes[0].Fields["category"] = "mutated"
	if got.Changes[1].Fields["category"] != "updates" {
		t.Fatal("per-item Fields maps alias each other")
	}
	if set["category"] != "updates" {
		t.Fatal("mutating a change leaked back into the --set map")
	}
}

// A bare, valid invocation of an audit command used to print that command's
// own help and exit 0. To a shell caller that reads as success; to an agent
// parsing --json it is prose where data should be, with no error to branch on.
// Every command here must instead produce a result or fail loudly. Two of them
// (seo audit, publish preview) legitimately run with no positional argument;
// the other four need an id and must say so with a non-zero exit.
func TestNovelCommandsNeverDegradeToHelpWhenInvokedBare(t *testing.T) {
	// Redirect the whole data surface to a temp dir instead of passing --db.
	// Passing any flag sets NFlag() > 0, which is exactly the condition the
	// old guard tested -- a test that passes --db cannot observe the bug.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home+"/data")
	t.Setenv("XDG_CONFIG_HOME", home+"/config")
	t.Setenv("XDG_STATE_HOME", home+"/state")
	t.Setenv("XDG_CACHE_HOME", home+"/cache")

	for _, args := range [][]string{
		{"seo", "audit"},
		{"redirects", "audit"},
		{"publish", "preview"},
		{"drift"},
		{"items", "bulk-set"},
		{"collections", "completeness"},
	} {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			root := newRootCmd(&rootFlags{})
			var out strings.Builder
			root.SetOut(&out)
			root.SetErr(&out)
			root.SilenceUsage = true
			root.SetArgs(args)

			err := root.Execute()
			got := out.String()
			helpShaped := strings.Contains(got, "Examples:") && strings.Contains(got, "Flags:")
			if err == nil && helpShaped {
				t.Fatalf("%q printed its own help and returned no error; want a result or an explicit failure", name)
			}
		})
	}
}
