// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package norma

import (
	"html"
	"regexp"
	"strings"
)

var (
	tagRe          = regexp.MustCompile(`(?s)<(/?)([a-zA-Z0-9]+)([^>]*)>`)
	classRe        = regexp.MustCompile(`class="([^"]*)"`)
	hrefURNRe      = regexp.MustCompile(`href="[^"]*?(urn:nir:[^"&]+)"`)
	blankRunsRe    = regexp.MustCompile(`\n{3,}`)
	spaceRunsRe    = regexp.MustCompile(`[ \t\x{00a0}]+`)
	annexHeadingRe = regexp.MustCompile(`^(Art\.\s*\d+[a-z-]*\.?)[ \t]*(\n|$)`)
	markerRe       = regexp.MustCompile(`^\(\(\s*(.*?)\s*\)\)$`)
	italicLineRe   = regexp.MustCompile(`(?m)^_([^\n]*\S)\s*$`)
	emptyHeadingRe = regexp.MustCompile(`(?m)^## ?$\n?`)
	articleNumRe   = regexp.MustCompile(`(?s)class="article-num-akn"[^>]*>(.*?)</`)
	voidTags       = map[string]bool{"br": true, "hr": true, "img": true, "input": true, "meta": true, "link": true, "wbr": true, "col": true, "area": true, "source": true}
)

// RenderOptions controls article HTML rendering.
type RenderOptions struct {
	Markdown bool // "## Art. 3" headings and bold comma numbers
	Links    bool // append [urn:...] after linked references
}

// RenderArticle converts Normattiva article HTML (AKN-derived markup) into
// readable text. Headings become their own lines, commi start new paragraphs,
// amendment markers like "(( ... ))" are kept verbatim.
func RenderArticle(src string, opt RenderOptions) string {
	var b strings.Builder
	pos := 0
	var stack []string // classes of open elements, to know what a closing tag ends
	pendingURN := ""
	for _, m := range tagRe.FindAllStringSubmatchIndex(src, -1) {
		b.WriteString(src[pos:m[0]])
		pos = m[1]
		closing := src[m[2]:m[3]] == "/"
		tag := strings.ToLower(src[m[4]:m[5]])
		attrs := src[m[6]:m[7]]
		class := ""
		if cm := classRe.FindStringSubmatch(attrs); cm != nil {
			class = cm[1]
		}
		if closing {
			cls := ""
			if n := len(stack); n > 0 {
				cls = stack[n-1]
				stack = stack[:n-1]
			}
			switch {
			case tag == "a":
				if opt.Links && pendingURN != "" {
					b.WriteString(" [" + pendingURN + "]")
				}
				pendingURN = ""
			case strings.Contains(cls, "comma-num-akn") && opt.Markdown:
				trimmed := strings.TrimRight(b.String(), " \t")
				b.Reset()
				b.WriteString(trimmed)
				b.WriteString("** ")
			case strings.Contains(cls, "article-num-akn") || strings.Contains(cls, "article-heading-akn"):
				b.WriteString("\n\n")
			case tag == "h1" || tag == "h2" || tag == "h3" || tag == "p" || tag == "div" || tag == "li":
				b.WriteString("\n")
			}
			continue
		}
		if !strings.HasSuffix(strings.TrimSpace(attrs), "/") && !voidTags[tag] {
			stack = append(stack, class)
		}
		switch {
		case tag == "br":
			b.WriteString("\n")
		case tag == "a":
			if um := hrefURNRe.FindStringSubmatch(attrs); um != nil {
				pendingURN = html.UnescapeString(um[1])
			}
		case strings.Contains(class, "article-num-akn"):
			b.WriteString("\n\n")
			if opt.Markdown {
				b.WriteString("## ")
			}
		case strings.Contains(class, "article-heading-akn"):
			b.WriteString("\n")
			if opt.Markdown {
				b.WriteString("_")
			}
		case strings.Contains(class, "art-comma-div-akn"):
			b.WriteString("\n\n")
		case strings.Contains(class, "comma-num-akn"):
			if opt.Markdown {
				b.WriteString("**")
			}
		case tag == "h1" || tag == "h2" || tag == "h3" || tag == "p" || tag == "li":
			b.WriteString("\n")
		}
	}
	b.WriteString(src[pos:])
	out := html.UnescapeString(b.String())
	if opt.Markdown {
		// Close italic headings: "_(Titolo)\n" -> "_(Titolo)_\n".
		out = italicLineRe.ReplaceAllString(out, "_${1}_")
	}
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(spaceRunsRe.ReplaceAllString(l, " "))
	}
	out = strings.Join(lines, "\n")
	out = blankRunsRe.ReplaceAllString(out, "\n\n")
	// Drop the empty "## " headings of the stripped preamble placeholders.
	out = emptyHeadingRe.ReplaceAllString(out, "")
	out = strings.TrimSpace(out)
	if opt.Markdown && !strings.HasPrefix(out, "## ") {
		// Code annexes carry "Art. 2043." as plain text: give it the same
		// heading level as AKN-marked articles.
		if m := annexHeadingRe.FindStringSubmatch(out); m != nil {
			out = "## " + strings.TrimSuffix(m[1], ".") + m[2] + out[len(m[0]):]
		}
	}
	return out
}

// ArticleHeading extracts the article number heading ("Art. 3") from HTML.
func ArticleHeading(src string) string {
	m := articleNumRe.FindStringSubmatch(src)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(m[1], "")))
}

// IsAbrogated reports whether the article text is only an abrogation note.
func IsAbrogated(src string) bool {
	return strings.Contains(src, "art_abrogato-akn") || strings.Contains(strings.ToUpper(src), "((ARTICOLO ABROGATO")
}

// ReferencedURNs lists the distinct URN:NIR links inside the article HTML.
func ReferencedURNs(src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range hrefURNRe.FindAllStringSubmatch(src, -1) {
		u := html.UnescapeString(m[1])
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// StripAmendmentMarkers removes the "(( ... ))" wrapper Normattiva puts
// around amended text, for short fields like article headings.
func StripAmendmentMarkers(s string) string {
	s = strings.TrimSpace(s)
	for {
		m := markerRe.FindStringSubmatch(s)
		if m == nil {
			return s
		}
		s = strings.TrimSpace(m[1])
	}
}
