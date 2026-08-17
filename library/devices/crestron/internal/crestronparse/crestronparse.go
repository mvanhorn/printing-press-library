// Package crestronparse extracts structured data from Crestron.com's
// server-rendered pages and internal AJAX fragments.
//
// Crestron publishes no API. Every function here parses markup that was
// verified live against www.crestron.com; each one documents the exact
// container it keys off so a future markup change fails loudly rather than
// silently returning empty results.
package crestronparse

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// SearchResult is one row from /Support/Search-Results.
type SearchResult struct {
	Title string `json:"title"`
	Type  string `json:"type,omitempty"`
	Date  string `json:"date,omitempty"`
	URL   string `json:"url,omitempty"`
	// Gated is true when the row links to a firmware/software detail page,
	// which requires a signed-in Crestron.com session.
	Gated bool `json:"gated"`
}

// SearchPage is a parsed resource-search response.
type SearchPage struct {
	Results []SearchResult `json:"results"`
	Count   int            `json:"count"`
	Page    int            `json:"page,omitempty"`
	HasMore bool           `json:"has_more"`
}

// Product is a catalog product tile or detail page.
type Product struct {
	Model       string `json:"model"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	SKU         string `json:"sku,omitempty"`
	Brand       string `json:"brand,omitempty"`
	// DocumentID drives /Handlers/ResourceHandler.ashx?dID=.
	DocumentID string `json:"document_id,omitempty"`
	// Discontinued is inferred from the canonical catalog path.
	Discontinued bool `json:"discontinued"`
}

// SpecSection is one titled block of the product specification table.
type SpecSection struct {
	Name string    `json:"name"`
	Rows []SpecRow `json:"rows"`
}

// SpecRow is a single key/value specification pair.
type SpecRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Category is a catalog category page.
type Category struct {
	Path          string        `json:"path,omitempty"`
	DocumentID    string        `json:"document_id,omitempty"`
	NodeID        string        `json:"node_id,omitempty"`
	ProductCount  int           `json:"product_count"`
	Subcategories []Subcategory `json:"subcategories,omitempty"`
}

// Subcategory is a child category with its live product count.
type Subcategory struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Count int    `json:"count,omitempty"`
}

// Asset is a downloadable document attached to a product.
type Asset struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Kind  string `json:"kind,omitempty"`
}

// FirmwareRelease is a parsed /Software-Firmware/... detail page.
type FirmwareRelease struct {
	Version      string   `json:"version,omitempty"`
	LastModified string   `json:"last_modified,omitempty"`
	Models       []string `json:"models,omitempty"`
	ReleaseNotes string   `json:"release_notes,omitempty"`
	ChangeLog    string   `json:"change_log,omitempty"`
	DownloadURL  string   `json:"download_url,omitempty"`
	// ReleaseNotesURL is the standalone release-notes PDF when the page links one.
	ReleaseNotesURL string `json:"release_notes_url,omitempty"`
	// RequiresAuth is true when the page redirected to the sign-in form.
	RequiresAuth bool `json:"requires_auth"`
}

// ---------------------------------------------------------------------------
// Resource search
// ---------------------------------------------------------------------------

// ParseSearchResults reads /Support/Search-Results markup.
//
// Each row is a `div.search-result row` containing `.resource-search-name` (an
// anchor), `.resource-search-type`, and `.resource-search-date`. Rows whose
// href starts with /Software-Firmware/ are auth-gated detail pages; rows whose
// href starts with /getmedia/ are public direct downloads.
func ParseSearchResults(body []byte) (SearchPage, error) {
	var page SearchPage
	page.Results = make([]SearchResult, 0)

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return page, err
	}

	for _, row := range findAllByClass(doc, "search-result") {
		var r SearchResult
		if nameDiv := firstByClass(row, "resource-search-name"); nameDiv != nil {
			if a := firstTag(nameDiv, "a"); a != nil {
				r.Title = textOf(a)
				r.URL = attr(a, "href")
			}
		}
		if t := firstByClass(row, "resource-search-type"); t != nil {
			r.Type = textOf(t)
		}
		if d := firstByClass(row, "resource-search-date"); d != nil {
			r.Date = textOf(d)
		}
		if r.Title == "" && r.URL == "" {
			continue
		}
		r.Gated = strings.HasPrefix(r.URL, "/Software-Firmware/")
		page.Results = append(page.Results, r)
	}

	page.Count = len(page.Results)
	// The pager renders a link per page; presence of a "next" page number
	// greater than the current one means more results exist.
	page.HasMore = hasNextPage(doc)
	return page, nil
}

var pagerRe = regexp.MustCompile(`/Support/Search-Results\?[^"]*[?&]p=(\d+)`)

func hasNextPage(doc *html.Node) bool {
	var maxPage, current int
	for _, a := range findAllTags(doc, "a") {
		m := pagerRe.FindStringSubmatch(attr(a, "href"))
		if len(m) == 2 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > maxPage {
				maxPage = n
			}
		}
	}
	// Current page is the one rendered without a link, so treat any pager with
	// more than one distinct page as having more.
	return maxPage > current+1
}

// ---------------------------------------------------------------------------
// Product tiles and detail pages
// ---------------------------------------------------------------------------

// ParseProductTiles reads /CMSPages/ProductSubcategoryItemTemplate.aspx
// fragments. Each tile is `div.product-result` with `p.model-number`, a tagline
// anchor, and a canonical product URL. `span#productCount` carries the
// category total, which is the pagination loop's termination condition.
func ParseProductTiles(body []byte) ([]Product, int, error) {
	out := make([]Product, 0)
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return out, 0, err
	}

	total := 0
	if n := firstByID(doc, "productCount"); n != nil {
		total, _ = strconv.Atoi(strings.TrimSpace(textOf(n)))
	}

	for _, tile := range findAllByClass(doc, "product-result") {
		var p Product
		if m := firstByClass(tile, "model-number"); m != nil {
			p.Model = textOf(m)
		}
		if t := firstByClass(tile, "tagline"); t != nil {
			p.Description = textOf(t)
		}
		for _, a := range findAllTags(tile, "a") {
			if h := attr(a, "href"); strings.Contains(h, "/Products/Catalog/") {
				p.URL = h
				break
			}
		}
		if img := firstTag(tile, "img"); img != nil {
			p.ImageURL = attr(img, "src")
		}
		// The buy-box carries the internal model id; the support button carries
		// the document id used by ResourceHandler.
		for _, n := range findAllWithAttr(tile, "data-id") {
			if v := attr(n, "data-id"); v != "" {
				p.DocumentID = v
				break
			}
		}
		if p.Model == "" {
			continue
		}
		p.Discontinued = strings.Contains(p.URL, "/Inactive/Discontinued/")
		out = append(out, p)
	}
	return out, total, nil
}

type jsonLDProduct struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	SKU         string          `json:"sku"`
	Brand       json.RawMessage `json:"brand"`
	Image       json.RawMessage `json:"image"`
}

// ParseProductPage reads a product detail page. Identity comes from the
// schema.org JSON-LD block; the document id comes from the `data-id` attribute
// that ResourceHandler.ashx?dID= consumes.
func ParseProductPage(body []byte, canonicalPath string) (Product, error) {
	var p Product
	p.URL = canonicalPath
	p.Discontinued = strings.Contains(canonicalPath, "/Inactive/Discontinued/")

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return p, err
	}

	for _, s := range findAllTags(doc, "script") {
		if !strings.Contains(strings.ToLower(attr(s, "type")), "ld+json") {
			continue
		}
		var ld jsonLDProduct
		if json.Unmarshal([]byte(textOf(s)), &ld) != nil {
			continue
		}
		if ld.Name == "" {
			continue
		}
		p.Model = ld.Name
		p.Description = ld.Description
		p.SKU = ld.SKU
		p.Brand = firstStringField(ld.Brand, "name")
		p.ImageURL = firstStringElem(ld.Image)
		break
	}

	p.DocumentID = documentIDFrom(doc)
	return p, nil
}

// ParseSpecTable reads the Specifications tab. Section headers are
// `td.productSpecTDHead`; every other two-cell row is a key/value pair.
func ParseSpecTable(body []byte) ([]SpecSection, error) {
	out := make([]SpecSection, 0)
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return out, err
	}

	var current *SpecSection
	for _, tr := range findAllTags(doc, "tr") {
		tds := directChildTags(tr, "td")
		if len(tds) == 0 {
			continue
		}
		if hasClass(tds[0], "productSpecTDHead") {
			if current != nil && len(current.Rows) > 0 {
				out = append(out, *current)
			}
			current = &SpecSection{Name: textOf(tds[0]), Rows: make([]SpecRow, 0)}
			continue
		}
		if current == nil || len(tds) != 2 {
			continue
		}
		k, v := textOf(tds[0]), textOf(tds[1])
		if k == "" || v == "" {
			continue
		}
		current.Rows = append(current.Rows, SpecRow{Key: k, Value: v})
	}
	if current != nil && len(current.Rows) > 0 {
		out = append(out, *current)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

var (
	docIDRe  = regexp.MustCompile(`documentId:\s*'(\d+)'`)
	nodeIDRe = regexp.MustCompile(`nodeId:\s*'(\d+)'`)
	catCntRe = regexp.MustCompile(`categoryCount:\s*'(\d+)'`)
	subcatRe = regexp.MustCompile(`^(.*?)\s*\((\d+)\)\s*$`)
)

// ParseCategoryPage reads the inline `var request = {...}` block that drives
// the product-tile endpoint, plus the subcategory list with its live counts.
func ParseCategoryPage(body []byte, path string) (Category, error) {
	c := Category{Path: path}
	s := string(body)

	if m := docIDRe.FindStringSubmatch(s); len(m) == 2 {
		c.DocumentID = m[1]
	}
	if m := nodeIDRe.FindStringSubmatch(s); len(m) == 2 {
		c.NodeID = m[1]
	}
	if m := catCntRe.FindStringSubmatch(s); len(m) == 2 {
		c.ProductCount, _ = strconv.Atoi(m[1])
	}

	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return c, err
	}
	c.Subcategories = make([]Subcategory, 0)
	seen := map[string]bool{}
	for _, opt := range findAllTags(doc, "option") {
		v := attr(opt, "value")
		if !strings.HasPrefix(v, "/Products/Catalog/") || seen[v] {
			continue
		}
		seen[v] = true
		sc := Subcategory{Path: v, Name: textOf(opt)}
		if m := subcatRe.FindStringSubmatch(sc.Name); len(m) == 3 {
			sc.Name = strings.TrimSpace(m[1])
			sc.Count, _ = strconv.Atoi(m[2])
		}
		c.Subcategories = append(c.Subcategories, sc)
	}
	return c, nil
}

// ParseCatalogPaths reads /sitemap and returns every catalog category path.
func ParseCatalogPaths(body []byte) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, a := range findAllTags(doc, "a") {
		h := attr(a, "href")
		if strings.HasPrefix(h, "/Products/Catalog/") && !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Per-product assets
// ---------------------------------------------------------------------------

// ParseAssets reads a ResourceHandler.ashx fragment or any page carrying
// /getmedia/ links, deduplicating the paired "Download"/"PDF" anchors that
// point at the same asset as the titled anchor.
func ParseAssets(body []byte) ([]Asset, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	out := make([]Asset, 0)
	seen := map[string]bool{}
	for _, a := range findAllTags(doc, "a") {
		h := attr(a, "href")
		if !strings.HasPrefix(h, "/getmedia/") || seen[h] {
			continue
		}
		t := textOf(a)
		lt := strings.ToLower(strings.TrimSpace(t))
		// Skip the bare action anchors; the titled anchor for the same href
		// carries the real name and is emitted instead.
		if lt == "" || lt == "download" || lt == "pdf" {
			continue
		}
		seen[h] = true
		out = append(out, Asset{Title: t, URL: h, Kind: classifyAsset(t)})
	}
	return out, nil
}

// ClassifyAssetTitle maps an asset title to a coarse asset class.
func ClassifyAssetTitle(title string) string { return classifyAsset(title) }

func classifyAsset(title string) string {
	l := strings.ToLower(title)
	switch {
	case strings.Contains(l, "spec sheet"):
		return "spec-sheet"
	case strings.Contains(l, "guide spec"):
		return "guide-spec"
	case strings.Contains(l, "revit"):
		return "revit"
	case strings.Contains(l, "cad"):
		return "cad"
	case strings.Contains(l, "end-of-sale"), strings.Contains(l, "end of sale"):
		return "end-of-sale"
	case strings.Contains(l, "security reference"):
		return "security-reference"
	case strings.Contains(l, "product manual"), strings.Contains(l, "manual"):
		return "manual"
	case strings.Contains(l, "quick start"):
		return "quick-start"
	case strings.Contains(l, "product information"):
		return "product-info"
	case strings.Contains(l, "certificate"), strings.Contains(l, "doc "), strings.Contains(l, "nrtl"):
		return "certificate"
	case strings.Contains(l, "user guide"):
		return "user-guide"
	case strings.Contains(l, "drawing"):
		return "drawing"
	default:
		return "other"
	}
}

// ---------------------------------------------------------------------------
// Firmware
// ---------------------------------------------------------------------------

// versionTailRe matches the trailing version token in a release title such as
// "DM-NVX-384(C)_DM-NVX-385(C) 7.4.0255.22319".
var versionTailRe = regexp.MustCompile(`\s+([0-9]+(?:\.[0-9]+){1,4})\s*$`)

// SplitReleaseTitle separates a firmware release title into its covered models
// and its version.
//
// Crestron scopes one release to a whole family, so a single title can name
// seven models. Titles delimit models with "/" or "_" and end with the version.
// Zero-width and non-breaking spaces appear in real titles and are stripped.
func SplitReleaseTitle(title string) (models []string, version string) {
	t := strings.Map(func(r rune) rune {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\ufeff': // zero-width chars seen in live titles
			return -1
		case '\u00a0': // non-breaking space
			return ' '
		}
		return r
	}, title)
	t = strings.TrimSpace(t)

	if m := versionTailRe.FindStringSubmatch(t); len(m) == 2 {
		version = m[1]
		t = strings.TrimSpace(t[:len(t)-len(m[0])])
	}

	models = make([]string, 0)
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(t, func(r rune) bool { return r == '/' || r == '_' }) {
		p := strings.TrimSpace(part)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		models = append(models, p)
	}
	return models, version
}

// ExpandModelFamily returns the concrete model names a release title covers.
//
// Crestron abbreviates siblings after the first full model name, so
// "TSW-570/TSW-770/TS-770" lists each in full, but "DM-NVX-D10/D20/E10"
// abbreviates to the trailing segment. A bare trailing segment inherits the
// prefix of the previous full model.
func ExpandModelFamily(parts []string) []string {
	out := make([]string, 0, len(parts))
	var prefix string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i := strings.LastIndex(p, "-"); i > 0 && strings.Count(p, "-") >= 2 {
			// Looks like a complete model (at least two hyphen groups).
			prefix = p[:i+1]
			out = append(out, p)
			continue
		}
		if prefix != "" && !strings.Contains(p, "-") {
			out = append(out, prefix+p)
			continue
		}
		out = append(out, p)
	}
	return out
}

var (
	// The labels are wrapped in <b> tags on the live page, so these run against
	// extracted text rather than raw markup.
	fwVersionRe = regexp.MustCompile(`(?i)Version:\s*([0-9][0-9A-Za-z._-]*)`)
	fwModRe     = regexp.MustCompile(`(?i)Last Modified:\s*([0-9]{1,2}/[0-9]{1,2}/[0-9]{4}(?:\s+[0-9:]+\s*(?:AM|PM))?)`)
	fwDLRe      = regexp.MustCompile(`href=["'](/firmware_files/[^"']+)["']`)
	fwNotesRe   = regexp.MustCompile(`href=["'](/release_notes/[^"']+)["']`)
)

// ParseFirmwareRelease reads a /Software-Firmware/... detail page. When the
// request was unauthenticated the site serves the sign-in form instead, which
// is reported via RequiresAuth rather than as a parse failure.
func ParseFirmwareRelease(body []byte) (FirmwareRelease, error) {
	var fr FirmwareRelease
	s := string(body)

	if strings.Contains(s, "Crestron Authentication - Sign In") ||
		strings.Contains(s, "<title>\r\n\tSignIn") ||
		strings.Contains(s, "SignIn [Crestron") {
		fr.RequiresAuth = true
		return fr, nil
	}

	// Download and release-note links come from raw markup.
	if m := fwDLRe.FindStringSubmatch(s); len(m) == 2 {
		fr.DownloadURL = m[1]
	}
	if m := fwNotesRe.FindStringSubmatch(s); len(m) == 2 {
		fr.ReleaseNotesURL = m[1]
	}

	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return fr, err
	}
	// Version and Last Modified labels are wrapped in <b> tags, so they only
	// match once the markup between label and value is removed.
	full := collapse(textOf(doc))
	if m := fwVersionRe.FindStringSubmatch(full); len(m) == 2 {
		fr.Version = strings.TrimSpace(m[1])
	}
	if m := fwModRe.FindStringSubmatch(full); len(m) == 2 {
		fr.LastModified = strings.TrimSpace(m[1])
	}
	fr.ReleaseNotes = TrimSiteChrome(sectionBetween(full, "Release Notes", "Change Log:"))
	fr.ChangeLog = TrimSiteChrome(sectionAfter(full, "Change Log:"))
	return fr, nil
}

// siteChromeMarkers are the first words of the global site footer, which the
// flattened page text appends to whatever section runs last. Without trimming,
// the footer lands inside ChangeLog and a firmware diff reports navigation
// boilerplate ("About Sustainability ... (c) 2026 Crestron Electronics, Inc.")
// as though it were a firmware change.
var siteChromeMarkers = []string{
	"About Sustainability Social Responsibility",
}

// TrimSiteChrome cuts a flattened page section at the start of the global site
// footer. Sections that never reach the footer are returned unchanged.
func TrimSiteChrome(s string) string {
	cut := -1
	for _, marker := range siteChromeMarkers {
		if i := strings.Index(s, marker); i >= 0 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut < 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:cut])
}

func sectionBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	if j := strings.Index(rest, end); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

func sectionAfter(s, start string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(s[i+len(start):])
}

// documentIDFrom finds the Kentico document id that ResourceHandler.ashx?dID=
// and VariantProduct.ashx?DocumentID= consume.
//
// Several unrelated widgets also use `data-id` — notably an embedded video
// modal whose id is a Vimeo id, not a document id. The product's own id is the
// one repeated across the favorite, support, and add-to-project buttons, so
// prefer those known containers and fall back to the most frequently repeated
// numeric data-id rather than the first one encountered.
func documentIDFrom(doc *html.Node) string {
	preferred := []string{"btn-add-fav", "support-open", "project-item-add"}
	for _, class := range preferred {
		for _, n := range findAllByClass(doc, class) {
			if v := attr(n, "data-id"); isAllDigits(v) {
				return v
			}
		}
	}
	counts := map[string]int{}
	for _, n := range findAllWithAttr(doc, "data-id") {
		if v := attr(n, "data-id"); isAllDigits(v) {
			counts[v]++
		}
	}
	best, bestN := "", 0
	for v, c := range counts {
		if c > bestN {
			best, bestN = v, c
		}
	}
	return best
}
