// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestRecordPolicyAndShouldRecord(t *testing.T) {
	cases := []struct {
		policy   string
		isNovel  bool
		noRecord bool
		want     bool
	}{
		{"", true, false, true},        // default novel-only: novel records
		{"", false, false, false},      // default: run does not
		{"always", false, false, true}, // always: run records too
		{"never", true, false, false},  // never: even novel skips
		{"novel-only", true, true, false}, // explicit opt-out wins
	}
	for _, c := range cases {
		got := shouldRecord(wavespeedProjectConfig{Record: c.policy}, c.isNovel, c.noRecord)
		if got != c.want {
			t.Errorf("shouldRecord(policy=%q novel=%v noRecord=%v) = %v, want %v", c.policy, c.isNovel, c.noRecord, got, c.want)
		}
	}
}

func TestMergeBrandIntoShotPrecedence(t *testing.T) {
	body := brandProfileBody{
		StyleAnchors: []string{"matte black"},
		Negative:     "blurry",
		Palette:      []string{"#000"},
		Models:       []string{"wavespeed-ai/brand-model"},
		Params:       map[string]any{"steps": 30, "guidance": 5.0},
	}
	// Shot already carries an explicit param (steps=10) and model — both must win
	// over the brand's values (explicit < active brand is the rule, but explicit
	// shot params represent the -i/--set layer which is HIGHER than brand here per
	// the resolver: brand only fills gaps).
	shot := Shot{
		Prompt: "hero",
		Model:  "wavespeed-ai/explicit",
		Params: map[string]any{"steps": 10},
	}
	got := mergeBrandIntoShot(shot, "helm", body)

	if got.Model != "wavespeed-ai/explicit" {
		t.Errorf("explicit model overwritten: %q", got.Model)
	}
	if got.Params["steps"] != 10 {
		t.Errorf("explicit steps overwritten: %v", got.Params["steps"])
	}
	if got.Params["guidance"] != 5.0 {
		t.Errorf("brand param not filled: %v", got.Params["guidance"])
	}
	if got.Params["negative_prompt"] != "blurry" {
		t.Errorf("negative not applied: %v", got.Params["negative_prompt"])
	}
	if got.Brand != "helm" {
		t.Errorf("brand name not set: %q", got.Brand)
	}
	// Anchors + palette appended to prompt.
	if want := "hero, matte black, palette: #000"; got.Prompt != want {
		t.Errorf("prompt = %q, want %q", got.Prompt, want)
	}
}

func TestMergeBrandFillsModelGap(t *testing.T) {
	body := brandProfileBody{Models: []string{"wavespeed-ai/brand-model"}}
	got := mergeBrandIntoShot(Shot{Prompt: "x"}, "helm", body)
	if got.Model != "wavespeed-ai/brand-model" {
		t.Fatalf("model gap not filled by brand: %q", got.Model)
	}
}

func TestResolveActiveBrandFlagWins(t *testing.T) {
	p := wavespeedProjectConfig{ActiveBrand: "active"}
	if got := resolveActiveBrand(p, "flag"); got != "flag" {
		t.Fatalf("flag should win: %q", got)
	}
	if got := resolveActiveBrand(p, ""); got != "active" {
		t.Fatalf("active fallback: %q", got)
	}
}

func TestDecodeShotlistShapes(t *testing.T) {
	// bare array
	shots, err := decodeShotlist([]byte(`[{"prompt":"a"},{"prompt":"b"}]`))
	if err != nil || len(shots) != 2 {
		t.Fatalf("array: %v %d", err, len(shots))
	}
	// envelope with results
	shots, err = decodeShotlist([]byte(`{"command":"plan","results":[{"prompt":"a"}]}`))
	if err != nil || len(shots) != 1 || shots[0].Prompt != "a" {
		t.Fatalf("envelope: %v %#v", err, shots)
	}
	// single object
	shots, err = decodeShotlist([]byte(`{"prompt":"solo"}`))
	if err != nil || len(shots) != 1 || shots[0].Prompt != "solo" {
		t.Fatalf("single: %v %#v", err, shots)
	}
	// empty
	if _, err := decodeShotlist([]byte("   ")); err == nil {
		t.Fatalf("empty should error")
	}
}

func TestExtractCostFromPricing(t *testing.T) {
	if got := extractCostFromPricing(json.RawMessage(`{"data":{"price":0.5}}`)); got != 0.5 {
		t.Errorf("price = %v", got)
	}
	if got := extractCostFromPricing(json.RawMessage(`{"total":2}`)); got != 2 {
		t.Errorf("total = %v", got)
	}
	if got := extractCostFromPricing(json.RawMessage(`{"nope":1}`)); got != 0 {
		t.Errorf("unknown = %v", got)
	}
	if got := extractCostFromPricing(nil); got != 0 {
		t.Errorf("nil = %v", got)
	}
}

func TestSuggestNextDedup(t *testing.T) {
	got := suggestNext("a", "b", "a", "", "c")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("suggestNext = %v", got)
	}
}

func TestToModelInputs(t *testing.T) {
	seed := int64(42)
	s := Shot{Prompt: "hi", Seed: &seed, Params: map[string]any{"steps": 20}, Inputs: map[string]any{"image": "u"}}
	in := s.toModelInputs()
	if in["prompt"] != "hi" || in["steps"] != 20 || in["image"] != "u" || in["seed"] != int64(42) {
		t.Fatalf("inputs = %#v", in)
	}
}
