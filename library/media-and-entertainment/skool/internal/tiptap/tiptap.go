// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

// Package tiptap converts a deliberately small subset of Markdown into the
// TipTap ProseMirror document JSON that Skool stores as a post, comment, or
// classroom page body.
//
// The subset is the contract, not a stepping stone: paragraphs, bold, ordered
// lists, bullet lists, links, and headings clamped to level 3. Skool's own
// editor emits no heading above H3 inside a lesson body, so a document that
// round-trips through this package is a document Skool's editor can re-open
// without silently rewriting it. Anything outside the subset is carried through
// as literal text rather than guessed at — a wrong node type is rejected by the
// API with no indication of which block was at fault, so staying literal keeps
// the failure legible.
package tiptap

import (
	"encoding/json"
	"strconv"
	"strings"
)

// MaxHeadingLevel is the deepest heading Skool's lesson editor round-trips.
// Markdown H1 and H2 are clamped up to it rather than dropped, so an imported
// document keeps its section breaks.
const MaxHeadingLevel = 3

// Mark is a TipTap inline mark (bold, link).
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Node is a TipTap document node. The zero value is not a valid document;
// build one with FromMarkdown.
type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
	Text    string         `json:"text,omitempty"`
}

// FromMarkdown parses src and returns the TipTap `doc` node. An empty or
// whitespace-only input yields a document holding one empty paragraph, because
// TipTap rejects a doc with no content.
func FromMarkdown(src string) Node {
	lines := splitLines(src)
	content := make([]Node, 0, len(lines))

	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			i++
			continue
		}

		if level, text, ok := parseHeading(trimmed); ok {
			content = append(content, Node{
				Type:    "heading",
				Attrs:   map[string]any{"level": level},
				Content: inline(text, nil),
			})
			i++
			continue
		}

		if _, ok := parseBullet(trimmed); ok {
			var items []Node
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				text, ok := parseBullet(t)
				if !ok {
					break
				}
				items = append(items, listItem(text))
				i++
			}
			content = append(content, Node{Type: "bulletList", Content: items})
			continue
		}

		if start, _, ok := parseOrdered(trimmed); ok {
			var items []Node
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				_, text, ok := parseOrdered(t)
				if !ok {
					break
				}
				items = append(items, listItem(text))
				i++
			}
			node := Node{Type: "orderedList", Content: items}
			// TipTap defaults start to 1 and omits the attr; only a list that
			// actually begins elsewhere needs it recorded.
			if start != 1 {
				node.Attrs = map[string]any{"start": start}
			}
			content = append(content, node)
			continue
		}

		// Paragraph: consume the soft-wrapped run up to the next blank line or
		// block opener, joining with spaces the way Markdown renderers do.
		var para []string
		for i < len(lines) {
			t := strings.TrimSpace(lines[i])
			if t == "" || isBlockOpener(t) {
				break
			}
			para = append(para, t)
			i++
		}
		content = append(content, Node{
			Type:    "paragraph",
			Content: inline(strings.Join(para, " "), nil),
		})
	}

	if len(content) == 0 {
		content = []Node{{Type: "paragraph"}}
	}
	return Node{Type: "doc", Content: content}
}

// JSONFromMarkdown returns the document as a compact JSON string. Skool's
// create/update payloads carry the document as a JSON *string* in the
// `content` field, not as a nested object, so this is what command flags want.
func JSONFromMarkdown(src string) (string, error) {
	b, err := json.Marshal(FromMarkdown(src))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func listItem(text string) Node {
	return Node{
		Type:    "listItem",
		Content: []Node{{Type: "paragraph", Content: inline(text, nil)}},
	}
}

func splitLines(src string) []string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	return strings.Split(src, "\n")
}

func isBlockOpener(trimmed string) bool {
	if _, _, ok := parseHeading(trimmed); ok {
		return true
	}
	if _, ok := parseBullet(trimmed); ok {
		return true
	}
	if _, _, ok := parseOrdered(trimmed); ok {
		return true
	}
	return false
}

// parseHeading recognises ATX headings and clamps the level to
// MaxHeadingLevel. The clamp runs in both directions: H1 and H2 are as far
// outside the lesson-body subset as H4 and below, so every recognised heading
// lands on exactly MaxHeadingLevel. The text keeps its heading node, so an
// imported document keeps its section breaks.
func parseHeading(trimmed string) (level int, text string, ok bool) {
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(trimmed) || trimmed[n] != ' ' {
		return 0, "", false
	}
	return MaxHeadingLevel, strings.TrimSpace(trimmed[n+1:]), true
}

func parseBullet(trimmed string) (text string, ok bool) {
	if len(trimmed) < 2 {
		return "", false
	}
	switch trimmed[0] {
	case '-', '*', '+':
	default:
		return "", false
	}
	if trimmed[1] != ' ' {
		return "", false
	}
	return strings.TrimSpace(trimmed[2:]), true
}

func parseOrdered(trimmed string) (number int, text string, ok bool) {
	n := 0
	for n < len(trimmed) && trimmed[n] >= '0' && trimmed[n] <= '9' {
		n++
	}
	if n == 0 || n+1 >= len(trimmed) {
		return 0, "", false
	}
	if trimmed[n] != '.' && trimmed[n] != ')' {
		return 0, "", false
	}
	if trimmed[n+1] != ' ' {
		return 0, "", false
	}
	num, err := strconv.Atoi(trimmed[:n])
	if err != nil {
		return 0, "", false
	}
	return num, strings.TrimSpace(trimmed[n+2:]), true
}

// inline walks s and emits text nodes carrying the accumulated marks. It
// recurses for bold spans and links so bold-inside-link and link-inside-bold
// both land as a single text node with two marks, which is how TipTap models
// overlapping inline formatting.
func inline(s string, marks []Mark) []Node {
	var out []Node
	var lit strings.Builder

	flush := func() {
		if lit.Len() == 0 {
			return
		}
		out = append(out, textNode(lit.String(), marks))
		lit.Reset()
	}

	for i := 0; i < len(s); {
		// Backslash escape: the next rune is literal.
		if s[i] == '\\' && i+1 < len(s) {
			lit.WriteByte(s[i+1])
			i += 2
			continue
		}

		// Code span: outside the subset, so the whole span — backticks
		// included — stays literal, and nothing inside it is read as bold
		// or as a link.
		if s[i] == '`' {
			end := codeSpanEnd(s, i)
			lit.WriteString(s[i:end])
			i = end
			continue
		}

		// Image: also outside the subset. Without this the "[alt](src)" tail
		// would be picked up as a link and the image would turn into one.
		if s[i] == '!' && i+1 < len(s) && s[i+1] == '[' {
			if _, _, next, ok := scanLink(s, i+1); ok {
				lit.WriteString(s[i:next])
				i = next
				continue
			}
		}

		if delim := boldDelim(s, i); delim != "" {
			if inner, next, ok := scanDelimited(s, i, delim); ok {
				flush()
				out = append(out, inline(inner, appendMark(marks, Mark{Type: "bold"}))...)
				i = next
				continue
			}
		}

		if s[i] == '[' {
			if label, href, next, ok := scanLink(s, i); ok {
				flush()
				out = append(out, inline(label, appendMark(marks, Mark{
					Type:  "link",
					Attrs: map[string]any{"href": href},
				}))...)
				i = next
				continue
			}
		}

		lit.WriteByte(s[i])
		i++
	}
	flush()
	return out
}

func textNode(text string, marks []Mark) Node {
	n := Node{Type: "text", Text: text}
	if len(marks) > 0 {
		n.Marks = append([]Mark(nil), marks...)
	}
	return n
}

// appendMark copies before appending so sibling spans never share the slice's
// backing array — without the copy, two bold runs in one paragraph can
// overwrite each other's marks.
func appendMark(marks []Mark, m Mark) []Mark {
	out := make([]Mark, 0, len(marks)+1)
	out = append(out, marks...)
	return append(out, m)
}

func boldDelim(s string, i int) string {
	if strings.HasPrefix(s[i:], "**") {
		return "**"
	}
	if strings.HasPrefix(s[i:], "__") {
		return "__"
	}
	return ""
}

// scanDelimited finds the closing delim for the span opening at i. An unclosed
// delimiter reports !ok so the caller keeps it as literal text.
func scanDelimited(s string, i int, delim string) (inner string, next int, ok bool) {
	rest := s[i+len(delim):]
	end := strings.Index(rest, delim)
	if end <= 0 {
		return "", 0, false
	}
	return rest[:end], i + len(delim) + end + len(delim), true
}

// codeSpanEnd returns the index just past the code span opening at i. A span
// opened by a run of n backticks closes at the next run of exactly n. With no
// closing run, only the opening backticks are consumed, so they stay literal
// and the rest of the line is parsed normally.
func codeSpanEnd(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	for j := i + n; j < len(s); {
		if s[j] != '`' {
			j++
			continue
		}
		run := 0
		for j+run < len(s) && s[j+run] == '`' {
			run++
		}
		if run == n {
			return j + run
		}
		j += run
	}
	return i + n
}

// scanLink parses [label](href) starting at i. The label may itself contain
// bold spans; it may not contain a nested link, which Markdown also forbids.
//
// The destination ends at the ")" that balances the opening one, so a URL with
// its own parentheses — https://example.com/a_(b) — survives whole. A
// backslash-escaped parenthesis is part of the URL and does not count toward
// the balance.
func scanLink(s string, i int) (label, href string, next int, ok bool) {
	closeIdx := strings.Index(s[i:], "]")
	if closeIdx < 0 {
		return "", "", 0, false
	}
	closeIdx += i
	if closeIdx+1 >= len(s) || s[closeIdx+1] != '(' {
		return "", "", 0, false
	}
	var dest strings.Builder
	depth := 0
	end := -1
scan:
	for j := closeIdx + 2; j < len(s); j++ {
		switch c := s[j]; {
		case c == '\\' && j+1 < len(s) && (s[j+1] == '(' || s[j+1] == ')'):
			dest.WriteByte(s[j+1])
			j++
		case c == '(':
			depth++
			dest.WriteByte(c)
		case c == ')' && depth == 0:
			end = j
			break scan
		case c == ')':
			depth--
			dest.WriteByte(c)
		default:
			dest.WriteByte(c)
		}
	}
	if end < 0 {
		return "", "", 0, false
	}
	label = s[i+1 : closeIdx]
	href = strings.TrimSpace(dest.String())
	if label == "" || href == "" {
		return "", "", 0, false
	}
	return label, href, end + 1, true
}
