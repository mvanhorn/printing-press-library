// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

// Package faaparse parses the FAA aircraftinquiry HTML pages into typed
// structures. The registry's markup is semantic: every value cell carries a
// data-label attribute and every section table a devkit-table-title caption,
// so extraction keys on those rather than on layout.
package faaparse

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

// Detail is a parsed registration detail page (N-number inquiry result).
type Detail struct {
	NNumber               string                         `json:"n_number,omitempty"`
	Status                string                         `json:"status,omitempty"`
	Description           map[string]string              `json:"description,omitempty"`
	Owner                 map[string]string              `json:"owner,omitempty"`
	Airworthiness         map[string]string              `json:"airworthiness,omitempty"`
	OtherOwnerNames       []string                       `json:"other_owner_names,omitempty"`
	TemporaryCertificates []map[string]string            `json:"temporary_certificates,omitempty"`
	FuelModifications     []string                       `json:"fuel_modifications,omitempty"`
	OtherSections         map[string][]map[string]string `json:"other_sections,omitempty"`
}

// List is a parsed multi-row result page (name, make/model, state, ... searches).
type List struct {
	Rows        []map[string]string `json:"rows"`
	ShowingFrom int                 `json:"showing_from,omitempty"`
	ShowingTo   int                 `json:"showing_to,omitempty"`
	Total       int                 `json:"total,omitempty"`
	Page        int                 `json:"page,omitempty"`
	Pages       int                 `json:"pages,omitempty"`
}

// Result is what ParseAuto returns: a detail page, a list page, or both view
// of the same document (a serial-number result is a list; an N-number result
// is a detail).
type Result struct {
	Kind   string  `json:"kind"` // "detail" or "list"
	Detail *Detail `json:"detail,omitempty"`
	List   *List   `json:"list,omitempty"`
	Error  string  `json:"error,omitempty"` // registry-reported error banner, if any
}

var (
	bannerRe  = regexp.MustCompile(`^\s*(N?[0-9A-Z]{1,6})\s+is\s+(.+?)\s*$`)
	showingRe = regexp.MustCompile(`Showing\s+([\d,]+)\s*-\s*([\d,]+)\s+of\s+([\d,]+)\s*\(Page\s+(\d+)\s+of\s+([\d,]+)\)`)
	enteredRe = regexp.MustCompile(`(?i)Entered:\s*(.+?)\s*$`)
	spacesRe  = regexp.MustCompile(`\s+`)
)

type table struct {
	caption string
	rows    []map[string]string // data-label -> text, in document order
	headers []string
}

// ParseAuto parses any aircraftinquiry result page. Pages whose tables carry
// detail-section captions (Aircraft Description, Registered Owner) parse as a
// Detail; pages with header-row tables parse as a List.
func ParseAuto(doc []byte) (*Result, error) {
	root, err := xhtml.Parse(bytes.NewReader(doc))
	if err != nil {
		return nil, err
	}
	tables := collectTables(root)
	res := &Result{}

	if msg := findErrorBanner(root); msg != "" {
		res.Error = msg
	}

	isDetail := false
	for _, t := range tables {
		c := strings.ToLower(t.caption)
		if strings.Contains(c, "aircraft description") || strings.Contains(c, "registered owner") {
			isDetail = true
			break
		}
	}

	if isDetail {
		res.Kind = "detail"
		res.Detail = buildDetail(root, tables)
		return res, nil
	}

	res.Kind = "list"
	res.List = buildList(root, tables)
	return res, nil
}

// ParseDetail parses a registration detail page.
func ParseDetail(doc []byte) (*Detail, error) {
	r, err := ParseAuto(doc)
	if err != nil {
		return nil, err
	}
	if r.Detail == nil {
		return nil, &NotDetailError{Kind: r.Kind, Error_: r.Error}
	}
	return r.Detail, nil
}

// ParseList parses a multi-row result page.
func ParseList(doc []byte) (*List, error) {
	r, err := ParseAuto(doc)
	if err != nil {
		return nil, err
	}
	if r.List == nil {
		return nil, &NotListError{Kind: r.Kind, Error_: r.Error}
	}
	return r.List, nil
}

// NotDetailError reports that the page was not a detail page.
type NotDetailError struct {
	Kind   string
	Error_ string
}

func (e *NotDetailError) Error() string {
	if e.Error_ != "" {
		return "registry: " + e.Error_
	}
	return "page is not a registration detail page (kind: " + e.Kind + ")"
}

// NotListError reports that the page was not a list page.
type NotListError struct {
	Kind   string
	Error_ string
}

func (e *NotListError) Error() string {
	if e.Error_ != "" {
		return "registry: " + e.Error_
	}
	return "page is not a result list page (kind: " + e.Kind + ")"
}

func buildDetail(root *xhtml.Node, tables []table) *Detail {
	d := &Detail{}
	if tail, status := findBanner(root); tail != "" {
		d.NNumber, d.Status = tail, status
	}
	for _, t := range tables {
		c := strings.ToLower(t.caption)
		switch {
		case strings.Contains(c, "aircraft description"):
			d.Description = mergeRows(t.rows)
		case strings.Contains(c, "registered owner"):
			d.Owner = mergeRows(t.rows)
		case strings.Contains(c, "other owner names"):
			for _, r := range t.rows {
				for _, v := range r {
					if v != "" {
						d.OtherOwnerNames = append(d.OtherOwnerNames, v)
					}
				}
			}
		case strings.Contains(c, "temporary certificates"):
			for _, r := range t.rows {
				if len(r) > 0 {
					d.TemporaryCertificates = append(d.TemporaryCertificates, snakeKeys(r))
				}
			}
		case strings.Contains(c, "fuel modifications"):
			for _, r := range t.rows {
				for _, v := range r {
					if v != "" && !strings.EqualFold(v, "none") {
						d.FuelModifications = append(d.FuelModifications, v)
					}
				}
			}
		case strings.Contains(c, "entered"):
			// "N-Number Entered: 16qs" — redundant with the banner; skip.
		default:
			labeled := hasLabels(t.rows)
			if !labeled {
				continue
			}
			// The airworthiness table's caption is a styled warning, not a
			// title; recognize it by its distinctive labels.
			if rowsHaveLabel(t.rows, "Type Certificate Data Sheet") || rowsHaveLabel(t.rows, "A/W Date") {
				d.Airworthiness = mergeRows(t.rows)
				continue
			}
			if d.OtherSections == nil {
				d.OtherSections = map[string][]map[string]string{}
			}
			name := snake(t.caption)
			if name == "" {
				name = "unlabeled"
			}
			var rows []map[string]string
			for _, r := range t.rows {
				if len(r) > 0 {
					rows = append(rows, snakeKeys(r))
				}
			}
			if len(rows) > 0 {
				d.OtherSections[name] = append(d.OtherSections[name], rows...)
			}
		}
	}
	d.Description = snakeKeys(d.Description)
	d.Owner = snakeKeys(d.Owner)
	d.Airworthiness = snakeKeys(d.Airworthiness)
	return d
}

func buildList(root *xhtml.Node, tables []table) *List {
	l := &List{Rows: []map[string]string{}}
	for _, t := range tables {
		if !hasLabels(t.rows) {
			continue
		}
		// Some result pages append an aggregate table (aircraft-type counts
		// with a SubTotal column); it is not part of the result rows.
		if isSummarySection(t) {
			continue
		}
		for _, r := range t.rows {
			if len(r) > 0 {
				l.Rows = append(l.Rows, snakeKeys(r))
			}
		}
	}
	if m := showingRe.FindStringSubmatch(textContent(root)); m != nil {
		l.ShowingFrom = atoiComma(m[1])
		l.ShowingTo = atoiComma(m[2])
		l.Total = atoiComma(m[3])
		l.Page = atoiComma(m[4])
		l.Pages = atoiComma(m[5])
	}
	return l
}

// mergeRows flattens label/value rows of a two-column detail table into one map.
func mergeRows(rows []map[string]string) map[string]string {
	out := map[string]string{}
	for _, r := range rows {
		for k, v := range r {
			if k == "" {
				continue
			}
			if _, seen := out[k]; seen && v == "" {
				continue
			}
			if v != "" || out[k] == "" {
				if cur, seen := out[k]; !seen || cur == "" {
					out[k] = v
				} else if v != "" && cur != v {
					out[k] = cur + "\n" + v
				}
			}
		}
	}
	return out
}

// isSummarySection reports whether a section is an aggregate-count footer
// (headers or labels like "SubTotal") rather than result rows.
func isSummarySection(t table) bool {
	for _, h := range t.headers {
		if strings.EqualFold(h, "SubTotal") {
			return true
		}
	}
	for _, r := range t.rows {
		if _, ok := r["SubTotal"]; ok {
			return true
		}
	}
	return false
}

func hasLabels(rows []map[string]string) bool {
	for _, r := range rows {
		if len(r) > 0 {
			return true
		}
	}
	return false
}

func rowsHaveLabel(rows []map[string]string, label string) bool {
	for _, r := range rows {
		if _, ok := r[label]; ok {
			return true
		}
	}
	return false
}

func snakeKeys(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[snake(k)] = v
	}
	return out
}

func snake(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastUnderscore := true // avoid leading underscore
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.TrimRight(b.String(), "_")
}

func atoiComma(s string) int {
	n, _ := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	return n
}

// collectTables walks the document collecting caption-delimited sections.
// FAA pages nest multiple <caption> elements inside a single <table>, using
// them as section dividers (Aircraft Description, Registered Owner, ...), so
// each caption starts a new logical section and the rows that follow it in
// document order belong to that section.
func collectTables(root *xhtml.Node) []table {
	var tables []table
	var walk func(n *xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "table" {
			tables = append(tables, parseTable(n)...)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return tables
}

func parseTable(tbl *xhtml.Node) []table {
	var sections []table
	cur := table{}
	started := false
	var currentRow map[string]string
	flushRow := func() {
		if len(currentRow) > 0 {
			cur.rows = append(cur.rows, currentRow)
		}
		currentRow = nil
	}
	flushSection := func() {
		flushRow()
		if started && (cur.caption != "" || len(cur.rows) > 0 || len(cur.headers) > 0) {
			sections = append(sections, cur)
		}
		cur = table{}
	}
	var walk func(n *xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "caption":
				txt := cleanText(textContent(n))
				// Styled empty captions (the red warning slot) don't start a
				// new section.
				if txt != "" {
					flushSection()
					started = true
					cur.caption = txt
				}
				return
			case "th":
				started = true
				if h := cleanText(textContent(n)); h != "" {
					cur.headers = append(cur.headers, h)
				}
				return
			case "tr":
				started = true
				flushRow()
				currentRow = map[string]string{}
			case "td":
				// Cells with an empty data-label are visual label cells
				// (the field name rendered for humans); only data-label-
				// bearing cells carry values.
				label := attr(n, "data-label")
				if label == "" {
					return
				}
				val := cleanText(textContent(n))
				if currentRow == nil {
					currentRow = map[string]string{}
				}
				if existing, ok := currentRow[label]; ok && existing != "" && val != "" && existing != val {
					currentRow[label] = existing + "\n" + val
				} else if _, ok := currentRow[label]; !ok || val != "" {
					currentRow[label] = val
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(tbl)
	flushSection()
	return sections
}

// findBanner locates the "<tail> is <Status>" paragraph.
func findBanner(root *xhtml.Node) (string, string) {
	var tail, status string
	var walk func(n *xhtml.Node) bool
	walk = func(n *xhtml.Node) bool {
		if n.Type == xhtml.TextNode {
			if m := bannerRe.FindStringSubmatch(cleanText(n.Data)); m != nil {
				tail, status = m[1], m[2]
				return true
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if walk(c) {
				return true
			}
		}
		return false
	}
	walk(root)
	return tail, status
}

// findErrorBanner surfaces the registry's red error captions/messages.
func findErrorBanner(root *xhtml.Node) string {
	var msg string
	var walk func(n *xhtml.Node)
	walk = func(n *xhtml.Node) {
		if msg != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "caption" {
			if strings.Contains(attr(n, "style"), "color:red") {
				txt := cleanText(textContent(n))
				// The airworthiness disclaimer is also styled red; only treat
				// short non-disclaimer text as an error.
				if txt != "" && len(txt) < 120 && !strings.Contains(strings.ToLower(txt), "airworthiness") {
					msg = txt
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return msg
}

func attr(n *xhtml.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func textContent(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(n *xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		if n.Type == xhtml.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func cleanText(s string) string {
	return strings.TrimSpace(spacesRe.ReplaceAllString(s, " "))
}
