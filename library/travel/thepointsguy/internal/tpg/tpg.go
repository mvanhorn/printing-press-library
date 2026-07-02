// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored engine for The Points Guy website surfaces: Algolia search,
// monthly points valuations, the credit-card sitemap, the RSS feed, and
// Next.js __NEXT_DATA__ extraction. All read-only, no auth.
package tpg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/cliutil"
)

// BaseURL is the public origin for The Points Guy.
const BaseURL = "https://thepointsguy.com"

// ValuationsPath is the monthly points-and-miles valuations article.
const ValuationsPath = "/loyalty-programs/monthly-valuations/"

const userAgent = "Mozilla/5.0 (compatible; thepointsguy-pp-cli/0.1; +https://github.com/mvanhorn/printing-press-library)"

// Client fetches replayable Points Guy surfaces with adaptive rate limiting.
type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter

	algoliaApp string
	algoliaKey string
}

// New builds a Client. ratePerSec bounds outbound requests; pass 0 for a
// conservative default.
func New(ratePerSec float64) *Client {
	if ratePerSec <= 0 {
		ratePerSec = 5
	}
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

// get issues a rate-limited GET and returns the body. HTTP 429 is surfaced as
// *cliutil.RateLimitError so callers can distinguish throttling from empty data.
func (c *Client) get(ctx context.Context, rawURL string, hdr map[string]string) ([]byte, error) {
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: rawURL, RetryAfter: cliutil.RetryAfter(resp), Body: string(body[:min(len(body), 200)])}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	c.limiter.OnSuccess()
	return body, nil
}

func (c *Client) post(ctx context.Context, rawURL string, body []byte, hdr map[string]string) ([]byte, error) {
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: rawURL, RetryAfter: cliutil.RetryAfter(resp), Body: string(rb[:min(len(rb), 200)])}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	c.limiter.OnSuccess()
	return rb, nil
}

// ---------------------------------------------------------------------------
// Algolia search
// ---------------------------------------------------------------------------

// IndexContent is TPG's primary content search index.
const IndexContent = "TPG_RUNWAY_PROD"

// IndexSuggestions is TPG's query-suggestions index.
const IndexSuggestions = "TPG_RUNWAY_PROD_query_suggestions"

var (
	reAppChunk   = regexp.MustCompile(`https://thepointsguy\.com/[0-9]+/_next/static/[^"\\]+_app-[^"\\]+\.js`)
	reAlgoliaKey = regexp.MustCompile(`"x-algolia-api-key":"([a-f0-9]{20,64})"`)
	reAlgoliaApp = regexp.MustCompile(`"x-algolia-application-id":"([A-Z0-9]{6,16})"`)
)

// DiscoverAlgolia resolves TPG's public Algolia application id and search-only
// key. Environment overrides win; otherwise the values are extracted from the
// site's _app JS bundle at runtime (they are public frontend values and rotate
// with deploys, so nothing is baked into source). The result is cached.
func (c *Client) DiscoverAlgolia(ctx context.Context) (app, key string, err error) {
	if c.algoliaApp != "" && c.algoliaKey != "" {
		return c.algoliaApp, c.algoliaKey, nil
	}
	if a, k := os.Getenv("THEPOINTSGUY_ALGOLIA_APP_ID"), os.Getenv("THEPOINTSGUY_ALGOLIA_API_KEY"); a != "" && k != "" {
		c.algoliaApp, c.algoliaKey = a, k
		return a, k, nil
	}
	home, err := c.get(ctx, BaseURL+"/", nil)
	if err != nil {
		return "", "", fmt.Errorf("discovering search credentials (homepage): %w", err)
	}
	chunk := reAppChunk.Find(home)
	if chunk == nil {
		return "", "", fmt.Errorf("could not locate the site bundle for search credential discovery")
	}
	js, err := c.get(ctx, string(chunk), nil)
	if err != nil {
		return "", "", fmt.Errorf("discovering search credentials (bundle): %w", err)
	}
	appM := reAlgoliaApp.FindSubmatch(js)
	keyM := reAlgoliaKey.FindSubmatch(js)
	if appM == nil || keyM == nil {
		return "", "", fmt.Errorf("could not extract Algolia credentials from the site bundle")
	}
	c.algoliaApp, c.algoliaKey = string(appM[1]), string(keyM[1])
	return c.algoliaApp, c.algoliaKey, nil
}

// SearchHit is one Algolia result.
type SearchHit struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Author   string `json:"author"`
	Date     string `json:"date"`
	ObjectID string `json:"objectID"`
}

// SearchResult is a page of Algolia hits.
type SearchResult struct {
	Query  string      `json:"query"`
	NbHits int         `json:"nbHits"`
	Hits   []SearchHit `json:"hits"`
}

// Search runs a full-text query against an Algolia index.
func (c *Client) Search(ctx context.Context, index, query string, hitsPerPage int) (*SearchResult, error) {
	app, key, err := c.DiscoverAlgolia(ctx)
	if err != nil {
		return nil, err
	}
	if hitsPerPage <= 0 {
		hitsPerPage = 10
	}
	reqBody, _ := json.Marshal(map[string]any{"query": query, "hitsPerPage": hitsPerPage})
	url := fmt.Sprintf("https://%s-dsn.algolia.net/1/indexes/%s/query", app, index)
	raw, err := c.post(ctx, url, reqBody, map[string]string{
		"x-algolia-application-id": app,
		"x-algolia-api-key":        key,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		NbHits int               `json:"nbHits"`
		Hits   []json.RawMessage `json:"hits"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}
	res := &SearchResult{Query: query, NbHits: parsed.NbHits}
	for _, h := range parsed.Hits {
		var hit SearchHit
		_ = json.Unmarshal(h, &hit)
		hit.Title = cliutil.CleanText(hit.Title)
		hit.URL = normalizeURL(hit.URL)
		res.Hits = append(res.Hits, hit)
	}
	return res, nil
}

// normalizeURL collapses the accidental double slash some TPG Algolia records
// carry after the host (e.g. ".com//news/").
func normalizeURL(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		scheme, rest := u[:i+3], u[i+3:]
		for strings.Contains(rest, "//") {
			rest = strings.ReplaceAll(rest, "//", "/")
		}
		return scheme + rest
	}
	return u
}

// Suggest returns query-completion suggestions. The suggestions index stores
// the completion under a "query" field rather than "title".
func (c *Client) Suggest(ctx context.Context, query string, limit int) ([]string, error) {
	app, key, err := c.DiscoverAlgolia(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	reqBody, _ := json.Marshal(map[string]any{"query": query, "hitsPerPage": limit})
	url := fmt.Sprintf("https://%s-dsn.algolia.net/1/indexes/%s/query", app, IndexSuggestions)
	raw, err := c.post(ctx, url, reqBody, map[string]string{
		"x-algolia-application-id": app,
		"x-algolia-api-key":        key,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Hits []struct {
			Query string `json:"query"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parsing suggestions: %w", err)
	}
	out := make([]string, 0, len(parsed.Hits))
	for _, h := range parsed.Hits {
		if q := cliutil.CleanText(h.Query); q != "" {
			out = append(out, q)
		}
	}
	return out, nil
}

// formatCredit renders a recommended-credit description + range.
func formatCredit(desc, rng string) string {
	desc = cliutil.CleanText(desc)
	rng = strings.TrimSpace(rng)
	switch {
	case desc != "" && rng != "":
		return desc + " (" + rng + ")"
	case desc != "":
		return desc
	default:
		return rng
	}
}

// ---------------------------------------------------------------------------
// Points valuations
// ---------------------------------------------------------------------------

// Valuation is one program's cents-per-point value for a given month.
type Valuation struct {
	Program       string  `json:"program"`
	Type          string  `json:"type"` // transferable | airline | hotel | other
	CentsPerPoint float64 `json:"cents_per_point"`
	Month         string  `json:"month"`
	SourceURL     string  `json:"source_url"`
}

var (
	reTable   = regexp.MustCompile(`(?s)<table.*?</table>`)
	reRow     = regexp.MustCompile(`(?s)<tr.*?</tr>`)
	reCell    = regexp.MustCompile(`(?s)<t[dh][^>]*>(.*?)</t[dh]>`)
	reTag     = regexp.MustCompile(`<[^>]+>`)
	reHeading = regexp.MustCompile(`(?s)<h[1-3][^>]*>(.*?)</h[1-3]>`)
	reMonth   = regexp.MustCompile(`([A-Z][a-z]+ \d{4}) valuation`)
	reWS      = regexp.MustCompile(`\s+`)
	reFloat   = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
)

// stripTags removes HTML tags (replacing each with a space so adjacent text
// nodes do not glue together), unescapes entities, and collapses whitespace.
func stripTags(s string) string {
	return strings.TrimSpace(reWS.ReplaceAllString(cliutil.CleanText(reTag.ReplaceAllString(s, " ")), " "))
}

func classifyValuationType(heading string) string {
	h := strings.ToLower(heading)
	switch {
	case strings.Contains(h, "airline"):
		return "airline"
	case strings.Contains(h, "hotel"):
		return "hotel"
	case strings.Contains(h, "credit card") || strings.Contains(h, "bilt") || strings.Contains(h, "points and miles worth"):
		return "transferable"
	default:
		return "other"
	}
}

// Valuations fetches and parses the monthly valuations article into structured
// (program, type, cents-per-point) rows. The month string is taken from the
// table header. Returns rows plus the reporting month.
func (c *Client) Valuations(ctx context.Context) ([]Valuation, string, error) {
	body, err := c.get(ctx, BaseURL+ValuationsPath, nil)
	if err != nil {
		return nil, "", err
	}
	vals, month := parseValuationsHTML(string(body), BaseURL+ValuationsPath)
	return vals, month, nil
}

// parseValuationsHTML is the pure, testable core of Valuations.
func parseValuationsHTML(html, sourceURL string) ([]Valuation, string) {
	month := ""
	if m := reMonth.FindStringSubmatch(html); m != nil {
		month = m[1]
	}
	var out []Valuation
	seen := map[string]bool{}
	for _, tbl := range reTable.FindAllString(html, -1) {
		rows := reRow.FindAllString(tbl, -1)
		if len(rows) < 2 {
			continue
		}
		// Header row must have a "valuation" column to be a valuations table.
		hdrCells := reCell.FindAllStringSubmatch(rows[0], -1)
		if len(hdrCells) < 2 || !strings.Contains(strings.ToLower(stripTags(hdrCells[1][1])), "valuation") {
			continue
		}
		// Classify from the nearest preceding heading.
		idx := strings.Index(html, tbl)
		pre := html[max(0, idx-1500):idx]
		heads := reHeading.FindAllStringSubmatch(pre, -1)
		heading := ""
		if len(heads) > 0 {
			heading = stripTags(heads[len(heads)-1][1])
		}
		vtype := classifyValuationType(heading)
		for _, r := range rows[1:] {
			cells := reCell.FindAllStringSubmatch(r, -1)
			if len(cells) < 2 {
				continue
			}
			program := stripTags(cells[0][1])
			cppStr := reFloat.FindString(stripTags(cells[1][1]))
			cpp, perr := strconv.ParseFloat(cppStr, 64)
			if program == "" || cppStr == "" || perr != nil {
				continue
			}
			key := strings.ToLower(program)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Valuation{
				Program:       program,
				Type:          vtype,
				CentsPerPoint: cpp,
				Month:         month,
				SourceURL:     sourceURL,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Program < out[j].Program })
	return out, month
}

// ---------------------------------------------------------------------------
// Card sitemap
// ---------------------------------------------------------------------------

var reCardLoc = regexp.MustCompile(`https://thepointsguy\.com/credit-cards/([a-z0-9-]+)/`)

// categorySlugs are non-card index pages that live under /credit-cards/.
var categorySlugs = map[string]bool{
	"rewards": true, "travel": true, "best": true, "airline": true, "visa": true,
	"citi": true, "chase": true, "amex": true, "business": true, "hotel": true,
	"no-foreign-transaction-fees": true, "airport-lounge-access": true,
	"no-annual-fee": true, "cash-back": true, "balance-transfer": true,
	"secured": true, "student": true, "0-apr": true,
}

// CardSlugs returns credit-card page slugs from the card sitemap. Category
// index pages are excluded unless includeCategories is true.
func (c *Client) CardSlugs(ctx context.Context, includeCategories bool) ([]string, error) {
	body, err := c.get(ctx, BaseURL+"/sitemap_cards.xml", nil)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range reCardLoc.FindAllStringSubmatch(string(body), -1) {
		slug := m[1]
		if seen[slug] {
			continue
		}
		if !includeCategories && (categorySlugs[slug] || strings.HasPrefix(slug, "best-")) {
			continue
		}
		seen[slug] = true
		out = append(out, slug)
	}
	sort.Strings(out)
	return out, nil
}

// ---------------------------------------------------------------------------
// __NEXT_DATA__ page extraction
// ---------------------------------------------------------------------------

var reNextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

// FetchPage returns the raw HTML for a page path (or absolute URL).
func (c *Client) FetchPage(ctx context.Context, path string) ([]byte, error) {
	if !strings.HasPrefix(path, "http") {
		path = BaseURL + path
	}
	return c.get(ctx, path, nil)
}

var reArticleLoc = regexp.MustCompile(`<loc>(https://thepointsguy\.com/[^<]+)</loc>`)

// ArticleSitemapURLs returns article URLs for a content category from the
// WordPress article sitemaps (paged). category is a slug like "news", "deals",
// "credit-cards", "airline", "hotel", "loyalty-programs".
func (c *Client) ArticleSitemapURLs(ctx context.Context, category string, max int) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for page := 1; page <= 10; page++ {
		u := fmt.Sprintf("%s/wp-sitemap-articles-%s-%d.xml", BaseURL, category, page)
		body, err := c.get(ctx, u, nil)
		if err != nil {
			if page == 1 {
				return nil, fmt.Errorf("no article sitemap for category %q", category)
			}
			break // no further pages
		}
		before := len(out)
		for _, m := range reArticleLoc.FindAllStringSubmatch(string(body), -1) {
			url := m[1]
			if seen[url] {
				continue
			}
			seen[url] = true
			out = append(out, url)
			if max > 0 && len(out) >= max {
				return out, nil
			}
		}
		if len(out) == before {
			break
		}
	}
	return out, nil
}

// CategoryCardSlugs fetches a best-of / category page and returns the card
// slugs it links to (excluding category index pages).
func (c *Client) CategoryCardSlugs(ctx context.Context, category string) ([]string, error) {
	body, err := c.get(ctx, BaseURL+"/credit-cards/"+category+"/", nil)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range reCardLoc.FindAllStringSubmatch(string(body), -1) {
		slug := m[1]
		if slug == category || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, slug)
	}
	sort.Strings(out)
	return out, nil
}

// NextData fetches a page and returns its parsed __NEXT_DATA__ JSON.
func (c *Client) NextData(ctx context.Context, path string) (map[string]json.RawMessage, error) {
	if !strings.HasPrefix(path, "http") {
		path = BaseURL + path
	}
	body, err := c.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	m := reNextData.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("no page data found at %s", path)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(m[1], &out); err != nil {
		return nil, fmt.Errorf("parsing page data: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Credit-card detail
// ---------------------------------------------------------------------------

// CardAPR is one APR line item from a card's terms. JSON tags match the
// upstream card page so the struct can be unmarshaled directly.
type CardAPR struct {
	Name        string `json:"name"`
	DisplayText string `json:"displayText"`
}

// Card is a structured credit-card record extracted from a TPG card page.
type Card struct {
	Slug              string    `json:"slug"`
	Name              string    `json:"name"`
	URL               string    `json:"url"`
	AnnualFee         string    `json:"annual_fee"`
	WelcomeBonus      string    `json:"welcome_bonus"`
	APRs              []CardAPR `json:"aprs"`
	Rewards           []string  `json:"rewards"`
	TPGRating         float64   `json:"tpg_rating"`
	RecommendedCredit string    `json:"recommended_credit"`
	Superlative       string    `json:"superlative"`
}

// CardDetail fetches a card page and extracts its structured terms.
func (c *Client) CardDetail(ctx context.Context, slug string) (*Card, error) {
	body, err := c.get(ctx, BaseURL+"/credit-cards/"+slug+"/", nil)
	if err != nil {
		return nil, err
	}
	m := reNextData.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("no page data for card %q", slug)
	}
	var nd struct {
		Props struct {
			PageProps struct {
				DehydratedState struct {
					Queries []struct {
						QueryKey []json.RawMessage `json:"queryKey"`
						State    struct {
							Data json.RawMessage `json:"data"`
						} `json:"state"`
					} `json:"queries"`
				} `json:"dehydratedState"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &nd); err != nil {
		return nil, fmt.Errorf("parsing card page: %w", err)
	}
	for _, q := range nd.Props.PageProps.DehydratedState.Queries {
		if len(q.QueryKey) == 0 {
			continue
		}
		var key string
		_ = json.Unmarshal(q.QueryKey[0], &key)
		if key != "PdpPage" {
			continue
		}
		var wrap struct {
			PdpPage struct {
				Card struct {
					Title             string    `json:"title"`
					AnnualFee         string    `json:"annualFee"`
					APRs              []CardAPR `json:"aprs"`
					TPGRating         float64   `json:"tpgRating"`
					Superlative       string    `json:"superlative"`
					RecommendedCredit struct {
						Description string `json:"description"`
						Range       string `json:"range"`
					} `json:"recommendedCredit"`
					IntroBonus struct {
						Description string `json:"description"`
					} `json:"introBonus"`
					RewardsRates struct {
						Multipliers []struct {
							Multiplier  string `json:"multiplier"`
							Description string `json:"description"`
						} `json:"multipliers"`
					} `json:"rewardsRates"`
				} `json:"card"`
			} `json:"pdpPage"`
		}
		if err := json.Unmarshal(q.State.Data, &wrap); err != nil {
			return nil, fmt.Errorf("parsing card data: %w", err)
		}
		cd := wrap.PdpPage.Card
		card := &Card{
			Slug:              slug,
			Name:              cliutil.CleanText(cd.Title),
			URL:               BaseURL + "/credit-cards/" + slug + "/",
			AnnualFee:         cliutil.CleanText(cd.AnnualFee),
			WelcomeBonus:      cliutil.CleanText(cd.IntroBonus.Description),
			APRs:              cd.APRs,
			TPGRating:         cd.TPGRating,
			RecommendedCredit: formatCredit(cd.RecommendedCredit.Description, cd.RecommendedCredit.Range),
			Superlative:       cliutil.CleanText(cd.Superlative),
		}
		for _, mlt := range cd.RewardsRates.Multipliers {
			r := strings.TrimSpace(mlt.Multiplier + " " + cliutil.CleanText(mlt.Description))
			if r != "" {
				card.Rewards = append(card.Rewards, r)
			}
		}
		if card.Name == "" {
			return nil, fmt.Errorf("no card data for %q (is the slug correct? try 'cards list')", slug)
		}
		return card, nil
	}
	return nil, fmt.Errorf("no card data for %q (is the slug correct? try 'cards list')", slug)
}

// ---------------------------------------------------------------------------
// Page metadata / article read
// ---------------------------------------------------------------------------

// PageMeta is a lightweight structured view of a content page.
type PageMeta struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Author      string `json:"author,omitempty"`
	Published   string `json:"published,omitempty"`
	Body        string `json:"body,omitempty"`
	URL         string `json:"url"`
}

func metaContent(html, property string) string {
	// Match <meta property|name="X" content="Y"> in either attribute order.
	res := []*regexp.Regexp{
		regexp.MustCompile(`(?i)<meta[^>]+(?:property|name)="` + regexp.QuoteMeta(property) + `"[^>]+content="([^"]*)"`),
		regexp.MustCompile(`(?i)<meta[^>]+content="([^"]*)"[^>]+(?:property|name)="` + regexp.QuoteMeta(property) + `"`),
	}
	for _, re := range res {
		if m := re.FindStringSubmatch(html); m != nil {
			return cliutil.CleanText(m[1])
		}
	}
	return ""
}

var (
	reTitle       = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reCanonical   = regexp.MustCompile(`(?i)<link[^>]+rel="canonical"[^>]+href="([^"]+)"`)
	reArticleBody = regexp.MustCompile(`(?s)"articleBody"\s*:\s*"((?:[^"\\]|\\.)*)"`)
)

// PageMetadata fetches a page and extracts title, description, author, date,
// canonical URL, and (when present) the JSON-LD article body.
func (c *Client) PageMetadata(ctx context.Context, path string) (*PageMeta, error) {
	body, err := c.FetchPage(ctx, path)
	if err != nil {
		return nil, err
	}
	html := string(body)
	m := &PageMeta{
		Title:       firstNonEmpty(metaContent(html, "og:title"), titleTag(html)),
		Description: firstNonEmpty(metaContent(html, "description"), metaContent(html, "og:description")),
		Author:      firstNonEmpty(metaContent(html, "author"), metaContent(html, "article:author")),
		Published:   metaContent(html, "article:published_time"),
		URL:         firstNonEmpty(canonicalURL(html), absURL(path)),
	}
	if am := reArticleBody.FindStringSubmatch(html); am != nil {
		// Unescape the JSON string body.
		var s string
		if json.Unmarshal([]byte(`"`+am[1]+`"`), &s) == nil {
			m.Body = cliutil.CleanText(s)
		}
	}
	if m.Body == "" {
		m.Body = firstParagraphs(html, 3)
	}
	// If the description is the generic site tagline, prefer the first paragraph.
	if strings.EqualFold(strings.TrimSpace(m.Description), "maximize your travel.") && m.Body != "" {
		m.Description = firstSentence(m.Body)
	}
	return m, nil
}

var reParagraph = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)

// firstParagraphs returns up to n substantial paragraphs of body text.
func firstParagraphs(html string, n int) string {
	var out []string
	for _, m := range reParagraph.FindAllStringSubmatch(html, -1) {
		txt := stripTags(m[1])
		// Skip short/boilerplate fragments (nav, captions, disclaimers).
		if len(txt) < 80 {
			continue
		}
		low := strings.ToLower(txt)
		if isBoilerplate(low) {
			continue
		}
		out = append(out, txt)
		if len(out) >= n {
			break
		}
	}
	return strings.Join(out, "\n\n")
}

// isBoilerplate reports whether a lowercased paragraph is site nav/footer/legal
// chrome rather than article content.
func isBoilerplate(low string) bool {
	for _, marker := range []string{
		"signing up", "privacy policy", "unsubscribe", "cookie settings",
		"newsletters legal", "site map", "do not sell", "editorial disclaimer",
		"the credit card offers", "facebook instagram", "back to glossary",
		"terms of use", "enter your email",
		"credit cards can transform lives", "we publish editorial",
		"opinions expressed here are the author", "we may earn compensation",
		"this may impact how or where products", "all rights reserved",
		"red ventures company", "copyright ©", "min read", "facebook twitter",
	} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func firstSentence(s string) string {
	if i := strings.IndexByte(s, '.'); i > 0 && i < 400 {
		return s[:i+1]
	}
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

func titleTag(html string) string {
	if m := reTitle.FindStringSubmatch(html); m != nil {
		return cliutil.CleanText(m[1])
	}
	return ""
}

func canonicalURL(html string) string {
	if m := reCanonical.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}

func absURL(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	return BaseURL + path
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// RSS feed
// ---------------------------------------------------------------------------

// FeedItem is one article from the RSS feed.
type FeedItem struct {
	Title    string `json:"title"`
	Link     string `json:"link"`
	PubDate  string `json:"pub_date"`
	Creator  string `json:"author"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	Content  string `json:"content,omitempty"`
	// Published is the parsed pub date, when available; persisted so a synced
	// item round-trips its date through the local store.
	Published time.Time `json:"published,omitempty"`
}

// Latest fetches and parses the RSS feed. Items are newest-first.
func (c *Client) Latest(ctx context.Context) ([]FeedItem, error) {
	body, err := c.get(ctx, BaseURL+"/feed/", nil)
	if err != nil {
		return nil, err
	}
	xml := string(body)
	var items []FeedItem
	for _, block := range splitTags(xml, "<item", "</item>") {
		it := FeedItem{
			Title:    firstTag(block, "title"),
			Link:     firstTag(block, "link"),
			PubDate:  firstTag(block, "pubDate"),
			Creator:  firstTag(block, "dc:creator"),
			Category: firstTag(block, "category"),
			Summary:  firstTag(block, "description"),
		}
		if t, perr := parseRSSDate(it.PubDate); perr == nil {
			it.Published = t
		}
		items = append(items, it)
	}
	return items, nil
}

func parseRSSDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		time.RFC1123Z, time.RFC1123,
		time.RFC3339, time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"Mon, 02 Jan 2006 15:04:05 -0700",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable date %q", s)
}

// splitTags returns the substrings between open and close markers.
func splitTags(s, open, close string) []string {
	var out []string
	for {
		i := strings.Index(s, open)
		if i < 0 {
			return out
		}
		j := strings.Index(s[i:], close)
		if j < 0 {
			return out
		}
		out = append(out, s[i:i+j])
		s = s[i+j+len(close):]
	}
}

var reCDATA = regexp.MustCompile(`(?s)<!\[CDATA\[(.*?)\]\]>`)

// firstTag extracts the text content of the first <tag>...</tag> in block.
func firstTag(block, tag string) string {
	re := regexp.MustCompile(`(?s)<` + regexp.QuoteMeta(tag) + `[^>]*>(.*?)</` + regexp.QuoteMeta(tag) + `>`)
	m := re.FindStringSubmatch(block)
	if m == nil {
		return ""
	}
	v := m[1]
	if cm := reCDATA.FindStringSubmatch(v); cm != nil {
		v = cm[1]
	}
	return cliutil.CleanText(v)
}
