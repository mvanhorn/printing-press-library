// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelOrphansHelpWires smoke-tests that the orphans command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelOrphansHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"orphans", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("orphans --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "orphans"} {
		if !strings.Contains(help, want) {
			t.Fatalf("orphans --help missing %q in output:\n%s", want, help)
		}
	}
}

// --- Phase 3 behavior tests ---

func TestComputeOrphans(t *testing.T) {
	d := orphanData{
		campaigns:       map[string]string{"cmpn_1": "One", "cmpn_2": "Two"},
		adGroups:        map[string]string{"adgrp_1": "G1"},
		adGroupCampaign: map[string]string{"adgrp_1": "cmpn_1"},
		ads:             map[string]string{"ad_1": "A1"},
		adAdGroup:       map[string]string{"ad_1": "adgrp_1"},
		audiences:       map[string]string{"aud_1": "Aud1", "aud_2": "Aud2"},
		referencedAuds:  map[string]bool{"aud_2": true},
	}
	got := computeOrphans(d)
	// cmpn_2 has no ad group; adgrp_1 has no ad -> but it DOES have ad_1, so not.
	// ad_1 valid; aud_1 unreferenced.
	wantKinds := map[string]bool{
		orphanCampaignNoAdGroups: true,
		orphanAudienceNoBid:      true,
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %+v", got)
	}
	for _, f := range got {
		if !wantKinds[f.Kind] {
			t.Fatalf("unexpected finding kind %q", f.Kind)
		}
	}
}

func TestNovelOrphans_EmptyStoreJSON(t *testing.T) {
	novelEmptyStore(t)
	out, err := runNovelCmd(t, "orphans", "--json")
	if err != nil {
		t.Fatalf("orphans empty: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("orphans empty expected [], got %q", out)
	}
}

func TestNovelOrphans_HappyPath(t *testing.T) {
	novelSeedStore(t)
	out, err := runNovelCmd(t, "orphans", "--json")
	if err != nil {
		t.Fatalf("orphans: %v", err)
	}
	var rows []orphanFinding
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("orphans json: %v\n%s", err, out)
	}
	found := map[string]string{}
	for _, r := range rows {
		found[r.Kind+"\x00"+r.ID] = r.Kind
	}
	if _, ok := found[orphanCampaignNoAdGroups+"\x00cmpn_2"]; !ok {
		t.Fatalf("cmpn_2 campaign-without-ad-groups missing: %+v", rows)
	}
	if _, ok := found[orphanAdGroupNoAds+"\x00adgrp_3"]; !ok {
		t.Fatalf("adgrp_3 ad-group-without-ads missing: %+v", rows)
	}
	if _, ok := found[orphanAudienceNoBid+"\x00aud_1"]; !ok {
		t.Fatalf("aud_1 audience-without-multiplier missing: %+v", rows)
	}
	// adgrp_1 and adgrp_2 each have an ad; both ads reference valid ad groups.
	if _, ok := found[orphanAdNoAdGroup]; ok {
		t.Fatalf("unexpected ad-without-ad-group: %+v", rows)
	}
}
