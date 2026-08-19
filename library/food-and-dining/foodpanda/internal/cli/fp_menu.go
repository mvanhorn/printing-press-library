// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored menu fetching and parsing shared by dish and menu-diff.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/cliutil"
)

// fpMenuEnvelope mirrors the /api/v5/vendors/{code} response shape.
type fpMenuEnvelope struct {
	Data struct {
		Code           string  `json:"code"`
		Name           string  `json:"name"`
		Rating         float64 `json:"rating"`
		MinDeliveryFee float64 `json:"minimum_delivery_fee"`
		Menus          []struct {
			Name           string `json:"name"`
			MenuCategories []struct {
				Name     string `json:"name"`
				Products []struct {
					ID                float64 `json:"id"`
					Name              string  `json:"name"`
					Description       string  `json:"description"`
					ProductVariations []struct {
						ID    float64 `json:"id"`
						Name  string  `json:"name"`
						Price float64 `json:"price"`
					} `json:"product_variations"`
				} `json:"products"`
			} `json:"menu_categories"`
		} `json:"menus"`
	} `json:"data"`
}

// fpProduct is one purchasable item, flattened out of the nested menu tree.
type fpProduct struct {
	VendorCode  string  `json:"vendor_code"`
	VendorName  string  `json:"vendor_name"`
	Category    string  `json:"category"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price"`
	Variation   string  `json:"variation,omitempty"`
	MatchedOn   string  `json:"matched_on,omitempty"`
}

// fpFetchMenu retrieves one vendor's full menu payload.
func fpFetchMenu(ctx context.Context, c *client.Client, country, vendorCode string) (json.RawMessage, error) {
	path := fmt.Sprintf("%s/%s", fpVendorHost(country), vendorCode)
	// The menu host is the ONLY endpoint that requires perseus headers; it 400s
	// with "perseus headers are absent" without them. They must NOT be sent to
	// the disco listing, where they cause the dynamic-pricing service to return
	// a 0 delivery fee for every vendor.
	return c.GetWithHeaders(ctx, path, map[string]string{
		"include":         "menus",
		"language_id":     "1",
		"opening_type":    "delivery",
		"basket_currency": "PKR",
	}, fpPerseusHeaders())
}

// fpParseMenu flattens a menu payload into products. Text is entity-decoded via
// cliutil.CleanText so upstream HTML entities do not leak into output.
func fpParseMenu(raw json.RawMessage) (vendorName string, products []fpProduct, err error) {
	var env fpMenuEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", nil, fmt.Errorf("parsing menu payload: %w", err)
	}
	d := env.Data
	products = make([]fpProduct, 0, 64)
	for _, m := range d.Menus {
		for _, cat := range m.MenuCategories {
			for _, p := range cat.Products {
				name := cliutil.CleanText(p.Name)
				desc := cliutil.CleanText(p.Description)
				if len(p.ProductVariations) == 0 {
					products = append(products, fpProduct{
						VendorCode: d.Code, VendorName: d.Name,
						Category: cliutil.CleanText(cat.Name), Name: name, Description: desc,
					})
					continue
				}
				for _, pv := range p.ProductVariations {
					products = append(products, fpProduct{
						VendorCode: d.Code, VendorName: d.Name,
						Category: cliutil.CleanText(cat.Name), Name: name, Description: desc,
						Price: fpRound2(pv.Price), Variation: cliutil.CleanText(pv.Name),
					})
				}
			}
		}
	}
	return d.Name, products, nil
}

// fpMenuFetchResult carries per-vendor success or failure so a failed fetch is
// never silently counted as "this vendor has no matching products".
type fpMenuFetchResult struct {
	VendorCode string
	VendorName string
	Raw        json.RawMessage
	Products   []fpProduct
	Err        error
}

// fpFetchMenus pulls menus for many vendors concurrently, preserving each
// error through the channel so aggregates can exclude failures explicitly.
func fpFetchMenus(ctx context.Context, c *client.Client, country string, vendors []fpVendor, concurrency int) []fpMenuFetchResult {
	if concurrency < 1 {
		concurrency = 4
	}
	out := make([]fpMenuFetchResult, len(vendors))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, v := range vendors {
		wg.Add(1)
		go func(i int, v fpVendor) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res := fpMenuFetchResult{VendorCode: v.Code, VendorName: v.Name}
			raw, err := fpFetchMenu(ctx, c, country, v.Code)
			if err != nil {
				res.Err = err
				out[i] = res
				return
			}
			name, prods, err := fpParseMenu(raw)
			if err != nil {
				res.Err = err
				out[i] = res
				return
			}
			if name != "" {
				res.VendorName = name
			}
			res.Raw, res.Products = raw, prods
			out[i] = res
		}(i, v)
	}
	wg.Wait()
	return out
}

// fpProductMatches reports whether a product matches a free-text query.
// Every token must appear somewhere in name, variation, category or description.
func fpProductMatches(p fpProduct, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	hay := strings.ToLower(p.Name + " " + p.Variation + " " + p.Category + " " + p.Description)
	for _, t := range tokens {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

func fpQueryTokens(q string) []string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(q)))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// fpMatchedOn reports where a product matched: "name" is a direct item-name hit,
// "category"/"description" are weaker context hits.
//
// This distinction matters: at a restaurant whose menu category is "Biryani",
// a side salad matches on category alone. Ranking purely by price would then
// surface the salad above every actual biryani.
func fpMatchedOn(p fpProduct, tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	name := strings.ToLower(p.Name + " " + p.Variation)
	all := true
	for _, t := range tokens {
		if !strings.Contains(name, t) {
			all = false
			break
		}
	}
	if all {
		return "name"
	}
	cat := strings.ToLower(p.Category)
	all = true
	for _, t := range tokens {
		if !strings.Contains(cat, t) {
			all = false
			break
		}
	}
	if all {
		return "category"
	}
	return "description"
}
