package gql

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// SearchResult is a single product resolved from a natural-language query.
type SearchResult struct {
	Name         string `json:"name"`
	ItemID       string `json:"item_id"`
	ProductID    string `json:"product_id"`
	PriceDisplay string `json:"price_display,omitempty"`
	Retailer     string `json:"retailer,omitempty"`
}

// InventoryTokenTTL is how long a fetched retailerInventorySessionToken stays
// in the local cache before the CLI re-bootstraps it.
const InventoryTokenTTL = 6 * time.Hour

// ResolveProducts runs the three-step chain proven during the 2026-04-11
// planning probes:
//
//  1. ShopCollectionScoped(retailerSlug) to bootstrap the retailerInventorySessionToken
//  2. Autosuggestions(token, query) to get search-term suggestions embedding productIds
//  3. Items(ids) to fetch real product names for those ids
//
// Returns up to `limit` products, ordered by autosuggestion rank (which the
// server already sorts by search relevance).
func ResolveProducts(ctx context.Context, sess *auth.Session, cfg *config.Config, st *store.Store, retailerSlug, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if retailerSlug == "" {
		return nil, fmt.Errorf("retailer slug is required")
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	client := NewClient(sess, cfg, st)

	// Step 1: get or bootstrap the retailer inventory token.
	tok, err := ensureInventoryToken(ctx, client, st, cfg, retailerSlug)
	if err != nil {
		return nil, fmt.Errorf("bootstrap retailer context for %s: %w", retailerSlug, err)
	}

	// Step 2: autosuggestions. Sanitize query -- the server returns empty for
	// queries containing '%' so we strip characters the autosuggest path
	// won't match on.
	sanitized := sanitizeQuery(query)
	session := uuid.NewString()
	autosugVars := map[string]any{
		"retailerInventorySessionToken": tok.Token,
		"query":                         sanitized,
		"autosuggestionSessionId":       session,
	}
	resp, err := client.Query(ctx, "Autosuggestions", autosugVars)
	if err != nil {
		return nil, fmt.Errorf("autosuggestions: %w", err)
	}

	productIDs, err := extractProductIDs(resp.RawBody, limit*3)
	if err != nil {
		return nil, err
	}
	if len(productIDs) == 0 {
		// Retry once with the longest alphabetic word from the query -- catches
		// cases like "2 milk" where the combined query matches nothing but
		// "milk" alone returns real suggestions.
		if word := longestWord(sanitized); word != "" && word != sanitized {
			autosugVars["query"] = word
			autosugVars["autosuggestionSessionId"] = uuid.NewString()
			resp, err = client.Query(ctx, "Autosuggestions", autosugVars)
			if err != nil {
				return nil, fmt.Errorf("autosuggestions retry: %w", err)
			}
			productIDs, _ = extractProductIDs(resp.RawBody, limit*3)
		}
	}
	if len(productIDs) == 0 {
		return nil, fmt.Errorf("no suggestions matched %q at %s", query, retailerSlug)
	}

	// Step 3: fetch real product data via Items with items_<locationId>-<productId>.
	locID := tok.LocationID
	if locID == "" {
		locID = "1576" // Costco fallback; in practice tok.LocationID is always set
	}
	var itemIDs []string
	for _, pid := range productIDs {
		itemIDs = append(itemIDs, "items_"+locID+"-"+pid)
	}

	itemsVars := map[string]any{
		"ids":        itemIDs,
		"shopId":     tok.ShopID,
		"zoneId":     tok.ZoneID,
		"postalCode": cfg.PostalCode,
	}
	resp, err = client.Query(ctx, "Items", itemsVars)
	if err != nil {
		return nil, fmt.Errorf("items lookup: %w", err)
	}

	results := parseItemsResponse(resp.RawBody, retailerSlug, limit)
	if len(results) == 0 {
		return nil, fmt.Errorf("items response empty for %q at %s", query, retailerSlug)
	}

	// Score by query relevance: prefer items whose name contains all query
	// tokens over items that merely match autosuggest ranking. Autosuggest
	// returns loosely-related products so a query like "corona" can return
	// "Corona Extra" alongside "Coronita" -- we want exact-ish matches first.
	rerank(results, query)

	// Persist to local product cache for offline fallback and future cart-show
	// name resolution.
	for _, r := range results {
		_ = st.UpsertProduct(store.Product{
			ItemID:       r.ItemID,
			ProductID:    r.ProductID,
			RetailerSlug: retailerSlug,
			Name:         r.Name,
		})
	}
	return results, nil
}

// ensureInventoryToken returns a fresh-enough retailerInventorySessionToken for
// the given retailer, bootstrapping via ShopCollectionScoped on cache miss.
// The response also carries shop id, retailer id, and retailer name, which we
// cache alongside the token.
func ensureInventoryToken(ctx context.Context, client *Client, st *store.Store, cfg *config.Config, slug string) (*store.InventoryToken, error) {
	if cached, err := st.GetInventoryToken(slug); err == nil && cached != nil && cached.ShopID != "" {
		return cached, nil
	}

	// PATCH (fix-shop-collection-coordinates):
	// Use the typed constructor so coordinates is omitted entirely when
	// neither latitude nor longitude is set (the common config path for
	// users who only set postal_code). Previously this builder always
	// sent {latitude: 0, longitude: 0}, which Instacart's UsersCoordinates
	// input rejects, causing every search/add/cart-show bootstrap to fail
	// with "no shops" — see mvanhorn/printing-press-library#501.
	vars := instacart.NewShopCollectionScopedVars(slug, cfg.PostalCode, cfg.AddressID, cfg.Latitude, cfg.Longitude)
	resp, err := client.Query(ctx, "ShopCollectionScoped", vars)
	if err != nil {
		return nil, fmt.Errorf("ShopCollectionScoped: %w", err)
	}

	shop, err := parseShopCollectionScoped(resp.RawBody)
	if err != nil {
		return nil, err
	}
	if shop.Token == "" {
		return nil, fmt.Errorf("no retailerInventorySessionToken in ShopCollectionScoped response for %s", slug)
	}

	locID, _ := parseTokenLocation(shop.Token)
	if locID == "" {
		locID = "1576" // fallback; should never hit in practice
	}

	tok := store.InventoryToken{
		RetailerSlug: slug,
		Token:        shop.Token,
		LocationID:   locID,
		ShopID:       shop.ShopID,
		ZoneID:       cfg.EffectiveZoneID(),
		FetchedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(InventoryTokenTTL),
	}
	_ = st.UpsertRetailer(store.Retailer{
		Slug:       slug,
		RetailerID: shop.RetailerID,
		ShopID:     shop.ShopID,
		ZoneID:     cfg.EffectiveZoneID(),
		Name:       shop.RetailerName,
		LocationID: locID,
	})
	if err := st.UpsertInventoryToken(tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

// parseShopCollectionScoped walks the response for the first shop entry and
// returns the bits we care about: inventory token, shop id, retailer id,
// retailer name. Returns an error when the response shape is unrecognized.
func parseShopCollectionScoped(raw []byte) (parsedShop, error) {
	var envelope struct {
		Data struct {
			ShopCollection struct {
				Shops []struct {
					ID                            string `json:"id"`
					RetailerInventorySessionToken string `json:"retailerInventorySessionToken"`
					Retailer                      struct {
						ID   string `json:"id"`
						Slug string `json:"slug"`
						Name string `json:"name"`
					} `json:"retailer"`
				} `json:"shops"`
			} `json:"shopCollection"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return parsedShop{}, fmt.Errorf("parse ShopCollectionScoped: %w", err)
	}
	if len(envelope.Data.ShopCollection.Shops) == 0 {
		return parsedShop{}, fmt.Errorf("ShopCollectionScoped returned no shops")
	}
	s := envelope.Data.ShopCollection.Shops[0]
	return parsedShop{
		Token:        s.RetailerInventorySessionToken,
		ShopID:       s.ID,
		RetailerID:   s.Retailer.ID,
		RetailerName: s.Retailer.Name,
	}, nil
}

type parsedShop struct {
	Token        string
	ShopID       string
	RetailerID   string
	RetailerName string
}

// parseTokenLocation extracts (locationId, retailerId) from a
// v1.<hash>.<customerId>-<zip>-<signed>-?-<retailerId>-<locationId>-?-? token.
// Observed field order (Costco token: v1.<hash>.<customerId>-<zip>-<signed>-1-<retailerId>-<locationId>-1-0):
//
//	0: customerId
//	1: zip
//	2: signed (arbitrary alnum)
//	3: separator constant
//	4: retailerId
//	5: retailerLocationId
//	6,7: additional separators
//
// zoneId is NOT encoded here despite earlier speculation -- the trailing
// fields are always "1-0" regardless of retailer or user, so they cannot be
// zone-specific. Callers must source zoneId from config.EffectiveZoneID.
func parseTokenLocation(token string) (locID, retID string) {
	parts := strings.Split(token, ".")
	if len(parts) < 3 {
		return "", ""
	}
	fields := strings.Split(parts[2], "-")
	if len(fields) >= 6 {
		retID = fields[4]
		locID = fields[5]
	}
	return
}

var imageThingIDRe = regexp.MustCompile(`image_thing_id=(\d+)`)

// extractProductIDs parses an Autosuggestions response and returns the
// embedded productIds from each suggestion's tracking URL, preserving
// autosuggest order and deduplicating.
func extractProductIDs(raw []byte, max int) ([]string, error) {
	matches := imageThingIDRe.FindAllSubmatch(raw, -1)
	seen := make(map[string]bool)
	var out []string
	for _, m := range matches {
		id := string(m[1])
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out, nil
}

// parseItemsResponse walks an Items GraphQL response and produces
// SearchResult entries. The Instacart response shape is
// {data: {items: [{id, name, ...}]}}.
func parseItemsResponse(raw []byte, retailerSlug string, limit int) []SearchResult {
	var envelope struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	var results []SearchResult
	for _, it := range envelope.Data.Items {
		id, _ := it["id"].(string)
		name, _ := it["name"].(string)
		if id == "" || name == "" {
			continue
		}
		prodID := id
		if idx := strings.LastIndex(id, "-"); idx > 0 {
			prodID = id[idx+1:]
		}
		results = append(results, SearchResult{
			Name:      name,
			ItemID:    id,
			ProductID: prodID,
			Retailer:  retailerSlug,
		})
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results
}

// sanitizeQuery strips characters that break Instacart's autosuggest matching
// and returns a clean version suitable for passing to the GraphQL endpoint.
func sanitizeQuery(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, "%", "")
	for strings.Contains(q, "  ") {
		q = strings.ReplaceAll(q, "  ", " ")
	}
	return strings.TrimSpace(q)
}

// longestWord returns the longest alphabetic word in the query. It's used as
// a retry fallback when the full query doesn't match any autosuggestion:
// "2% milk" sanitized is "2 milk"; longestWord picks "milk" which does match.
// Prefers alphabetic tokens over numeric ones.
func longestWord(q string) string {
	fields := strings.Fields(q)
	best := ""
	for _, f := range fields {
		// skip tokens that are purely numeric or shorter than 3 chars
		if len(f) < 3 {
			continue
		}
		allDigit := true
		for _, r := range f {
			if r < '0' || r > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			continue
		}
		if len(f) > len(best) {
			best = f
		}
	}
	return best
}

// rerank promotes results whose names contain all query tokens (case-insensitive)
// over results that just happen to be in the autosuggest response. Autosuggest
// returns a broad category of products; we want the closest name match first.
func rerank(results []SearchResult, query string) {
	tokens := strings.Fields(strings.ToLower(sanitizeQuery(query)))
	if len(tokens) == 0 {
		return
	}
	score := func(name string) int {
		l := strings.ToLower(name)
		matched := 0
		for _, t := range tokens {
			if strings.Contains(l, t) {
				matched++
			}
		}
		return matched
	}
	// Stable-ish bubble: swap adjacent pairs where the latter scores higher.
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if score(results[j].Name) > score(results[i].Name) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
