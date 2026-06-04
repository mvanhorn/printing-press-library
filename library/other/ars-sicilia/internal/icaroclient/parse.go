package icaroclient

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// ParseShortList walks the `<ul id="shortListTable">` block, skips the header
// `<li class="intestazione">` and returns one Record per data row. It also
// extracts the total page count from the pagination block ("Pagina N di M").
func ParseShortList(body string, arc Archive, baseURL string) ([]Record, int, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("parsing shortList HTML: %w", err)
	}
	totalPages := extractTotalPages(root)

	ul := findNodeByID(root, "shortListTable")
	if ul == nil {
		// No results — Icaro renders a different fragment in that case but it
		// is not an error per se.
		return nil, totalPages, nil
	}
	var rows []Record
	for c := ul.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "li" {
			continue
		}
		if hasClass(c, "intestazione") {
			continue
		}
		rec := parseRow(c, arc, baseURL)
		if rec.DocID == 0 && rec.Title == "" && len(rec.Fields) == 0 {
			continue
		}
		rows = append(rows, rec)
	}
	return rows, totalPages, nil
}

// ParseDoc reads a doc<NNN>-1.jsp body and lifts out the title, the principal
// text block, and any name-value pairs we can find in the sidebar.
func ParseDoc(body string, arc Archive, docID int) (Doc, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return Doc{}, fmt.Errorf("parsing doc HTML: %w", err)
	}
	doc := Doc{
		DocID:  docID,
		Fields: map[string]string{},
	}
	// Title node is usually the first h2/h3 inside the main content blocchi.
	if t := firstTextOfTag(root, "h3"); t != "" {
		doc.Title = collapseSpaces(t)
	}
	if doc.Title == "" {
		if t := firstTextOfTag(root, "h2"); t != "" {
			doc.Title = collapseSpaces(t)
		}
	}
	// Body candidates: divs with class containing "testo_gestionale".
	var bodyChunks []string
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "div" {
			return
		}
		if !hasClass(n, "testo_gestionale") {
			return
		}
		text := collapseSpaces(textContent(n))
		if text != "" {
			bodyChunks = append(bodyChunks, text)
		}
	})
	if len(bodyChunks) > 0 {
		doc.Body = strings.Join(bodyChunks, "\n\n")
	} else {
		// Fallback: dump all top-level <p> content.
		var ps []string
		walk(root, func(n *html.Node) {
			if n.Type != html.ElementNode || n.Data != "p" {
				return
			}
			t := collapseSpaces(textContent(n))
			if t != "" {
				ps = append(ps, t)
			}
		})
		doc.Body = strings.Join(ps, "\n")
	}
	return doc, nil
}

// parseRow extracts a Record from a single shortList <li>. The href carries
// the doc ID via `javascript: showDoc(N)`; each div is one column and uses
// the inner `<strong>` text minus the `<span class="simobile">` label.
func parseRow(li *html.Node, arc Archive, baseURL string) Record {
	rec := Record{Fields: map[string]string{}}
	if href := attr(li, "href"); href != "" {
		if id := extractShowDocID(href); id > 0 {
			rec.DocID = id
		}
	}
	var divs []*html.Node
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "div" {
			divs = append(divs, c)
		}
	}
	for i, div := range divs {
		colName := ""
		if i < len(arc.Columns) {
			colName = arc.Columns[i]
		} else {
			colName = fmt.Sprintf("col%d", i+1)
		}
		// Strip the `<span class="simobile">LABEL</span>` prefix that the
		// portal uses for mobile column labels — read the actual label
		// from the span rather than guessing where it ends.
		label := findSimobileLabel(div)
		text := stripSimobileLabel(textContent(div), label)
		text = collapseSpaces(text)
		// Last column carries the title in <h3><a> and an excerpt in <p>.
		if i == len(divs)-1 {
			if title := strings.TrimSpace(firstTextOfTag(div, "h3")); title != "" {
				rec.Title = collapseSpaces(title)
			}
			// First non-h3 <p> after the title is the excerpt.
			if excerpt := nthPText(div, 0); excerpt != "" && excerpt != rec.Title {
				rec.Excerpt = collapseSpaces(excerpt)
			}
			// Save the raw column text too, minus the title which is already lifted.
			if rec.Title != "" {
				text = strings.TrimSpace(strings.TrimPrefix(text, rec.Title))
				text = strings.TrimSpace(text)
			}
		}
		if text != "" {
			rec.Fields[colName] = text
		}
	}
	if rec.DocID > 0 {
		rec.URL = fmt.Sprintf("%s/icaro/doc%s-1.jsp?icaQueryId=1&icaDocId=%d", baseURL, arc.ID, rec.DocID)
	}
	return rec
}

// ---------------------------------------------------------------- helpers

func findNodeByID(n *html.Node, id string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "id" && a.Val == id {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if r := findNodeByID(c, id); r != nil {
			return r
		}
	}
	return nil
}

func hasClass(n *html.Node, want string) bool {
	if n == nil {
		return false
	}
	for _, a := range n.Attr {
		if a.Key != "class" {
			continue
		}
		for _, c := range strings.Fields(a.Val) {
			if c == want {
				return true
			}
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

var reShowDoc = regexp.MustCompile(`showDoc\((\d+)\)`)

func extractShowDocID(s string) int {
	m := reShowDoc.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// textContent returns the concatenated text descendants of n.
func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textContent(c))
	}
	return b.String()
}

// stripSimobileLabel removes the leading column label that the portal renders
// inside `<span class="simobile">LABEL</span>` (mobile column hint). label is
// the text already extracted from the simobile span. The previous heuristic
// (chop at first ".") was wrong for values like "L.R. 1" or dates
// "16.01.2024", and the label may be followed by an NBSP rather than a plain
// space — so we trim both forms after the prefix match.
func stripSimobileLabel(text, label string) string {
	text = strings.TrimLeft(text, " \t\r\n\xa0")
	label = strings.TrimSpace(label)
	if label == "" {
		return strings.TrimSpace(text)
	}
	if strings.HasPrefix(text, label) {
		return strings.TrimLeft(text[len(label):], " \t\r\n\xa0")
	}
	return strings.TrimSpace(text)
}

// findSimobileLabel returns the text content of the first descendant span
// carrying class "simobile" under n, or "".
func findSimobileLabel(n *html.Node) string {
	var out string
	walk(n, func(node *html.Node) {
		if out != "" {
			return
		}
		if node.Type == html.ElementNode && node.Data == "span" && hasClass(node, "simobile") {
			out = strings.TrimSpace(textContent(node))
		}
	})
	return out
}

func collapseSpaces(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func firstTextOfTag(n *html.Node, tag string) string {
	if n == nil {
		return ""
	}
	if n.Type == html.ElementNode && n.Data == tag {
		return textContent(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := firstTextOfTag(c, tag); t != "" {
			return t
		}
	}
	return ""
}

// nthPText returns the text content of the (n+1)-th <p> child under root.
func nthPText(root *html.Node, idx int) string {
	count := 0
	var found string
	walk(root, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "p" {
			return
		}
		if count == idx {
			found = textContent(node)
		}
		count++
	})
	return found
}

func walk(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

var rePagination = regexp.MustCompile(`Pagina\s+\d+\s+di\s+(\d+)`)

func extractTotalPages(root *html.Node) int {
	// Look for a node with class "pagina_di" whose text matches "Pagina N di M".
	var found int
	walk(root, func(n *html.Node) {
		if found > 0 || n.Type != html.ElementNode {
			return
		}
		if !hasClass(n, "pagina_di") {
			return
		}
		if m := rePagination.FindStringSubmatch(textContent(n)); len(m) >= 2 {
			if v, err := strconv.Atoi(m[1]); err == nil {
				found = v
			}
		}
	})
	if found > 0 {
		return found
	}
	return 1
}
