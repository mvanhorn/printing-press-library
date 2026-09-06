// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func ent(kind, id, name, typ, data string) gtmEntity {
	return gtmEntity{Kind: kind, EntityID: id, Name: name, Type: typ, Data: json.RawMessage(data)}
}

func auditCounts(entities []gtmEntity) map[string]int {
	counts := map[string]int{}
	for _, f := range runAudit(entities) {
		counts[f.Check]++
	}
	return counts
}

func TestRunAudit(t *testing.T) {
	entities := []gtmEntity{
		ent("tag", "1", "Dead", "html", `{"name":"Dead","type":"html","firingTriggerId":[]}`),
		ent("tag", "2", "Live GA4", "googtag", `{"name":"Live GA4","type":"googtag","firingTriggerId":["100"],"consentSettings":{"consentStatus":"needed"},"parameter":[{"value":"{{Used}}"}]}`),
		ent("tag", "3", "Pixel", "html", `{"name":"Pixel","type":"html","firingTriggerId":["100"]}`),
		ent("tag", "4", "AllPages", "googtag", `{"name":"AllPages","type":"googtag","firingTriggerId":["2147479553"],"consentSettings":{"consentStatus":"needed"}}`),
		ent("trigger", "100", "Click", "click", `{"name":"Click","triggerId":"100"}`),
		ent("trigger", "200", "Orphan", "pageview", `{"name":"Orphan","triggerId":"200"}`),
		ent("variable", "10", "Used", "c", `{"name":"Used","variableId":"10"}`),
		ent("variable", "11", "Unused", "c", `{"name":"Unused","variableId":"11"}`),
	}
	got := auditCounts(entities)
	want := map[string]int{
		"dead-tag":        1, // Dead
		"missing-consent": 2, // Dead (html, notSet) + Pixel (html, notSet)
		"custom-html":     2, // Dead + Pixel
		"orphan-trigger":  1, // Orphan
		"unused-variable": 1, // Unused
		"all-pages":       1, // AllPages
	}
	for check, n := range want {
		if got[check] != n {
			t.Errorf("check %q: got %d, want %d (all: %v)", check, got[check], n, got)
		}
	}
}

func TestBuildRefIndex(t *testing.T) {
	entities := []gtmEntity{
		ent("tag", "1", "GA4", "googtag", `{"name":"GA4","firingTriggerId":["100"],"blockingTriggerId":["200"],"parameter":[{"value":"{{Measurement ID}}"}]}`),
		ent("trigger", "100", "Fire", "click", `{"triggerId":"100"}`),
		ent("trigger", "200", "Block", "click", `{"triggerId":"200"}`),
		ent("variable", "10", "Measurement ID", "c", `{"name":"Measurement ID"}`),
	}
	idx := buildRefIndex(entities)
	if got := idx.tagTriggers["1"]; len(got) != 2 {
		t.Errorf("tag 1 triggers: got %v, want 2 (firing+blocking)", got)
	}
	if got := idx.varUsedBy["Measurement ID"]; len(got) != 1 || got[0] != "tag:GA4" {
		t.Errorf("var usedBy: got %v, want [tag:GA4]", got)
	}
	if got := idx.trigUsedBy["100"]; len(got) != 1 || got[0] != "tag:GA4" {
		t.Errorf("trigger 100 usedBy: got %v, want [tag:GA4]", got)
	}
}

func TestVariableRefsIn(t *testing.T) {
	got := variableRefsIn(json.RawMessage(`{"a":"{{One}}","b":"x {{Two}} y {{One}}"}`))
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 unique refs", got)
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	if !seen["One"] || !seen["Two"] {
		t.Errorf("missing expected refs in %v", got)
	}
}

func TestRunConsentReport(t *testing.T) {
	entities := []gtmEntity{
		ent("tag", "1", "Gated", "googtag", `{"consentSettings":{"consentStatus":"needed","consentType":{"list":[{"value":"analytics_storage"}]}}}`),
		ent("tag", "2", "Ungated", "html", `{}`),
		ent("trigger", "100", "x", "click", `{}`),
	}
	rows := runConsentReport(entities)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (tags only)", len(rows))
	}
	// Ungated sorts first.
	if rows[0].Gated || rows[0].Tag != "Ungated" {
		t.Errorf("row0 = %+v, want ungated 'Ungated' first", rows[0])
	}
	if !rows[1].Gated || len(rows[1].ConsentTypes) != 1 {
		t.Errorf("row1 = %+v, want gated with 1 consent type", rows[1])
	}
}

func TestDiffSnapshots(t *testing.T) {
	a := []gtmEntity{
		ent("tag", "1", "Keep", "html", `{"name":"Keep","paused":false}`),
		ent("tag", "2", "Gone", "html", `{"name":"Gone"}`),
	}
	b := []gtmEntity{
		ent("tag", "1", "Keep", "html", `{"name":"Keep","paused":true}`),
		ent("tag", "3", "New", "html", `{"name":"New"}`),
	}
	changes := diffSnapshots(a, b)
	ops := map[string]diffChange{}
	for _, c := range changes {
		ops[c.Op+":"+c.Name] = c
	}
	if _, ok := ops["removed:Gone"]; !ok {
		t.Error("expected Gone removed")
	}
	if _, ok := ops["added:New"]; !ok {
		t.Error("expected New added")
	}
	chg, ok := ops["changed:Keep"]
	if !ok {
		t.Fatal("expected Keep changed")
	}
	if len(chg.ChangedFields) != 1 || chg.ChangedFields[0] != "paused" {
		t.Errorf("changed fields = %v, want [paused]", chg.ChangedFields)
	}
}

func TestDiffIgnoresVolatileFields(t *testing.T) {
	a := []gtmEntity{ent("tag", "1", "T", "html", `{"name":"T","fingerprint":"111","path":"a"}`)}
	b := []gtmEntity{ent("tag", "1", "T", "html", `{"name":"T","fingerprint":"222","path":"b"}`)}
	if changes := diffSnapshots(a, b); len(changes) != 0 {
		t.Errorf("volatile-only diff should be empty, got %v", changes)
	}
}

func TestResolveSnapshotScoped(t *testing.T) {
	// newest-first; container 9 is active (newest), container 5 is older.
	snaps := []gtmSnapshot{
		{ID: 2, ContainerID: "9", Source: "workspace:7", PulledAt: "2026-06-17T11:00:00Z"},
		{ID: 3, ContainerID: "5", Source: "live", PulledAt: "2026-06-17T10:30:00Z"},
		{ID: 1, ContainerID: "9", Source: "live", PulledAt: "2026-06-17T10:00:00Z"},
	}
	cases := []struct {
		ref, prefer string
		wantID      int64
	}{
		{"", "", 2},               // default -> newest overall
		{"live", "", 1},           // active container (9) live, not FSCC(5)
		{"workspace:7", "", 2},    // active container workspace
		{"container:5", "", 3},    // explicit container switch
		{"live", "5", 3},          // scoped to container 5
		{"snapshot:1", "", 1},     // by id
		{"3", "", 3},              // bare id
	}
	for _, tc := range cases {
		got, err := resolveSnapshotScoped(snaps, tc.ref, tc.prefer)
		if err != nil {
			t.Errorf("ref %q prefer %q: %v", tc.ref, tc.prefer, err)
			continue
		}
		if got.ID != tc.wantID {
			t.Errorf("ref %q prefer %q: got snapshot %d, want %d", tc.ref, tc.prefer, got.ID, tc.wantID)
		}
	}
}

func TestFleetRowFor(t *testing.T) {
	snap := gtmSnapshot{ContainerID: "9", ContainerName: "Web", PublicID: "GTM-AAAA", Source: "live"}
	entities := []gtmEntity{
		ent("tag", "1", "GA4", "googtag", `{"type":"googtag","consentSettings":{"consentStatus":"needed"},"parameter":[{"value":"G-ABC12345"}]}`),
		ent("tag", "2", "HTML", "html", `{"type":"html"}`),
		ent("trigger", "100", "t", "click", `{}`),
		ent("variable", "10", "v", "c", `{}`),
	}
	row := fleetRowFor(snap, entities)
	if row.Tags != 2 || row.Triggers != 1 || row.Variables != 1 {
		t.Errorf("counts = %d/%d/%d, want 2/1/1", row.Tags, row.Triggers, row.Variables)
	}
	if row.CustomHTML != 1 {
		t.Errorf("customHTML = %d, want 1", row.CustomHTML)
	}
	// 2 tracking tags (googtag + html), 1 gated -> 50%.
	if row.ConsentPct != 50 {
		t.Errorf("consentPct = %d, want 50", row.ConsentPct)
	}
	if len(row.MeasurementIDs) != 1 || row.MeasurementIDs[0] != "G-ABC12345" {
		t.Errorf("measurementIDs = %v, want [G-ABC12345]", row.MeasurementIDs)
	}
}

func TestParseEntityID(t *testing.T) {
	cases := []struct{ kind, data, wantID, wantName string }{
		{"tag", `{"name":"T","tagId":"5"}`, "5", "T"},
		{"trigger", `{"name":"Tr","triggerId":"9"}`, "9", "Tr"},
		{"variable", `{"name":"V","variableId":"3"}`, "3", "V"},
	}
	for _, tc := range cases {
		e := parseEntity(json.RawMessage(tc.data), tc.kind)
		if e.EntityID != tc.wantID || e.Name != tc.wantName {
			t.Errorf("parseEntity(%s) = id %q name %q, want %q/%q", tc.data, e.EntityID, e.Name, tc.wantID, tc.wantName)
		}
	}
}

func TestPullContainerLive(t *testing.T) {
	// Fake getter returns a container record, an account, and a live version.
	fake := fakeGetter{responses: map[string]string{
		"/tagmanager/v2/accounts/601/containers/9":              `{"name":"Web","publicId":"GTM-AAAA"}`,
		"/tagmanager/v2/accounts/601":                           `{"name":"PBL"}`,
		"/tagmanager/v2/accounts/601/containers/9/versions:live": `{"containerVersionId":"12","tag":[{"name":"GA4","tagId":"1","type":"googtag"}],"trigger":[{"name":"T","triggerId":"100"}],"variable":[{"name":"V","variableId":"10"}]}`,
	}}
	db := newTestDB(t)
	snap, err := pullContainer(testCtx(), fake, db, "601", "9", "live", fixedNow)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if snap.ContainerName != "Web" || snap.PublicID != "GTM-AAAA" || snap.VersionID != "12" {
		t.Errorf("snapshot meta wrong: %+v", snap)
	}
	if snap.EntityCount != 3 {
		t.Errorf("entity count = %d, want 3", snap.EntityCount)
	}
	ents, err := snapshotEntities(testCtx(), db, snap.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 3 {
		t.Errorf("stored %d entities, want 3", len(ents))
	}
}
