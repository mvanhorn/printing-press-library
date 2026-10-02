// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ferry

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func parseHTML(b []byte) (*html.Node, error) { return html.Parse(bytes.NewReader(b)) }
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func nodes(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if pred(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}
func tag(n *html.Node, s string) bool { return n.Type == html.ElementNode && n.Data == s }
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if tag(n, "script") || tag(n, "style") || n.Type == html.CommentNode {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// The portal adds informational link captions inside some sailing headings.
// Keep the sailing label itself while excluding those navigation captions.
func sailingText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if tag(n, "a") || tag(n, "script") || tag(n, "style") || n.Type == html.CommentNode {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func article(doc *html.Node) *html.Node {
	for _, n := range nodes(doc, func(n *html.Node) bool { return tag(n, "article") && attr(n, "id") == "article" }) {
		return n
	}
	return doc
}
func elements(n *html.Node, name string) []*html.Node {
	return nodes(n, func(n *html.Node) bool { return tag(n, name) })
}
func short(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
func tableRows(table *html.Node) [][]*html.Node {
	var rows [][]*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n != table && tag(n, "table") {
			return
		}
		if tag(n, "tr") {
			var cells []*html.Node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if tag(c, "th") || tag(c, "td") {
					cells = append(cells, c)
				}
			}
			rows = append(rows, cells)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}
func tableGrid(table *html.Node) ([][]string, error) {
	type span struct {
		value     string
		remaining int
	}
	carry := map[int]span{}
	var out [][]string
	for _, cells := range tableRows(table) {
		if len(out) > 100 {
			return nil, fmt.Errorf("source table row limit exceeded")
		}
		row := make([]string, 20)
		occupied := map[int]bool{}
		width := 0
		for col, s := range carry {
			row[col] = s.value
			occupied[col] = true
			if col+1 > width {
				width = col + 1
			}
			s.remaining--
			if s.remaining == 0 {
				delete(carry, col)
			} else {
				carry[col] = s
			}
		}
		col := 0
		for _, cell := range cells {
			for occupied[col] {
				col++
			}
			cs, _ := strconv.Atoi(attr(cell, "colspan"))
			rs, _ := strconv.Atoi(attr(cell, "rowspan"))
			if cs == 0 {
				cs = 1
			}
			if rs == 0 {
				rs = 1
			}
			if cs < 1 || rs < 1 || cs > 10 || rs > 50 || col+cs > 20 {
				return nil, fmt.Errorf("unexpected source table span")
			}
			v := text(cell)
			for j := 0; j < cs; j++ {
				row[col+j] = v
				occupied[col+j] = true
				if rs > 1 {
					carry[col+j] = span{v, rs - 1}
				}
			}
			col += cs
			if col > width {
				width = col
			}
		}
		out = append(out, row[:width])
	}
	return out, nil
}

var timeRE = regexp.MustCompile(`\b([0-2][0-9]:[0-5][0-9])\b`)

func extractTime(s string) (string, error) {
	m := timeRE.FindStringSubmatch(s)
	if len(m) < 2 || m[1][:2] > "23" {
		return "", fmt.Errorf("invalid source time %q", short(s, 70))
	}
	return m[1], nil
}
func weekdayNumbers(s string) ([]int, error) {
	clean := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(s), ".", ""), " ", "")
	labels := map[string][]int{"sun-thu": {0, 1, 2, 3, 4}, "fri-sat": {5, 6}, "mon-thu": {1, 2, 3, 4}, "fri": {5}, "sat": {6}, "sun": {0}}
	if v, ok := labels[clean]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("unsupported source weekday label %q", s)
}
func ParseTimetable(b []byte, r Route, line string) (Timetable, error) {
	doc, err := parseHTML(b)
	if err != nil {
		return Timetable{}, err
	}
	a := article(doc)
	var table *html.Node
	for _, t := range elements(a, "table") {
		if strings.Contains(text(t), "Day of the week") {
			table = t
			break
		}
	}
	if table == nil {
		return Timetable{}, fmt.Errorf("official timetable shape changed: weekday table missing")
	}
	grid, err := tableGrid(table)
	if err != nil {
		return Timetable{}, err
	}
	if len(grid) < 3 {
		return Timetable{}, fmt.Errorf("official timetable has no weekday data rows")
	}
	idx := 1
	direction := "outbound"
	if line == r.InboundLine {
		idx = 3
		direction = "inbound"
	}
	out := Timetable{Route: r, Line: line, Direction: direction, Rules: []Rule{}, DateValidity: "Published normal weekday pattern; exceptions, cancellations and dated sailing inventory require the official date lookup."}
	for _, p := range elements(a, "p") {
		s := text(p)
		if strings.HasPrefix(s, "since ") {
			out.SourceEffectiveCaption = s
			break
		}
	}
	for _, row := range grid[2:] {
		if len(row) != 5 {
			return out, fmt.Errorf("official timetable shape changed: expected five weekday columns")
		}
		nums, err := weekdayNumbers(row[0])
		if err != nil {
			return out, err
		}
		dep, err := extractTime(row[idx])
		if err != nil {
			return out, err
		}
		arr, err := extractTime(row[idx+1])
		if err != nil {
			return out, err
		}
		if !strings.Contains(strings.ToLower(row[idx+1]), "next morning") {
			return out, fmt.Errorf("source arrival day offset is not established")
		}
		var days []string
		for _, d := range nums {
			days = append(days, []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}[d])
		}
		out.Rules = append(out.Rules, Rule{days, nums, row[0], dep, arr, 1})
	}
	if len(out.Rules) == 0 {
		return out, fmt.Errorf("official timetable has no weekday rules")
	}
	return out, nil
}
