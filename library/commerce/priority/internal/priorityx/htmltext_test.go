// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.

package priorityx

import "testing"

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello world", "hello world"},
		{"style block dropped", "<style> p {margin:0} </style><p>note</p>", "note"},
		{"breaks to newlines", "<p>line one</p><p>line two</p>", "line one\nline two"},
		{"entities unescaped", "Tom &amp; Jerry&#39;s", "Tom & Jerry's"},
		{"nested tags", "<div><b>rush</b> order <i>today</i></div>", "rush order today"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripHTML(tt.in); got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
