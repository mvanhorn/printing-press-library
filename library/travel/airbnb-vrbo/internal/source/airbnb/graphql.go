package airbnb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/auth"
)

const (
	wishlistIndexHash = "b8b421d802c399b55fb6ac1111014807a454184ad38f198365beb7836c018c18"
	wishlistItemsHash = "c0f9d9474bb20eb7af2f94f8e022750a5ed9b7437613e1d9aa91aadea87e4467"
	bookItHash        = "5560c774d764520fc721f6dffca10d9cff03b25e9907478ded8530caf679d716"
)

func WishlistList(ctx context.Context) ([]Wishlist, error) {
	var root any
	if err := defaultClient.graphQLGet(ctx, "/api/v3/WishlistIndexPageQuery/"+wishlistIndexHash, nil, &root); err != nil {
		return nil, err
	}
	items := findObjects(root, []string{"wishlists", "wishlist"})
	out := make([]Wishlist, 0, len(items))
	for _, m := range items {
		id := str(m["id"])
		name := firstStringByKeys(m, "name", "title")
		if id == "" && name == "" {
			continue
		}
		out = append(out, Wishlist{ID: id, Name: name, Count: int(num(firstByKey(m, "count"))), Raw: m})
	}
	return out, nil
}

func WishlistItems(ctx context.Context, listingIDs []string) ([]WishlistItem, error) {
	params := url.Values{}
	if len(listingIDs) > 0 {
		params.Set("listing_ids", strings.Join(listingIDs, ","))
	}
	var root any
	if err := defaultClient.graphQLGet(ctx, "/api/v3/WishlistItemsAsyncQuery/"+wishlistItemsHash, params, &root); err != nil {
		return nil, err
	}
	objects := findObjects(root, []string{"listingId", "listing_id", "id"})
	out := make([]WishlistItem, 0, len(objects))
	for _, m := range objects {
		id := firstStringByKeys(m, "listingId", "listing_id", "id")
		if id == "" {
			continue
		}
		out = append(out, WishlistItem{ListingID: id, WishlistID: firstStringByKeys(m, "wishlistId", "wishlist_id"), Title: firstStringByKeys(m, "title", "name"), Raw: m})
	}
	return out, nil
}

func BookingPrice(ctx context.Context, listingID, checkin, checkout string, guests int) (*PriceBreakdown, error) {
	variables := map[string]any{
		"id":       RelayListingID(listingID),
		"checkin":  checkin,
		"checkout": checkout,
		"adults":   guests,
	}
	params := url.Values{}
	b, _ := json.Marshal(variables)
	params.Set("variables", string(b))
	var root any
	if err := defaultClient.graphQLGet(ctx, "/api/v3/StaysPdpBookItQuery/"+bookItHash, params, &root); err != nil {
		return nil, err
	}
	return priceBreakdownFromAny(root), nil
}

func (c *Client) graphQLGet(ctx context.Context, path string, params url.Values, out *any) error {
	cookies, err := auth.LoadCookies()
	if err != nil {
		return err
	}
	jar, _ := cookiejar.New(nil)
	base, _ := url.Parse(airbnbBase)
	jar.SetCookies(base, cookies)
	old := c.http.Jar
	c.http.Jar = jar
	defer func() { c.http.Jar = old }()
	u, _ := url.Parse(airbnbBase + path)
	q := u.Query()
	for k, vals := range params {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	q.Set("extensions", `{"persistedQuery":{"version":1,"sha256Hash":"`+path[strings.LastIndex(path, "/")+1:]+`"}}`)
	u.RawQuery = q.Encode()
	data, err := c.do(ctx, "GET", u.String(), airbnbUA, nil, map[string]string{"Accept": "application/json"})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse Airbnb GraphQL response: %w", err)
	}
	return nil
}

func priceBreakdownFromAny(root any) *PriceBreakdown {
	p := &PriceBreakdown{Currency: "USD", Fees: map[string]float64{}, Raw: root}
	for _, obj := range findObjects(root, []string{"label", "amount"}) {
		label := strings.ToLower(firstStringByKeys(obj, "label", "title", "feeType"))
		amount := num(firstByKey(obj, "amount"))
		if amount == 0 {
			amount = amountFromText(firstStringByKeys(obj, "price", "formattedAmount"))
		}
		switch {
		case strings.Contains(label, "clean"):
			p.Fees["cleaning"] += amount
		case strings.Contains(label, "service"):
			p.Fees["service"] += amount
		case strings.Contains(label, "tax"):
			p.Fees["tax"] += amount
		case strings.Contains(label, "total"):
			p.Total = amount
		case strings.Contains(label, "subtotal"):
			p.Subtotal = amount
		}
	}
	if p.Total == 0 {
		p.Total = amountFromText(firstStringByKeys(root, "total", "totalPrice", "localizedTotalPrice"))
	}
	if p.PerNight == 0 {
		p.PerNight = amountFromText(firstStringByKeys(root, "perNight", "perNightPrice"))
	}
	return p
}

func findObjects(root any, keys []string) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, k := range keys {
				if _, ok := x[k]; ok {
					out = append(out, x)
					break
				}
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(root)
	return out
}

func graphQLBody(operation, hash string, variables map[string]any) *bytes.Buffer {
	body := map[string]any{
		"operationName": operation,
		"variables":     variables,
		"extensions": map[string]any{"persistedQuery": map[string]any{
			"version": 1, "sha256Hash": hash,
		}},
	}
	b, _ := json.Marshal(body)
	return bytes.NewBuffer(b)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

var _ = http.MethodGet
