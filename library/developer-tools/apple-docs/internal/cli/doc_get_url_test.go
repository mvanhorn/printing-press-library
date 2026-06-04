// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestAppleDocsSourceURL pins the DocC-identifier-to-canonical-URL mapping the
// `doc get --markdown` and `bundle` commands emit in their Source: lines.
// The bug this guards against shipped to live-check once and showed up as
// "https://developer.apple.comSwiftUI/documentation/SwiftUI/View/onAppear..."
// — host + path with no separator and the module name doubled.
func TestAppleDocsSourceURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "SwiftUI method",
			in:   "doc://com.apple.SwiftUI/documentation/SwiftUI/View/onAppear(perform:)",
			want: "https://developer.apple.com/documentation/swiftui/view/onappear(perform:)",
		},
		{
			name: "UIKit class",
			in:   "doc://com.apple.UIKit/documentation/UIKit/UITableView",
			want: "https://developer.apple.com/documentation/uikit/uitableview",
		},
		{
			name: "Swift module root",
			in:   "doc://com.apple.Swift/documentation/Swift/Void",
			want: "https://developer.apple.com/documentation/swift/void",
		},
		{
			name: "framework root only",
			in:   "doc://com.apple.SwiftUI/documentation/SwiftUI",
			want: "https://developer.apple.com/documentation/swiftui",
		},
		{
			name: "fallback when no doc:// prefix",
			in:   "/documentation/SwiftUI/View",
			want: "https://developer.apple.com/documentation/swiftui/view",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := appleDocsSourceURL(tc.in)
			if got != tc.want {
				t.Errorf("appleDocsSourceURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
