package crestronparse

import (
	"encoding/json"
	"strings"

	"golang.org/x/net/html"
)

// rawTextTags are elements whose contents are raw text per the HTML spec.
// Walking them as element subtrees leaks markup into extracted text.
var rawTextTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
}

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	if n == nil {
		return false
	}
	for _, f := range strings.Fields(attr(n, "class")) {
		if f == class {
			return true
		}
	}
	return false
}

func walk(n *html.Node, fn func(*html.Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func findAllByClass(root *html.Node, class string) []*html.Node {
	var out []*html.Node
	walk(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && hasClass(n, class) {
			out = append(out, n)
		}
		return true
	})
	return out
}

func firstByClass(root *html.Node, class string) *html.Node {
	var found *html.Node
	walk(root, func(n *html.Node) bool {
		if found != nil {
			return false
		}
		if n.Type == html.ElementNode && hasClass(n, class) {
			found = n
			return false
		}
		return true
	})
	return found
}

func firstByID(root *html.Node, id string) *html.Node {
	var found *html.Node
	walk(root, func(n *html.Node) bool {
		if found != nil {
			return false
		}
		if n.Type == html.ElementNode && attr(n, "id") == id {
			found = n
			return false
		}
		return true
	})
	return found
}

func findAllTags(root *html.Node, tag string) []*html.Node {
	var out []*html.Node
	walk(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == tag {
			out = append(out, n)
		}
		return true
	})
	return out
}

func firstTag(root *html.Node, tag string) *html.Node {
	var found *html.Node
	walk(root, func(n *html.Node) bool {
		if found != nil {
			return false
		}
		if n.Type == html.ElementNode && n.Data == tag {
			found = n
			return false
		}
		return true
	})
	return found
}

func findAllWithAttr(root *html.Node, key string) []*html.Node {
	var out []*html.Node
	walk(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && attr(n, key) != "" {
			out = append(out, n)
		}
		return true
	})
	return out
}

func directChildTags(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	if n == nil {
		return out
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			out = append(out, c)
		}
	}
	return out
}

// textOf returns the collapsed visible text of a subtree. Raw-text elements
// are skipped so script and style bodies never leak into extracted values.
func textOf(n *html.Node) string {
	var b strings.Builder
	walk(n, func(c *html.Node) bool {
		if c.Type == html.ElementNode && rawTextTags[c.Data] {
			// Script contents are wanted only when the caller asked for the
			// script node itself (JSON-LD extraction).
			return c == n
		}
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
			b.WriteString(" ")
		}
		if c.Type == html.ElementNode && (c.Data == "br" || c.Data == "p" || c.Data == "div" || c.Data == "tr") {
			b.WriteString(" ")
		}
		return true
	})
	return collapse(b.String())
}

func collapse(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\u200b", "")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// firstStringField reads a named string field out of a JSON value that may be
// an object or a bare string.
func firstStringField(raw json.RawMessage, field string) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if v, ok := m[field]; ok {
		var out string
		if json.Unmarshal(v, &out) == nil {
			return out
		}
	}
	return ""
}

// firstStringElem reads the first element of a JSON value that may be a string
// or an array of strings.
func firstStringElem(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		return arr[0]
	}
	return ""
}
