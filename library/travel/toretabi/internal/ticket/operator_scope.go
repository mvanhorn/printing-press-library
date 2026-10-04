// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ticket

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Scope only the product layouts observed on the linked official documents.
// A matching document title is not authority to inspect unrelated page panels.
func operatorContentScope(doc *html.Node, t Ticket) (*html.Node, string, string, string) {
	host := ""
	if len(t.OperatorURLs) > 0 {
		if u, e := url.Parse(t.OperatorURLs[0]); e == nil {
			host = u.Hostname()
		}
	}
	selector := ""
	var candidates []*html.Node
	walk(doc, func(n *html.Node) {
		ok := false
		switch host {
		case "railway.jr-central.co.jp":
			selector = "section[class~=main]"
			ok = n.Data == "section" && hasClass(n, "main")
		case "www.jrhokkaido.co.jp":
			selector = "section[class~=detail-ticketSec]"
			ok = n.Data == "section" && hasClass(n, "detail-ticketSec")
		case "www.jreast.co.jp":
			selector = "main#contents section.contentsWrapper"
			ok = n.Data == "section" && hasClass(n, "contentsWrapper") && ancestorID(n, "contents") != nil
		case "tickets.jr-odekake.net":
			selector = "div.ticketLayout__main"
			ok = n.Data == "div" && hasClass(n, "ticketLayout__main")
		case "www.jr-eki.com":
			selector = "div#contents div.dtl"
			ok = n.Data == "div" && hasClass(n, "dtl") && ancestorID(n, "contents") != nil
		case "www.jrkyushu-kippu.jp":
			selector = "div#jkContainer div#jkContents"
			ok = n.Data == "div" && attr(n, "id") == "jkContents" && ancestorID(n, "jkContainer") != nil
		}
		if ok {
			candidates = append(candidates, n)
		}
	})
	var roots []*html.Node
	var names []string
	for _, n := range candidates {
		identityRoot := n
		switch host {
		case "www.jreast.co.jp":
			shared := ancestorID(n, "mainContents")
			if shared == nil {
				continue
			}
			identityRoot = first(shared, func(x *html.Node) bool { return attr(x, "id") == "mainVisual" })
		case "www.jr-eki.com":
			shared := ancestorID(n, "contents")
			identityRoot = first(shared, func(x *html.Node) bool { return hasClass(x, "vi") })
		case "www.jrkyushu-kippu.jp":
			identityRoot = ancestorID(n, "jkContainer")
		case "tickets.jr-odekake.net":
			identityRoot = first(n, func(x *html.Node) bool {
				return x.Data == "article" && hasClass(x, "ticketArticle") && hasClass(x, "-detail")
			})
		}
		headings := operatorIdentityHeadings(identityRoot, t.NameJA)
		if len(headings) == 1 {
			roots = append(roots, n)
			names = append(names, text(headings[0]))
		}
	}
	if len(candidates) > 0 {
		if len(roots) != 1 {
			return nil, selector, "", "Known product layout has missing or ambiguous product identity; no rule evidence accepted."
		}
		return cloneOperatorProduct(roots[0], t.NameJA), selector, names[0], ""
	}
	// A compact semantic product article/section is acceptable when its own
	// heading identifies the target. Never widen this fallback to body/main.
	roots = nil
	names = nil
	seen := map[*html.Node]bool{}
	for _, h := range operatorIdentityHeadings(doc, t.NameJA) {
		if h.Data == "h3" || insideOperatorPanel(h) {
			continue
		}
		for n := h.Parent; n != nil; n = n.Parent {
			if n.Data == "article" || n.Data == "section" {
				if !seen[n] {
					roots = append(roots, n)
					names = append(names, text(h))
					seen[n] = true
				}
				break
			}
			if n.Data == "body" || n.Data == "main" {
				break
			}
		}
	}
	if len(roots) != 1 {
		return nil, "", "", "No unique product container bound to its own matching heading; document-wide rules are not accepted."
	}
	return cloneOperatorProduct(roots[0], t.NameJA), "matched-heading " + roots[0].Data, names[0], ""
}
func ancestorID(n *html.Node, id string) *html.Node {
	for x := n; x != nil; x = x.Parent {
		if attr(x, "id") == id {
			return x
		}
	}
	return nil
}
func operatorIdentityHeadings(root *html.Node, name string) []*html.Node {
	out := []*html.Node{}
	if root == nil || name == "" {
		return out
	}
	walk(root, func(n *html.Node) {
		if (n.Data == "h1" || n.Data == "h2" || n.Data == "h3") && !insideOperatorPanel(n) && strings.Contains(normName(text(n)), normName(name)) {
			out = append(out, n)
		}
	})
	return out
}
func operatorPanel(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.Data {
	case "aside", "nav", "footer", "script", "style":
		return true
	}
	v := strings.ToLower(attr(n, "class") + " " + attr(n, "id"))
	for _, s := range []string{"related", "recommend", "ranking", "sidebar", "sidesearch", "breadcrumb"} {
		if strings.Contains(v, s) {
			return true
		}
	}
	return false
}
func insideOperatorPanel(n *html.Node) bool {
	for x := n; x != nil; x = x.Parent {
		if operatorPanel(x) {
			return true
		}
	}
	return false
}
func cloneOperatorProduct(n *html.Node, name string) *html.Node {
	if n.Data == "article" {
		h := first(n, func(x *html.Node) bool { return x.Data == "h1" || x.Data == "h2" || x.Data == "h3" })
		if h == nil || !strings.Contains(normName(text(h)), normName(name)) {
			return nil
		}
	}
	if operatorPanel(n) {
		return nil
	}
	copy := &html.Node{Type: n.Type, DataAtom: n.DataAtom, Data: n.Data, Namespace: n.Namespace, Attr: append([]html.Attribute(nil), n.Attr...)}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if x := cloneOperatorProduct(c, name); x != nil {
			copy.AppendChild(x)
		}
	}
	return copy
}
