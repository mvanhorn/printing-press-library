// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
	"github.com/spf13/cobra"
)

// needsGrubhubToken reports whether a command belongs to the raw `restaurants`
// endpoint subtree, which calls the API through the generated client and so
// needs an anonymous bearer minted into config first. The friendly commands
// mint on their own path and do not rely on this.
func needsGrubhubToken(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "restaurants" {
			return true
		}
	}
	return false
}

// grubhubClient mints/loads an anonymous bearer token (zero credential setup)
// and returns a configured API client. Generated and hand-written commands both
// authenticate through the token this writes into config.
func grubhubClient(ctx context.Context, flags *rootFlags) (*client.Client, error) {
	if _, err := grubhub.EnsureToken(ctx, flags.configPath); err != nil {
		return nil, authErr(err)
	}
	return flags.newClient()
}

// geocodeAddress resolves a street address to Grubhub coordinates.
func geocodeAddress(ctx context.Context, c *client.Client, address string) (grubhub.Coordinates, error) {
	raw, err := c.Get(ctx, "/geocode", map[string]string{"address": address})
	if err != nil {
		return grubhub.Coordinates{}, err
	}
	return grubhub.ParseGeocode(raw)
}

// searchOptions are the knobs the friendly search commands expose.
type searchOptions struct {
	orderMethod string // "delivery" or "pickup"
	pageSize    int
}

// searchCards runs a restaurant search at a coordinate and returns the parsed
// cards plus the total available result count.
func searchCards(ctx context.Context, c *client.Client, coord grubhub.Coordinates, opts searchOptions) ([]grubhub.Card, int, error) {
	if opts.orderMethod == "" {
		opts.orderMethod = "delivery"
	}
	if opts.pageSize <= 0 {
		opts.pageSize = 40
	}
	params := map[string]string{
		"location":           grubhub.FormatPoint(coord.Lng, coord.Lat),
		"orderMethod":        opts.orderMethod,
		"locationMode":       "DELIVERY",
		"facetSet":           "umamiV6",
		"sortSetId":          "umamiv3",
		"sponsoredSize":      "0",
		"pageNum":            "1",
		"pageSize":           strconv.Itoa(opts.pageSize),
		"includeOffers":      "true",
		"hideHateoasLinks":   "true",
		"countOmittingTimes": "true",
	}
	raw, err := c.Get(ctx, "/restaurants/search", params)
	if err != nil {
		return nil, 0, err
	}
	return grubhub.ParseSearch(raw)
}
