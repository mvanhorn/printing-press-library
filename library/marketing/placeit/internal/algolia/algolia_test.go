// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package algolia

import "testing"

func TestIndexForSort(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", IndexMain, false},
		{"relevance", IndexMain, false},
		{"newest", IndexNewest, false},
		{"NEW", IndexNewest, false},
		{"best-selling", IndexBestSelling, false},
		{"popular", IndexBestSelling, false},
		{"free", IndexFree, false},
		{"  free  ", IndexFree, false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := IndexForSort(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("IndexForSort(%q) expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("IndexForSort(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("IndexForSort(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCategoryFacet(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"mockups", "Mockups", false},
		{"Mockup", "Mockups", false},
		{"logos", "Logos", false},
		{"videos", "Videos", false},
		{"designs", "Design Templates", false},
		{"design-templates", "Design Templates", false},
		{"nope", "", true},
	}
	for _, c := range cases {
		got, err := CategoryFacet(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("CategoryFacet(%q) expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("CategoryFacet(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("CategoryFacet(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNewRespectsEnvOverride(t *testing.T) {
	t.Setenv("PLACEIT_ALGOLIA_APP_ID", "TESTAPP")
	t.Setenv("PLACEIT_ALGOLIA_API_KEY", "testkey")
	c := New(0)
	if c.AppID != "TESTAPP" || c.APIKey != "testkey" {
		t.Errorf("New did not honor env overrides: got %q/%q", c.AppID, c.APIKey)
	}
}
