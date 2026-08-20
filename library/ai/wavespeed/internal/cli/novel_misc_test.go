// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestParseComposeSteps(t *testing.T) {
	steps, err := parseComposeSteps("text->image,image->upscale,image->video", []string{"m1", "m2", "m3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 {
		t.Fatalf("steps = %#v", steps)
	}
	if steps[0].From != "text" || steps[0].To != "image" || steps[0].Model != "m1" {
		t.Fatalf("step0 = %#v", steps[0])
	}
	if steps[2].Model != "m3" {
		t.Fatalf("step2 model = %q", steps[2].Model)
	}

	// Fewer models than steps → reuse last.
	steps, _ = parseComposeSteps("text->image,image->video", []string{"only"})
	if steps[1].Model != "only" {
		t.Fatalf("model reuse: %q", steps[1].Model)
	}

	if _, err := parseComposeSteps("garbage", nil); err == nil {
		t.Fatalf("expected error for malformed steps")
	}
}

func TestStepInputKey(t *testing.T) {
	if stepInputKey("video") != "video" {
		t.Error("video → video")
	}
	if stepInputKey("image") != "image" {
		t.Error("image → image")
	}
	if stepInputKey("text") != "image" {
		t.Error("default → image")
	}
}

func TestBuildVariantShots(t *testing.T) {
	base := Shot{Prompt: "hero", Model: "m"}
	seedShots, err := buildVariantShots(base, "seed", variantsFlags{count: 3})
	if err != nil || len(seedShots) != 3 {
		t.Fatalf("seed sweep = %v %d", err, len(seedShots))
	}
	if seedShots[0].Seed == nil || *seedShots[2].Seed != 3 {
		t.Fatalf("seeds = %#v", seedShots)
	}

	styleShots, err := buildVariantShots(base, "style", variantsFlags{values: []string{"noir", "pastel"}})
	if err != nil || len(styleShots) != 2 || styleShots[0].Prompt != "hero, noir" {
		t.Fatalf("style sweep = %v %#v", err, styleShots)
	}

	if _, err := buildVariantShots(base, "style", variantsFlags{}); err == nil {
		t.Fatalf("style sweep without values should error")
	}
	if _, err := buildVariantShots(base, "bogus", variantsFlags{}); err == nil {
		t.Fatalf("unknown vary should error")
	}
}

func TestAspectTargets(t *testing.T) {
	got := aspectTargets(aspectsFlags{platforms: []string{"instagram", "tiktok"}, aspects: []string{"1:1"}})
	// 1:1 (explicit) + 4:5 (ig default) + 9:16 (tiktok default), deduped.
	want := map[string]bool{"1:1": true, "4:5": true, "9:16": true}
	if len(got) != 3 {
		t.Fatalf("targets = %v", got)
	}
	for _, a := range got {
		if !want[a] {
			t.Fatalf("unexpected aspect %q in %v", a, got)
		}
	}
}

func TestModelSupportsOutpaint(t *testing.T) {
	if !modelSupportsOutpaint("wavespeed-ai/flux-fill") {
		t.Error("fill → outpaint capable")
	}
	if modelSupportsOutpaint("wavespeed-ai/flux-dev") {
		t.Error("plain model not outpaint capable")
	}
}

func TestParseSince(t *testing.T) {
	if zero, _ := parseSince(""); !zero.IsZero() {
		t.Error("empty → zero time")
	}
	got, err := parseSince("30d")
	if err != nil || time.Since(got) < 29*24*time.Hour {
		t.Fatalf("30d = %v %v", got, err)
	}
	if _, err := parseSince("2026-05-01"); err != nil {
		t.Fatalf("date parse: %v", err)
	}
	if _, err := parseSince("bogus"); err == nil {
		t.Fatalf("bogus should error")
	}
}

func TestPatchBrandBody(t *testing.T) {
	cmd := newBrandInitCmd(&rootFlags{})
	_ = cmd.Flags().Set("voice", "premium")
	_ = cmd.Flags().Set("palette", "#000,#fff")
	var bf brandFlags
	// Re-bind the flag values by reading them back.
	bf.voice, _ = cmd.Flags().GetString("voice")
	bf.palette, _ = cmd.Flags().GetStringSlice("palette")
	var body brandProfileBody
	changed := patchBrandBody(cmd, &bf, &body)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if body.Voice != "premium" || len(body.Palette) != 2 {
		t.Fatalf("patched body = %#v", body)
	}
}
