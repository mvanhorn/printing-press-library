package jreast

import (
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/cliutil"
	"golang.org/x/net/html"
	"strings"
)

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func class(n *html.Node, key string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == key {
			return true
		}
	}
	return false
}
func walk(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	out := make([]*html.Node, 0)
	var f func(*html.Node)
	f = func(n *html.Node) {
		if pred(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return out
}
func firstClass(n *html.Node, key string) *html.Node {
	a := walk(n, func(n *html.Node) bool { return class(n, key) })
	if len(a) > 0 {
		return a[0]
	}
	return nil
}
func text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Data == "script" || n.Data == "style" || n.Data == "noscript" {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.Join(strings.Fields(cliutil.CleanText(b.String())), " ")
}
func group(n *html.Node) (string, string) {
	for p := n.Parent; p != nil; p = p.Parent {
		if id := attr(p, "id"); strings.HasPrefix(id, "direction_") {
			h := walk(p, func(x *html.Node) bool { return x.Data == "h2" })
			name := ""
			if len(h) > 0 {
				name = text(h[0])
			}
			return id, name
		}
		if class(p, "area_wrapper") {
			h := walk(p, func(x *html.Node) bool { return x.Data == "h2" && strings.HasPrefix(attr(x, "id"), "direction_") })
			if len(h) > 0 {
				return attr(h[0], "id"), text(h[0])
			}
		}
	}
	return "", ""
}
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
