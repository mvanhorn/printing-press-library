package gaclient

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var rePubblicato = regexp.MustCompile(`Pubblicato il\s+(\d{2}/\d{2}/\d{4})`)

// ExtractDataDeposito returns the publication date ("Pubblicato il DD/MM/YYYY")
// from a provvedimento document, or "" if absent.
func ExtractDataDeposito(docHTML string) string {
	if m := rePubblicato.FindStringSubmatch(docHTML); m != nil {
		return m[1]
	}
	return ""
}

// HTMLToText converts a provvedimento document to clean plain text: one
// paragraph per blank-line-separated block, tags stripped, entities decoded.
func HTMLToText(docHTML string) string {
	return render(docHTML, false)
}

// HTMLToMarkdown converts a provvedimento document to clean Markdown.
// Block paragraphs are separated by blank lines, <i> becomes *italic*, line
// breaks are preserved, and the trailing signature table becomes a simple list.
func HTMLToMarkdown(docHTML string) string {
	return render(docHTML, true)
}

func render(docHTML string, markdown bool) string {
	doc, err := html.Parse(strings.NewReader(docHTML))
	if err != nil {
		// Fall back to a naive tag strip if parsing fails.
		return strings.TrimSpace(reTags.ReplaceAllString(docHTML, " "))
	}
	var blocks []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p":
				if t := inline(n, markdown); strings.TrimSpace(t) != "" {
					blocks = append(blocks, strings.TrimSpace(t))
				}
				return
			case "tr":
				cells := rowCells(n, markdown)
				if joined := strings.TrimSpace(strings.Join(cells, "  ")); joined != "" {
					prefix := ""
					if markdown {
						prefix = "- "
					}
					blocks = append(blocks, prefix+joined)
				}
				return
			case "script", "style", "head":
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	out := strings.Join(blocks, "\n\n")
	out = regexp.MustCompile(`\n{3,}`).ReplaceAllString(out, "\n\n")
	return strings.TrimSpace(out)
}

// inline renders the inline content of a node to a single string.
func inline(n *html.Node, markdown bool) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			switch n.Data {
			case "br":
				b.WriteString("\n")
				return
			case "i", "em":
				if markdown {
					b.WriteString("*")
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						walk(c)
					}
					b.WriteString("*")
					return
				}
			case "b", "strong":
				if markdown {
					b.WriteString("**")
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						walk(c)
					}
					b.WriteString("**")
					return
				}
			case "img":
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	// Collapse runs of spaces/tabs but keep intentional newlines from <br>.
	s := html.UnescapeString(b.String())
	s = regexp.MustCompile(`[ \t]+`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(` *\n *`).ReplaceAllString(s, "\n")
	return s
}

func rowCells(tr *html.Node, markdown bool) []string {
	var cells []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			if t := strings.TrimSpace(inline(c, markdown)); t != "" {
				cells = append(cells, t)
			}
		}
	}
	return cells
}
