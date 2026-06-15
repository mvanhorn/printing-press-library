package classroomwrite

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	inlineBoldRe = regexp.MustCompile(`\*\*(.+?)\*\*`)
	tableSepRe   = regexp.MustCompile(`^\|[\s\-:|]+\|$`)
)

// MarkdownToLessonDesc converts markdown lesson body to Skool metadata.desc format.
// Skool stores lesson HTML bodies as TipTap JSON prefixed with "[v2]".
func MarkdownToLessonDesc(markdown string) (string, error) {
	nodes, err := markdownToTipTapNodes(markdown)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(nodes)
	if err != nil {
		return "", err
	}
	return "[v2]" + string(raw), nil
}

func markdownToTipTapNodes(markdown string) ([]map[string]any, error) {
	text := strings.ReplaceAll(markdown, "\r\n", "\n")
	blocks := splitMarkdownBlocks(text)
	out := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		if strings.HasPrefix(block, "```") {
			out = append(out, codeFenceNodes(block)...)
			continue
		}
		paras := strings.Split(block, "\n\n")
		for _, para := range paras {
			para = strings.TrimSpace(para)
			if para == "" {
				continue
			}
			if isTableBlock(para) {
				out = append(out, tableNodes(para)...)
				continue
			}
			out = append(out, paragraphFromBlock(para))
		}
	}
	if len(out) == 0 {
		out = append(out, map[string]any{"type": "paragraph", "content": textNodes("", nil)})
	}
	return out, nil
}

func splitMarkdownBlocks(text string) []string {
	var blocks []string
	var b strings.Builder
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			if inFence {
				b.WriteString(line)
				b.WriteString("\n")
				blocks = append(blocks, b.String())
				b.Reset()
				inFence = false
				continue
			}
			if b.Len() > 0 {
				blocks = append(blocks, strings.TrimRight(b.String(), "\n"))
				b.Reset()
			}
			inFence = true
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}
		if inFence {
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}
		if trim == "" {
			if b.Len() > 0 {
				blocks = append(blocks, strings.TrimRight(b.String(), "\n"))
				b.Reset()
			}
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	if b.Len() > 0 {
		blocks = append(blocks, strings.TrimRight(b.String(), "\n"))
	}
	return blocks
}

func codeFenceNodes(block string) []map[string]any {
	lines := strings.Split(block, "\n")
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[:len(lines)-1]
	}
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		out = append(out, map[string]any{
			"type":    "paragraph",
			"content": inlineContent(line),
		})
	}
	return out
}

func isTableBlock(para string) bool {
	for _, line := range strings.Split(para, "\n") {
		if strings.Contains(strings.TrimSpace(line), "|") {
			return true
		}
	}
	return false
}

func tableNodes(para string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, line := range strings.Split(para, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || tableSepRe.MatchString(line) {
			continue
		}
		cells := strings.Split(line, "|")
		parts := make([]string, 0, len(cells))
		for _, c := range cells {
			c = strings.TrimSpace(c)
			if c != "" {
				parts = append(parts, c)
			}
		}
		if len(parts) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"type":    "paragraph",
			"content": inlineContent(strings.Join(parts, " — ")),
		})
	}
	return out
}

func paragraphFromBlock(para string) map[string]any {
	lines := strings.Split(para, "\n")
	content := make([]map[string]any, 0, len(lines)*4)
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var lineContent []map[string]any
		switch {
		case strings.HasPrefix(line, "## "):
			lineContent = textNodes(strings.TrimPrefix(line, "## "), map[string]any{"bold": true})
		case strings.HasPrefix(line, "# "):
			lineContent = textNodes(strings.TrimPrefix(line, "# "), map[string]any{"bold": true})
		case strings.HasPrefix(line, "> "):
			lineContent = inlineContent(strings.TrimPrefix(line, "> "))
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			lineContent = inlineContent("• " + strings.TrimSpace(line[2:]))
		default:
			if n, rest, ok := strings.Cut(line, ". "); ok && len(n) > 0 && len(n) <= 2 && n[0] >= '0' && n[0] <= '9' {
				lineContent = inlineContent(n + ". " + rest)
			} else {
				lineContent = inlineContent(line)
			}
		}
		content = append(content, lineContent...)
		if i < len(lines)-1 {
			content = append(content, textNodes("\n", nil)...)
		}
	}
	if len(content) == 0 {
		content = inlineContent(para)
	}
	return map[string]any{"type": "paragraph", "content": content}
}

func inlineContent(text string) []map[string]any {
	if text == "" {
		return textNodes("", nil)
	}
	var out []map[string]any
	loc := inlineBoldRe.FindStringSubmatchIndex(text)
	for loc != nil {
		if loc[0] > 0 {
			out = append(out, textNodes(text[:loc[0]], nil)...)
		}
		out = append(out, textNodes(text[loc[2]:loc[3]], map[string]any{"bold": true})...)
		text = text[loc[1]:]
		loc = inlineBoldRe.FindStringSubmatchIndex(text)
	}
	if text != "" {
		out = append(out, textNodes(text, nil)...)
	}
	if len(out) == 0 {
		out = textNodes("", nil)
	}
	return out
}

func textNodes(text string, marks map[string]any) []map[string]any {
	if text == "" {
		return nil
	}
	node := map[string]any{
		"type": "text",
		"text": text,
	}
	if len(marks) > 0 {
		ms := make([]map[string]any, 0, len(marks))
		for k := range marks {
			ms = append(ms, map[string]any{"type": k})
		}
		node["marks"] = ms
	}
	return []map[string]any{node}
}
