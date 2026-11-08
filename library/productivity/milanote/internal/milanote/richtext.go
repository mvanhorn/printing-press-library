// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Milanote stores note text as a tiptap (ProseMirror) document. This file
// builds those documents from Markdown and renders them back to Markdown or
// plain text.

// Doc wraps block nodes in a document node.
func Doc(blocks ...map[string]any) map[string]any {
	content := make([]any, len(blocks))
	for i, b := range blocks {
		content[i] = b
	}
	return map[string]any{"type": "doc", "content": content}
}

// Paragraph builds a left-aligned paragraph from inline Markdown.
func Paragraph(markdown string) map[string]any {
	return paragraphAligned(markdown, "left")
}

func paragraphAligned(markdown, align string) map[string]any {
	node := map[string]any{"type": "paragraph", "attrs": map[string]any{"key": nodeKey(), "textAlign": align}}
	if inline := Inline(markdown); len(inline) > 0 {
		node["content"] = inline
	}
	return node
}

// Heading builds a heading node. Milanote offers two heading sizes; deeper
// levels are clamped to 2.
func Heading(level int, markdown, align string) map[string]any {
	if level < 1 {
		level = 1
	}
	if level > 2 {
		level = 2
	}
	if align == "" {
		align = "left"
	}
	node := map[string]any{"type": "heading", "attrs": map[string]any{"key": nodeKey(), "textAlign": align, "level": level}}
	if inline := Inline(markdown); len(inline) > 0 {
		node["content"] = inline
	}
	return node
}

// BulletList builds a bullet list with one paragraph per item.
func BulletList(items ...string) map[string]any {
	return map[string]any{"type": "bulletList", "attrs": map[string]any{"key": nodeKey()}, "content": listItems(items)}
}

// OrderedList builds a numbered list starting at start.
func OrderedList(start int, items ...string) map[string]any {
	if start < 1 {
		start = 1
	}
	return map[string]any{"type": "orderedList", "attrs": map[string]any{"key": nodeKey(), "start": start}, "content": listItems(items)}
}

func listItems(items []string) []any {
	out := make([]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{"type": "listItem", "content": []any{Paragraph(it)}})
	}
	return out
}

// Callout builds a callout block with an emoji icon.
func Callout(icon, markdown string) map[string]any {
	if icon == "" {
		icon = "💡"
	}
	return map[string]any{
		"type":    "callout",
		"attrs":   map[string]any{"key": nodeKey(), "icon": icon, "iconCode": IconCode(icon)},
		"content": []any{Paragraph(markdown)},
	}
}

// IconCode renders an emoji as Milanote's iconCode (hex code points joined
// by dashes, e.g. "26A0-FE0F").
func IconCode(icon string) string {
	var parts []string
	for _, r := range icon {
		parts = append(parts, fmt.Sprintf("%04X", r))
	}
	return strings.Join(parts, "-")
}

// TaskListDoc builds the textContent of a TASK_LIST element.
func TaskListDoc(items []Task) map[string]any {
	nodes := make([]any, 0, len(items))
	for _, t := range items {
		id := t.ID
		if id == "" {
			id = NewTaskID()
		}
		nodes = append(nodes, map[string]any{
			"type": "taskItem",
			"attrs": map[string]any{
				"taskId": id, "checked": t.Checked, "dueDate": nil,
				"assignments": []any{}, "labels": []any{}, "reactions": []any{},
				"showAssignments": false, "showDueDate": false,
			},
			"content": []any{Paragraph(t.Text)},
		})
	}
	return map[string]any{
		"type":    "doc",
		"attrs":   map[string]any{"hiddenTaskListTitle": nil},
		"content": []any{map[string]any{"type": "taskList", "content": nodes}},
	}
}

// TaskListContent builds the full content object of a TASK_LIST element.
func TaskListContent(title string, items []Task) map[string]any {
	var t any
	if title != "" {
		t = title
	}
	return map[string]any{"title": t, "showTitle": title != "", "version": "V2", "textContent": TaskListDoc(items)}
}

// SetTaskChecked returns a copy of a task-list document with matching items
// ticked or unticked. match receives each item's index and text.
func SetTaskChecked(doc map[string]any, checked bool, match func(i int, t Task) bool) (map[string]any, int) {
	cp := deepCopy(doc).(map[string]any)
	changed, i := 0, 0
	walk(cp, func(node map[string]any) bool {
		if node["type"] != "taskItem" {
			return true
		}
		attrs, _ := node["attrs"].(map[string]any)
		if attrs == nil {
			attrs = map[string]any{}
			node["attrs"] = attrs
		}
		t := Task{Text: PlainText(node)}
		t.ID, _ = attrs["taskId"].(string)
		t.Checked, _ = attrs["checked"].(bool)
		if match(i, t) && t.Checked != checked {
			attrs["checked"] = checked
			changed++
		}
		i++
		return false
	})
	return cp, changed
}

func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = deepCopy(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = deepCopy(x)
		}
		return out
	}
	return v
}

// --- inline Markdown ---

var inlineRules = []struct {
	re   *regexp.Regexp
	mark string
}{
	{regexp.MustCompile("^`([^`]+)`"), "code"},
	{regexp.MustCompile(`^\[([^\]]+)\]\(([^)\s]+)\)`), "link"},
	{regexp.MustCompile(`^\*\*(.+?)\*\*`), "bold"},
	{regexp.MustCompile(`^__(.+?)__`), "bold"},
	{regexp.MustCompile(`^~~(.+?)~~`), "strike"},
	{regexp.MustCompile(`^\*([^*\s][^*]*?)\*`), "italic"},
	{regexp.MustCompile(`^_([^_\s][^_]*?)_(?:\W|$)`), "italic"},
	{regexp.MustCompile(`^<u>(.+?)</u>`), "underline"},
}

// Inline parses inline Markdown (bold, italic, strike, underline, code,
// links) into tiptap text nodes.
func Inline(s string) []any {
	return inlineWith(s, nil)
}

func inlineWith(s string, marks []map[string]any) []any {
	var out []any
	var plain strings.Builder
	flush := func() {
		if plain.Len() == 0 {
			return
		}
		out = append(out, textNode(plain.String(), marks))
		plain.Reset()
	}
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) && strings.ContainsRune("\\`*_~[]()<>#-+.!", rune(s[i+1])) {
			plain.WriteByte(s[i+1])
			i += 2
			continue
		}
		matched := false
		for _, rule := range inlineRules {
			m := rule.re.FindStringSubmatchIndex(s[i:])
			if m == nil {
				continue
			}
			flush()
			inner := s[i+m[2] : i+m[3]]
			end := m[1]
			if rule.mark == "italic" && strings.HasPrefix(s[i:], "_") {
				end = m[3] + 1
			}
			switch rule.mark {
			case "code":
				out = append(out, textNode(inner, append(cloneMarks(marks), map[string]any{"type": "code"})))
			case "link":
				href := s[i+m[4] : i+m[5]]
				out = append(out, inlineWith(inner, append(cloneMarks(marks), map[string]any{
					"type":  "link",
					"attrs": map[string]any{"href": href, "target": "_blank", "rel": "noopener noreferrer nofollow", "class": nil},
				}))...)
			default:
				out = append(out, inlineWith(inner, append(cloneMarks(marks), map[string]any{"type": rule.mark}))...)
			}
			i += end
			matched = true
			break
		}
		if !matched {
			plain.WriteByte(s[i])
			i++
		}
	}
	flush()
	return out
}

func cloneMarks(m []map[string]any) []map[string]any {
	out := make([]map[string]any, len(m))
	copy(out, m)
	return out
}

func textNode(text string, marks []map[string]any) map[string]any {
	node := map[string]any{"type": "text", "text": text}
	if len(marks) > 0 {
		ms := make([]any, len(marks))
		for i, m := range marks {
			ms[i] = m
		}
		node["marks"] = ms
	}
	return node
}

// --- block Markdown ---

// Block is one top-level Markdown block as the layout engine sees it.
type Block struct {
	Kind  string // heading, paragraph, bullets, numbers, tasks, callout, quote, code, rule, image
	Level int
	Text  string
	Items []string
	Tasks []Task
	Start int
	Icon  string
	URL   string
}

var (
	reHeading = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	reBullet  = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	reNumber  = regexp.MustCompile(`^(\s*)(\d+)[.)]\s+(.*)$`)
	reTask    = regexp.MustCompile(`^\s*[-*+]\s+\[([ xX])\]\s+(.*)$`)
	reRule    = regexp.MustCompile(`^\s*(?:(?:-\s*){3,}|(?:\*\s*){3,}|(?:_\s*){3,})$`)
	reImage   = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)\s]+)\)\s*$`)
	reAlert   = regexp.MustCompile(`^\[!(\w+)\]\s*(.*)$`)
)

var alertIcons = map[string]string{
	"note": "📝", "tip": "💡", "important": "❗", "warning": "⚠️", "caution": "🛑", "info": "ℹ️", "todo": "✅",
}

// ParseBlocks splits Markdown into top-level blocks.
func ParseBlocks(md string) []Block {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []Block
	var para []string
	flushPara := func() {
		if len(para) > 0 {
			out = append(out, Block{Kind: "paragraph", Text: strings.Join(para, " ")})
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			flushPara()
		case strings.HasPrefix(trimmed, "```"):
			flushPara()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			out = append(out, Block{Kind: "code", Items: code})
		case reHeading.MatchString(trimmed):
			flushPara()
			m := reHeading.FindStringSubmatch(trimmed)
			out = append(out, Block{Kind: "heading", Level: len(m[1]), Text: m[2]})
		case reRule.MatchString(trimmed):
			flushPara()
			out = append(out, Block{Kind: "rule"})
		case reImage.MatchString(trimmed):
			flushPara()
			m := reImage.FindStringSubmatch(trimmed)
			out = append(out, Block{Kind: "image", Text: m[1], URL: m[2]})
		case reTask.MatchString(line):
			flushPara()
			var tasks []Task
			for ; i < len(lines) && reTask.MatchString(lines[i]); i++ {
				m := reTask.FindStringSubmatch(lines[i])
				tasks = append(tasks, Task{Text: m[2], Checked: m[1] != " "})
			}
			i--
			out = append(out, Block{Kind: "tasks", Tasks: tasks})
		case reBullet.MatchString(line):
			flushPara()
			var items []string
			for ; i < len(lines) && reBullet.MatchString(lines[i]) && !reTask.MatchString(lines[i]); i++ {
				m := reBullet.FindStringSubmatch(lines[i])
				items = append(items, nestPrefix(m[1])+m[2])
			}
			i--
			out = append(out, Block{Kind: "bullets", Items: items})
		case reNumber.MatchString(line):
			flushPara()
			var items []string
			start := 0
			for ; i < len(lines) && reNumber.MatchString(lines[i]); i++ {
				m := reNumber.FindStringSubmatch(lines[i])
				if start == 0 {
					start, _ = strconv.Atoi(m[2])
				}
				items = append(items, nestPrefix(m[1])+m[3])
			}
			i--
			out = append(out, Block{Kind: "numbers", Items: items, Start: start})
		case strings.HasPrefix(trimmed, ">"):
			flushPara()
			var quote []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				quote = append(quote, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")))
			}
			i--
			text := strings.TrimSpace(strings.Join(quote, " "))
			if m := reAlert.FindStringSubmatch(text); m != nil {
				icon := alertIcons[strings.ToLower(m[1])]
				if icon == "" {
					icon = "💡"
				}
				out = append(out, Block{Kind: "callout", Icon: icon, Text: m[2]})
			} else if r, size := firstRune(text); size > 0 && isEmoji(r) {
				icon := emojiPrefix(text)
				out = append(out, Block{Kind: "callout", Icon: icon, Text: strings.TrimSpace(text[len(icon):])})
			} else {
				out = append(out, Block{Kind: "quote", Text: text})
			}
		default:
			para = append(para, trimmed)
		}
	}
	flushPara()
	return out
}

// nestPrefix marks nested list items with a leading tab so the builder can
// render them one level deeper.
func nestPrefix(indent string) string {
	if len(strings.ReplaceAll(indent, "\t", "  ")) >= 2 {
		return "\t"
	}
	return ""
}

func firstRune(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return 0, 0
}

func isEmoji(r rune) bool {
	return r >= 0x2190 && r <= 0x2BFF || r >= 0x1F000 && r <= 0x1FAFF || r == 0x2139
}

func emojiPrefix(s string) string {
	end := 0
	for i, r := range s {
		if i == 0 || r == 0xFE0F || r == 0x200D || isEmoji(r) || (r >= 0x1F3FB && r <= 0x1F3FF) {
			end = i + len(string(r))
			continue
		}
		break
	}
	return s[:end]
}

// BlockNodes renders blocks as tiptap nodes for a single note.
func BlockNodes(blocks []Block) []map[string]any {
	var out []map[string]any
	for _, b := range blocks {
		switch b.Kind {
		case "heading":
			if b.Level <= 2 {
				out = append(out, Heading(b.Level, b.Text, "left"))
			} else {
				out = append(out, Paragraph("**"+b.Text+"**"))
			}
		case "paragraph":
			out = append(out, Paragraph(b.Text))
		case "bullets":
			out = append(out, nestedList("bulletList", 0, b.Items))
		case "numbers":
			out = append(out, nestedList("orderedList", b.Start, b.Items))
		case "tasks":
			items := make([]string, len(b.Tasks))
			for i, t := range b.Tasks {
				box := "☐ "
				if t.Checked {
					box = "☑ "
				}
				items[i] = box + t.Text
			}
			out = append(out, BulletList(items...))
		case "callout":
			out = append(out, Callout(b.Icon, b.Text))
		case "quote":
			out = append(out, Paragraph("*"+escapeStars(b.Text)+"*"))
		case "code":
			for _, line := range b.Items {
				if strings.TrimSpace(line) == "" {
					out = append(out, Paragraph(""))
					continue
				}
				out = append(out, map[string]any{
					"type":    "paragraph",
					"attrs":   map[string]any{"key": nodeKey(), "textAlign": "left"},
					"content": []any{textNode(line, []map[string]any{{"type": "code"}})},
				})
			}
		case "image":
			out = append(out, Paragraph("["+b.Text+"]("+b.URL+")"))
		case "rule":
			out = append(out, Paragraph(""))
		}
	}
	return out
}

func escapeStars(s string) string { return strings.ReplaceAll(s, "*", `\*`) }

// nestedList builds a list where items prefixed with a tab nest one level
// under the previous item.
func nestedList(kind string, start int, items []string) map[string]any {
	var top []any
	for _, it := range items {
		if strings.HasPrefix(it, "\t") && len(top) > 0 {
			parent := top[len(top)-1].(map[string]any)
			kids := parent["content"].([]any)
			last, _ := kids[len(kids)-1].(map[string]any)
			if last == nil || last["type"] != kind {
				last = map[string]any{"type": kind, "attrs": listAttrs(kind, 1), "content": []any{}}
				kids = append(kids, last)
				parent["content"] = kids
			}
			last["content"] = append(last["content"].([]any), map[string]any{"type": "listItem", "content": []any{Paragraph(strings.TrimPrefix(it, "\t"))}})
			continue
		}
		top = append(top, map[string]any{"type": "listItem", "content": []any{Paragraph(strings.TrimPrefix(it, "\t"))}})
	}
	return map[string]any{"type": kind, "attrs": listAttrs(kind, start), "content": top}
}

func listAttrs(kind string, start int) map[string]any {
	if kind == "orderedList" {
		if start < 1 {
			start = 1
		}
		return map[string]any{"key": nodeKey(), "start": start}
	}
	return map[string]any{"key": nodeKey()}
}

// MarkdownDoc parses Markdown into a full note document.
func MarkdownDoc(md string) map[string]any {
	nodes := BlockNodes(ParseBlocks(md))
	if len(nodes) == 0 {
		nodes = []map[string]any{Paragraph("")}
	}
	return Doc(nodes...)
}

// --- rendering ---

// PlainText flattens a tiptap node (or document) to text, one line per block.
func PlainText(node any) string {
	var b strings.Builder
	plainInto(&b, node)
	return strings.TrimSpace(collapseBlank(b.String()))
}

func plainInto(b *strings.Builder, node any) {
	switch n := node.(type) {
	case string:
		b.WriteString(n)
	case []any:
		for _, k := range n {
			plainInto(b, k)
		}
	case map[string]any:
		switch n["type"] {
		case "text":
			s, _ := n["text"].(string)
			b.WriteString(s)
			return
		case "hardBreak":
			b.WriteString("\n")
			return
		}
		plainInto(b, n["content"])
		switch n["type"] {
		case "paragraph", "heading", "callout", "codeBlock", "blockquote":
			b.WriteString("\n")
		}
	}
}

func collapseBlank(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// ToMarkdown renders a tiptap document back to Markdown.
func ToMarkdown(node any) string {
	var b strings.Builder
	mdBlocks(&b, node, "")
	return strings.TrimSpace(collapseBlank(b.String()))
}

func mdBlocks(b *strings.Builder, node any, indent string) {
	n, ok := node.(map[string]any)
	if !ok {
		if list, ok := node.([]any); ok {
			for _, k := range list {
				mdBlocks(b, k, indent)
			}
		}
		return
	}
	kids, _ := n["content"].([]any)
	attrs, _ := n["attrs"].(map[string]any)
	switch n["type"] {
	case "doc":
		for _, k := range kids {
			mdBlocks(b, k, indent)
		}
	case "heading":
		level := int(num(attrs["level"]))
		if level < 1 {
			level = 1
		}
		b.WriteString(strings.Repeat("#", level) + " " + mdInline(kids) + "\n\n")
	case "paragraph":
		b.WriteString(indent + mdInline(kids) + "\n\n")
	case "callout":
		icon, _ := attrs["icon"].(string)
		var inner strings.Builder
		for _, k := range kids {
			if m, ok := k.(map[string]any); ok {
				inner.WriteString(mdInline(asList(m["content"])) + " ")
			}
		}
		b.WriteString("> " + strings.TrimSpace(icon+" "+strings.TrimSpace(inner.String())) + "\n\n")
	case "blockquote":
		var inner strings.Builder
		mdBlocks(&inner, kids, "")
		for _, line := range strings.Split(strings.TrimSpace(inner.String()), "\n") {
			b.WriteString("> " + line + "\n")
		}
		b.WriteString("\n")
	case "codeBlock":
		b.WriteString("```\n" + PlainText(n) + "\n```\n\n")
	case "bulletList", "orderedList", "taskList":
		start := int(num(attrs["start"]))
		if start < 1 {
			start = 1
		}
		for i, k := range kids {
			item, _ := k.(map[string]any)
			if item == nil {
				continue
			}
			prefix := "- "
			switch n["type"] {
			case "orderedList":
				prefix = strconv.Itoa(start+i) + ". "
			case "taskList":
				ia, _ := item["attrs"].(map[string]any)
				if checked, _ := ia["checked"].(bool); checked {
					prefix = "- [x] "
				} else {
					prefix = "- [ ] "
				}
			}
			first := true
			for _, part := range asList(item["content"]) {
				pm, _ := part.(map[string]any)
				if pm == nil {
					continue
				}
				if pm["type"] == "paragraph" && first {
					b.WriteString(indent + prefix + mdInline(asList(pm["content"])) + "\n")
					first = false
					continue
				}
				mdBlocks(b, pm, indent+"  ")
			}
		}
		if indent == "" {
			b.WriteString("\n")
		}
	default:
		for _, k := range kids {
			mdBlocks(b, k, indent)
		}
	}
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func mdInline(nodes []any) string {
	var b strings.Builder
	for _, k := range nodes {
		n, _ := k.(map[string]any)
		if n == nil {
			continue
		}
		if n["type"] == "hardBreak" {
			b.WriteString("  \n")
			continue
		}
		text, _ := n["text"].(string)
		if text == "" {
			continue
		}
		pre, post := "", ""
		for _, m := range asList(n["marks"]) {
			mm, _ := m.(map[string]any)
			switch mm["type"] {
			case "bold":
				pre, post = pre+"**", "**"+post
			case "italic":
				pre, post = pre+"*", "*"+post
			case "strike":
				pre, post = pre+"~~", "~~"+post
			case "underline":
				pre, post = pre+"<u>", "</u>"+post
			case "code":
				pre, post = pre+"`", "`"+post
			case "link":
				la, _ := mm["attrs"].(map[string]any)
				href, _ := la["href"].(string)
				pre, post = pre+"[", "]("+href+")"+post
			}
		}
		b.WriteString(pre + text + post)
	}
	return b.String()
}
