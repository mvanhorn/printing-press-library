package roadside

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/cliutil"
)

// The attrlist fragment returned by attractionsByState.php and
// nearbyAttractions.php is a <ul class="attrlist"> of <li id="attr-<ID>-li">
// items. Each item carries the attraction name, street, "City, ST", an
// optional distance label (nearby only), and a "More..." detail link.
var (
	liStartRe    = regexp.MustCompile(`<li id="attr-(\d+)-li">`)
	nameRe       = regexp.MustCompile(`class="attractname"[^>]*>([^<]*)</a>`)
	streetRe     = regexp.MustCompile(`class="street">([^<]*)<`)
	cityStateRe  = regexp.MustCompile(`class="cityState">([^<]*)<`)
	locationRe   = regexp.MustCompile(`class="location">([\s\S]*?)</div>`)
	moreLinkRe   = regexp.MustCompile(`class="mapmorelink"[^>]*href="([^"]*)"`)
	distanceMiRe = regexp.MustCompile(`(?i)(<\s*1|[\d.]+)\s*mi`)
	// distancePhraseRe captures the canonical distance phrase ("~3 mi. away",
	// "<1 mi. away", "12.4 mi. away") out of a location blob that may carry a
	// "- Location Approximate -" prefix and embedded newlines.
	distancePhraseRe = regexp.MustCompile(`(?i)([~<]?\s*[\d.]*\s*mi\.?\s*away)`)
)

// normalizeDistanceLabel extracts the clean distance phrase from a raw
// location label. The site sometimes wraps the distance in a multiline
// "- Location Approximate - (~26 mi. away" blob; this collapses internal
// whitespace and returns just the "~26 mi. away" phrase. When no distance
// phrase is present it returns the whitespace-collapsed input unchanged.
func normalizeDistanceLabel(label string) string {
	collapsed := strings.Join(strings.Fields(label), " ")
	if m := distancePhraseRe.FindStringSubmatch(collapsed); m != nil {
		return strings.Join(strings.Fields(m[1]), " ")
	}
	return collapsed
}

// ParseAttrList parses an attrlist HTML fragment into attractions, in the
// order the site returned them (which is distance-sorted for nearby queries).
func ParseAttrList(html string) []Attraction {
	starts := liStartRe.FindAllStringSubmatchIndex(html, -1)
	out := make([]Attraction, 0, len(starts))
	for i, m := range starts {
		id := html[m[2]:m[3]]
		chunkEnd := len(html)
		if i+1 < len(starts) {
			chunkEnd = starts[i+1][0]
		}
		chunk := html[m[0]:chunkEnd]
		a := Attraction{ID: id}
		if mm := nameRe.FindStringSubmatch(chunk); mm != nil {
			a.Name = cliutil.CleanText(mm[1])
		}
		if mm := streetRe.FindStringSubmatch(chunk); mm != nil {
			a.Street = cliutil.CleanText(mm[1])
		}
		if mm := cityStateRe.FindStringSubmatch(chunk); mm != nil {
			a.City, a.State = SplitCityState(cliutil.CleanText(mm[1]))
		}
		if mm := locationRe.FindStringSubmatch(chunk); mm != nil {
			label := cliutil.CleanText(mm[1])
			label = strings.Trim(strings.TrimSpace(label), "()")
			label = normalizeDistanceLabel(label)
			a.Distance = label
			a.DistanceMi = ParseDistanceMiles(label)
		}
		detail := ""
		if mm := moreLinkRe.FindStringSubmatch(chunk); mm != nil {
			detail = canonicalDetailPath(mm[1], id)
		}
		if detail == "" {
			detail = "/tip/" + id
		}
		a.DetailPath = detail
		a.SourceURL = AttractionURL(detail, id)
		a.Categories = Classify(a)
		if a.Name == "" {
			// An item with no name is not a usable attraction row; skip it
			// rather than emit a blank record into the cache.
			continue
		}
		out = append(out, a)
	}
	return out
}

// canonicalDetailPath keeps /tip/ and /story/ links as-is (stripping any
// query string) and falls back to /tip/<id> for redirect links such as
// /shared/redirectFeatureLink.php that do not point at a stable detail page.
func canonicalDetailPath(href, id string) string {
	href = strings.TrimSpace(href)
	if i := strings.IndexByte(href, '?'); i >= 0 {
		href = href[:i]
	}
	if strings.HasPrefix(href, "/tip/") || strings.HasPrefix(href, "/story/") {
		return href
	}
	return "/tip/" + id
}

// SplitCityState splits "Christmas, FL" into ("Christmas", "FL"). When there
// is no comma, the whole string is treated as the city.
func SplitCityState(s string) (city, state string) {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, ","); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

// ParseDistanceMiles parses "<1 mi. away" -> 0.5, "3 mi. away" -> 3,
// "12.4 mi. away" -> 12.4. Returns 0 when no distance is present.
func ParseDistanceMiles(label string) float64 {
	m := distanceMiRe.FindStringSubmatch(label)
	if m == nil {
		return 0
	}
	v := strings.TrimSpace(m[1])
	if strings.HasPrefix(v, "<") {
		return 0.5
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

// Detail-page extraction.
var (
	ogTitleRe     = regexp.MustCompile(`<meta property="og:title" content="([^"]*)"`)
	ogDescRe      = regexp.MustCompile(`<meta property="og:description" content="([^"]*)"`)
	ogURLRe       = regexp.MustCompile(`<meta property="og:url" content="([^"]*)"`)
	ogImageRe     = regexp.MustCompile(`<meta property="og:image" content="([^"]*)"`)
	titleTagRe    = regexp.MustCompile(`<title>([^<]*)</title>`)
	addressRe     = regexp.MustCompile(`(?s)<dt>Address:</dt>.*?<a href="/map/[^"]*">([^<]+)</a>`)
	addressAlt    = regexp.MustCompile(`(?s)<dt>Address:</dt>\s*<dd>([^<]+)</dd>`)
	directionRe   = regexp.MustCompile(`(?s)<dt>Directions:</dt>\s*<dd>([^<]+)</dd>`)
	paragraphRe   = regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)
	tagStripRe    = regexp.MustCompile(`<[^>]+>`)
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style|noscript)\b[^>]*>.*?</(script|style|noscript)>`)
	// editorialRe captures the editorial blurb that directly follows the
	// attraction heading on /tip pages: ...fieldReviewListIcon"></div></a><p>...</p>
	editorialRe = regexp.MustCompile(`(?s)fieldReviewListIcon"></div></a>\s*<p>(.*?)</p>`)
)

// ParseDetail parses a /tip/<id> or /story/<id> page into a Detail. id is the
// requested id; detailPath is the path used to fetch it (for SourceURL).
func ParseDetail(id, detailPath, html string) Detail {
	d := Detail{}
	d.ID = id

	title := ""
	if m := ogTitleRe.FindStringSubmatch(html); m != nil {
		title = cliutil.CleanText(m[1])
	} else if m := titleTagRe.FindStringSubmatch(html); m != nil {
		title = cliutil.CleanText(m[1])
	}
	// Title shape: "City, ST - Name".
	if i := strings.Index(title, " - "); i >= 0 {
		left := strings.TrimSpace(title[:i])
		d.Name = strings.TrimSpace(title[i+3:])
		d.City, d.State = SplitCityState(left)
	} else {
		d.Name = title
	}

	if m := ogDescRe.FindStringSubmatch(html); m != nil {
		d.Summary = cliutil.CleanText(m[1])
	}
	if m := ogImageRe.FindStringSubmatch(html); m != nil {
		d.ImageURL = strings.TrimSpace(m[1])
	}
	if m := addressRe.FindStringSubmatch(html); m != nil {
		d.Street = cliutil.CleanText(m[1])
	} else if m := addressAlt.FindStringSubmatch(html); m != nil {
		d.Street = cliutil.CleanText(m[1])
	}
	if m := directionRe.FindStringSubmatch(html); m != nil {
		d.Directions = cliutil.CleanText(m[1])
	}
	// Prefer the precise editorial blurb; fall back to the first prose <p>.
	if m := editorialRe.FindStringSubmatch(html); m != nil {
		text := cliutil.CleanText(tagStripRe.ReplaceAllString(m[1], " "))
		d.Writeup = strings.Join(strings.Fields(text), " ")
	}
	if d.Writeup == "" {
		d.Writeup = firstEditorialParagraph(html)
	}

	// Prefer the canonical og:url; fall back to the fetch path.
	if m := ogURLRe.FindStringSubmatch(html); m != nil && strings.TrimSpace(m[1]) != "" {
		d.SourceURL = strings.TrimSpace(m[1])
	} else {
		d.SourceURL = AttractionURL(detailPath, id)
	}
	if detailPath != "" {
		d.DetailPath = detailPath
	} else {
		d.DetailPath = "/tip/" + id
	}
	d.Categories = Classify(d.Attraction)
	return d
}

// firstEditorialParagraph returns the first substantial <p> on a detail page,
// which is the editorial description. <script>/<style> blocks are removed first
// (some <p> elements embed ad-toggle scripts), and visitor-tip, boilerplate, and
// code-like paragraphs are skipped.
func firstEditorialParagraph(html string) string {
	clean := scriptStyleRe.ReplaceAllString(html, " ")
	for _, m := range paragraphRe.FindAllStringSubmatch(clean, -1) {
		text := cliutil.CleanText(tagStripRe.ReplaceAllString(m[1], " "))
		text = strings.Join(strings.Fields(text), " ")
		if len(text) < 40 {
			continue
		}
		low := strings.ToLower(text)
		// Skip site chrome (nav/header/footer) and visitor-tip boilerplate.
		if strings.HasPrefix(low, "reports and tips") ||
			strings.Contains(low, "roadsideamerica.com") ||
			strings.Contains(low, "online guide") ||
			strings.Contains(low, "skip to main") ||
			strings.Contains(low, "all rights reserved") ||
			strings.Contains(low, "privacy policy") {
			continue
		}
		// Skip leftover code/script text that slipped through stripping.
		if strings.Contains(low, "function ") || strings.Contains(low, "var ") ||
			strings.Contains(text, "{") || strings.Contains(text, "}") || strings.Contains(text, "();") {
			continue
		}
		return text
	}
	return ""
}
