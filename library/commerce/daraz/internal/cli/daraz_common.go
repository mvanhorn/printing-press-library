// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared helpers for the Daraz novel commands (price-history,
// watch, since, deals, value, compare, seller stats, products get). Not
// generated; safe to edit. Everything here rides on the public, replayable
// catalog search JSON (https://www.daraz.pk/catalog/?ajax=true) plus the local
// SQLite store, which is what makes the offline/price-intelligence features
// possible.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/daraz/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/daraz/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/daraz/internal/store"

	"github.com/spf13/cobra"
)

const darazUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// darazProduct is one entry from mods.listItems[] in the catalog search JSON.
type darazProduct struct {
	ItemID        string `json:"itemId"`
	Name          string `json:"name"`
	Price         string `json:"price"`
	OriginalPrice string `json:"originalPrice"`
	Discount      string `json:"discount"`
	RatingScore   string `json:"ratingScore"`
	Review        string `json:"review"`
	Sold          string `json:"itemSoldCntShow"`
	Location      string `json:"location"`
	SellerName    string `json:"sellerName"`
	SellerID      string `json:"sellerId"`
	BrandName     string `json:"brandName"`
	InStock       bool   `json:"inStock"`
	ItemURL       string `json:"itemUrl"`
	Image         string `json:"image"`
}

var moneyRe = regexp.MustCompile(`[0-9][0-9,]*(?:\.[0-9]+)?`)

// parseMoney pulls a float out of a price-ish string ("7199", "Rs. 1,499.50").
// It finds the first number (allowing thousands separators) and ignores any
// surrounding currency text or punctuation.
func parseMoney(s string) float64 {
	m := moneyRe.FindString(s)
	if m == "" {
		return 0
	}
	m = strings.ReplaceAll(m, ",", "")
	f, _ := strconv.ParseFloat(m, 64)
	return f
}

// leadingInt reads the first run of digits ("546 sold" -> 546).
func leadingInt(s string) int {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			break
		}
	}
	n, _ := strconv.Atoi(b.String())
	return n
}

func (p darazProduct) priceF() float64 { return parseMoney(p.Price) }
func (p darazProduct) origF() float64  { return parseMoney(p.OriginalPrice) }
func (p darazProduct) ratingF() float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(p.RatingScore), 64)
	return f
}
func (p darazProduct) reviewN() int { return leadingInt(p.Review) }
func (p darazProduct) soldN() int   { return parseSold(p.Sold) }

var soldRe = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*([kmb])?`)

// parseSold reads Daraz's sold-count strings, expanding k/m/b shorthand so a
// "1.2k sold" item counts as ~1200, not 1. The deals composite uses this as the
// sales-volume factor, so truncating "k" would systematically underrank the
// most popular listings.
func parseSold(s string) int {
	m := soldRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	f, _ := strconv.ParseFloat(m[1], 64)
	switch strings.ToLower(m[2]) {
	case "k":
		f *= 1e3
	case "m":
		f *= 1e6
	case "b":
		f *= 1e9
	}
	return int(f)
}

// discountPct prefers the site's "66% Off" string, falling back to a computed
// (original-price - price)/original-price when the label is missing.
func (p darazProduct) discountPct() float64 {
	if n := leadingInt(p.Discount); n > 0 {
		return float64(n)
	}
	o, pr := p.origF(), p.priceF()
	if o > pr && o > 0 {
		return (o - pr) / o * 100
	}
	return 0
}

func (p darazProduct) fullURL() string {
	if strings.HasPrefix(p.ItemURL, "//") {
		return "https:" + p.ItemURL
	}
	return p.ItemURL
}

// dealScore is the composite used by `deals`: discount %, weighted by rating
// (0..1 of 5 stars) and by sales volume (log-damped so a single mega-seller
// does not dominate). Pure function — unit tested.
func dealScore(discountPct, rating float64, sold int) float64 {
	r := rating / 5.0
	if r <= 0 {
		r = 0.2 // unrated items are not zeroed out, just discounted
	}
	volume := math.Log10(float64(sold) + 10) // sold=0 -> 1.0
	return discountPct * r * volume
}

// medianFloat returns the median of xs (0 for empty). Pure — unit tested.
func medianFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	n := len(c)
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// flexInt accepts a JSON number OR a quoted-number string (Daraz returns
// totalResults both ways depending on the query). Never errors.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		*f = flexInt(n)
	} else if g, err := strconv.ParseFloat(s, 64); err == nil {
		*f = flexInt(int(g))
	}
	return nil
}

// asStr coerces any JSON scalar (string, number, bool) to a string. Daraz's
// catalog payload is loosely typed — IDs and counts arrive as either strings or
// numbers — so every field is normalized through here.
func asStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	}
	return false
}

func productFromMap(m map[string]any) darazProduct {
	return darazProduct{
		ItemID:        asStr(m["itemId"]),
		Name:          asStr(m["name"]),
		Price:         asStr(m["price"]),
		OriginalPrice: asStr(m["originalPrice"]),
		Discount:      asStr(m["discount"]),
		RatingScore:   asStr(m["ratingScore"]),
		Review:        asStr(m["review"]),
		Sold:          asStr(m["itemSoldCntShow"]),
		Location:      asStr(m["location"]),
		SellerName:    asStr(m["sellerName"]),
		SellerID:      asStr(m["sellerId"]),
		BrandName:     asStr(m["brandName"]),
		InStock:       asBool(m["inStock"]),
		ItemURL:       asStr(m["itemUrl"]),
		Image:         asStr(m["image"]),
	}
}

// fetchSearch calls the catalog endpoint once and returns the page's items plus
// the reported total result count.
func fetchSearch(ctx context.Context, c *client.Client, query string, page int, sortOrder, price string) ([]darazProduct, int, error) {
	params := map[string]string{"ajax": "true", "q": query}
	if page > 0 {
		params["page"] = strconv.Itoa(page)
	}
	if sortOrder != "" {
		params["sort"] = sortOrder
	}
	if price != "" {
		params["price"] = price
	}
	raw, err := c.Get(ctx, "/catalog/", params)
	if err != nil {
		return nil, 0, err
	}
	var env struct {
		Mods struct {
			ListItems []map[string]any `json:"listItems"`
			MainInfo  struct {
				TotalResults flexInt `json:"totalResults"`
			} `json:"mainInfo"`
		} `json:"mods"`
		MainInfo struct {
			TotalResults flexInt `json:"totalResults"`
		} `json:"mainInfo"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, 0, fmt.Errorf("parsing Daraz search response: %w", err)
	}
	total := int(env.Mods.MainInfo.TotalResults)
	if total == 0 {
		total = int(env.MainInfo.TotalResults)
	}
	items := make([]darazProduct, 0, len(env.Mods.ListItems))
	for _, m := range env.Mods.ListItems {
		items = append(items, productFromMap(m))
	}
	return items, total, nil
}

// scanSearch paginates the catalog endpoint up to maxScanPages (40 items/page),
// stopping early once limit matches are gathered or a short/empty page is hit.
// Under live-dogfood it curtails to a single page to fit the 30s budget.
func scanSearch(ctx context.Context, c *client.Client, query, sortOrder, price string, maxScanPages, limit int) (items []darazProduct, total int, scanned int, err error) {
	if maxScanPages < 1 {
		maxScanPages = 1
	}
	if cliutil.IsDogfoodEnv() && maxScanPages > 1 {
		maxScanPages = 1
	}
	for page := 1; page <= maxScanPages; page++ {
		batch, t, e := fetchSearch(ctx, c, query, page, sortOrder, price)
		if e != nil {
			return items, total, scanned, e
		}
		if t > 0 {
			total = t
		}
		scanned += len(batch)
		if len(batch) == 0 {
			break
		}
		items = append(items, batch...)
		if limit > 0 && len(items) >= limit {
			break
		}
		if len(batch) < 40 {
			break
		}
	}
	return items, total, scanned, nil
}

// openDarazStore opens the local SQLite store and ensures the Daraz-specific
// tables exist.
func openDarazStore(ctx context.Context, _ *rootFlags) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, defaultDBPath("daraz-pp-cli"))
	if err != nil {
		return nil, err
	}
	if err := ensureDarazTables(ctx, s.DB()); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func ensureDarazTables(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS daraz_products_seen (
  item_id TEXT PRIMARY KEY,
  name TEXT, price REAL, original_price REAL, discount_pct REAL,
  rating REAL, review_count INTEGER, sold INTEGER,
  seller_id TEXT, seller_name TEXT, brand TEXT, location TEXT,
  item_url TEXT, last_seen INTEGER);
CREATE INDEX IF NOT EXISTS idx_daraz_seen_seller ON daraz_products_seen(seller_id);
CREATE TABLE IF NOT EXISTS daraz_price_snapshots (
  item_id TEXT, name TEXT, price REAL, ts INTEGER);
CREATE INDEX IF NOT EXISTS idx_daraz_snap_item ON daraz_price_snapshots(item_id, ts);
CREATE TABLE IF NOT EXISTS daraz_search_snapshots (
  query TEXT, item_id TEXT, name TEXT, price REAL, ts INTEGER);
CREATE INDEX IF NOT EXISTS idx_daraz_search_q ON daraz_search_snapshots(query, ts);
`)
	return err
}

// recordProducts upserts every item into products_seen and appends a price
// snapshot, so the local price database compounds with normal use. Best-effort:
// store failures never break the user-facing command.
func recordProducts(ctx context.Context, s *store.Store, items []darazProduct) {
	if s == nil || len(items) == 0 {
		return
	}
	now := time.Now().Unix()
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return
	}
	for _, p := range items {
		if p.ItemID == "" {
			continue
		}
		_, _ = tx.ExecContext(ctx, `
INSERT INTO daraz_products_seen
 (item_id,name,price,original_price,discount_pct,rating,review_count,sold,seller_id,seller_name,brand,location,item_url,last_seen)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(item_id) DO UPDATE SET
   name=excluded.name, price=excluded.price, original_price=excluded.original_price,
   discount_pct=excluded.discount_pct, rating=excluded.rating, review_count=excluded.review_count,
   sold=excluded.sold, seller_id=excluded.seller_id, seller_name=excluded.seller_name,
   brand=excluded.brand, location=excluded.location, item_url=excluded.item_url, last_seen=excluded.last_seen`,
			p.ItemID, p.Name, p.priceF(), p.origF(), p.discountPct(), p.ratingF(), p.reviewN(), p.soldN(),
			p.SellerID, p.SellerName, p.BrandName, p.Location, p.fullURL(), now)
		// Only append a price snapshot when the price actually changed since
		// the most recent one for this item. Repeated searches at the same
		// price would otherwise accumulate identical rows without bound; this
		// keeps daraz_price_snapshots to one row per genuine price change.
		_, _ = tx.ExecContext(ctx, `INSERT INTO daraz_price_snapshots (item_id,name,price,ts)
			SELECT ?,?,?,?
			WHERE NOT EXISTS (
				SELECT 1 FROM daraz_price_snapshots s
				WHERE s.item_id = ? AND s.price = ?
				  AND s.ts = (SELECT MAX(ts) FROM daraz_price_snapshots WHERE item_id = ?)
			)`,
			p.ItemID, p.Name, p.priceF(), now, p.ItemID, p.priceF(), p.ItemID)
	}
	_ = tx.Commit()
}

// emitDaraz routes a Go value through the standard output pipeline so every
// novel command gets --json/--agent/--select/--csv/--compact for free, with a
// human table fallback for arrays of objects.
func emitDaraz(cmd *cobra.Command, flags *rootFlags, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		var items []map[string]any
		if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
			return printAutoTable(cmd.OutOrStdout(), items)
		}
	}
	return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
}

// snapEntry is one item's price at the time a search snapshot was taken.
type snapEntry struct {
	name  string
	price float64
}

// recordSearchSnapshot stores the current membership+price of a query so a later
// `since` run can diff against it. Uses a single shared timestamp per snapshot.
func recordSearchSnapshot(ctx context.Context, s *store.Store, query string, items []darazProduct) {
	if s == nil {
		return
	}
	now := time.Now().Unix()
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return
	}
	for _, p := range items {
		if p.ItemID == "" {
			continue
		}
		_, _ = tx.ExecContext(ctx, `INSERT INTO daraz_search_snapshots (query,item_id,name,price,ts) VALUES (?,?,?,?,?)`,
			query, p.ItemID, p.Name, p.priceF(), now)
	}
	_ = tx.Commit()
}

// loadLastSearchSnapshot returns the most recent prior snapshot for a query as a
// map of itemID -> {name, price}. ts is 0 when no prior snapshot exists.
func loadLastSearchSnapshot(ctx context.Context, s *store.Store, query string) (ts int64, entries map[string]snapEntry, err error) {
	entries = map[string]snapEntry{}
	row := s.DB().QueryRowContext(ctx, `SELECT COALESCE(MAX(ts),0) FROM daraz_search_snapshots WHERE query=?`, query)
	if err = row.Scan(&ts); err != nil || ts == 0 {
		return ts, entries, err
	}
	rows, err := s.DB().QueryContext(ctx, `SELECT item_id,name,price FROM daraz_search_snapshots WHERE query=? AND ts=?`, query, ts)
	if err != nil {
		return ts, entries, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var name sql.NullString
		var price sql.NullFloat64
		if err := rows.Scan(&id, &name, &price); err != nil {
			continue
		}
		entries[id] = snapEntry{name: name.String, price: price.Float64}
	}
	return ts, entries, rows.Err()
}

// emptyMirrorHint prints the standard "no local mirror" guidance and returns an
// empty JSON result for machine consumers. Returns nil (an empty local cache is
// not an error).
func emptyMirrorHint(cmd *cobra.Command, flags *rootFlags, hint string) error {
	fmt.Fprintln(cmd.ErrOrStderr(), hint)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return nil
}

// ---- Product detail (PDP) via schema.org JSON-LD ----

type productDetail struct {
	ItemID        string  `json:"itemId"`
	Name          string  `json:"name"`
	Brand         string  `json:"brand,omitempty"`
	Category      string  `json:"category,omitempty"`
	SKU           string  `json:"sku,omitempty"`
	Description   string  `json:"description,omitempty"`
	Availability  string  `json:"availability,omitempty"`
	Price         float64 `json:"price,omitempty"`
	OriginalPrice float64 `json:"originalPrice,omitempty"`
	PriceAsOf     string  `json:"priceAsOf,omitempty"`
	Image         string  `json:"image,omitempty"`
	URL           string  `json:"url"`
}

var ldJSONRe = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)

func ldStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if s, ok := t["name"].(string); ok {
			return s
		}
	}
	return ""
}

func firstImage(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func lastPathSeg(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// extractProductDetail parses the PDP's schema.org Product JSON-LD. Price is not
// present in JSON-LD on Daraz, so callers fill it from the local store.
func extractProductDetail(html, itemID string) *productDetail {
	pd := &productDetail{
		ItemID: itemID,
		URL:    fmt.Sprintf("https://www.daraz.pk/products/-i%s.html", itemID),
	}
	for _, m := range ldJSONRe.FindAllStringSubmatch(html, -1) {
		var obj map[string]any
		if json.Unmarshal([]byte(strings.TrimSpace(m[1])), &obj) != nil {
			continue
		}
		if t, _ := obj["@type"].(string); t != "Product" {
			continue
		}
		pd.Name = ldStr(obj["name"])
		pd.Category = ldStr(obj["category"])
		pd.SKU = ldStr(obj["sku"])
		pd.Brand = ldStr(obj["brand"])
		pd.Image = firstImage(obj["image"])
		if d := ldStr(obj["description"]); d != "" {
			pd.Description = cliutil.CleanText(d)
		}
		if u := ldStr(obj["url"]); u != "" {
			pd.URL = u
		}
		if off, ok := obj["offers"].(map[string]any); ok {
			pd.Availability = lastPathSeg(ldStr(off["availability"]))
		}
	}
	return pd
}
