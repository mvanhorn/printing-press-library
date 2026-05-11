// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package igutil

import "testing"

func TestParseShortcode(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"bare shortcode", "B0Lz_4QH", "B0Lz_4QH", false},
		{"post URL", "https://www.instagram.com/p/B0Lz_4QH/", "B0Lz_4QH", false},
		{"post URL no trailing slash", "https://www.instagram.com/p/B0Lz_4QH", "B0Lz_4QH", false},
		{"reel URL", "https://www.instagram.com/reel/CXabc12_xyz/", "CXabc12_xyz", false},
		{"tv URL", "https://www.instagram.com/tv/D9-_QRStuvw/", "D9-_QRStuvw", false},
		{"reels plural URL", "https://www.instagram.com/reels/CXabc12_xyz/", "CXabc12_xyz", false},
		{"URL with query", "https://www.instagram.com/p/B0Lz_4QH/?igsh=abc", "B0Lz_4QH", false},
		{"empty", "", "", true},
		{"garbage", "not a shortcode at all because spaces", "", true},
		{"unknown path", "https://www.instagram.com/explore/", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseShortcode(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShortcodeMediaIDRoundTrip(t *testing.T) {
	// Round-trip the canonical example: B0Lz_4QH -> numeric -> back. The
	// minimum valid input length is 5 (matches IG's actual shortcode shape),
	// so smaller inputs intentionally don't round-trip.
	id, err := ShortcodeToMediaID("B0Lz_4QH")
	if err != nil {
		t.Fatalf("ShortcodeToMediaID(B0Lz_4QH): %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty media id")
	}
	back := MediaIDToShortcode(id)
	if back != "B0Lz_4QH" {
		t.Errorf("round-trip B0Lz_4QH -> %s -> %s", id, back)
	}

	// Suffix stripping: "<numeric>_<owner>" should produce the same result as
	// the bare numeric.
	withSuffix := MediaIDToShortcode(id + "_1234567890")
	if withSuffix != back {
		t.Errorf("suffix-stripped round-trip %q != %q", withSuffix, back)
	}

	// A second known shortcode for breadth.
	for _, sc := range []string{"CXabc12_xyz", "DABCdefghij", "B0Lz_4QH"} {
		id, err := ShortcodeToMediaID(sc)
		if err != nil {
			t.Fatalf("ShortcodeToMediaID(%q): %v", sc, err)
		}
		if MediaIDToShortcode(id) != sc {
			t.Errorf("round-trip %q failed: %q -> %q", sc, id, MediaIDToShortcode(id))
		}
	}
}

func TestShortcodeToMediaIDInvalid(t *testing.T) {
	if _, err := ShortcodeToMediaID(""); err == nil {
		t.Error("expected error on empty input")
	}
	if _, err := ShortcodeToMediaID("not valid!"); err == nil {
		t.Error("expected error on invalid characters")
	}
}
