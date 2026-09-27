package jalan

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var (
	asciiSpaceRE = regexp.MustCompile(`[\t\n\r\f\v\x{00a0} ]+`)
	propertyIDRE = regexp.MustCompile(`(?:yadNo|/yad)([0-9]{6})`)
	offerIDRE    = regexp.MustCompile(`^(?:sa_)?yd([0-9]{6})pc([0-9]+)rc([0-9]+)$`)
	moneyRE      = regexp.MustCompile(`([0-9][0-9,]*)\s*円`)
	countRE      = regexp.MustCompile(`([0-9][0-9,]*)\s*(?:軒|件)`)
)

func parseHTML(doc string) (*html.Node, error) {
	if strings.TrimSpace(doc) == "" {
		return nil, fmt.Errorf("jalan: empty HTML response")
	}
	n, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return nil, fmt.Errorf("jalan: parse HTML: %w", err)
	}
	return n, nil
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
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}
func findAll(n *html.Node, matches func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	var walk func(*html.Node)
	walk = func(v *html.Node) {
		if v == nil {
			return
		}
		if matches(v) {
			out = append(out, v)
		}
		for c := v.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}
func findFirst(n *html.Node, matches func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if matches(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirst(c, matches); found != nil {
			return found
		}
	}
	return nil
}
func classNode(n *html.Node, class string) *html.Node {
	return findFirst(n, func(v *html.Node) bool { return v.Type == html.ElementNode && hasClass(v, class) })
}
func classNodes(n *html.Node, class string) []*html.Node {
	return findAll(n, func(v *html.Node) bool { return v.Type == html.ElementNode && hasClass(v, class) })
}
func idNode(n *html.Node, id string) *html.Node {
	return findFirst(n, func(v *html.Node) bool { return v.Type == html.ElementNode && attr(v, "id") == id })
}
func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(v *html.Node) {
		if v == nil {
			return
		}
		if v.Type == html.ElementNode && (v.Data == "script" || v.Data == "style" || v.Data == "noscript" || v.Data == "template") {
			return
		}
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
			b.WriteByte(' ')
		}
		for c := v.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return clean(b.String())
}
func clean(s string) string                       { return strings.TrimSpace(asciiSpaceRE.ReplaceAllString(s, " ")) }
func classText(n *html.Node, class string) string { return textOf(classNode(n, class)) }
func excerpt(s string) string {
	r := []rune(clean(s))
	if len(r) > 240 {
		return string(r[:237]) + "…"
	}
	return string(r)
}
func evidence(field, text, sourceURL string) Evidence {
	return Evidence{Field: field, Text: excerpt(text), URL: sourceURL}
}
func addEvidence(dst *[]Evidence, field, text, sourceURL string) {
	if clean(text) == "" {
		return
	}
	e := evidence(field, text, sourceURL)
	for _, x := range *dst {
		if x == e {
			return
		}
	}
	*dst = append(*dst, e)
}
func resolveURL(base, ref string) string {
	r, err := url.Parse(ref)
	if err != nil || ref == "" || (r.Scheme != "" && r.Scheme != "http" && r.Scheme != "https") {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return b.ResolveReference(r).String()
}
func parseAmount(s string) *int64 {
	m := moneyRE.FindStringSubmatch(s)
	if len(m) != 2 {
		return nil
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(m[1], ",", ""), 10, 64)
	if err != nil {
		return nil
	}
	return &n
}
func parseCount(s string) *int {
	m := countRE.FindStringSubmatch(s)
	if len(m) != 2 {
		return nil
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if err != nil {
		return nil
	}
	return &n
}
func boolPtr(v bool) *bool { return &v }
func priceQuote(amountText, basisText, sourceURL string) Price {
	p := emptyPrice()
	p.Amount = parseAmount(amountText)
	p.QuoteText = clean(amountText)
	p.OccupancyText = clean(basisText)
	switch {
	case strings.Contains(basisText, "合計") || strings.Contains(basisText, "宿泊料金"):
		p.Basis = "whole_stay"
	case strings.Contains(basisText, "1泊") && (strings.Contains(basisText, "1名") || strings.Contains(basisText, "一人")):
		p.Basis = "per_person_per_night"
	case strings.Contains(basisText, "1泊") && (strings.Contains(basisText, "1室") || strings.Contains(basisText, "1部屋")):
		p.Basis = "per_room_per_night"
	}
	if strings.Contains(basisText, "税込") {
		p.TaxInclusion = "included"
	} else if strings.Contains(basisText, "税別") {
		p.TaxInclusion = "excluded"
	}
	if strings.Contains(basisText, "サービス料込") {
		p.ServiceChargeInclusion = "included"
	} else if strings.Contains(basisText, "サービス料別") {
		p.ServiceChargeInclusion = "excluded"
	}
	addEvidence(&p.Evidence, "price.amount", amountText, sourceURL)
	addEvidence(&p.Evidence, "price.basis", basisText, sourceURL)
	return p
}
func nextPage(n *html.Node) bool {
	return findFirst(n, func(v *html.Node) bool {
		if v.Type != html.ElementNode || v.Data != "a" {
			return false
		}
		label := textOf(v)
		return (hasClass(v, "next") || strings.Contains(label, "次へ") || attr(v, "rel") == "next") && (attr(v, "href") != "" || attr(v, "onclick") != "")
	}) != nil
}
func tablePairs(n *html.Node) map[string]string {
	out := map[string]string{}
	for _, row := range findAll(n, func(v *html.Node) bool { return v.Type == html.ElementNode && v.Data == "tr" }) {
		var heading string
		for c := row.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if c.Data == "th" {
				heading = textOf(c)
			} else if c.Data == "td" && heading != "" {
				out[heading] = textOf(c)
				heading = ""
			}
		}
	}
	return out
}
func labelledDL(n *html.Node, label string) string {
	for _, dl := range findAll(n, func(v *html.Node) bool { return v.Type == html.ElementNode && v.Data == "dl" }) {
		dt := findFirst(dl, func(v *html.Node) bool { return v.Type == html.ElementNode && v.Data == "dt" })
		if strings.TrimRight(textOf(dt), "：: ") == label {
			return textOf(findFirst(dl, func(v *html.Node) bool { return v.Type == html.ElementNode && v.Data == "dd" }))
		}
	}
	return ""
}
