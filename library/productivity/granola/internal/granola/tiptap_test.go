// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRender_Heading(t *testing.T) {
	in := `{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Hi"}]}]}`
	got, err := Render(json.RawMessage(in))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(got, "## Hi") {
		t.Errorf("expected `## Hi` prefix, got %q", got)
	}
}

func TestRender_BulletList(t *testing.T) {
	in := `{"type":"doc","content":[{"type":"bulletList","content":[
		{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"alpha"}]}]},
		{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"beta"}]}]}
	]}]}`
	got, err := Render(json.RawMessage(in))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "- alpha") || !strings.Contains(got, "- beta") {
		t.Errorf("expected `- alpha` and `- beta`, got %q", got)
	}
}

func TestRender_Marks(t *testing.T) {
	in := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"plain "},
		{"type":"text","text":"bold","marks":[{"type":"bold"}]},
		{"type":"text","text":" then "},
		{"type":"text","text":"linked","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}
	]}]}`
	got, err := Render(json.RawMessage(in))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "**bold**") {
		t.Errorf("missing bold: %q", got)
	}
	if !strings.Contains(got, "[linked](https://example.com)") {
		t.Errorf("missing link: %q", got)
	}
}

func TestRender_OrderedList(t *testing.T) {
	in := `{"type":"doc","content":[{"type":"orderedList","content":[
		{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]}]},
		{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"second"}]}]}
	]}]}`
	got, err := Render(json.RawMessage(in))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "1. first") || !strings.Contains(got, "2. second") {
		t.Errorf("expected ordered list, got %q", got)
	}
}

func TestRender_CodeBlock(t *testing.T) {
	in := `{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"package main"}]}]}`
	got, err := Render(json.RawMessage(in))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "```go") || !strings.Contains(got, "package main") {
		t.Errorf("expected fenced code block, got %q", got)
	}
}

func TestRender_Empty(t *testing.T) {
	got, err := Render(nil)
	if err != nil {
		t.Fatalf("Render(nil): %v", err)
	}
	if got != "" {
		t.Errorf("expected empty for nil input, got %q", got)
	}
}
