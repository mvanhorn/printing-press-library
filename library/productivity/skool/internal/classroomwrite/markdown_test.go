package classroomwrite

import (
	"strings"
	"testing"
)

func TestMarkdownToLessonDesc(t *testing.T) {
	md := "# Title\n\nHello **world**.\n\n## Section\n\n- item one"
	desc, err := MarkdownToLessonDesc(md)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(desc, "[v2]") {
		t.Fatalf("expected [v2] prefix, got %q", desc[:10])
	}
	if !strings.Contains(desc, "Hello") {
		t.Fatal("missing body text")
	}
	if strings.Contains(desc, "**") {
		t.Fatal("literal bold markers must not remain in TipTap JSON")
	}
	if !strings.Contains(desc, `"type":"bold"`) && !strings.Contains(desc, `"type": "bold"`) {
		t.Fatal("expected bold marks in output")
	}
}

func TestMarkdownCodeFence(t *testing.T) {
	md := "Intro\n\n```\nline one\nline two\n```\n\nOutro"
	desc, err := MarkdownToLessonDesc(md)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(desc, "```") {
		t.Fatal("code fences must not remain")
	}
	if !strings.Contains(desc, "line one") || !strings.Contains(desc, "line two") {
		t.Fatal("code fence lines missing")
	}
}

func TestStripFrontmatter(t *testing.T) {
	raw := "---\ntitle: x\n---\n\nBody text"
	body, err := StripFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	if body != "Body text" {
		t.Fatalf("got %q", body)
	}
}
