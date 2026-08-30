// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

// Package qsys harvests Q-SYS equipment documentation from the three vendor
// sites that carry it.
//
// The split is not arbitrary and it drives the whole design:
//
//   - help.qsys.com carries configuration, networking, and compatibility
//     guidance as static MadCap Flare HTML. It carries NO electrical
//     specifications; the CX-Q overview page mentions no wattage, impedance,
//     or THD figures at all.
//   - www.qsys.com carries the product pages, and each links spec sheets and
//     user manuals as PDFs. Those PDFs are the only source of real numbers.
//   - support.qsys.com carries the knowledge base: 1,906 articles under
//     /en_US/{category}/{slug}, where the category ("known-issues",
//     "errorstatus-messages", "awareness", "troubleshooting", ...) is the only
//     classification published. Error and status articles are titled with the
//     literal string Q-SYS Designer displays, slugified.
//
// No site can answer "does this equipment list run on Designer 9.4" or "what
// does this fault string mean for my gear", which is why the harvested corpus
// is joined locally.
package qsys

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/qsys/internal/cliutil"
)

const (
	HelpHost    = "https://help.qsys.com"
	ProductHost = "https://www.qsys.com"
	SupportHost = "https://support.qsys.com"

	userAgent = "qsys-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"

	// Both hosts are static vendor sites with no published rate limit. Two
	// requests per second is deliberately polite: a full harvest is ~1,000
	// documents and there is no reason to hammer a documentation server.
	defaultRate = 2.0
)

// Client fetches from both Q-SYS documentation hosts under a shared adaptive
// rate limiter.
type Client struct {
	hc  *http.Client
	lim *cliutil.AdaptiveLimiter
}

func New() *Client {
	return &Client{
		hc:  &http.Client{Timeout: 45 * time.Second},
		lim: cliutil.NewAdaptiveLimiter(defaultRate),
	}
}

// get performs a rate-limited GET. A 429 or 503 returns *cliutil.RateLimitError
// rather than an empty body, so callers can distinguish "throttled" from
// "nothing there" — conflating those silently corrupts every downstream count.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	if c.lim != nil {
		c.lim.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		if c.lim != nil {
			c.lim.OnRateLimit()
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, &cliutil.RateLimitError{
			URL:        url,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       string(body),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
	}
	if c.lim != nil {
		c.lim.OnSuccess()
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

// ---------- sitemaps ----------

var locRE = regexp.MustCompile(`(?s)<loc>\s*(.*?)\s*</loc>`)

// Sitemap returns every <loc> URL. Both hosts publish a flat urlset.
func (c *Client) Sitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	body, err := c.get(ctx, sitemapURL)
	if err != nil {
		return nil, err
	}
	matches := locRE.FindAllStringSubmatch(string(body), -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		u := cliutil.CleanText(m[1])
		if strings.HasPrefix(u, "http") {
			out = append(out, u)
		}
	}
	return out, nil
}

// HelpPages filters a help.qsys.com sitemap down to real documentation pages.
// The raw sitemap is dominated by image assets — 1,439 PNGs against 753 actual
// .htm pages — so counting raw <loc> entries overstates the corpus threefold.
func HelpPages(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if strings.HasSuffix(strings.ToLower(u), ".htm") && strings.Contains(u, "/Content/") {
			out = append(out, u)
		}
	}
	return out
}

// ProductPages filters a qsys.com sitemap down to product detail pages.
func ProductPages(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if !strings.Contains(u, "/products-solutions/") {
			continue
		}
		// Keep leaf product pages (family/slug), not the family index itself.
		rest := strings.TrimSuffix(strings.SplitN(u, "/products-solutions/", 2)[1], "/")
		if rest == "" || strings.Count(rest, "/") < 1 {
			continue
		}
		out = append(out, u)
	}
	return out
}

// ---------- HTML text extraction ----------

var (
	// RE2 has no backreferences, so each tag pair is spelled out rather than
	// captured and referenced. Keep this list explicit.
	dropRE   = regexp.MustCompile(`(?is)<script\b.*?</script>|<style\b.*?</style>|<head\b.*?</head>|<nav\b.*?</nav>|<footer\b.*?</footer>`)
	tagRE    = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRE  = regexp.MustCompile(`\s+`)
	titleRE  = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	h1RE     = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	anchorRE = regexp.MustCompile(`(?is)<a\b[^>]*href\s*=\s*["']([^"']+)["']`)
)

// Text strips markup to readable prose. Uses cliutil.CleanText for entity
// unescaping rather than a local implementation — hand-rolled unescaping is
// how &#39; leaks into shipped output.
func Text(htmlSrc string) string {
	s := dropRE.ReplaceAllString(htmlSrc, " ")
	s = tagRE.ReplaceAllString(s, " ")
	s = cliutil.CleanText(s)
	return strings.TrimSpace(spaceRE.ReplaceAllString(s, " "))
}

// Title prefers <h1> over <title>: Flare leaves many <title> tags empty.
func Title(htmlSrc string) string {
	if m := h1RE.FindStringSubmatch(htmlSrc); m != nil {
		if t := Text(m[1]); t != "" {
			return t
		}
	}
	if m := titleRE.FindStringSubmatch(htmlSrc); m != nil {
		return Text(m[1])
	}
	return ""
}

// ---------- help pages ----------

type Page struct {
	URL     string `json:"url"`
	Section string `json:"section"`
	Title   string `json:"title"`
	Text    string `json:"text"`
}

// Section is the first path segment under /Content/ (e.g. "Networking").
func Section(u string) string {
	i := strings.Index(u, "/Content/")
	if i < 0 {
		return ""
	}
	rest := u[i+len("/Content/"):]
	if j := strings.Index(rest, "/"); j >= 0 {
		return rest[:j]
	}
	return "(root)"
}

func (c *Client) Page(ctx context.Context, url string) (Page, error) {
	body, err := c.get(ctx, url)
	if err != nil {
		return Page{}, err
	}
	src := string(body)
	return Page{
		URL:     url,
		Section: Section(url),
		Title:   Title(src),
		Text:    Text(src),
	}, nil
}

// ---------- products ----------

type Product struct {
	Model        string `json:"model"`
	Title        string `json:"title"`
	IsProduct    bool   `json:"is_product"`
	Family       string `json:"family"`
	Slug         string `json:"slug"`
	URL          string `json:"url"`
	Overview     string `json:"overview"`
	SpecPDFURL   string `json:"spec_pdf_url"`
	ManualPDFURL string `json:"manual_pdf_url"`
	SpecText     string `json:"spec_text"`
	Discontinued bool   `json:"discontinued"`
}

// FamilySlug splits a product URL into its family and slug segments.
func FamilySlug(u string) (family, slug string) {
	parts := strings.SplitN(u, "/products-solutions/", 2)
	if len(parts) < 2 {
		return "", ""
	}
	seg := strings.Split(strings.Trim(parts[1], "/"), "/")
	if len(seg) == 0 {
		return "", ""
	}
	family = seg[0]
	slug = seg[len(seg)-1]
	return family, slug
}

// modelFromSlug turns "cx-q-series" into "CX-Q". Vendor slugs are lowercase and
// hyphenated; the compatibility matrix and spec sheets use the uppercase series
// name, so normalizing here is what lets the two sources join at all.
func modelFromSlug(slug string) string {
	s := strings.TrimSuffix(slug, "-series")
	parts := strings.Split(s, "-")
	for i, p := range parts {
		parts[i] = strings.ToUpper(p)
	}
	return strings.Join(parts, "-")
}

var pdfHrefRE = regexp.MustCompile(`(?i)\.pdf(\?.*)?$`)

// Product fetches a product page and resolves its linked spec sheet and manual.
// Spec-sheet PDFs are NOT in the qsys.com sitemap, so the only way to find them
// is to scrape each product page's links. Link placement varies by product
// line, so a miss here is expected and is reported by `coverage` rather than
// being swallowed.
func (c *Client) Product(ctx context.Context, url string) (Product, error) {
	body, err := c.get(ctx, url)
	if err != nil {
		return Product{}, err
	}
	src := string(body)
	family, slug := FamilySlug(url)

	p := Product{
		Model:        modelFromSlug(slug),
		Title:        Title(src),
		Family:       family,
		Slug:         slug,
		URL:          url,
		Overview:     Text(src),
		Discontinued: family == "discontinued-products",
	}

	for _, m := range anchorRE.FindAllStringSubmatch(src, -1) {
		href := cliutil.CleanText(m[1])
		if !pdfHrefRE.MatchString(href) {
			continue
		}
		abs := absURL(href, ProductHost)
		low := strings.ToLower(abs)
		// Only PDFs under the vendor's product-resource tree count. An earlier
		// version accepted ANY linked PDF as the spec sheet, which made every
		// marketing article that happened to link a whitepaper look like a
		// product. Classification must be explicit, never a fallback.
		if !strings.Contains(low, "resource-files") && !strings.Contains(low, "productresources") {
			continue
		}
		switch {
		case (strings.Contains(low, "spec") || strings.Contains(low, "datasheet")) && p.SpecPDFURL == "":
			p.SpecPDFURL = abs
		case strings.Contains(low, "manual") && p.ManualPDFURL == "":
			p.ManualPDFURL = abs
		}
	}

	// /products-solutions/ also hosts marketing and explainer articles
	// ("intro-to-q-sys-control", "little-billy") that are structurally
	// identical to product pages. Rather than guess from the slug, treat the
	// presence of a spec sheet or manual as the signal: real products publish
	// documentation, articles do not.
	p.IsProduct = p.SpecPDFURL != "" || p.ManualPDFURL != ""
	return p, nil
}

func absURL(href, host string) string {
	switch {
	case strings.HasPrefix(href, "http"):
		return href
	case strings.HasPrefix(href, "//"):
		return "https:" + href
	case strings.HasPrefix(href, "/"):
		return host + href
	default:
		return host + "/" + href
	}
}

// ErrNoPDFTool signals that pdftotext is absent. Spec text is an enhancement,
// never a hard requirement: the source PDF URL is always returned regardless,
// so the user can still reach the authoritative numbers.
var ErrNoPDFTool = errors.New("pdftotext not found on PATH; install poppler to extract spec-sheet text")

// PDFText downloads a PDF and extracts its text via pdftotext.
func (c *Client) PDFText(ctx context.Context, url string) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", ErrNoPDFTool
	}
	body, err := c.get(ctx, url)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "pdftotext", "-q", "-", "-")
	cmd.Stdin = strings.NewReader(string(body))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("extracting text from %s: %w", url, err)
	}
	return strings.TrimSpace(spaceRE.ReplaceAllString(string(out), " ")), nil
}

// ---------- compatibility matrix ----------

type CompatRow struct {
	QDSVersion      string `json:"qds_version"`
	ReleaseDate     string `json:"release_date"`
	AddedHardware   string `json:"added_hardware"`
	RemovedHardware string `json:"removed_hardware"`
}

const CompatMatrixPath = "/Content/Q-SYS_Compatibility/Hardware_Compatibility_QDS_Version.htm"

var (
	tableRE = regexp.MustCompile(`(?is)<table.*?</table>`)
	rowRE   = regexp.MustCompile(`(?is)<tr.*?</tr>`)
	cellRE  = regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]>`)
)

// CompatMatrix parses the per-Designer-version hardware support table.
//
// The table is selected by header content rather than by position, because
// position is exactly the kind of assumption that breaks silently when a vendor
// adds an intro table. A row is kept only when its first cell looks like a
// version number.
func (c *Client) CompatMatrix(ctx context.Context) ([]CompatRow, error) {
	body, err := c.get(ctx, HelpHost+CompatMatrixPath)
	if err != nil {
		return nil, err
	}
	return ParseCompatMatrix(string(body))
}

var versionRE = regexp.MustCompile(`^\d+(\.\d+)*$`)

func ParseCompatMatrix(htmlSrc string) ([]CompatRow, error) {
	for _, tbl := range tableRE.FindAllString(htmlSrc, -1) {
		rows := rowRE.FindAllString(tbl, -1)
		if len(rows) < 2 {
			continue
		}
		header := cells(rows[0])
		if !headerHas(header, "version") {
			continue
		}
		out := make([]CompatRow, 0, len(rows))
		for _, r := range rows[1:] {
			cs := cells(r)
			if len(cs) < 2 || !versionRE.MatchString(cs[0]) {
				continue
			}
			row := CompatRow{QDSVersion: cs[0], ReleaseDate: at(cs, 1)}
			row.AddedHardware = at(cs, 2)
			row.RemovedHardware = at(cs, 3)
			out = append(out, row)
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, errors.New("no Designer-version compatibility table found; the vendor page layout may have changed (run `qsys-pp-cli coverage`)")
}

func cells(row string) []string {
	ms := cellRE.FindAllStringSubmatch(row, -1)
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, Text(m[1]))
	}
	return out
}

func headerHas(header []string, want string) bool {
	for _, h := range header {
		if strings.Contains(strings.ToLower(h), want) {
			return true
		}
	}
	return false
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}

// SupportsModel reports whether a model appears in a row's added-hardware text.
// Matching is case-insensitive substring against the series name, because the
// matrix lists series ("MPA-Q Series Network Amplifiers") while a BOM lists
// SKUs ("MPA-Q 4x250"). Callers must treat a miss as "not found in matrix",
// never as "unsupported" — the two are different claims.
func SupportsModel(row CompatRow, model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	return strings.Contains(strings.ToLower(row.AddedHardware), m)
}

// ---------- support articles ----------

// SupportArticle is one knowledge-base article from support.qsys.com.
//
// Category is the sitemap path segment ("errorstatus-messages",
// "known-issues", "awareness", "troubleshooting", ...) and it is the only
// classification the vendor publishes. Every downstream filter keys off it, so
// it is stored verbatim rather than being folded into a friendlier label.
type SupportArticle struct {
	URL      string `json:"url"`
	Category string `json:"category"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Text     string `json:"text"`
}

const supportArticlePrefix = "/en_US/"

// SupportArticles filters a support.qsys.com sitemap down to article URLs.
//
// Every article is shaped https://support.qsys.com/en_US/{category}/{slug}, so
// anything with a different depth is a category index or a locale root and is
// dropped. Counting raw <loc> entries would overstate the corpus.
func SupportArticles(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if cat, slug := SupportCategorySlug(u); cat != "" && slug != "" {
			out = append(out, u)
		}
	}
	return out
}

// SupportCategorySlug splits a support article URL into its category and slug.
// Returns empty strings when the URL is not a two-segment article path.
func SupportCategorySlug(u string) (category, slug string) {
	i := strings.Index(u, supportArticlePrefix)
	if i < 0 {
		return "", ""
	}
	rest := strings.Trim(u[i+len(supportArticlePrefix):], "/")
	if rest == "" {
		return "", ""
	}
	seg := strings.Split(rest, "/")
	if len(seg) != 2 || seg[0] == "" || seg[1] == "" {
		return "", ""
	}
	return seg[0], seg[1]
}

// SupportArticle fetches one knowledge-base article and extracts its text.
func (c *Client) SupportArticle(ctx context.Context, url string) (SupportArticle, error) {
	body, err := c.get(ctx, url)
	if err != nil {
		return SupportArticle{}, err
	}
	src := string(body)
	cat, slug := SupportCategorySlug(url)
	return SupportArticle{
		URL:      url,
		Category: cat,
		Slug:     slug,
		Title:    Title(src),
		Text:     Text(src),
	}, nil
}
