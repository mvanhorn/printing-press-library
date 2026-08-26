// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package agoda

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// citySearchTemplate is the verbatim citySearch request captured from live
// browser traffic. Agoda disables GraphQL introspection, so the 30KB operation
// document cannot be reconstructed from a schema and is replayed as-is with
// only the search-specific fields rewritten.
//
//go:embed queries/citysearch_request.json
var citySearchTemplate []byte

const searchPageTypeID = "103"

// buildCitySearch clones the captured template and rewrites only the fields a
// caller can vary.
//
// Currency is written to two independent places on purpose. Setting only
// searchCriteria.currency looks like it works but silently returns Agoda's
// geo-default currency; PricingSummaryRequest.pricing.currency is the field that
// actually drives the returned prices. Both must agree.
func buildCitySearch(opts SearchOptions) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(citySearchTemplate, &payload); err != nil {
		return nil, fmt.Errorf("decoding citySearch template: %w", err)
	}

	checkIn, err := time.Parse("2006-01-02", opts.CheckIn)
	if err != nil {
		return nil, fmt.Errorf("parsing checkin %q: %w", opts.CheckIn, err)
	}
	checkOut := checkIn.AddDate(0, 0, opts.Nights)

	vars, ok := payload["variables"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("citySearch template missing variables")
	}
	csr, ok := vars["CitySearchRequest"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("citySearch template missing CitySearchRequest")
	}
	csr["cityId"] = opts.CityID

	sr, ok := csr["searchRequest"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("citySearch template missing searchRequest")
	}
	sc, ok := sr["searchCriteria"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("citySearch template missing searchCriteria")
	}
	sc["checkInDate"] = checkIn.Format("2006-01-02T00:00:00.000Z")
	sc["localCheckInDate"] = checkIn.Format("2006-01-02")
	sc["bookingDate"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	sc["los"] = opts.Nights
	sc["rooms"] = opts.Rooms
	sc["adults"] = opts.Adults
	sc["children"] = opts.Children
	sc["childAges"] = intsOrEmpty(opts.ChildAges)
	sc["currency"] = opts.Currency
	sc["isUserLoggedIn"] = opts.Authenticated
	if opts.SortField != "" {
		sc["sorting"] = map[string]any{
			"sortField":  opts.SortField,
			"sortOrder":  opts.SortOrder,
			"sortParams": nil,
		}
	}

	// searchId, userId, and ipAddress are non-nullable Strings in Agoda's
	// schema: sending null fails variable type validation for the whole
	// operation. A fresh random searchId per call also keeps requests from
	// looking like a replay of one session.
	searchID := newRequestID()
	if ctx, ok := sr["searchContext"].(map[string]any); ok {
		ctx["locale"] = opts.Locale
		ctx["origin"] = opts.Origin
		ctx["memberId"] = opts.MemberID
		ctx["searchId"] = searchID
	}

	if csum, ok := vars["ContentSummaryRequest"].(map[string]any); ok {
		if cctx, ok := csum["context"].(map[string]any); ok {
			cctx["locale"] = opts.Locale
			cctx["userOrigin"] = opts.Origin
			cctx["rawUserId"] = nil
			cctx["memberId"] = opts.MemberID
			if crit, ok := cctx["searchCriteria"].(map[string]any); ok {
				crit["cityId"] = opts.CityID
			}
			if occ, ok := cctx["occupancy"].(map[string]any); ok {
				occ["numberOfAdults"] = opts.Adults
				occ["numberOfChildren"] = opts.Children
				occ["checkIn"] = checkIn.Format("2006-01-02T00:00:00.000Z")
			}
		}
	}

	// The second, load-bearing currency field. See doc comment above.
	if psr, ok := vars["PricingSummaryRequest"].(map[string]any); ok {
		if pricing, ok := psr["pricing"].(map[string]any); ok {
			pricing["currency"] = opts.Currency
		}
		if pctx, ok := psr["context"].(map[string]any); ok {
			if ci, ok := pctx["clientInfo"].(map[string]any); ok {
				ci["origin"] = opts.Origin
				ci["searchId"] = searchID
			}
			if si, ok := pctx["sessionInfo"].(map[string]any); ok {
				si["isLogin"] = opts.Authenticated
				si["memberId"] = opts.MemberID
			}
		}
		if pricing, ok := psr["pricing"].(map[string]any); ok {
			pricing["bookingDate"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
			pricing["checkIn"] = checkIn.Format("2006-01-02T00:00:00.000Z")
			pricing["checkout"] = checkOut.Format("2006-01-02T00:00:00.000Z")
			// Agoda's pricing service keys off the local (timezone-free) date
			// pair, not the UTC checkIn/checkout above. Leaving these at the
			// captured template values made every search return the template's
			// dates' prices, so --checkin and --nights were silently ignored.
			pricing["localCheckInDate"] = checkIn.Format("2006-01-02")
			pricing["localCheckoutDate"] = checkOut.Format("2006-01-02")
		}
	}

	return json.Marshal(payload)
}

func intsOrEmpty(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}

// CitySearch runs one hotel search and returns normalized properties carrying
// both the advertised and the true all-in price.
func (c *Client) CitySearch(ctx context.Context, opts SearchOptions) ([]Property, error) {
	payload, err := buildCitySearch(opts)
	if err != nil {
		return nil, err
	}
	data, err := c.postGraphQL(ctx, "/graphql/search", payload, searchPageTypeID)
	if err != nil {
		return nil, err
	}
	return parseCitySearch(data, opts)
}

// parseCitySearch walks the deeply nested citySearch envelope into flat rows.
//
// Agoda nests the numbers we need six levels below the property
// (pricing.offers[].roomOffers[].room.pricing[].price), and any level can be
// absent for a sold-out or unavailable property, so every hop is checked.
func parseCitySearch(data []byte, opts SearchOptions) ([]Property, error) {
	var env struct {
		Data struct {
			CitySearch struct {
				Properties []json.RawMessage `json:"properties"`
			} `json:"citySearch"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decoding citySearch response: %w", err)
	}

	nights := opts.Nights
	if nights <= 0 {
		nights = 1
	}

	out := make([]Property, 0, len(env.Data.CitySearch.Properties))
	for _, raw := range env.Data.CitySearch.Properties {
		var node map[string]any
		if err := json.Unmarshal(raw, &node); err != nil {
			continue
		}
		p := Property{Currency: opts.Currency}

		if v, ok := numberAt(node, "propertyId"); ok {
			p.PropertyID = int(v)
		}
		info := mapAt(node, "content", "informationSummary")
		if info != nil {
			p.Name, _ = info["displayName"].(string)
			if v, ok := numberAt(info, "rating"); ok {
				p.StarRating = v
			}
			if addr := mapFrom(info["address"]); addr != nil {
				p.Address = joinAddress(addr)
			}
			if geo := mapFrom(info["geoInfo"]); geo != nil {
				if v, ok := numberAt(geo, "latitude"); ok {
					p.Latitude = v
				}
				if v, ok := numberAt(geo, "longitude"); ok {
					p.Longitude = v
				}
			}
		}
		if rv := mapAt(node, "content", "reviews"); rv != nil {
			if cs := mapFrom(rv["cumulative"]); cs != nil {
				if v, ok := numberAt(cs, "score"); ok {
					p.ReviewScore = v
				}
				if v, ok := numberAt(cs, "reviewCount"); ok {
					p.ReviewCount = int(v)
				}
			}
		}
		if b, ok := node["soldOut"].(bool); ok {
			p.SoldOut = b
		}

		pricing := mapFrom(node["pricing"])
		if pricing == nil {
			out = append(out, p)
			continue
		}
		if cur := firstStringAt(pricing, "currency"); cur != "" {
			p.Currency = cur
		}
		if priceNode := findPriceNode(pricing, 0); priceNode != nil {
			perBook := mapFrom(priceNode["perBook"])
			if perBook != nil {
				if v, ok := displayPrice(perBook["exclusive"]); ok {
					p.PriceAdvertised = v
				}
				if v, ok := displayPrice(perBook["inclusive"]); ok {
					p.PriceAllIn = v
				}
				if x := mapFrom(perBook["inclusive"]); x != nil {
					if v, ok := numberAt(x, "crossedOutPrice"); ok {
						p.CrossedOutPrice = v
					}
				}
			}
		}
		if p.PriceAdvertised > 0 && p.PriceAllIn > 0 {
			p.HiddenAmount = round2(p.PriceAllIn - p.PriceAdvertised)
			p.HiddenPct = round2((p.PriceAllIn - p.PriceAdvertised) / p.PriceAdvertised * 100)
		}
		if p.PriceAllIn > 0 {
			p.PerNightAllIn = round2(p.PriceAllIn / float64(nights))
		}
		p.PriceAdvertised = round2(p.PriceAdvertised)
		p.PriceAllIn = round2(p.PriceAllIn)
		// The real signal lives at pricing.payment.cancellation. The sibling
		// isEasyCancel flag is a different concept and is false even for stays
		// that are genuinely free-cancellation.
		if can := mapAt(pricing, "payment", "cancellation"); can != nil {
			if t, _ := can["cancellationType"].(string); t != "" {
				p.FreeCancellation = strings.EqualFold(t, "FreeCancellation")
			}
			if dt, _ := can["freeCancellationDate"].(string); dt != "" {
				p.FreeCancellationUntil = normalizeDate(dt)
			}
		}
		p.BookingURL = bookingURL(p.PropertyID, opts)

		out = append(out, p)
	}
	return out, nil
}

// findPriceNode selects the price node a search result should report.
//
// Two properties matter here. First, determinism: Go randomizes map iteration
// order, so recursing over an offers map and returning the first hit made the
// reported price vary between runs on the same response. Second, meaning: a
// property can carry several room offers, and the figure a search result should
// show is the cheapest bookable one (Agoda's own envelope calls this
// cheapestRoomOffer).
//
// Collecting every candidate and choosing the minimum all-in price satisfies
// both: the result no longer depends on map ordering, and it is the offer a
// traveler would actually book.
func findPriceNode(node any, depth int) map[string]any {
	candidates := collectPriceNodes(node, depth, nil)
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	bestPrice, bestOK := nodeAllInPrice(best)
	for _, c := range candidates[1:] {
		price, ok := nodeAllInPrice(c)
		switch {
		case ok && !bestOK:
			best, bestPrice, bestOK = c, price, true
		case ok && bestOK && price < bestPrice:
			best, bestPrice = c, price
		}
	}
	return best
}

// collectPriceNodes walks the envelope in a stable order, gathering every node
// that exposes the perBook price triple. Map keys are sorted so traversal does
// not depend on Go's randomized map iteration.
func collectPriceNodes(node any, depth int, acc []map[string]any) []map[string]any {
	if depth > 10 {
		return acc
	}
	switch v := node.(type) {
	case map[string]any:
		if _, ok := v["perBook"]; ok {
			return append(acc, v)
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			acc = collectPriceNodes(v[k], depth+1, acc)
		}
	case []any:
		for _, child := range v {
			acc = collectPriceNodes(child, depth+1, acc)
		}
	}
	return acc
}

// nodeAllInPrice reads the all-in (inclusive) per-booking price used to rank
// competing offers.
//
// It deliberately does NOT fall back to the exclusive figure. Ranking is by
// true all-in cost, and the exclusive price is the pre-tax advertised number,
// so returning it here would let an exclusive-only offer beat a genuine all-in
// offer purely by being the smaller of two different quantities - and the
// selected node would then carry no inclusive price at all, dropping the real
// all-in figure from search, ranking, fee, and cheapest-price output.
//
// A node without an inclusive price reports not-ok and only wins when no
// candidate has one, in which case selection falls back to the first node in
// sorted traversal order and stays deterministic.
func nodeAllInPrice(node map[string]any) (float64, bool) {
	perBook := mapFrom(node["perBook"])
	if perBook == nil {
		return 0, false
	}
	if v, ok := displayPrice(perBook["inclusive"]); ok && v > 0 {
		return v, true
	}
	return 0, false
}

func displayPrice(node any) (float64, bool) {
	m := mapFrom(node)
	if m == nil {
		return 0, false
	}
	return numberAt(m, "display")
}

func mapFrom(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func mapAt(node map[string]any, keys ...string) map[string]any {
	cur := node
	for _, k := range keys {
		if cur == nil {
			return nil
		}
		cur = mapFrom(cur[k])
	}
	return cur
}

// numberAt accepts both JSON numbers and JSON-encoded numeric strings. Agoda
// mixes the two across its pricing envelope, and json.Unmarshal into float64
// silently yields 0 for the string form.
func numberAt(node map[string]any, key string) (float64, bool) {
	if node == nil {
		return 0, false
	}
	switch v := node[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func firstStringAt(node any, key string) string {
	switch v := node.(type) {
	case map[string]any:
		if s, ok := v[key].(string); ok && s != "" {
			return s
		}
		for _, child := range v {
			if got := firstStringAt(child, key); got != "" {
				return got
			}
		}
	case []any:
		for _, child := range v {
			if got := firstStringAt(child, key); got != "" {
				return got
			}
		}
	}
	return ""
}

func joinAddress(addr map[string]any) string {
	parts := []string{}
	for _, k := range []string{"address1", "city", "country"} {
		if s, ok := addr[k].(string); ok && s != "" {
			parts = append(parts, s)
		}
		if sub := mapFrom(addr[k]); sub != nil {
			if s, ok := sub["name"].(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// bookingURL reconstructs a deep link that preserves the caller's dates,
// occupancy, and currency so the user lands on the same stay they priced.
func bookingURL(propertyID int, opts SearchOptions) string {
	if propertyID == 0 {
		return ""
	}
	q := url.Values{}
	// partnersearch.aspx keys the property off "hid". The older
	// agoda.html?hotelId= form this CLI first captured now 404s to
	// pagenotfound.html, so every emitted booking link was dead.
	q.Set("hid", strconv.Itoa(propertyID))
	q.Set("checkIn", opts.CheckIn)
	q.Set("los", strconv.Itoa(opts.Nights))
	q.Set("adults", strconv.Itoa(opts.Adults))
	q.Set("rooms", strconv.Itoa(opts.Rooms))
	if opts.Children > 0 {
		q.Set("children", strconv.Itoa(opts.Children))
	}
	if opts.Currency != "" {
		q.Set("currencyCode", opts.Currency)
	}
	// finalPriceView=1 asks Agoda to render the all-in figure this CLI reports,
	// so the landing page agrees with the number we showed.
	q.Set("finalPriceView", "1")
	return DefaultBaseURL + "/partners/partnersearch.aspx?" + q.Encode()
}

func round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}
