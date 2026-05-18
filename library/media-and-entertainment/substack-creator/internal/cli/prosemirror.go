// PATCH: prosemirror-converter — entire file is a hand-extension over the
// generator output. Substack's editor expects ProseMirror JSON with
// substack-specific node names (strong/em, highlighted_code_block,
// inline_latex with persistentExpression, latex_block, paywall) that the
// generator does not know about. Without this converter, drafts create
// can only emit raw strings, which the editor rejects with a render-time
// error. Schema names were extracted from Substack's editor JS bundle.
// Recorded in .printing-press-patches.json.
package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// uuidV4 returns a random UUID v4-like string. Substack's editor uses
// these for node ids on latex_block, highlighted_code_block, etc.
// We don't need cryptographic guarantees; just unique-per-node.
func uuidV4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]))
}

// Substack uses ProseMirror with a custom schema. The names below are
// verified against a real Substack draft (drafts get on a sample post)
// so the editor accepts them and renders correctly.
//
// VERIFIED NODE TYPES (from real Substack draft):
//   paragraph, heading (attrs.level), blockquote, bullet_list, ordered_list
//   list_item, captionedImage (wraps image2), image2, code_block
//   twitter2, youtube2, vimeo, spotify2, soundcloud  (typed embeds — URL-based)
//
// VERIFIED MARK TYPES:
//   strong  (NOT "bold" — that's the markdown name, Substack uses "strong")
//   em      (NOT "italic" — Substack uses "em")
//   link    (attrs.href + attrs.title; title may be null)
//   code    (inline code)
//
// EXPERIMENTAL (may or may not render — UI test required):
//   paywall, latex, latex_block, pullquote, button, horizontal_rule
//
// Supported markdown syntax in body input:
//   # / ## / ### / #### heading
//   regular paragraph
//   - bullet / * bullet
//   1. ordered
//   > blockquote
//   ```code```            -> code_block
//   --- or ***            -> horizontal_rule
//   $$ math $$            -> latex_block (display math)
//   **bold**              -> strong mark
//   *italic*              -> em mark
//   `code`                -> code mark
//   [text](url)           -> link mark
//   $inline math$         -> latex (inline math)
//   ![alt](url)           -> captionedImage > image2
//   [paywall]             -> paywall node
//   https://twitter.com/X/status/Y on its own line       -> twitter2
//   https://www.youtube.com/watch?v=ID                   -> youtube2
//   https://vimeo.com/ID                                 -> vimeo
//   https://open.spotify.com/...                         -> spotify2
//   https://soundcloud.com/...                           -> soundcloud

type pmNode struct {
	Type    string         `json:"type"`
	Content []*pmNode      `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []pmMark       `json:"marks,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

type pmMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

func markdownToProseMirrorExt(md string) string {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	md = strings.TrimSpace(md)
	if md == "" {
		return `{"type":"doc","content":[]}`
	}

	doc := &pmNode{Type: "doc"}
	lines := strings.Split(md, "\n")

	i := 0
	for i < len(lines) {
		line := lines[i]
		trim := strings.TrimSpace(line)

		if trim == "" {
			i++
			continue
		}

		// Horizontal rule
		if trim == "---" || trim == "***" {
			doc.Content = append(doc.Content, &pmNode{Type: "horizontal_rule"})
			i++
			continue
		}

		// Substack paywall node: atom, group:block, empty attrs.
		// Verified from editor JS bundle:
		//   {attrs:{}, inline:false, group:"block", atom:true, selectable:true,
		//    isolating:false, defining:false, draggable:true}
		if trim == "[paywall]" || trim == "{{paywall}}" {
			doc.Content = append(doc.Content, &pmNode{Type: "paywall"})
			i++
			continue
		}

		// Typed embeds: detect URL-only lines and route to the correct Substack embed node
		if node := detectEmbed(trim); node != nil {
			doc.Content = append(doc.Content, node)
			i++
			continue
		}

		// Code block (fenced)
		if strings.HasPrefix(trim, "```") {
			lang := strings.TrimPrefix(trim, "```")
			i++
			var codeBuf strings.Builder
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				if codeBuf.Len() > 0 {
					codeBuf.WriteString("\n")
				}
				codeBuf.WriteString(lines[i])
				i++
			}
			if i < len(lines) {
				i++ // skip closing ```
			}
			// Substack code block: highlighted_code_block with attrs.language + attrs.nodeId.
			// Verified from editor JS bundle:
			//   content:"text*", group:"block", code:!0, defining:!0, isolating:!0,
			//   attrs:{language:{default:"plaintext"}, nodeId:{default:null}}
			if lang == "" {
				lang = "plaintext"
			}
			doc.Content = append(doc.Content, &pmNode{
				Type: "highlighted_code_block",
				Attrs: map[string]any{
					"language": lang,
					"nodeId":   uuidV4(),
				},
				Content: []*pmNode{{Type: "text", Text: codeBuf.String()}},
			})
			continue
		}

		// Substack LaTeX block (display math).
		// Verified attrs: {persistentExpression:{default:""}, id:{default:""}}
		if strings.HasPrefix(trim, "$$") {
			rest := strings.TrimPrefix(trim, "$$")
			var latexBuf strings.Builder
			if strings.HasSuffix(rest, "$$") && rest != "" {
				latexBuf.WriteString(strings.TrimSuffix(rest, "$$"))
				i++
			} else {
				latexBuf.WriteString(rest)
				i++
				for i < len(lines) && !strings.HasSuffix(strings.TrimSpace(lines[i]), "$$") {
					if latexBuf.Len() > 0 {
						latexBuf.WriteString("\n")
					}
					latexBuf.WriteString(lines[i])
					i++
				}
				if i < len(lines) {
					closingLine := strings.TrimSuffix(strings.TrimSpace(lines[i]), "$$")
					if closingLine != "" {
						if latexBuf.Len() > 0 {
							latexBuf.WriteString("\n")
						}
						latexBuf.WriteString(closingLine)
					}
					i++
				}
			}
			doc.Content = append(doc.Content, makeLatexBlock(strings.TrimSpace(latexBuf.String())))
			continue
		}

		// Headings (1-4)
		if h, rest := parseHeading(trim); h > 0 {
			doc.Content = append(doc.Content, &pmNode{
				Type:    "heading",
				Attrs:   map[string]any{"level": h},
				Content: parseInline(rest),
			})
			i++
			continue
		}

		// Blockquote
		if strings.HasPrefix(trim, "> ") || trim == ">" {
			var quoteLines []string
			for i < len(lines) {
				lt := strings.TrimSpace(lines[i])
				if strings.HasPrefix(lt, "> ") {
					quoteLines = append(quoteLines, strings.TrimPrefix(lt, "> "))
				} else if lt == ">" {
					quoteLines = append(quoteLines, "")
				} else {
					break
				}
				i++
			}
			para := &pmNode{Type: "paragraph", Content: parseInline(strings.Join(quoteLines, " "))}
			doc.Content = append(doc.Content, &pmNode{
				Type:    "blockquote",
				Content: []*pmNode{para},
			})
			continue
		}

		// Bullet list
		if isBulletLine(trim) {
			var items []*pmNode
			for i < len(lines) && isBulletLine(strings.TrimSpace(lines[i])) {
				text := strings.TrimSpace(lines[i])
				if strings.HasPrefix(text, "- ") {
					text = strings.TrimPrefix(text, "- ")
				} else if strings.HasPrefix(text, "* ") {
					text = strings.TrimPrefix(text, "* ")
				}
				items = append(items, &pmNode{
					Type: "list_item",
					Content: []*pmNode{
						{Type: "paragraph", Content: parseInline(text)},
					},
				})
				i++
			}
			doc.Content = append(doc.Content, &pmNode{
				Type:    "bullet_list",
				Attrs:   map[string]any{"tight": false},
				Content: items,
			})
			continue
		}

		// Ordered list
		if isOrderedLine(trim) {
			var items []*pmNode
			for i < len(lines) && isOrderedLine(strings.TrimSpace(lines[i])) {
				text := orderedItemText(strings.TrimSpace(lines[i]))
				items = append(items, &pmNode{
					Type: "list_item",
					Content: []*pmNode{
						{Type: "paragraph", Content: parseInline(text)},
					},
				})
				i++
			}
			doc.Content = append(doc.Content, &pmNode{
				Type:    "ordered_list",
				Attrs:   map[string]any{"order": 1, "tight": false},
				Content: items,
			})
			continue
		}

		// Inline image on its own line -> captionedImage
		if m := imagePat.FindStringSubmatch(trim); m != nil && len(trim) == len(m[0]) {
			doc.Content = append(doc.Content, makeCaptionedImage(m[2], m[1]))
			i++
			continue
		}

		// Default: paragraph (gather lines until blank or block start)
		var paraLines []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !isBlockStart(strings.TrimSpace(lines[i])) {
			paraLines = append(paraLines, lines[i])
			i++
		}
		doc.Content = append(doc.Content, &pmNode{
			Type:    "paragraph",
			Content: parseInline(strings.Join(paraLines, " ")),
		})
	}

	out, _ := json.Marshal(doc)
	return string(out)
}

func parseHeading(s string) (int, string) {
	for _, prefix := range []struct {
		p string
		l int
	}{{"#### ", 4}, {"### ", 3}, {"## ", 2}, {"# ", 1}} {
		if strings.HasPrefix(s, prefix.p) {
			return prefix.l, strings.TrimPrefix(s, prefix.p)
		}
	}
	return 0, s
}

func makeLatexBlock(value string) *pmNode {
	// Substack's editor uses 'persistentExpression' + 'id', NOT 'value'.
	// Confirmed via JS bundle: schema.nodes.latex_block.create({persistentExpression, id}).
	return &pmNode{
		Type: "latex_block",
		Attrs: map[string]any{
			"persistentExpression": value,
			"id":                   uuidV4(),
		},
	}
}

func makeCaptionedImage(src, alt string) *pmNode {
	imgAttrs := map[string]any{
		"src":         src,
		"alt":         nilIfEmpty(alt),
		"bytes":       nil,
		"fullscreen":  nil,
		"height":      nil,
		"href":        nil,
		"resizeWidth": nil,
		"title":       nil,
		"type":        nil,
		"width":       nil,
	}
	return &pmNode{
		Type: "captionedImage",
		Content: []*pmNode{{
			Type:  "image2",
			Attrs: imgAttrs,
		}},
	}
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// detectEmbed checks a single-line URL against known Substack embed providers
// and returns the appropriate typed node (twitter2, youtube2, etc.).
func detectEmbed(s string) *pmNode {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return nil
	}
	// Only treat as embed if the line is JUST a URL (no other text)
	if strings.ContainsAny(s, " \t") {
		return nil
	}
	low := strings.ToLower(s)
	if strings.Contains(low, "twitter.com/") || strings.Contains(low, "x.com/") {
		return &pmNode{Type: "twitter2", Attrs: map[string]any{"url": s}}
	}
	if strings.Contains(low, "youtube.com/watch") || strings.Contains(low, "youtu.be/") {
		videoID := extractYouTubeID(s)
		return &pmNode{Type: "youtube2", Attrs: map[string]any{"videoId": videoID}}
	}
	if strings.Contains(low, "vimeo.com/") {
		return &pmNode{Type: "vimeo", Attrs: map[string]any{"videoId": extractVimeoID(s)}}
	}
	if strings.Contains(low, "open.spotify.com/") {
		return &pmNode{Type: "spotify2", Attrs: map[string]any{"url": s}}
	}
	if strings.Contains(low, "soundcloud.com/") {
		return &pmNode{Type: "soundcloud", Attrs: map[string]any{"url": s}}
	}
	return nil
}

func extractYouTubeID(u string) string {
	if i := strings.Index(u, "v="); i >= 0 {
		rest := u[i+2:]
		if amp := strings.IndexByte(rest, '&'); amp >= 0 {
			return rest[:amp]
		}
		return rest
	}
	if i := strings.Index(u, "youtu.be/"); i >= 0 {
		return u[i+9:]
	}
	return ""
}

func extractVimeoID(u string) string {
	parts := strings.Split(u, "/")
	for j := len(parts) - 1; j >= 0; j-- {
		if parts[j] != "" {
			return parts[j]
		}
	}
	return ""
}

func isBulletLine(s string) bool {
	return strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ")
}

func isOrderedLine(s string) bool {
	for i, r := range s {
		if r == '.' && i > 0 && i < len(s)-1 && s[i+1] == ' ' {
			prefix := s[:i]
			if _, err := strconv.Atoi(prefix); err == nil {
				return true
			}
			return false
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return false
}

func orderedItemText(s string) string {
	for i := range s {
		if s[i] == '.' && i < len(s)-1 && s[i+1] == ' ' {
			return s[i+2:]
		}
	}
	return s
}

func isBlockStart(s string) bool {
	if h, _ := parseHeading(s); h > 0 {
		return true
	}
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") {
		return true
	}
	if strings.HasPrefix(s, "> ") || s == ">" {
		return true
	}
	if strings.HasPrefix(s, "```") {
		return true
	}
	if strings.HasPrefix(s, "$$") {
		return true
	}
	if s == "---" || s == "***" {
		return true
	}
	if s == "[paywall]" || s == "{{paywall}}" {
		return true
	}
	if isOrderedLine(s) {
		return true
	}
	if detectEmbed(s) != nil {
		return true
	}
	return false
}

// parseInline handles inline marks and inline-image syntax.
// Important: when a leading "pre" segment exists before a matched pattern,
// parse it recursively so earlier marks (bold/italic/code) inside that
// pre-segment are also converted — otherwise `**bold** then [link](url)`
// loses the bold mark on the bold text before the link.
func parseInline(s string) []*pmNode {
	if s == "" {
		return nil
	}
	var out []*pmNode
	rest := s

	emit := func(pre string) {
		if pre == "" {
			return
		}
		// Recursively parse the pre-segment so nested marks survive.
		for _, n := range parseInline(pre) {
			out = append(out, n)
		}
	}

	for len(rest) > 0 {
		// Find the earliest match among all patterns. This avoids
		// greedily consuming a pattern when an earlier pattern of a
		// different kind exists in the same segment.
		type cand struct {
			start, end int
			kind       string
			match      []int
		}
		var earliest *cand

		pick := func(kind string, m []int) {
			if m == nil {
				return
			}
			if earliest == nil || m[0] < earliest.start {
				earliest = &cand{start: m[0], end: m[1], kind: kind, match: m}
			}
		}
		pick("link", regexpFind(linkPat, rest))
		pick("bold", regexpFind(boldPat, rest))
		pick("italic", regexpFind(italicPat, rest))
		pick("code", regexpFind(codePat, rest))
		pick("latex", regexpFind(latexInlinePat, rest))

		if earliest == nil {
			out = append(out, &pmNode{Type: "text", Text: rest})
			break
		}

		emit(rest[:earliest.start])
		m := earliest.match
		switch earliest.kind {
		case "link":
			out = append(out, &pmNode{
				Type: "text",
				Text: rest[m[2]:m[3]],
				Marks: []pmMark{{
					Type: "link",
					Attrs: map[string]any{
						"href":  rest[m[4]:m[5]],
						"title": nil,
					},
				}},
			})
		case "bold":
			out = append(out, &pmNode{
				Type:  "text",
				Text:  rest[m[2]:m[3]],
				Marks: []pmMark{{Type: "strong"}},
			})
		case "italic":
			out = append(out, &pmNode{
				Type:  "text",
				Text:  rest[m[2]:m[3]],
				Marks: []pmMark{{Type: "em"}},
			})
		case "code":
			out = append(out, &pmNode{
				Type:  "text",
				Text:  rest[m[2]:m[3]],
				Marks: []pmMark{{Type: "code"}},
			})
		case "latex":
			// Substack inline LaTeX node is 'inline_latex' (verified from JS bundle).
			// 'latex_upgraded_inline' was a feature-flag name, NOT a node type.
			// Spec: atom:true, inline:true, group:"inline", attrs:{persistentExpression, id}
			out = append(out, &pmNode{
				Type: "inline_latex",
				Attrs: map[string]any{
					"persistentExpression": rest[m[2]:m[3]],
					"id":                   uuidV4(),
				},
			})
		}
		rest = rest[earliest.end:]
	}
	return out
}

var (
	imagePat       = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	linkPat        = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	boldPat        = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicPat      = regexp.MustCompile(`\*([^*\s][^*]*[^*\s]|\S)\*`)
	codePat        = regexp.MustCompile("`([^`]+)`")
	// Inline LaTeX requires the opening $ to be followed by a non-space, non-digit
	// character and the closing $ to be preceded by a non-space character. This
	// avoids matching across monetary dollar signs like "$10 and $20" — the
	// previous greedy [^$]+ would have eaten the middle prose as a single
	// inline_latex node, dropping content from the rendered body.
	latexInlinePat = regexp.MustCompile(`\$([^\s\d$][^$\n]*[^\s$]|[^\s\d$])\$`)
)

func regexpFind(re *regexp.Regexp, s string) []int {
	return re.FindStringSubmatchIndex(s)
}
