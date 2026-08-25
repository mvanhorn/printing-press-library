// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored real-delivery-fee resolution.
//
// Delivery pricing on foodpanda has three distinct layers, verified 2026-08-08:
//
//	1. disco listing WITHOUT perseus  -> minimum_delivery_fee is a flat floor
//	   (every vendor in Lahore returned 99). Useful as a base, not a real price.
//	2. disco listing WITH perseus     -> the dynamic-pricing service returns 0
//	   for every vendor. Never send perseus to the listing.
//	3. vendor detail WITH perseus + Authorization -> the true per-vendor fee
//	   (n3qk returned 229 with a session vs 0 without).
//
// So a truthful "what does delivery cost me" board needs one detail call per
// vendor. That is bounded by the caller's --limit, run concurrently, and each
// failure is preserved rather than silently collapsing to 0.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/client"
)

type fpVendorFeeEnvelope struct {
	Data struct {
		Code               string  `json:"code"`
		Name               string  `json:"name"`
		MinimumDeliveryFee float64 `json:"minimum_delivery_fee"`
		MinimumOrderAmount float64 `json:"minimum_order_amount"`
		DeliveryConditions []struct {
			DeliveryFee        float64 `json:"delivery_fee"`
			MinimumOrderAmount float64 `json:"minimum_order_amount"`
		} `json:"delivery_conditions"`
	} `json:"data"`
}

// fpVendorFee is the resolved real cost of ordering from one vendor.
type fpVendorFee struct {
	Code        string
	DeliveryFee float64
	MinOrder    float64
	Resolved    bool
	Err         error
}

// fpFetchVendorFee resolves one vendor's true delivery fee. Requires an
// authenticated session; without one the upstream returns 0 and Resolved stays
// false so callers never present an unpriced vendor as free.
func fpFetchVendorFee(ctx context.Context, c *client.Client, country, code string) fpVendorFee {
	out := fpVendorFee{Code: code}
	path := fmt.Sprintf("%s/%s", fpVendorHost(country), code)
	raw, err := c.GetWithHeaders(ctx, path, map[string]string{
		"language_id":     "1",
		"opening_type":    "delivery",
		"basket_currency": "PKR",
	}, fpPerseusHeaders())
	if err != nil {
		out.Err = err
		return out
	}
	var env fpVendorFeeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		out.Err = fmt.Errorf("parsing vendor fee for %s: %w", code, err)
		return out
	}
	d := env.Data
	out.MinOrder = d.MinimumOrderAmount
	fee := d.MinimumDeliveryFee
	if len(d.DeliveryConditions) > 0 && d.DeliveryConditions[0].DeliveryFee > 0 {
		fee = d.DeliveryConditions[0].DeliveryFee
		if d.DeliveryConditions[0].MinimumOrderAmount > 0 {
			out.MinOrder = d.DeliveryConditions[0].MinimumOrderAmount
		}
	}
	out.DeliveryFee = fee
	out.Resolved = fee > 0
	return out
}

// fpFetchVendorFees resolves fees for many vendors concurrently, preserving
// per-vendor errors so a failed lookup is never counted as free delivery.
func fpFetchVendorFees(ctx context.Context, c *client.Client, country string, codes []string, concurrency int) map[string]fpVendorFee {
	if concurrency < 1 {
		concurrency = 6
	}
	out := make(map[string]fpVendorFee, len(codes))
	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, code := range codes {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			f := fpFetchVendorFee(ctx, c, country, code)
			mu.Lock()
			out[code] = f
			mu.Unlock()
		}(code)
	}
	wg.Wait()
	return out
}
