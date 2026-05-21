package thparse

import (
	"regexp"
	"strings"
)

var (
	// PATCH: Go regexp has no backreferences, so these match either quote style.
	cardHrefRE  = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["'](?:https?://www\.trendhunter\.com)?/trends/([a-z0-9][a-z0-9-]*)["'][^>]*>`)
	imgTagRE    = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	cdnSrcRE    = regexp.MustCompile(`(?is)\bsrc\s*=\s*["'](https?://[^'"]*trendhunterstatic\.com[^'"]*)["']`)
	titleAttrRE = regexp.MustCompile(`(?is)\btitle\s*=\s*["']([^"']*)["']`)
)

func ParseCardList(body []byte, source string) ([]Trend, error) {
	raw := string(body)
	matches := cardHrefRE.FindAllStringSubmatchIndex(raw, -1)
	bySlug := map[string]Trend{}
	for _, idx := range matches {
		if len(idx) < 4 {
			continue
		}
		slug := raw[idx[2]:idx[3]]
		start := idx[0] - 900
		if start < 0 {
			start = 0
		}
		end := idx[1] + 1600
		if end > len(raw) {
			end = len(raw)
		}
		window := raw[start:end]
		title, image := cardTitleImage(window)
		if title == "" {
			title = strings.ReplaceAll(slug, "-", " ")
		}
		if _, ok := bySlug[slug]; !ok {
			bySlug[slug] = Trend{
				Slug:      slug,
				Title:     title,
				ImageURL:  image,
				SourceURL: "https://www.trendhunter.com/trends/" + slug,
				Source:    source,
			}
		}
	}
	out := make([]Trend, 0, len(bySlug))
	for _, slug := range sortedTrendSlugs(bySlug) {
		out = append(out, bySlug[slug])
	}
	return out, nil
}

func cardTitleImage(window string) (string, string) {
	title := ""
	image := ""
	for _, img := range imgTagRE.FindAllString(window, -1) {
		if image == "" {
			if m := cdnSrcRE.FindStringSubmatch(img); len(m) >= 2 {
				image = m[1]
			}
		}
		if title == "" {
			title = attrValue(img, "alt")
		}
	}
	if title == "" {
		if m := titleAttrRE.FindStringSubmatch(window); len(m) >= 2 {
			title = m[1]
		}
	}
	if title == "" {
		title = cleanText(window)
		if len(title) > 120 {
			title = strings.TrimSpace(title[:120])
		}
	}
	return collapse(title), image
}

func sortedTrendSlugs(m map[string]Trend) []string {
	seen := make(map[string]int, len(m))
	for slug := range m {
		seen[slug] = 1
	}
	return sortedKeys(seen)
}
