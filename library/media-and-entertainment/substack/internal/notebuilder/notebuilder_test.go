package notebuilder

import (
	"encoding/json"
	"strings"
	"testing"
)

func parseDoc(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, string(raw))
	}
	if doc["type"] != "doc" {
		t.Fatalf("expected type=doc, got %v", doc["type"])
	}
	return doc
}

func TestPlainText(t *testing.T) {
	raw, err := BuildProseMirrorJSON("hello world")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseDoc(t, raw)
	content := doc["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("want 1 paragraph, got %d", len(content))
	}
	p := content[0].(map[string]any)
	inner := p["content"].([]any)
	if len(inner) != 1 {
		t.Fatalf("want 1 inline node, got %d", len(inner))
	}
	n := inner[0].(map[string]any)
	if n["type"] != "text" || n["text"] != "hello world" {
		t.Fatalf("unexpected node: %+v", n)
	}
}

func TestBold(t *testing.T) {
	raw, _ := BuildProseMirrorJSON("hello **bold** world")
	if !strings.Contains(string(raw), `"strong"`) {
		t.Fatalf("expected strong mark in: %s", raw)
	}
	if !strings.Contains(string(raw), `"bold"`) {
		t.Fatalf("expected bold text in: %s", raw)
	}
}

func TestItalic(t *testing.T) {
	raw, _ := BuildProseMirrorJSON("a _slanted_ phrase")
	if !strings.Contains(string(raw), `"em"`) {
		t.Fatalf("expected em mark in: %s", raw)
	}
	if !strings.Contains(string(raw), `"slanted"`) {
		t.Fatalf("expected italic content text in: %s", raw)
	}
}

func TestLink(t *testing.T) {
	raw, _ := BuildProseMirrorJSON("see [the docs](https://example.com/x) here")
	s := string(raw)
	if !strings.Contains(s, `"link"`) {
		t.Fatalf("expected link mark in: %s", s)
	}
	if !strings.Contains(s, `"https://example.com/x"`) {
		t.Fatalf("expected link href in: %s", s)
	}
	if !strings.Contains(s, `"the docs"`) {
		t.Fatalf("expected link label in: %s", s)
	}
}

func TestMultipleParagraphs(t *testing.T) {
	raw, _ := BuildProseMirrorJSON("first paragraph.\n\nsecond paragraph.")
	doc := parseDoc(t, raw)
	content := doc["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("want 2 paragraphs, got %d", len(content))
	}
}

func TestMixed(t *testing.T) {
	raw, _ := BuildProseMirrorJSON("a **bold** and _italic_ and [link](https://x.io) all together")
	s := string(raw)
	for _, want := range []string{`"strong"`, `"em"`, `"link"`, `"bold"`, `"italic"`, `"https://x.io"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in output: %s", want, s)
		}
	}
}

func TestHardBreak(t *testing.T) {
	raw, _ := BuildProseMirrorJSON("line one\nline two")
	s := string(raw)
	if !strings.Contains(s, `"hardBreak"`) {
		t.Fatalf("expected hardBreak in: %s", s)
	}
}

func TestEmptyInput(t *testing.T) {
	raw, err := BuildProseMirrorJSON("")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseDoc(t, raw)
	content := doc["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("want 1 (empty) paragraph, got %d", len(content))
	}
}
