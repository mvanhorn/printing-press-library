// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"encoding/json"
	"fmt"
	"strings"
)

// tiptapNode is the recursive shape of a TipTap document node. The cache
// stores nodes with camelCase types (bulletList, listItem, orderedList,
// codeBlock, hardBreak) — older TipTap versions used snake_case; we accept
// both.
type tiptapNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []tiptapNode   `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []tiptapMark   `json:"marks,omitempty"`
}

type tiptapMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Render decodes a TipTap-shaped JSON blob and returns canonical
// markdown. Returns ("", nil) for empty input. Unknown node types fall
// through to walking their children; this preserves text content even
// when Granola introduces a new node shape.
func Render(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var root tiptapNode
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", fmt.Errorf("tiptap: %w", err)
	}
	var b strings.Builder
	r := renderer{out: &b}
	r.walk(root, 0, "")
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

type renderer struct {
	out *strings.Builder
}

func (r *renderer) walk(n tiptapNode, depth int, listPrefix string) {
	switch normType(n.Type) {
	case "doc", "":
		for _, c := range n.Content {
			r.walk(c, depth, "")
		}
	case "paragraph":
		// Indent only if we are inside a list (listPrefix non-empty); the
		// caller wraps with the appropriate marker.
		if listPrefix != "" {
			r.out.WriteString(listPrefix)
			listPrefix = ""
		}
		for _, c := range n.Content {
			r.walk(c, depth, listPrefix)
		}
		r.out.WriteString("\n")
	case "heading":
		level := 1
		if l, ok := n.Attrs["level"]; ok {
			switch v := l.(type) {
			case float64:
				level = int(v)
			case int:
				level = v
			}
		}
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		r.out.WriteString(strings.Repeat("#", level))
		r.out.WriteString(" ")
		for _, c := range n.Content {
			r.walk(c, depth, "")
		}
		r.out.WriteString("\n\n")
	case "bulletList":
		for _, c := range n.Content {
			r.walk(c, depth+1, "- ")
		}
		if depth == 0 {
			r.out.WriteString("\n")
		}
	case "orderedList":
		i := 1
		for _, c := range n.Content {
			r.walk(c, depth+1, fmt.Sprintf("%d. ", i))
			i++
		}
		if depth == 0 {
			r.out.WriteString("\n")
		}
	case "listItem":
		// Indent at depth-1 (depth was incremented by parent list).
		indent := strings.Repeat("  ", depth-1)
		if indent != "" {
			r.out.WriteString(indent)
		}
		// Pass listPrefix into the first child so the paragraph emits "- text".
		first := true
		for _, c := range n.Content {
			if first {
				r.walk(c, depth, listPrefix)
				first = false
			} else {
				r.walk(c, depth, "")
			}
		}
	case "text":
		r.out.WriteString(applyMarks(n.Text, n.Marks))
	case "paragraphBreak":
		r.out.WriteString("\n\n")
	case "hardBreak":
		r.out.WriteString("\n")
	case "blockquote":
		// Inline children with "> " prefix on each emitted line.
		var inner strings.Builder
		ir := renderer{out: &inner}
		for _, c := range n.Content {
			ir.walk(c, depth, "")
		}
		for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
			r.out.WriteString("> ")
			r.out.WriteString(line)
			r.out.WriteString("\n")
		}
		r.out.WriteString("\n")
	case "codeBlock":
		lang := ""
		if l, ok := n.Attrs["language"]; ok {
			if s, ok := l.(string); ok {
				lang = s
			}
		}
		r.out.WriteString("```")
		r.out.WriteString(lang)
		r.out.WriteString("\n")
		for _, c := range n.Content {
			if c.Type == "text" {
				r.out.WriteString(c.Text)
			}
		}
		r.out.WriteString("\n```\n\n")
	case "horizontalRule":
		r.out.WriteString("\n---\n\n")
	default:
		// Unknown node — fall through to children.
		for _, c := range n.Content {
			r.walk(c, depth, listPrefix)
		}
	}
}

// normType normalizes TipTap node types. Older docs use snake_case; v6
// uses camelCase. We accept either.
func normType(t string) string {
	switch t {
	case "bullet_list":
		return "bulletList"
	case "ordered_list":
		return "orderedList"
	case "list_item":
		return "listItem"
	case "hard_break":
		return "hardBreak"
	case "paragraph_break":
		return "paragraphBreak"
	case "code_block":
		return "codeBlock"
	case "horizontal_rule":
		return "horizontalRule"
	}
	return t
}

// applyMarks wraps text with TipTap marks. Marks compose in the order
// they appear in the mark list — bold then italic then code then link.
func applyMarks(text string, marks []tiptapMark) string {
	if text == "" {
		return ""
	}
	out := text
	for _, m := range marks {
		switch m.Type {
		case "bold", "strong":
			out = "**" + out + "**"
		case "italic", "em":
			out = "*" + out + "*"
		case "code":
			out = "`" + out + "`"
		case "strike":
			out = "~~" + out + "~~"
		case "link":
			href := ""
			if h, ok := m.Attrs["href"]; ok {
				if s, ok := h.(string); ok {
					href = s
				}
			}
			out = "[" + out + "](" + href + ")"
		}
	}
	return out
}
