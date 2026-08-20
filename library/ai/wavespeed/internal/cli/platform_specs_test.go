// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestPlatformSpecsIntegrity(t *testing.T) {
	for name, spec := range platformSpecs {
		if spec.Platform != name {
			t.Errorf("%s: spec.Platform = %q", name, spec.Platform)
		}
		if len(spec.Formats) == 0 {
			t.Errorf("%s: no formats", name)
		}
		if _, ok := formatFor(spec, spec.DefaultFormat); !ok {
			t.Errorf("%s: default format %q not in formats", name, spec.DefaultFormat)
		}
		for _, f := range spec.Formats {
			if f.Width <= 0 || f.Height <= 0 {
				t.Errorf("%s/%s: bad dims %dx%d", name, f.Format, f.Width, f.Height)
			}
			if f.IsVideo && f.DurationCapSec <= 0 {
				t.Errorf("%s/%s: video format must have a duration cap", name, f.Format)
			}
		}
	}
}

func TestInstagramReelDurationCap(t *testing.T) {
	spec, ok := lookupPlatform("Instagram")
	if !ok {
		t.Fatal("instagram not found (case-insensitive lookup)")
	}
	reel, ok := formatFor(spec, "reel")
	if !ok {
		t.Fatal("reel format missing")
	}
	if reel.DurationCapSec != 90 {
		t.Fatalf("IG reel cap = %d, want 90", reel.DurationCapSec)
	}
}

func TestKnownPlatformsSorted(t *testing.T) {
	got := knownPlatforms()
	if len(got) != 4 {
		t.Fatalf("platforms = %v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("not sorted: %v", got)
		}
	}
}

func TestDefaultFormatFor(t *testing.T) {
	f, ok := defaultFormatFor("instagram")
	if !ok || f.Format != "feed" || f.AspectRatio != "4:5" {
		t.Fatalf("ig default = %#v ok=%v", f, ok)
	}
	if _, ok := defaultFormatFor("myspace"); ok {
		t.Fatalf("unknown platform should not resolve")
	}
}
