// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
package awwwards

import (
	"math"
	"testing"
)

func TestParseHex(t *testing.T) {
	tests := []struct {
		in      string
		want    RGB
		wantErr bool
	}{
		{"#0F4C81", RGB{15, 76, 129}, false},
		{"0f4c81", RGB{15, 76, 129}, false},
		{"#FFF", RGB{255, 255, 255}, false},
		{"#080807", RGB{8, 8, 7}, false},
		{"nope", RGB{}, true},
		{"#12345", RGB{}, true},
		{"", RGB{}, true},
	}
	for _, tt := range tests {
		got, err := ParseHex(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseHex(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseHex(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestDistance(t *testing.T) {
	tests := []struct {
		a, b RGB
		want float64
	}{
		{RGB{0, 0, 0}, RGB{0, 0, 0}, 0},
		{RGB{255, 255, 255}, RGB{0, 0, 0}, math.Sqrt(3 * 255 * 255)},
		{RGB{10, 0, 0}, RGB{0, 0, 0}, 10},
	}
	for _, tt := range tests {
		if got := Distance(tt.a, tt.b); math.Abs(got-tt.want) > 0.001 {
			t.Errorf("Distance(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestHueFamily(t *testing.T) {
	tests := []struct {
		hex  string
		want string
	}{
		{"#000000", "black"},
		{"#FFFFFF", "white"},
		{"#808080", "gray"},
		{"#FF0000", "red"},
		{"#FF8800", "orange"},
		{"#FFEE00", "yellow"},
		{"#00CC44", "green"},
		{"#00BBDD", "cyan"},
		{"#0044FF", "blue"},
		{"#8800EE", "purple"},
		{"#FF00AA", "pink"},
	}
	for _, tt := range tests {
		c, err := ParseHex(tt.hex)
		if err != nil {
			t.Fatalf("ParseHex(%q): %v", tt.hex, err)
		}
		if got := HueFamily(c); got != tt.want {
			t.Errorf("HueFamily(%s) = %q, want %q", tt.hex, got, tt.want)
		}
	}
}

func TestHueFamilyBoundaries(t *testing.T) {
	// Threshold-adjacent cases: hue cutoffs at 15/45/345 degrees and the
	// low-saturation / lightness neutral boundaries.
	tests := []struct {
		r, g, b int
		want    string
	}{
		{255, 64, 0, "orange"},   // h=15 exactly -> orange bucket (h < 15 is red)
		{255, 192, 0, "yellow"},  // h=45.2 -> yellow bucket (255,191,0 is 44.9 = orange)
		{255, 0, 63, "red"},      // h=345.2 -> red bucket (>= 345); 255,0,64 is 344.9 = pink
		{30, 30, 30, "black"},    // dark neutral, l < 0.15
		{240, 240, 240, "white"}, // light neutral, l > 0.87
		{120, 128, 122, "gray"},  // low-saturation mid tone, d < 0.09
	}
	for _, tt := range tests {
		got := HueFamily(RGB{tt.r, tt.g, tt.b})
		if got != tt.want {
			t.Errorf("HueFamily(%d,%d,%d) = %q, want %q", tt.r, tt.g, tt.b, got, tt.want)
		}
	}
}
