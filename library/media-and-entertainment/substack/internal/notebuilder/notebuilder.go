// Package notebuilder converts a small Markdown subset to Substack's
// ProseMirror JSON representation used by the /comment/feed (Notes)
// endpoint. Reference: jakub-k-slys/substack-api/main/src/domain/note-builder.ts.
//
// Supported subset:
//   - Paragraphs separated by a blank line.
//   - Bold via **text** or *text*.
//   - Italic via _text_.
//   - Links via [label](https://example.com).
//   - Hard line breaks via a single newline inside a paragraph (rendered
//     as a hardBreak node).
package notebuilder

import (
	"encoding/json"
	"strings"
)

// BuildProseMirrorJSON converts md to a Substack-shaped ProseMirror doc.
func BuildProseMirrorJSON(md string) ([]byte, error) {
	doc := map[string]any{
		"type":    "doc",
		"content": buildContent(md),
	}
	return json.Marshal(doc)
}

func buildContent(md string) []map[string]any {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	paragraphs := splitParagraphs(md)
	out := make([]map[string]any, 0, len(paragraphs))
	for _, p := range paragraphs {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, map[string]any{
			"type":    "paragraph",
			"content": buildInline(p),
		})
	}
	if len(out) == 0 {
		// Empty doc — Substack tolerates an empty paragraph; emit one so
		// the round-trip stays valid JSON shape rather than nil content.
		out = append(out, map[string]any{
			"type":    "paragraph",
			"content": []map[string]any{},
		})
	}
	return out
}

// splitParagraphs splits md on blank lines.
func splitParagraphs(md string) []string {
	var out []string
	var cur strings.Builder
	for _, line := range strings.Split(md, "\n") {
		if strings.TrimSpace(line) == "" {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// buildInline scans the paragraph text and emits a slice of text/link
// nodes with bold/italic marks. Each newline inside a paragraph turns
// into a hardBreak node so multi-line stanzas render correctly.
func buildInline(p string) []map[string]any {
	var nodes []map[string]any

	// Split on newlines to insert hardBreak between lines.
	lines := strings.Split(p, "\n")
	for i, line := range lines {
		nodes = append(nodes, parseLine(line)...)
		if i < len(lines)-1 {
			nodes = append(nodes, map[string]any{"type": "hardBreak"})
		}
	}
	return nodes
}

// parseLine handles a single line's worth of inline markdown.
func parseLine(line string) []map[string]any {
	var out []map[string]any
	i := 0
	for i < len(line) {
		// Try a link first: [label](url)
		if line[i] == '[' {
			if node, consumed, ok := tryLink(line[i:]); ok {
				out = append(out, node)
				i += consumed
				continue
			}
		}
		// Bold: **...** or __...__
		if i+1 < len(line) && (line[i:i+2] == "**" || line[i:i+2] == "__") {
			marker := line[i : i+2]
			end := strings.Index(line[i+2:], marker)
			if end >= 0 {
				out = append(out, textNode(line[i+2:i+2+end], "strong"))
				i = i + 2 + end + 2
				continue
			}
		}
		// Italic: *...* or _..._
		if line[i] == '*' || line[i] == '_' {
			marker := string(line[i])
			end := strings.Index(line[i+1:], marker)
			if end >= 0 {
				inner := line[i+1 : i+1+end]
				if inner != "" && !strings.ContainsAny(inner, "*_") {
					out = append(out, textNode(inner, "em"))
					i = i + 1 + end + 1
					continue
				}
			}
		}
		// Plain run until next markdown sentinel.
		j := i
		for j < len(line) {
			c := line[j]
			if c == '*' || c == '_' || c == '[' {
				break
			}
			j++
		}
		if j == i {
			// No progress — emit a single char to avoid infinite loop.
			out = append(out, plainText(string(line[i])))
			i++
			continue
		}
		out = append(out, plainText(line[i:j]))
		i = j
	}
	return out
}

// tryLink parses [label](url) starting at s[0]=='['. Returns the link
// node, bytes consumed, and ok flag.
func tryLink(s string) (map[string]any, int, bool) {
	if len(s) < 4 || s[0] != '[' {
		return nil, 0, false
	}
	closeBracket := strings.IndexByte(s, ']')
	if closeBracket < 0 || closeBracket+1 >= len(s) || s[closeBracket+1] != '(' {
		return nil, 0, false
	}
	closeParen := strings.IndexByte(s[closeBracket+2:], ')')
	if closeParen < 0 {
		return nil, 0, false
	}
	label := s[1:closeBracket]
	url := s[closeBracket+2 : closeBracket+2+closeParen]
	node := map[string]any{
		"type": "text",
		"text": label,
		"marks": []map[string]any{{
			"type":  "link",
			"attrs": map[string]any{"href": url},
		}},
	}
	return node, closeBracket + 2 + closeParen + 1, true
}

func plainText(s string) map[string]any {
	return map[string]any{"type": "text", "text": s}
}

func textNode(s, mark string) map[string]any {
	return map[string]any{
		"type":  "text",
		"text":  s,
		"marks": []map[string]any{{"type": mark}},
	}
}
