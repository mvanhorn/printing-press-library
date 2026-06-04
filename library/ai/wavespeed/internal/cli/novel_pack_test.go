// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPackShots(t *testing.T) {
	// Platforms only → per-platform default aspect.
	shots := buildPackShots(packFlags{concept: "hero", platforms: []string{"instagram", "tiktok"}}, "", brandProfileBody{})
	if len(shots) != 2 {
		t.Fatalf("platforms = %#v", shots)
	}
	if shots[0].AspectRatio != "4:5" || shots[1].AspectRatio != "9:16" {
		t.Fatalf("default aspects wrong: %#v", shots)
	}

	// Platforms × aspects → cartesian.
	shots = buildPackShots(packFlags{concept: "hero", platforms: []string{"instagram"}, aspects: []string{"16:9", "1:1"}}, "", brandProfileBody{})
	if len(shots) != 2 {
		t.Fatalf("platform×aspect = %#v", shots)
	}
	if shots[0].Params["aspect_ratio"] != "16:9" {
		t.Fatalf("aspect not in params: %#v", shots[0].Params)
	}

	// Seed propagates.
	shots = buildPackShots(packFlags{concept: "hero", platforms: []string{"x"}, seed: 7}, "", brandProfileBody{})
	if shots[0].Seed == nil || *shots[0].Seed != 7 {
		t.Fatalf("seed not set: %#v", shots[0].Seed)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Helm Black Hero": "helm-black-hero",
		"  spaces  ":       "spaces",
		"!!!":              "pack",
		"Already-Slug":     "already-slug",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDirSafe(t *testing.T) {
	if dirSafe("") != "general" {
		t.Error("empty platform → general")
	}
	if dirSafe("Instagram") != "instagram" {
		t.Error("platform slugified")
	}
}

func writeFixturePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestValidateImageDims(t *testing.T) {
	dir := t.TempDir()
	// Instagram feed expects 1080x1350. Write a mismatched 100x100 PNG.
	bad := filepath.Join(dir, "feed.png")
	writeFixturePNG(t, bad, 100, 100)
	dims, warn := validateImageDims([]string{bad}, Shot{Platform: "instagram", Format: "feed"})
	if dims != "100x100" {
		t.Fatalf("dims = %q", dims)
	}
	if warn == "" {
		t.Fatalf("expected a dimension-mismatch warning")
	}

	// Matching size → no warning.
	good := filepath.Join(dir, "good.png")
	writeFixturePNG(t, good, 1080, 1350)
	dims, warn = validateImageDims([]string{good}, Shot{Platform: "instagram", Format: "feed"})
	if dims != "1080x1350" || warn != "" {
		t.Fatalf("matching dims should not warn: %q %q", dims, warn)
	}
}
