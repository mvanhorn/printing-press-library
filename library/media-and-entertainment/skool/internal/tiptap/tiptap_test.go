// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package tiptap

import (
	"encoding/json"
	"strings"
	"testing"
)

// compactJSON round-trips through a map so key order stops mattering.
func compactJSON(t *testing.T, s string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("test fixture is not valid JSON: %v\n%s", err, s)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("remarshaling fixture: %v", err)
	}
	return string(b)
}

func gotJSON(t *testing.T, md string) string {
	t.Helper()
	s, err := JSONFromMarkdown(md)
	if err != nil {
		t.Fatalf("JSONFromMarkdown(%q): %v", md, err)
	}
	return compactJSON(t, s)
}

func TestFromMarkdownCoversTheDeclaredSubset(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			name: "plain paragraph",
			md:   "Hello there.",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello there."}]}]}`,
		},
		{
			name: "two paragraphs split on the blank line",
			md:   "First.\n\nSecond.",
			want: `{"type":"doc","content":[
				{"type":"paragraph","content":[{"type":"text","text":"First."}]},
				{"type":"paragraph","content":[{"type":"text","text":"Second."}]}]}`,
		},
		{
			name: "soft-wrapped lines join into one paragraph",
			md:   "A sentence that\nwraps in the source.",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[
				{"type":"text","text":"A sentence that wraps in the source."}]}]}`,
		},
		{
			name: "asterisk bold",
			md:   "the **trend** beats the day",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[
				{"type":"text","text":"the "},
				{"type":"text","text":"trend","marks":[{"type":"bold"}]},
				{"type":"text","text":" beats the day"}]}]}`,
		},
		{
			name: "underscore bold",
			md:   "__load__ first",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[
				{"type":"text","text":"load","marks":[{"type":"bold"}]},
				{"type":"text","text":" first"}]}]}`,
		},
		{
			name: "two bold runs in one paragraph keep separate marks",
			md:   "**a** plain **b**",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[
				{"type":"text","text":"a","marks":[{"type":"bold"}]},
				{"type":"text","text":" plain "},
				{"type":"text","text":"b","marks":[{"type":"bold"}]}]}]}`,
		},
		{
			name: "link",
			md:   "see [the chart](https://example.com/c) now",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[
				{"type":"text","text":"see "},
				{"type":"text","text":"the chart","marks":[{"type":"link","attrs":{"href":"https://example.com/c"}}]},
				{"type":"text","text":" now"}]}]}`,
		},
		{
			name: "bold inside a link label carries both marks",
			md:   "[**go**](https://example.com)",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[
				{"type":"text","text":"go","marks":[
					{"type":"link","attrs":{"href":"https://example.com"}},
					{"type":"bold"}]}]}]}`,
		},
		{
			name: "bullet list",
			md:   "- one\n- two",
			want: `{"type":"doc","content":[{"type":"bulletList","content":[
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]},
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"two"}]}]}]}]}`,
		},
		{
			name: "ordered list starting at one omits the start attr",
			md:   "1. first\n2. second",
			want: `{"type":"doc","content":[{"type":"orderedList","content":[
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]}]},
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"second"}]}]}]}]}`,
		},
		{
			name: "ordered list starting elsewhere records start",
			md:   "3. third\n4. fourth",
			want: `{"type":"doc","content":[{"type":"orderedList","attrs":{"start":3},"content":[
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"third"}]}]},
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"fourth"}]}]}]}]}`,
		},
		{
			name: "bold inside a list item",
			md:   "- the **trend**",
			want: `{"type":"doc","content":[{"type":"bulletList","content":[
				{"type":"listItem","content":[{"type":"paragraph","content":[
					{"type":"text","text":"the "},
					{"type":"text","text":"trend","marks":[{"type":"bold"}]}]}]}]}]}`,
		},
		{
			name: "h3 stays h3",
			md:   "### Reading the signals",
			want: `{"type":"doc","content":[{"type":"heading","attrs":{"level":3},
				"content":[{"type":"text","text":"Reading the signals"}]}]}`,
		},
		{
			name: "list directly after a paragraph still opens a list",
			md:   "Why it matters:\n- trend\n- load",
			want: `{"type":"doc","content":[
				{"type":"paragraph","content":[{"type":"text","text":"Why it matters:"}]},
				{"type":"bulletList","content":[
					{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"trend"}]}]},
					{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"load"}]}]}]}]}`,
		},
		{
			name: "empty input yields one empty paragraph",
			md:   "   \n\n",
			want: `{"type":"doc","content":[{"type":"paragraph"}]}`,
		},
		{
			name: "unclosed bold stays literal",
			md:   "a **b",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a **b"}]}]}`,
		},
		{
			name: "escaped asterisks stay literal",
			md:   `a \*\*b\*\* c`,
			want: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a **b** c"}]}]}`,
		},
		{
			name: "bare bracket text is not a link",
			md:   "see [TODO] later",
			want: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"see [TODO] later"}]}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := gotJSON(t, tc.md)
			want := compactJSON(t, tc.want)
			if got != want {
				t.Errorf("FromMarkdown(%q)\n got: %s\nwant: %s", tc.md, got, want)
			}
		})
	}
}

// Skool's lesson editor has no heading deeper than H3. H1 and H2 are clamped
// up to it so section breaks survive an import instead of being dropped.
func TestHeadingsClampToH3(t *testing.T) {
	for _, md := range []string{"# One", "## Two", "### Three", "#### Four", "###### Six"} {
		doc := FromMarkdown(md)
		if len(doc.Content) != 1 || doc.Content[0].Type != "heading" {
			t.Fatalf("FromMarkdown(%q) did not produce a single heading: %+v", md, doc.Content)
		}
		level, ok := doc.Content[0].Attrs["level"].(int)
		if !ok {
			t.Fatalf("FromMarkdown(%q) heading level is not an int: %#v", md, doc.Content[0].Attrs["level"])
		}
		// Exactly MaxHeadingLevel, not merely within range: the lesson-body
		// subset is "H3 at most", so an H1 that survives as level 1 is a
		// heading Skool's editor rewrites on open.
		if level != MaxHeadingLevel {
			t.Errorf("FromMarkdown(%q) level = %d, want %d", md, level, MaxHeadingLevel)
		}
	}
}

// Seven hashes is not a heading in Markdown, so it must stay paragraph text
// rather than being clamped into one.
func TestSevenHashesIsNotAHeading(t *testing.T) {
	doc := FromMarkdown("####### nope")
	if doc.Content[0].Type != "paragraph" {
		t.Errorf("seven hashes produced %q, want paragraph", doc.Content[0].Type)
	}
}

// Constructs outside the declared subset must not silently become the wrong
// node type — they pass through as paragraph text so the failure is visible in
// the output rather than rejected by the API with no indication of the cause.
func TestUnsupportedConstructsPassThroughAsText(t *testing.T) {
	for _, md := range []string{"> quote", "    indented code", "![img](https://e.com/i.png)", "---"} {
		doc := FromMarkdown(md)
		if len(doc.Content) == 0 {
			t.Fatalf("FromMarkdown(%q) dropped the block entirely", md)
		}
		if got := doc.Content[0].Type; got != "paragraph" && got != "bulletList" {
			t.Errorf("FromMarkdown(%q) first node = %q, want paragraph", md, got)
		}
	}
}

func TestJSONFromMarkdownIsAJSONString(t *testing.T) {
	s, err := JSONFromMarkdown("**hi**")
	if err != nil {
		t.Fatalf("JSONFromMarkdown: %v", err)
	}
	if !strings.HasPrefix(s, `{"type":"doc"`) {
		t.Errorf("JSONFromMarkdown = %q, want a doc node", s)
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(s), &probe); err != nil {
		t.Errorf("JSONFromMarkdown output does not parse: %v", err)
	}
}
