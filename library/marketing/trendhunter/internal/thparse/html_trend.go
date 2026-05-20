package thparse

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

var (
	// PATCH: Go regexp does not support backreferences; match either quote style.
	h1TitleRE   = regexp.MustCompile(`(?is)<h1\b[^>]*class\s*=\s*["'][^"']*\btha__title1\b[^"']*["'][^>]*>(.*?)</h1>`)
	metaTagRE   = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	linkHrefRE  = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["']([^"']*)["'][^>]*>`)
	trendHrefRE = regexp.MustCompile(`(?is)href\s*=\s*["'](?:https?://www\.trendhunter\.com)?/trends/([a-z0-9][a-z0-9-]*)["']`)
	authorRE    = regexp.MustCompile(`(?is)\bby\s+([A-Z][a-zA-Z\s.]+?)\s*[<\(]`)
	trendIDRE   = regexp.MustCompile(`\b(\d{5,7})\b`)
	storyDivRE  = regexp.MustCompile(`(?is)<div\b[^>]*class\s*=\s*["'][^"']*\btha__story[^"']*["'][^>]*>(.*?)</div>`)
	ldJSONRE    = regexp.MustCompile(`(?is)<script\b[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
)

func ParseTrendPage(body []byte, pageURL string) (*Trend, error) {
	raw := string(body)
	slug := trendSlugFromURL(pageURL)
	if slug == "" {
		slug = slugFromURL(pageURL)
	}
	t := &Trend{
		Slug:      slug,
		SourceURL: pageURL,
		Source:    "detail",
	}
	if m := h1TitleRE.FindStringSubmatch(raw); len(m) >= 2 {
		t.Title = cleanText(m[1])
	}
	if t.Title == "" {
		t.Title = metaContent(raw, "property", "og:title")
	}
	t.Description = metaContent(raw, "property", "og:description")
	if t.Description == "" {
		t.Description = metaContent(raw, "name", "description")
	}
	t.ImageURL = metaContent(raw, "property", "og:image")
	t.Keywords = splitKeywords(metaContent(raw, "name", "keywords"))
	t.Category = firstCategory(raw)
	if m := authorRE.FindStringSubmatch(raw); len(m) >= 2 {
		t.Author = collapse(m[1])
	}
	if m := trendIDRE.FindStringSubmatch(raw); len(m) >= 2 {
		t.TrendID = m[1]
	}
	t.RelatedSlugs = relatedSlugs(raw, t.Slug)
	t.FAQ = parseFAQ(raw)
	if m := storyDivRE.FindStringSubmatch(raw); len(m) >= 2 {
		t.BodyText = truncateRunes(cleanText(m[1]), 4000)
	}
	return t, nil
}

func metaContent(raw, key, val string) string {
	for _, tag := range metaTagRE.FindAllString(raw, -1) {
		if strings.EqualFold(attrValue(tag, key), val) {
			return collapse(attrValue(tag, "content"))
		}
	}
	return ""
}

func splitKeywords(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = collapse(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return uniqueStrings(out)
}

var categorySet = map[string]struct{}{
	"advertising": {}, "ai": {}, "art": {}, "autos": {}, "business": {}, "culture": {}, "design": {}, "eco": {}, "education": {}, "entertainment": {}, "fashion": {}, "food": {}, "gadgets": {}, "health": {}, "home": {}, "kids": {}, "lifestyle": {}, "luxury": {}, "marketing": {}, "media": {}, "mobile": {}, "music": {}, "pets": {}, "retail": {}, "science": {}, "social": {}, "sports": {}, "tech": {}, "toys": {}, "travel": {}, "wellness": {},
}

func firstCategory(raw string) string {
	for _, m := range linkHrefRE.FindAllStringSubmatch(raw, -1) {
		if len(m) < 2 {
			continue
		}
		href := strings.Trim(m[1], "/")
		if strings.Contains(href, "/") || href == "" {
			continue
		}
		if _, ok := categorySet[href]; ok {
			return href
		}
	}
	return ""
}

func relatedSlugs(raw, own string) []string {
	var slugs []string
	for _, m := range trendHrefRE.FindAllStringSubmatch(raw, -1) {
		if len(m) < 2 {
			continue
		}
		slug := m[1]
		if slug != own {
			slugs = append(slugs, slug)
		}
	}
	return uniqueStrings(slugs)
}

func parseFAQ(raw string) []FAQ {
	var out []FAQ
	for _, m := range ldJSONRE.FindAllStringSubmatch(raw, -1) {
		if len(m) < 2 {
			continue
		}
		payload := strings.TrimSpace(html.UnescapeString(m[1]))
		payload = strings.ReplaceAll(payload, `\/`, `/`)
		var v any
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			continue
		}
		out = append(out, findFAQ(v)...)
	}
	return out
}

func findFAQ(v any) []FAQ {
	switch x := v.(type) {
	case []any:
		var out []FAQ
		for _, item := range x {
			out = append(out, findFAQ(item)...)
		}
		return out
	case map[string]any:
		if isFAQPage(x["@type"]) {
			return faqEntities(x["mainEntity"])
		}
		var out []FAQ
		for _, item := range x {
			out = append(out, findFAQ(item)...)
		}
		return out
	default:
		return nil
	}
}

func isFAQPage(v any) bool {
	switch x := v.(type) {
	case string:
		return strings.EqualFold(x, "FAQPage")
	case []any:
		for _, item := range x {
			if isFAQPage(item) {
				return true
			}
		}
	}
	return false
}

func faqEntities(v any) []FAQ {
	items, ok := v.([]any)
	if !ok {
		items = []any{v}
	}
	out := make([]FAQ, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		q, _ := m["name"].(string)
		answer := answerText(m["acceptedAnswer"])
		if q != "" || answer != "" {
			out = append(out, FAQ{Question: cleanText(q), Answer: cleanText(answer)})
		}
	}
	return out
}

func answerText(v any) string {
	switch x := v.(type) {
	case map[string]any:
		if s, ok := x["text"].(string); ok {
			return s
		}
	case []any:
		var parts []string
		for _, item := range x {
			if s := answerText(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}
