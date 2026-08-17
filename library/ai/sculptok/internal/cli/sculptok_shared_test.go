// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestDrawCost(t *testing.T) {
	cases := []struct {
		kind, style, drawHD string
		want                int
	}{
		{"depthmap", "", "", 10},
		{"depthmap", "normal", "", 10},
		{"depthmap", "pro", "", 15},
		{"depthmap", "pro", "2k", 15},
		{"depthmap", "pro", "4k", 30},
		{"restore", "", "", 2},
		{"threed", "", "", 10},
		{"stl", "", "", 3},
		{"unknown", "", "", 0},
	}
	for _, c := range cases {
		if got := drawCost(c.kind, c.style, c.drawHD); got != c.want {
			t.Errorf("drawCost(%q,%q,%q) = %d; want %d", c.kind, c.style, c.drawHD, got, c.want)
		}
	}
}

func TestImageField(t *testing.T) {
	if got := imageField("stl"); got != "image_url" {
		t.Errorf("imageField(stl) = %q; want image_url", got)
	}
	for _, k := range []string{"depthmap", "threed", "restore"} {
		if got := imageField(k); got != "imageUrl" {
			t.Errorf("imageField(%q) = %q; want imageUrl", k, got)
		}
	}
}

func TestIsImageURL(t *testing.T) {
	if !isImageURL("https://x/y.png") || !isImageURL("http://x/y.png") {
		t.Error("isImageURL should accept http(s) URLs")
	}
	if isImageURL("/local/path.png") || isImageURL("photo.jpg") {
		t.Error("isImageURL should reject local paths")
	}
}
