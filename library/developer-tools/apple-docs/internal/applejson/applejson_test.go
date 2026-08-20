// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

package applejson

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDocPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"swiftui", "/tutorials/data/documentation/swiftui.json"},
		{"SwiftUI", "/tutorials/data/documentation/swiftui.json"},
		{"/swiftui/view/", "/tutorials/data/documentation/swiftui/view.json"},
		{"documentation/swiftui/view", "/tutorials/data/documentation/swiftui/view.json"},
		{"tutorials/data/documentation/foundation", "/tutorials/data/documentation/foundation.json"},
		{"swiftui/view/onappear(perform:)", "/tutorials/data/documentation/swiftui/view/onappear(perform:).json"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := DocPath(tc.in)
			if got != tc.want {
				t.Fatalf("DocPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseDoc(t *testing.T) {
	raw := []byte(`{
		"identifier": {"url": "doc://com.apple.SwiftUI/documentation/SwiftUI/View", "interfaceLanguage": "swift"},
		"kind": "symbol",
		"metadata": {
			"title": "View",
			"role": "symbol",
			"symbolKind": "protocol",
			"modules": [{"name": "SwiftUI"}],
			"platforms": [
				{"name": "iOS", "introducedAt": "13.0"},
				{"name": "macOS", "introducedAt": "10.15"},
				{"name": "visionOS", "introducedAt": "1.0"}
			]
		},
		"abstract": [
			{"type": "text", "text": "A type that represents part of your app's user interface and "},
			{"type": "text", "text": "provides modifiers that you use to configure views."}
		],
		"primaryContentSections": [
			{
				"kind": "declarations",
				"declarations": [
					{"tokens": [
						{"kind": "keyword", "text": "protocol"},
						{"kind": "text", "text": " "},
						{"kind": "identifier", "text": "View"}
					]}
				]
			}
		],
		"references": {
			"doc://com.apple.SwiftUI/documentation/SwiftUI/Text": {
				"identifier": "doc://com.apple.SwiftUI/documentation/SwiftUI/Text",
				"title": "Text",
				"kind": "symbol",
				"url": "/documentation/swiftui/text"
			}
		}
	}`)
	page, err := ParseDoc(raw)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if page.Title != "View" {
		t.Fatalf("Title = %q, want View", page.Title)
	}
	if page.SymbolKind != "protocol" {
		t.Fatalf("SymbolKind = %q, want protocol", page.SymbolKind)
	}
	if !strings.Contains(page.Abstract, "user interface") {
		t.Fatalf("Abstract missing expected text: %q", page.Abstract)
	}
	if got := page.Declaration; !strings.Contains(got, "protocol View") {
		t.Fatalf("Declaration = %q, want to contain 'protocol View'", got)
	}
	if len(page.Platforms) != 3 {
		t.Fatalf("Platforms len = %d, want 3", len(page.Platforms))
	}
	if !page.IsAvailableOn("iOS") {
		t.Errorf("expected IsAvailableOn(iOS) true")
	}
	if !page.IsAvailableOn("visionOS") {
		t.Errorf("expected IsAvailableOn(visionOS) true")
	}
	if page.IsAvailableOn("linux") {
		t.Errorf("expected IsAvailableOn(linux) false")
	}
}

func TestParseDocDeprecated(t *testing.T) {
	raw := []byte(`{
		"identifier": {"url": "doc://com.apple.UIKit/documentation/UIKit/UITableView"},
		"metadata": {
			"title": "UITableView",
			"platforms": [
				{"name": "iOS", "introducedAt": "2.0", "deprecatedAt": "18.0", "deprecated": true},
				{"name": "visionOS", "unavailable": true}
			]
		}
	}`)
	page, err := ParseDoc(raw)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if !page.IsDeprecatedOn("iOS") {
		t.Errorf("expected IsDeprecatedOn(iOS) true")
	}
	if page.IsAvailableOn("iOS") {
		t.Errorf("expected IsAvailableOn(iOS) false (deprecated)")
	}
	if page.IsAvailableOn("visionOS") {
		t.Errorf("expected IsAvailableOn(visionOS) false (unavailable)")
	}
}

func TestPathStem(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/documentation/swiftui/view/onappear(perform:)", "onappear"},
		{"/documentation/swiftui/view", "view"},
		{"/documentation/swiftui/", "swiftui"},
		{"/documentation/foundation/url/init(string:)", "init"},
	}
	for _, tc := range cases {
		if got := PathStem(tc.in); got != tc.want {
			t.Errorf("PathStem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLevenshteinClose(t *testing.T) {
	cases := []struct {
		a, b string
		max  int
		want bool
	}{
		{"navigationview", "navigationstack", 5, true},
		{"foo", "foo", 0, true},
		{"abc", "xyz", 1, false},
		{"onchange", "onchanged", 1, true},
	}
	for _, tc := range cases {
		got := LevenshteinClose(tc.a, tc.b, tc.max)
		if got != tc.want {
			t.Errorf("LevenshteinClose(%q,%q,%d) = %v, want %v", tc.a, tc.b, tc.max, got, tc.want)
		}
	}
}

func TestWalkSwift(t *testing.T) {
	raw := []byte(`{
		"interfaceLanguages": {
			"swift": [
				{"title": "Root", "type": "module", "path": "/documentation/swiftui", "children": [
					{"title": "Essentials", "type": "groupMarker"},
					{"title": "View", "type": "protocol", "path": "/documentation/swiftui/view", "children": [
						{"title": "onAppear", "type": "method", "path": "/documentation/swiftui/view/onappear", "deprecated": true}
					]}
				]}
			]
		}
	}`)
	idx, err := ParseIndex(raw)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	var titles []string
	var deprecated int
	idx.WalkSwift(func(n *IndexNode) {
		titles = append(titles, n.Title)
		if n.Deprecated {
			deprecated++
		}
	})
	wantTitles := []string{"Root", "View", "onAppear"}
	if len(titles) != len(wantTitles) {
		t.Fatalf("titles = %v, want %v", titles, wantTitles)
	}
	for i, want := range wantTitles {
		if titles[i] != want {
			t.Fatalf("titles[%d] = %q, want %q", i, titles[i], want)
		}
	}
	if deprecated != 1 {
		t.Fatalf("deprecated count = %d, want 1", deprecated)
	}
}

func TestExtractDeclarationMultipleSections(t *testing.T) {
	raw := []byte(`[
		{"kind": "content"},
		{"kind": "declarations", "declarations": [
			{"tokens": [{"text": "func "}, {"text": "onAppear"}, {"text": "(perform: () -> Void)"}]}
		]}
	]`)
	var sections []json.RawMessage
	if err := json.Unmarshal(raw, &sections); err != nil {
		t.Fatal(err)
	}
	got := ExtractDeclaration(sections)
	if !strings.Contains(got, "onAppear") {
		t.Fatalf("ExtractDeclaration = %q, want to contain 'onAppear'", got)
	}
}
