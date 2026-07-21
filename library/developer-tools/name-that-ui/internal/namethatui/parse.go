package namethatui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func ParseComponents(page []byte, baseURL string) ([]Component, error) {
	texts := rscTexts(page)
	for _, text := range texts {
		raw, ok := balancedAfter(text, `"entries":`)
		if !ok {
			continue
		}
		var out []Component
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode component entries: %w", err)
		}
		for i := range out {
			out[i].ID = out[i].Platform + "/" + out[i].Slug
			out[i].SourceURL = strings.TrimRight(baseURL, "/") + "/" + out[i].Platform + "/" + out[i].Slug
			out[i].AKA = nonNil(out[i].AKA)
			out[i].Fuzzy = nonNil(out[i].Fuzzy)
			out[i].API = nonNil(out[i].API)
			out[i].Parts = nonNil(out[i].Parts)
			out[i].Related = nonNil(out[i].Related)
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return []Component{}, fmt.Errorf("NameThatUI RSC entries array not found")
}

func ParseStylesIndex(page []byte, baseURL string) ([]Style, error) {
	doc, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return nil, err
	}
	var out []Style
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" && attr(n, "type") == "application/ld+json" {
			var v any
			if json.Unmarshal([]byte(nodeText(n)), &v) == nil {
				addStyles(&out, v, baseURL)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(out) == 0 {
		return []Style{}, fmt.Errorf("NameThatUI styles ItemList not found")
	}
	return out, nil
}

func ParseStylePage(page []byte, style Style) (Style, error) {
	style.Signals = []Signal{}
	style.Sections = []Section{}
	for _, text := range rscTexts(page) {
		if raw, ok := balancedAfter(text, `"signals":`); ok {
			if err := json.Unmarshal(raw, &style.Signals); err != nil {
				return style, fmt.Errorf("decode style signals: %w", err)
			}
			break
		}
	}
	doc, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return style, fmt.Errorf("parse style HTML: %w", err)
	}
	var current *Section
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "h1" || n.Data == "h2" || n.Data == "h3" {
				h := strings.TrimSpace(nodeText(n))
				if h != "" {
					style.Sections = append(style.Sections, Section{Heading: normalizeHeading(h), SourceURL: style.SourceURL})
					current = &style.Sections[len(style.Sections)-1]
				}
			} else if current != nil && (n.Data == "p" || n.Data == "li") {
				t := strings.TrimSpace(nodeText(n))
				if t != "" {
					if current.Text != "" {
						current.Text += "\n"
					}
					current.Text += t
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	for i := range style.Sections {
		sum := sha256.Sum256([]byte(style.Sections[i].Heading + "\n" + style.Sections[i].Text))
		style.Sections[i].ContentHash = hex.EncodeToString(sum[:])
	}
	return style, nil
}

func addStyles(out *[]Style, v any, base string) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if typ, _ := m["@type"].(string); typ == "ItemList" {
		if xs, ok := m["itemListElement"].([]any); ok {
			for _, x := range xs {
				z, _ := x.(map[string]any)
				name, _ := z["name"].(string)
				href, _ := z["url"].(string)
				u, err := url.Parse(href)
				if err == nil {
					u = mustResolve(base, u)
					slug := strings.Trim(strings.TrimPrefix(u.Path, "/styles"), "/")
					if slug != "" {
						*out = append(*out, Style{ID: slug, Slug: slug, Name: name, SourceURL: u.String(), Signals: []Signal{}, Sections: []Section{}})
					}
				}
			}
		}
	}
	for _, x := range m {
		addStyles(out, x, base)
	}
}
func mustResolve(base string, u *url.URL) *url.URL {
	b, _ := url.Parse(base)
	return b.ResolveReference(u)
}
func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}
func nodeText(n *html.Node) string {
	var b strings.Builder
	var f func(*html.Node)
	f = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return b.String()
}
func normalizeHeading(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }
func nonNil[T any](v []T) []T {
	if v == nil {
		return make([]T, 0)
	}
	return v
}
