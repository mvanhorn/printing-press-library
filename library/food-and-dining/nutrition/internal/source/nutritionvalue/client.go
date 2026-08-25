// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package nutritionvalue is a hand-authored source client for
// NutritionValue.org. The site has no API; it serves server-rendered HTML over
// standard HTTP. This client fetches and extracts the site's search results,
// food-detail derived analytics (net carbs, omega-6/omega-3 ratio, per-nutrient
// %DV including amino acids), and precomputed nutrient-ranking pages. These are
// the fields the USDA FoodData Central API does not expose, which is the whole
// reason NutritionValue.org is a first-class source in this CLI.
//
// NutritionValue.org's food IDs are the same USDA FDC IDs used elsewhere in the
// CLI, so the two sources join on a shared key.
//
// The site's HTML carries a comment discouraging scripted access. This client
// stays deliberately polite: a browser User-Agent, an adaptive rate limiter
// defaulting to well under one request/second of sustained load, and no bulk
// crawling.
package nutritionvalue

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/cliutil"
)

const (
	baseURL   = "https://www.nutritionvalue.org"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// Client fetches from NutritionValue.org with a per-source adaptive limiter.
type Client struct {
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// New returns a NutritionValue.org client. The limiter starts conservative
// (1 req/s) out of respect for the site's stated preferences.
func New() *Client {
	return &Client{
		HTTP:    &http.Client{},
		limiter: cliutil.NewAdaptiveLimiter(1.0),
	}
}

// fetch performs one rate-limited GET with a browser User-Agent and returns the
// response body. A 429 surfaces as a typed *cliutil.RateLimitError so callers
// can distinguish throttling from "no data".
//
// pp:client-call
func (c *Client) fetch(ctx context.Context, rawURL string) (string, error) {
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", baseURL+"/")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return "", &cliutil.RateLimitError{URL: rawURL, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nutritionvalue.org returned HTTP %d for %s", resp.StatusCode, rawURL)
	}
	c.limiter.OnSuccess()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// SearchResult is one row from a NutritionValue.org search.
type SearchResult struct {
	Name string `json:"name"`
	Slug string `json:"slug"` // detail-page path, e.g. /Cheese%2C_cheddar_nutritional_value.html
	ID   string `json:"id"`   // NutritionValue.org id == USDA FDC id for generic foods
}

var (
	// Detail-page links: href='/<slug>_nutritional_value.html'
	reDetailLink = regexp.MustCompile(`href='(/[^']*_nutritional_value\.html)'`)
	// Compare/favorites links carry the numeric (or s-prefixed) id in order.
	reCompareID = regexp.MustCompile(`comparefoods\.php\?action=add&(?:amp;)?id=(s?\d+)`)
	// Nutrient table rows: data-tooltip='Name' ... <td class='right'>12.34&nbsp;g</td>
	reNutrientRow = regexp.MustCompile(`data-tooltip='([^']+)'[^>]*>[^<]*</a>\s*</td>\s*<td class='right'>([0-9.]+)(?:&nbsp;|\s)*([a-zA-Z%µ]*)`)
	// Omega table: <td class='center'>0.18 g</td><td class='center'>1.10 g</td><td class='center'>6.20</td>
	// (?s) so the ".*?" between the table class and the first cell spans the
	// intervening <tr>/<th> markup even when the server splits it across lines.
	reOmega = regexp.MustCompile(`(?s)class='wide results omega'.*?<td class='center'>([0-9.]+)\s*g</td><td class='center'>([0-9.]+)\s*g</td><td class='center'>([0-9.]+)</td>`)
	// Ranking-page rows link to detail pages; capture slug + the leading amount cell.
	reTitle = regexp.MustCompile(`<title>([^<]*)</title>`)
)

// Search returns NutritionValue.org search results for a free-text query. The
// query may also be a UPC barcode; the site accepts both.
func (c *Client) Search(ctx context.Context, query string) ([]SearchResult, error) {
	u := baseURL + "/search.php?food_query=" + url.QueryEscape(query)
	body, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseSearchRows(body), nil
}

// parseSearchRows pairs each detail-page link with the id from the same result
// row. On NutritionValue.org search rows, the food-name anchor (slug) is
// followed within the same row by a "Compare" anchor carrying the id. Rather
// than zip two page-global lists by index (which drifts if any stray detail or
// compare link exists outside a result row), we take each slug's document
// position and bind it to the first compare id that appears after it within a
// bounded window, so slug and id always come from the same row.
const searchRowWindow = 3000

func parseSearchRows(body string) []SearchResult {
	slugLoc := reDetailLink.FindAllStringSubmatchIndex(body, -1)
	idLoc := reCompareID.FindAllStringSubmatchIndex(body, -1)
	out := make([]SearchResult, 0, len(slugLoc))
	for _, sl := range slugLoc {
		slugStart, slugEnd := sl[0], sl[1]
		slug := body[sl[2]:sl[3]]
		id := ""
		for _, il := range idLoc {
			// First compare id that begins after this slug and within the
			// bounded row window.
			if il[0] >= slugEnd && il[0]-slugStart <= searchRowWindow {
				id = body[il[2]:il[3]]
				break
			}
		}
		out = append(out, SearchResult{
			Name: slugToName(slug),
			Slug: slug,
			ID:   id,
		})
	}
	return out
}

func nameTokens(s string) map[string]bool {
	m := map[string]bool{}
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == ',' || r == '_' || r == '-' || r == '(' || r == ')'
	}) {
		if len(t) >= 3 {
			m[t] = true
		}
	}
	return m
}

// nameOverlap returns how much of the query name is covered by the candidate
// name (shared tokens over query tokens). Coverage, not Jaccard: a candidate
// with extra qualifiers ("Bananas, ripe and slightly ripe, raw" for query
// "Bananas, raw") should still score high. The single-shared-token trap that
// coverage alone would fall into (any "Fish*" scoring 1.0 for query "Fish") is
// handled by the caller's minimum-shared-token guard, not by diluting the score.
func nameOverlap(a, b string) float64 {
	ta, tb := nameTokens(a), nameTokens(b)
	if len(ta) == 0 {
		return 0
	}
	hit := sharedTokenCount(ta, tb)
	return float64(hit) / float64(len(ta))
}

// sharedTokenCount counts tokens present in both sets.
func sharedTokenCount(ta, tb map[string]bool) int {
	hit := 0
	for t := range ta {
		if tb[t] {
			hit++
		}
	}
	return hit
}

// searchQuery derives a NutritionValue.org search term from a USDA description
// by dropping any parenthetical suffix (program notes, qualifiers) that the
// site's food titles do not carry.
func searchQuery(description string) string {
	q := description
	if idx := strings.Index(q, "("); idx > 0 {
		q = q[:idx]
	}
	return strings.TrimSpace(q)
}

func slugToName(slug string) string {
	name := strings.TrimSuffix(strings.TrimPrefix(slug, "/"), "_nutritional_value.html")
	if dec, err := url.QueryUnescape(name); err == nil {
		name = dec
	}
	name = strings.ReplaceAll(name, "_", " ")
	return cliutil.CleanText(name)
}

// FoodDetail holds the NutritionValue.org-derived analytics for one food. These
// are the fields the USDA API does not return.
type FoodDetail struct {
	Name       string   `json:"name"`
	Slug       string   `json:"slug"`
	NetCarbs   *float64 `json:"net_carbs_g,omitempty"`
	Omega3     *float64 `json:"omega_3_g,omitempty"`
	Omega6     *float64 `json:"omega_6_g,omitempty"`
	OmegaRatio *float64 `json:"omega_6_3_ratio,omitempty"`
	// Nutrients is the full NutritionValue.org table keyed by nutrient name,
	// including per-nutrient %DV for amino acids that USDA does not surface.
	Nutrients map[string]DetailNutrient `json:"nutrients"`
}

// DetailNutrient is one row of the NutritionValue.org nutrient table.
type DetailNutrient struct {
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

// FoodBySlug fetches a NutritionValue.org detail page by its slug path and
// extracts the derived analytics.
func (c *Client) FoodBySlug(ctx context.Context, slug string) (*FoodDetail, error) {
	if !strings.HasPrefix(slug, "/") {
		slug = "/" + slug
	}
	body, err := c.fetch(ctx, baseURL+slug)
	if err != nil {
		return nil, err
	}
	return parseFoodDetail(body, slug), nil
}

// FoodByID resolves a food id (== USDA FDC id) plus its description to a
// NutritionValue.org detail page, then extracts derived analytics. It searches
// the site by description and matches the row whose id equals the requested id;
// if no id matches it falls back to the first result (best-effort name match).
func (c *Client) FoodByID(ctx context.Context, id, description string) (*FoodDetail, error) {
	if strings.TrimSpace(description) == "" {
		return nil, fmt.Errorf("nutritionvalue lookup needs a description for id %s", id)
	}
	// USDA descriptions can carry parenthetical program notes (e.g. "Cheese,
	// cheddar (Includes foods for USDA's Food Distribution Program)") that no
	// NutritionValue.org title matches. Search on the core name before the
	// first parenthesis; keep the full description for the name-overlap guard.
	query := searchQuery(description)
	results, err := c.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no NutritionValue.org match for %q", query)
	}
	var chosen SearchResult
	matchedByID := false
	for _, r := range results {
		if r.ID == id {
			chosen = r
			matchedByID = true
			break
		}
	}
	if !matchedByID {
		// No exact id match (e.g. branded s-prefixed ids never equal a numeric
		// FDC id). Only accept the best result if its name clearly overlaps the
		// USDA description; otherwise refuse rather than attach a different
		// food's analytics to this id.
		queryTokens := nameTokens(query)
		best := results[0]
		bestScore := nameOverlap(query, best.Name)
		for _, r := range results[1:] {
			if s := nameOverlap(query, r.Name); s > bestScore {
				best, bestScore = r, s
			}
		}
		// Require both a high coverage AND at least two shared tokens, so a
		// single shared token (e.g. query "Fish" vs "Fish oil, cod liver") can
		// never clear the bar and attach the wrong food's analytics. Very short
		// queries (one usable token) cannot meet the two-token floor and are
		// refused — a safe miss beats a wrong enrichment.
		shared := sharedTokenCount(queryTokens, nameTokens(best.Name))
		if bestScore < 0.5 || shared < 2 {
			return nil, fmt.Errorf("no confident NutritionValue.org match for %q (best candidate %q)", query, best.Name)
		}
		chosen = best
	}
	return c.FoodBySlug(ctx, chosen.Slug)
}

func parseFoodDetail(body, slug string) *FoodDetail {
	d := &FoodDetail{Slug: slug, Nutrients: map[string]DetailNutrient{}}
	if m := reTitle.FindStringSubmatch(body); m != nil {
		d.Name = cliutil.CleanText(strings.TrimSuffix(strings.TrimSpace(m[1]), " nutrition facts and analysis."))
	}
	for _, m := range reNutrientRow.FindAllStringSubmatch(body, -1) {
		name := cliutil.CleanText(m[1])
		amt, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		unit := strings.TrimSpace(m[3])
		// Keep the first occurrence of each nutrient (the primary table row).
		if _, seen := d.Nutrients[name]; !seen {
			d.Nutrients[name] = DetailNutrient{Amount: amt, Unit: unit}
		}
		if strings.EqualFold(name, "Net carbs") {
			v := amt
			d.NetCarbs = &v
		}
	}
	if m := reOmega.FindStringSubmatch(body); m != nil {
		if o3, err := strconv.ParseFloat(m[1], 64); err == nil {
			d.Omega3 = &o3
		}
		if o6, err := strconv.ParseFloat(m[2], 64); err == nil {
			d.Omega6 = &o6
		}
		if ratio, err := strconv.ParseFloat(m[3], 64); err == nil {
			d.OmegaRatio = &ratio
		}
	}
	return d
}

// RankRow is one entry from a NutritionValue.org nutrient-ranking page.
type RankRow struct {
	Rank   int     `json:"rank"`
	Name   string  `json:"name"`
	Slug   string  `json:"slug"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

// nutrientPagePart maps a nutrient argument to the URL segment used on
// NutritionValue.org's ranking pages (foods_by_<part>_content.html). Names use
// spaces and specific casing on the site; we accept common aliases.
var nutrientPageName = map[string]string{
	"protein":      "Protein",
	"calories":     "Calories",
	"carbohydrate": "Carbohydrate",
	"carbs":        "Carbohydrate",
	"fiber":        "Fiber",
	"sugars":       "Sugars",
	"fat":          "Fat",
	"calcium":      "Calcium",
	"iron":         "Iron",
	"potassium":    "Potassium",
	"magnesium":    "Magnesium",
	"sodium":       "Sodium",
	"zinc":         "Zinc",
	"vitamin c":    "Vitamin C",
	"vitamin a":    "Vitamin A, RAE",
	"vitamin d":    "Vitamin D",
	"vitamin b6":   "Vitamin B6",
	"vitamin b12":  "Vitamin B12",
	"vitamin k1":   "Vitamin K1",
	"folate":       "Folate, DFE",
	"cholesterol":  "Cholesterol",
	"caffeine":     "Caffeine",
	"water":        "Water",
	"choline":      "Choline",
	"selenium":     "Selenium",
	"copper":       "Copper",
	"manganese":    "Manganese",
	"phosphorus":   "Phosphorus",
	"niacin":       "Niacin",
	"riboflavin":   "Riboflavin",
	"thiamin":      "Thiamin",
}

// NutrientPageName resolves a user nutrient argument to the site's page name,
// falling back to the argument as-is (title-cased) when not in the alias table.
func NutrientPageName(arg string) string {
	if v, ok := nutrientPageName[strings.ToLower(strings.TrimSpace(arg))]; ok {
		return v
	}
	return arg
}

var reRankRow = regexp.MustCompile(`href='(/[^']*_nutritional_value\.html)[^']*'[^>]*class='table_item_name'>([^<]+)</a></td>\s*<td class='right'>([0-9.]+)(?:&nbsp;|\s)*([a-zA-Z%µ]+)`)

// Rank fetches a NutritionValue.org nutrient-ranking page (highest or lowest)
// and extracts the ranked food rows. order is "highest" or "lowest".
func (c *Client) Rank(ctx context.Context, nutrient, order string, limit int) ([]RankRow, error) {
	page := NutrientPageName(nutrient)
	suffix := "_content.html"
	if strings.EqualFold(order, "lowest") {
		suffix = "_content_lowest.html"
	}
	u := baseURL + "/foods_by_" + url.PathEscape(page) + suffix
	body, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	rows := parseRankRows(body)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func parseRankRows(body string) []RankRow {
	matches := reRankRow.FindAllStringSubmatch(body, -1)
	out := make([]RankRow, 0, len(matches))
	rank := 0
	for _, m := range matches {
		amt, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			continue
		}
		rank++
		out = append(out, RankRow{
			Rank:   rank,
			Name:   cliutil.CleanText(strings.TrimSpace(m[2])),
			Slug:   m[1],
			Amount: amt,
			Unit:   strings.TrimSpace(m[4]),
		})
	}
	return out
}
