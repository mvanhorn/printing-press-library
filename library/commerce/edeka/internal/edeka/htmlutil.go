package edeka

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// parseHTML parses a page; the shop's pages are large but well-formed enough
// for the tolerant HTML5 parser.
func parseHTML(page string) (*html.Node, error) {
	return html.Parse(strings.NewReader(page))
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

func hasClass(n *html.Node, class string) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

// findAll returns every element below n (inclusive) for which match is true.
// Matching elements are not descended into when stopAtMatch is set.
func findAll(n *html.Node, match func(*html.Node) bool, stopAtMatch bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.ElementNode && match(x) {
			out = append(out, x)
			if stopAtMatch {
				return
			}
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func findFirst(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	var found *html.Node
	var walk func(*html.Node) bool
	walk = func(x *html.Node) bool {
		if x.Type == html.ElementNode && match(x) {
			found = x
			return true
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			if walk(c) {
				return true
			}
		}
		return false
	}
	walk(n)
	return found
}

func byClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool { return hasClass(n, class) }
}

func byID(id string) func(*html.Node) bool {
	return func(n *html.Node) bool { return attr(n, "id") == id }
}

func byTagAttr(tag, key, val string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Data == tag && attr(n, key) == val }
}

var spaceRun = regexp.MustCompile(`\s+`)

// text returns the visible text below n with whitespace collapsed (script,
// style and svg content skipped).
func text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style" || x.Data == "svg" || x.Data == "title") {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteByte(' ')
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(spaceRun.ReplaceAllString(b.String(), " "))
}

var priceRe = regexp.MustCompile(`(-?\d{1,3}(?:\.\d{3})*|-?\d+),(\d{2})`)

// ParseCents reads a German price such as "1,59€", "1.234,50 €" or "12,90 €/kg"
// (the first amount) into cents. ok is false when no amount is found.
func ParseCents(s string) (int, bool) {
	m := priceRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	euros, err := strconv.Atoi(strings.ReplaceAll(m[1], ".", ""))
	if err != nil {
		return 0, false
	}
	cents, _ := strconv.Atoi(m[2])
	if strings.HasPrefix(m[1], "-") {
		return -(-euros*100 + cents), true
	}
	return euros*100 + cents, true
}

// FormatCents renders cents as a German euro amount, for example "1,59 €".
func FormatCents(c int) string {
	sign := ""
	if c < 0 {
		sign = "-"
		c = -c
	}
	return sign + strconv.Itoa(c/100) + "," + twoDigits(c%100) + " €"
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// cleanURL drops the query string (session ids, list context) from a shop link.
func cleanURL(u string) string {
	if i := strings.IndexByte(u, '?'); i >= 0 {
		u = u[:i]
	}
	return strings.TrimSpace(u)
}

var stokenRe = regexp.MustCompile(`name="stoken" value="([A-Za-z0-9]+)"`)

// ExtractStoken returns the shop's form token (CSRF) from a page, or "".
func ExtractStoken(page string) string {
	if m := stokenRe.FindStringSubmatch(page); m != nil {
		return m[1]
	}
	return ""
}

// looksLikeHTML reports whether a response body is an HTML page (the shop
// answers form posts it rejects with a full page instead of JSON).
func looksLikeHTML(b []byte) bool {
	s := strings.TrimSpace(string(b[:min(len(b), 512)]))
	ls := strings.ToLower(s)
	return strings.HasPrefix(ls, "<!doctype") || strings.HasPrefix(ls, "<html") || strings.Contains(ls, "<head")
}

// htmlNode aliases the parser node type for helpers outside this file.
type htmlNode = html.Node
