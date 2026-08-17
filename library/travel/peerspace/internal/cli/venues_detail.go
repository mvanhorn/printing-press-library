// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared pure-HTTP listing detail fetch (GET /v1/listings/{id}) for hydrate + venues get.
// Captured/validated 2026-07-16 against listing page traffic + live probe.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
)

const listingDetailPathPrefix = "/v1/listings/"

// listingAuthHeaders returns Authorization: Bearer <PSAccess> when available.
// Listing detail works with cookies alone in many cases; bearer matches SPA.
func listingAuthHeaders(c *client.Client) map[string]string {
	if c == nil || c.Config == nil {
		return nil
	}
	if bearer := psAccessBearerFromCookieHeader(c.Config.CookieCredential()); bearer != "" {
		return map[string]string{"Authorization": bearer}
	}
	return nil
}

// fetchListingDetail loads GET /v1/listings/{id} via Surf client (cookie jar).
func fetchListingDetail(ctx context.Context, c *client.Client, listingID string) (json.RawMessage, error) {
	listingID = strings.TrimSpace(listingID)
	if listingID == "" {
		return nil, fmt.Errorf("listing id is required")
	}
	path := listingDetailPathPrefix + listingID
	headers := listingAuthHeaders(c)
	var (
		data json.RawMessage
		err  error
	)
	if len(headers) > 0 {
		data, err = c.GetWithHeaders(ctx, path, nil, headers)
	} else {
		data, err = c.Get(ctx, path, nil)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// upsertListingDetail stores a full detail document under resource_type=venues.
func upsertListingDetail(ctx context.Context, listingID string, data json.RawMessage) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("peerspace-pp-cli"))
	if err != nil {
		return err
	}
	defer db.Close()
	// Prefer extracting id from payload; fall back to requested id.
	id := listingID
	if l, ok := venuex.ParseListing(data); ok && l.ID != "" {
		id = l.ID
	}
	return db.Upsert("venues", id, data)
}

// parseAndUpsertListingDetail parses, upserts, and returns normalized listing.
func parseAndUpsertListingDetail(ctx context.Context, listingID string, data json.RawMessage) (venuex.Listing, error) {
	l, ok := venuex.ParseListing(data)
	if !ok {
		return venuex.Listing{}, fmt.Errorf("response is not a listing object for %s", listingID)
	}
	if l.ID == "" {
		l.ID = listingID
	}
	l.Hydrated = true
	l.FormatFit = venuex.InferFormatFit(l)
	if err := upsertListingDetail(ctx, l.ID, data); err != nil {
		return l, err
	}
	return l, nil
}
