// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// HTML parser for the Extron literature index pages.

package extron

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	headingRe   = regexp.MustCompile(`(?s)<h([1-6])[^>]*>(.*?)</h[1-6]>`)
	rowRe       = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	fileLinkRe  = regexp.MustCompile(`(?s)<a[^>]*idFileUrl[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	nobrRe      = regexp.MustCompile(`(?s)<nobr[^>]*>(.*?)</nobr>`)
	tagRe       = regexp.MustCompile(`(?s)<[^>]+>`)
	nextLinkRe  = regexp.MustCompile(`(?s)<a\b[^>]*>`)
	hrefRe      = regexp.MustCompile(`href="([^"]+)"`)
	classNextRe = regexp.MustCompile(`class="[^"]*link-next[^"]*"`)
	filetypeRe  = regexp.MustCompile(`filetype=(\d+)`)
	tabidRe     = regexp.MustCompile(`tabid=(\d+)`)
	pageRe      = regexp.MustCompile(`page=(\d+)`)
)

// categoryFromHeading turns "Brochure (M - 95 files)" or "Revit BIM files"
// into the category name ("Brochure", "Revit BIM").
func categoryFromHeading(raw string) string {
	text := strings.TrimSpace(tagRe.ReplaceAllString(raw, ""))
	text = strings.TrimSpace(text)
	lower := strings.ToLower(text)
	if i := strings.Index(lower, " ("); i > 0 {
		return strings.TrimSpace(text[:i])
	}
	if strings.HasSuffix(lower, " files") {
		return strings.TrimSpace(text[:len(text)-len(" files")])
	}
	return text
}

// PageRef describes a "Next" pagination link for one category's table on an
// index page. The literature library pages each category at 20 rows/page via
// filetype+tabid+page query params.
type PageRef struct {
	Category string `json:"category"`
	Filetype string `json:"filetype"`
	Tabid    string `json:"tabid"`
	Page     int    `json:"page"`
}

// ParseIndex extracts literature rows from a literature.aspx page body. Each
// row's category is the heading of the table it appeared under.
func ParseIndex(body []byte) ([]Doc, error) {
	docs, _, err := ParseIndexWithRefs(body)
	return docs, err
}

// ParseIndexWithRefs parses the docs and, per category, the first "Next"
// pagination link (page=2) so callers can page through large categories.
func ParseIndexWithRefs(body []byte) ([]Doc, map[string]PageRef, error) {
	html := string(body)
	sections := splitByHeadings(html)
	docs := make([]Doc, 0)
	refs := make(map[string]PageRef)
	seen := map[string]bool{}
	for _, sec := range sections {
		category := sec.heading
		for _, row := range rowRe.FindAllStringSubmatch(sec.body, -1) {
			if len(row) < 2 {
				continue
			}
			cell := row[1]
			m := fileLinkRe.FindStringSubmatch(cell)
			if m == nil {
				continue
			}
			href := strings.TrimSpace(m[1])
			title := strings.Join(strings.Fields(tagRe.ReplaceAllString(m[2], "")), " ")
			if title == "" || href == "" {
				continue
			}
			// The row's <nobr> cells are Rev, Date, Size, Type in order.
			nobrs := make([]string, 0, 4)
			for _, n := range nobrRe.FindAllStringSubmatch(cell, -1) {
				nobrs = append(nobrs, strings.TrimSpace(tagRe.ReplaceAllString(n[1], "")))
			}
			doc := Doc{
				Title:    title,
				Category: category,
				URL:      href,
			}
			if len(nobrs) > 0 {
				doc.Rev = nobrs[0]
			}
			if len(nobrs) > 1 {
				doc.Date = nobrs[1]
			}
			if len(nobrs) > 2 {
				doc.Size = nobrs[2]
			}
			if len(nobrs) > 3 {
				doc.Type = nobrs[3]
			}
			if seen[doc.URL] {
				continue
			}
			seen[doc.URL] = true
			docs = append(docs, doc)
		}
		// First Next link inside this section identifies the category's
		// pagination parameters.
		if _, ok := refs[category]; !ok {
			if href := firstNextHref(sec.body); href != "" {
				refs[category] = refFromHref(category, href)
			}
		}
	}
	if len(docs) == 0 {
		return nil, nil, fmt.Errorf("no literature rows parsed from index page (page shape may have changed)")
	}
	return docs, refs, nil
}

// firstNextHref returns the href of the first anchor whose class contains
// "link-next" (the per-category "Next" pagination link).
func firstNextHref(body string) string {
	for _, a := range nextLinkRe.FindAllString(body, -1) {
		if classNextRe.MatchString(a) {
			if m := hrefRe.FindStringSubmatch(a); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

func refFromHref(category, href string) PageRef {	ref := PageRef{Category: category, Page: 2}
	if m := filetypeRe.FindStringSubmatch(href); m != nil {
		ref.Filetype = m[1]
	}
	if m := tabidRe.FindStringSubmatch(href); m != nil {
		ref.Tabid = m[1]
	}
	if m := pageRe.FindStringSubmatch(href); m != nil {
		ref.Page = atoiDefault(m[1], 2)
	}
	return ref
}

func atoiDefault(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return def
	}
	return n
}

// ParseCategoryPage parses a paginated single-category page (20 rows, no
// category headings) and tags every row with the given category.
func ParseCategoryPage(body []byte, category string) ([]Doc, error) {
	html := string(body)
	docs := make([]Doc, 0)
	seen := map[string]bool{}
	for _, row := range rowRe.FindAllStringSubmatch(html, -1) {
		if len(row) < 2 {
			continue
		}
		cell := row[1]
		m := fileLinkRe.FindStringSubmatch(cell)
		if m == nil {
			continue
		}
		href := strings.TrimSpace(m[1])
		title := strings.Join(strings.Fields(tagRe.ReplaceAllString(m[2], "")), " ")
		if title == "" || href == "" {
			continue
		}
		if seen[href] {
			continue
		}
		seen[href] = true
		doc := Doc{
			Title:    title,
			Category: category,
			URL:      href,
		}
		nobrs := make([]string, 0, 4)
		for _, n := range nobrRe.FindAllStringSubmatch(cell, -1) {
			nobrs = append(nobrs, strings.TrimSpace(tagRe.ReplaceAllString(n[1], "")))
		}
		if len(nobrs) > 0 {
			doc.Rev = nobrs[0]
		}
		if len(nobrs) > 1 {
			doc.Date = nobrs[1]
		}
		if len(nobrs) > 2 {
			doc.Size = nobrs[2]
		}
		if len(nobrs) > 3 {
			doc.Type = nobrs[3]
		}
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no literature rows parsed from category page (page shape may have changed)")
	}
	return docs, nil
}

type section struct {
	heading string
	body    string
}

// splitByHeadings slices html into heading-led regions. Content before the
// first heading gets an empty heading (category "Literature").
func splitByHeadings(html string) []section {
	sections := make([]section, 0)
	loc := headingRe.FindStringIndex(html)
	if loc == nil {
		return []section{{heading: "", body: html}}
	}
	// Content before the first heading.
	if loc[0] > 0 {
		sections = append(sections, section{heading: "", body: html[:loc[0]]})
	}
	rest := html[loc[0]:]
	for {
		loc = headingRe.FindStringSubmatchIndex(rest)
		if loc == nil {
			break
		}
		heading := categoryFromHeading(rest[loc[4]:loc[5]])
		end := len(rest)
		if next := headingRe.FindStringIndex(rest[loc[1]:]); next != nil {
			end = loc[1] + next[0]
		}
		sections = append(sections, section{heading: heading, body: rest[loc[1]:end]})
		if end >= len(rest) {
			break
		}
		rest = rest[end:]
	}
	return sections
}
