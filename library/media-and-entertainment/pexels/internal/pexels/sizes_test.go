// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package pexels

import "testing"

func TestPickPhotoSize(t *testing.T) {
	src := map[string]string{
		"original": "https://images.pexels.com/orig.jpg",
		"large2x":  "https://images.pexels.com/large2x.jpg",
		"large":    "https://images.pexels.com/large.jpg",
		"medium":   "https://images.pexels.com/medium.jpg",
		"small":    "https://images.pexels.com/small.jpg",
		"tiny":     "https://images.pexels.com/tiny.jpg",
	}
	// landscape original 4000x3000 -> aspect 1.333
	cases := []struct {
		name               string
		targetW, targetH   int
		wantLabel          string
		wantMinW, wantMinH int
	}{
		// small (173x130) has the smallest pixel area of the scaled renditions.
		{"no constraint picks smallest by area", 0, 0, "small", 0, 130},
		{"720p needs large or larger", 1280, 720, "large2x", 1280, 720},
		{"medium fits 400x300", 400, 300, "medium", 400, 300},
		{"huge target falls back to original", 9000, 9000, "original", 0, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			label, url, w, h := PickPhotoSize(src, 4000, 3000, tc.targetW, tc.targetH)
			if label != tc.wantLabel {
				t.Errorf("label = %q, want %q", label, tc.wantLabel)
			}
			if url == "" {
				t.Errorf("url empty")
			}
			if tc.wantMinW > 0 && w < tc.wantMinW {
				t.Errorf("w = %d, want >= %d", w, tc.wantMinW)
			}
			if tc.wantMinH > 0 && h < tc.wantMinH {
				t.Errorf("h = %d, want >= %d", h, tc.wantMinH)
			}
		})
	}
}

func TestPickPhotoSizeEmpty(t *testing.T) {
	label, url, _, _ := PickPhotoSize(map[string]string{}, 100, 100, 0, 0)
	if label != "" || url != "" {
		t.Errorf("expected empty result for empty src, got label=%q url=%q", label, url)
	}
}

func TestPickPhotoSizePortraitLargeRenditionsPreserveAspectRatio(t *testing.T) {
	src := map[string]string{
		"original": "https://images.pexels.com/orig.jpg",
		"large2x":  "https://images.pexels.com/large2x.jpg",
		"large":    "https://images.pexels.com/large.jpg",
		"medium":   "https://images.pexels.com/medium.jpg",
		"small":    "https://images.pexels.com/small.jpg",
	}

	label, _, w, h := PickPhotoSize(src, 1000, 2000, 800, 0)
	if label != "original" {
		t.Fatalf("label = %q, want original because portrait large/large2x are narrower than target", label)
	}
	if w != 1000 || h != 2000 {
		t.Fatalf("dimensions = %dx%d, want 1000x2000", w, h)
	}

	candidates := photoCandidates(src, 1000, 2000)
	got := map[string]SizeCandidate{}
	for _, c := range candidates {
		got[c.Label] = c
	}
	if got["large"].W != 325 || got["large"].H != 650 {
		t.Fatalf("large = %dx%d, want 325x650", got["large"].W, got["large"].H)
	}
	if got["large2x"].W != 650 || got["large2x"].H != 1300 {
		t.Fatalf("large2x = %dx%d, want 650x1300", got["large2x"].W, got["large2x"].H)
	}
}

func TestPickVideoFile(t *testing.T) {
	files := []VideoFile{
		{Quality: "hls", Width: 0, Height: 0, Link: "manifest.m3u8"},
		{Quality: "sd", Width: 640, Height: 360, Link: "sd.mp4"},
		{Quality: "hd", Width: 1280, Height: 720, Link: "hd.mp4"},
		{Quality: "uhd", Width: 3840, Height: 2160, Link: "uhd.mp4"},
	}
	cases := []struct {
		name             string
		targetW, targetH int
		wantLink         string
	}{
		{"smallest meeting 720p", 1280, 720, "hd.mp4"},
		{"no constraint smallest sized", 0, 0, "sd.mp4"},
		{"over target falls to largest", 5000, 5000, "uhd.mp4"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, ok := PickVideoFile(files, tc.targetW, tc.targetH)
			if !ok {
				t.Fatal("expected ok")
			}
			if f.Link != tc.wantLink {
				t.Errorf("link = %q, want %q", f.Link, tc.wantLink)
			}
		})
	}
}

func TestPickVideoFileByQuality(t *testing.T) {
	files := []VideoFile{
		{Quality: "sd", Width: 640, Height: 360, Link: "sd.mp4"},
		{Quality: "hd", Width: 1280, Height: 720, Link: "hd.mp4"},
	}
	f, ok := PickVideoFileByQuality(files, "hd")
	if !ok || f.Link != "hd.mp4" {
		t.Errorf("got %q ok=%v, want hd.mp4", f.Link, ok)
	}
	// Unknown quality falls back to largest.
	f, ok = PickVideoFileByQuality(files, "uhd")
	if !ok || f.Link != "hd.mp4" {
		t.Errorf("fallback got %q, want hd.mp4", f.Link)
	}
	// Empty.
	if _, ok := PickVideoFileByQuality(nil, "hd"); ok {
		t.Error("expected ok=false for empty files")
	}
}
