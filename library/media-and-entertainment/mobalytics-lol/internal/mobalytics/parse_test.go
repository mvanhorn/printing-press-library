// Copyright 2026 QuantumGlitch and contributors. Licensed under Apache-2.0. See LICENSE.

package mobalytics

import "testing"

func TestParseTierList_basic(t *testing.T) {
	html := `xxx"slug":"jinx","riftTiers":[{"__typename":"ChampionTiersV1riftTiersChildDto","role":"ADC","skillLevel":"low-elo","tags":null,"tier":"S"}]yyy"slug":"ezreal","riftTiers":[{"__typename":"ChampionTiersV1riftTiersChildDto","role":"ADC","skillLevel":"low-elo","tags":null,"tier":"B"},{"__typename":"X","role":"ADC","skillLevel":"high-elo","tags":null,"tier":"C"}]zzz`
	rows := ParseTierList(html)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].Slug != "jinx" || rows[0].Tier != "S" || rows[0].Role != "ADC" {
		t.Errorf("row 0 unexpected: %+v", rows[0])
	}
}

func TestFilterTierRows_byRole(t *testing.T) {
	rows := []TierRow{
		{Slug: "jinx", Role: "ADC", SkillLevel: "low-elo", Tier: "S"},
		{Slug: "darius", Role: "TOP", SkillLevel: "low-elo", Tier: "A"},
	}
	out := FilterTierRows(rows, "ADC", "")
	if len(out) != 1 || out[0].Slug != "jinx" {
		t.Errorf("expected only jinx, got %+v", out)
	}
}

func TestSortTierRows_rank(t *testing.T) {
	rows := []TierRow{
		{Slug: "c1", Tier: "C"},
		{Slug: "s1", Tier: "S"},
		{Slug: "a1", Tier: "A"},
		{Slug: "splus", Tier: "S+"},
	}
	SortTierRows(rows)
	want := []string{"splus", "s1", "a1", "c1"}
	for i, w := range want {
		if rows[i].Slug != w {
			t.Errorf("position %d: want %s, got %s", i, w, rows[i].Slug)
		}
	}
}

func TestBuildToItemset_mapHint(t *testing.T) {
	b := ChampionBuild{Type: "core", Items: []ItemBlock{{Type: "starter", Items: []int{1001}}}}
	is := BuildToItemset(b, 99, "jinx", "aram")
	if is.Map != "HA" {
		t.Errorf("expected ARAM map hint HA, got %q", is.Map)
	}
	if len(is.Blocks) != 1 || len(is.Blocks[0].Items) != 1 {
		t.Errorf("expected 1 block with 1 item, got %+v", is.Blocks)
	}
	is2 := BuildToItemset(b, 99, "jinx", "rift")
	if is2.Map != "any" {
		t.Errorf("expected non-ARAM map hint any, got %q", is2.Map)
	}
}
