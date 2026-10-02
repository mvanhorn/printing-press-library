// Copyright 2026 zjsng. Licensed under Apache-2.0.
package cycling

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Area struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	URL  string `json:"source_url"`
}
type PriceCell struct {
	Text    string `json:"source_text"`
	Yen     *int   `json:"yen"`
	Setting string `json:"setting"`
}
type PriceRow struct {
	Duration string               `json:"duration_label"`
	Rates    map[string]PriceCell `json:"rates_by_model"`
}
type PriceTable struct {
	Area   string     `json:"area"`
	Models []string   `json:"models"`
	Rows   []PriceRow `json:"rows"`
}

var slugRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
var yenRE = regexp.MustCompile(`^[0-9,]+円$`)

func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}
func text(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
		return ""
	}
	var s strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s.WriteString(text(c))
	}
	return strings.Join(strings.Fields(s.String()), " ")
}
func descendants(n *html.Node, tag string) []*html.Node {
	out := make([]*html.Node, 0)
	var walk func(*html.Node)
	walk = func(p *html.Node) {
		if p.Type == html.ElementNode && p.Data == tag {
			out = append(out, p)
		}
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

// ParseAreas accepts only regional URLs actually advertised by the source index.
func ParseAreas(body []byte) ([]Area, error) {
	doc, e := html.Parse(bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	out := make([]Area, 0)
	seen := map[string]bool{}
	for _, n := range descendants(doc, "a") {
		u, e := url.Parse(attr(n, "href"))
		if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			continue
		}
		if u.Host != "" && u.Host != "www.hellocycling.jp" {
			continue
		}
		if u.Scheme != "" && u.Scheme != "https" {
			continue
		}
		path := strings.Trim(u.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[0] != "price" || !slugRE.MatchString(parts[1]) || seen[parts[1]] {
			continue
		}
		seen[parts[1]] = true
		out = append(out, Area{parts[1], text(n), PriceURL + parts[1] + "/"})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("pricing index has no advertised areas; source layout may have changed")
	}
	if len(out) > 60 {
		return nil, fmt.Errorf("pricing area list exceeds 60; source layout requires review")
	}
	return out, nil
}
func cells(row *html.Node, tag string) []string {
	out := make([]string, 0)
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			out = append(out, text(c))
		}
	}
	return out
}

// ParsePrices retains each municipality table and model column without flattening exceptions.
func ParsePrices(body []byte, areaName string) ([]PriceTable, error) {
	doc, e := html.Parse(bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	mains := descendants(doc, "main")
	if len(mains) != 1 {
		return nil, fmt.Errorf("price page is missing the source main section")
	}
	out := make([]PriceTable, 0)
	for _, table := range descendants(mains[0], "table") {
		label := areaName
		for p := table.Parent; p != nil; p = p.Parent {
			if p.Type == html.ElementNode && p.Data == "details" {
				summary := descendants(p, "summary")
				if len(summary) > 0 {
					label = text(summary[0])
				}
				break
			}
		}
		pt := PriceTable{Area: label, Models: make([]string, 0), Rows: make([]PriceRow, 0)}
		heads := descendants(table, "thead")
		if len(heads) != 1 {
			return nil, fmt.Errorf("price table %s has unexpected header structure", label)
		}
		for _, tr := range descendants(heads[0], "tr") {
			if list := cells(tr, "td"); len(list) > 0 {
				pt.Models = list
			}
		}
		if len(pt.Models) == 0 || len(pt.Models) > 10 {
			return nil, fmt.Errorf("price table %s model columns changed", label)
		}
		bodies := descendants(table, "tbody")
		if len(bodies) != 1 {
			return nil, fmt.Errorf("price table %s missing body", label)
		}
		for _, tr := range descendants(bodies[0], "tr") {
			labels := cells(tr, "th")
			values := cells(tr, "td")
			if len(labels) != 1 || len(values) != len(pt.Models) {
				return nil, fmt.Errorf("price table %s row/model count mismatch", label)
			}
			row := PriceRow{Duration: labels[0], Rates: map[string]PriceCell{}}
			for i, value := range values {
				cell := PriceCell{Text: value, Setting: "unknown"}
				if value == "設定なし" {
					cell.Setting = "not_set"
				} else if yenRE.MatchString(value) {
					n, e := strconv.Atoi(strings.TrimSuffix(strings.ReplaceAll(value, ",", ""), "円"))
					if e != nil {
						return nil, e
					}
					cell.Yen = &n
					cell.Setting = "published"
				}
				row.Rates[pt.Models[i]] = cell
			}
			pt.Rows = append(pt.Rows, row)
		}
		if len(pt.Rows) == 0 || len(pt.Rows) > 20 {
			return nil, fmt.Errorf("price table %s row count outside bounds", label)
		}
		out = append(out, pt)
	}
	if len(out) == 0 || len(out) > 30 {
		return nil, fmt.Errorf("price page has no valid bounded rate tables")
	}
	return out, nil
}
func (c *Client) PriceAreas(ctx context.Context) ([]Area, int64, error) {
	b, e := c.get(ctx, PriceURL, pageLimit)
	if e != nil {
		return nil, 0, e
	}
	a, e := ParseAreas(b)
	return a, int64(len(b)), e
}
func (c *Client) Prices(ctx context.Context, slug string) (map[string]any, error) {
	if !slugRE.MatchString(slug) {
		return nil, fmt.Errorf("--area must be an advertised slug; use pricing areas")
	}
	areas, n, e := c.PriceAreas(ctx)
	if e != nil {
		return nil, e
	}
	var area *Area
	for _, a := range areas {
		if a.Slug == slug {
			copy := a
			area = &copy
			break
		}
	}
	if area == nil {
		return nil, fmt.Errorf("--area %q is not advertised; use pricing areas", slug)
	}
	b, e := c.get(ctx, area.URL, pageLimit)
	if e != nil {
		return nil, e
	}
	tables, e := ParsePrices(b, area.Name)
	if e != nil {
		return nil, e
	}
	return map[string]any{"meta": map[string]any{"source": "live", "observed_at": time.Now().UTC(), "timezone": "Asia/Tokyo", "source_url": area.URL, "request_count": 2, "response_bytes": n + int64(len(b)), "currency": "JPY"}, "results": tables, "constraints": []string{"Confirm the exact rate in the app before starting. Area and vehicle model can change the rate, even at the same station.", "Fractions of a minute round up. Where no initial 30-minute price is set, 15-minute pricing applies immediately.", "Municipality exception tables override the area's base table. A 'not_set' cell is not a zero price and does not establish vehicle availability.", "These are source-published price bands, not a ride quote. GBFS class 2 cannot identify city/sports/e-Bike models or assign these prices to a station bike."}, "constraints_source_url": PriceURL}, nil
}
