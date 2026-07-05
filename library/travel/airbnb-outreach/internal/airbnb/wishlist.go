// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import "encoding/json"

// Wishlists returns the signed-in user's wishlists (saved-listing collections).
func (c *Client) Wishlists(limit, offset int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 12
	}
	vars := map[string]any{
		"networkCacheVersion": 1,
		"limit":               limit,
		"offset":              offset,
		"treatmentFlags":      []string{"wishlist_should_load_service"},
	}
	return c.Query("WishlistIndexPageQuery", vars)
}

// WishlistItems returns saved-listing details for a set of listing IDs.
func (c *Client) WishlistItems(listingIDs []string, listingType string) (json.RawMessage, error) {
	if listingType == "" {
		listingType = "HOME"
	}
	ids := make([]string, 0, len(listingIDs))
	for _, id := range listingIDs {
		ids = append(ids, NumericID(id))
	}
	vars := map[string]any{
		"listingIds":          ids,
		"listingType":         listingType,
		"networkCacheVersion": 1,
	}
	return c.Query("WishlistItemsAsyncQuery", vars)
}
