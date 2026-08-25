// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

// attr returns the value of a node attribute, or "".
func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// hasClass reports whether a node's class attribute contains the given class.
func hasClass(n *xhtml.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

// walk visits every element node in depth-first order.
func walk(n *xhtml.Node, visit func(*xhtml.Node)) {
	if n == nil {
		return
	}
	if n.Type == xhtml.ElementNode {
		visit(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, visit)
	}
}

// findAll returns every element for which match returns true.
func findAll(n *xhtml.Node, match func(*xhtml.Node) bool) []*xhtml.Node {
	var out []*xhtml.Node
	walk(n, func(el *xhtml.Node) {
		if match(el) {
			out = append(out, el)
		}
	})
	return out
}

// firstWithClass returns the first descendant element carrying the class.
func firstWithClass(n *xhtml.Node, class string) *xhtml.Node {
	var found *xhtml.Node
	walk(n, func(el *xhtml.Node) {
		if found == nil && hasClass(el, class) {
			found = el
		}
	})
	return found
}

// textOf returns the concatenated, whitespace-collapsed text of a node.
func textOf(n *xhtml.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var rec func(*xhtml.Node)
	rec = func(x *xhtml.Node) {
		if x.Type == xhtml.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return collapseWS(b.String())
}

var wsRe = regexp.MustCompile(`\s+`)

func collapseWS(s string) string {
	return strings.TrimSpace(wsRe.ReplaceAllString(s, " "))
}

var priceRe = regexp.MustCompile(`\d[\d.,]*\d|\d`)

// parsePrice extracts the first numeric value from a price string like
// "39.98 €", "£32.45", or "1.234,56 €" and returns it as a float.
func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	m := priceRe.FindString(s)
	if m == "" {
		return 0
	}
	// Normalize thousands/decimal separators. When both "." and "," are
	// present, the rightmost one is the decimal separator (handles both
	// "1.234,56" EU and "1,234.56" US); strip the other as thousands. When
	// only "," is present, treat it as the decimal separator.
	dot, comma := strings.LastIndex(m, "."), strings.LastIndex(m, ",")
	switch {
	case dot >= 0 && comma >= 0:
		if comma > dot { // EU: comma is decimal
			m = strings.ReplaceAll(m, ".", "")
			m = strings.ReplaceAll(m, ",", ".")
		} else { // US: dot is decimal
			m = strings.ReplaceAll(m, ",", "")
		}
	case comma >= 0:
		m = strings.ReplaceAll(m, ",", ".")
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0
	}
	return f
}

var intRe = regexp.MustCompile(`\d+`)

func parseInt(s string) int {
	m := intRe.FindString(s)
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(m)
	return n
}
