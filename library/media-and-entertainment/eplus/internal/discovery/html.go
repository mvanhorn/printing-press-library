package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"net/url"
	"regexp"
	"strings"
)

func document(b []byte) (*html.Node, error) { return html.Parse(bytes.NewReader(b)) }
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
func hasClass(n *html.Node, c string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == c {
			return true
		}
	}
	return false
}
func nodes(n *html.Node, match func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if match(x) {
			out = append(out, x)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	if n != nil {
		walk(n)
	}
	return out
}
func classes(n *html.Node, c string) []*html.Node {
	return nodes(n, func(x *html.Node) bool { return hasClass(x, c) })
}
func first(n *html.Node, c string) *html.Node {
	v := classes(n, c)
	if len(v) > 0 {
		return v[0]
	}
	return nil
}
func text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style" || x.Data == "input" || x.Data == "select") {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteString(" ")
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func rawText(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}
func ct(n *html.Node, c string) string { return text(first(n, c)) }
func absolute(base, s string) string {
	b, e := url.Parse(base)
	if e != nil {
		return ""
	}
	u, e := url.Parse(s)
	if e != nil {
		return ""
	}
	return b.ResolveReference(u).String()
}
func canonical(n *html.Node, fallback string) string {
	for _, x := range nodes(n, func(x *html.Node) bool { return x.Data == "link" && attr(x, "rel") == "canonical" }) {
		if s := attr(x, "href"); s != "" {
			return absolute(fallback, s)
		}
	}
	return fallback
}
func heading(n *html.Node) string {
	a := nodes(n, func(x *html.Node) bool { return x.Data == "h1" })
	if len(a) > 0 {
		return text(a[0])
	}
	return ""
}

var domesticIDRE = regexp.MustCompile(`^[0-9]{10}(?:-P003[0-9]{4}(?:P021[0-9]{3})?)?$`)
var bookingRE = regexp.MustCompile(`https://(?:sp\.)?atom\.eplus\.jp/sys/main\.jsp\?[^'"\s]+`)

func DomesticURL(id string) (string, error) {
	if strings.HasPrefix(id, "https://eplus.jp/sf/detail/") {
		u, e := url.Parse(id)
		if e != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", fmt.Errorf("use a domestic detail ID or canonical HTTPS detail URL")
		}
		id = strings.TrimPrefix(u.Path, "/sf/detail/")
	}
	if !domesticIDRE.MatchString(id) {
		return "", fmt.Errorf("invalid domestic detail ID %q", id)
	}
	return "https://eplus.jp/sf/detail/" + id, nil
}
func eventID(u string) string {
	m := regexp.MustCompile(`/sf/detail/(\d{10})`).FindStringSubmatch(u)
	if m != nil {
		return m[1]
	}
	return ""
}
func bookingParts(u string) Row {
	r := Row{}
	q, e := url.Parse(u)
	if e != nil {
		return r
	}
	for _, v := range strings.Split(q.Query().Get("prm"), ":") {
		a := strings.SplitN(v, "=", 2)
		if len(a) == 2 {
			r[a[0]] = a[1]
		}
	}
	return r
}
func searchJSON(n *html.Node) (Row, error) {
	v := nodes(n, func(x *html.Node) bool { return x.Data == "script" && attr(x, "id") == "json" })
	if len(v) == 0 {
		return nil, fmt.Errorf("domestic search contract missing script#json (challenge or page changed)")
	}
	var envelope Row
	if e := json.Unmarshal([]byte(rawText(v[0])), &envelope); e != nil {
		return nil, fmt.Errorf("domestic search JSON: %w", e)
	}
	if envelope["error"] != nil {
		return nil, fmt.Errorf("domestic search returned an error")
	}
	data := obj(envelope["data"])
	if _, ok := data["record_list"]; !ok {
		return nil, fmt.Errorf("domestic search record_list missing")
	}
	return data, nil
}
