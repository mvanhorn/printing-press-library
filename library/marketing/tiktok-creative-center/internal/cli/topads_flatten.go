// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored: top-ads items from the Creative Center Top Contents library
// key on a NESTED primary key — itemInfo.itemID. The store's ID extraction
// (store.ExtractResourceID / store.LookupFieldValue) is top-level only, so
// before UpsertBatch we hoist itemInfo.itemID onto a top-level "id" field.
// This makes top-ads rows cacheable and offline-queryable for the content,
// competitor, since, and decide commands. The original nested fields are
// preserved on the stored data blob; only a convenience top-level id is added.

package cli

import (
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/marketing/tiktok-creative-center/internal/store"
)

// flattenTopAdsIDs hoists itemInfo.itemID onto a top-level "id" for each
// top-ads item so store.ExtractResourceID can resolve it. Items already
// carrying a top-level id, or lacking itemInfo.itemID, pass through unchanged.
func flattenTopAdsIDs(items []json.RawMessage) []json.RawMessage {
	if len(items) == 0 {
		return items
	}
	out := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		out = append(out, flattenTopAdID(item))
	}
	return out
}

func flattenTopAdID(item json.RawMessage) json.RawMessage {
	obj, err := store.DecodeJSONObject(item)
	if err != nil {
		return item
	}
	itemInfo, ok := obj["itemInfo"].(map[string]any)
	if !ok {
		// No nested itemInfo to derive a canonical ID from; leave any
		// existing top-level id as-is rather than discarding it.
		return item
	}
	// Exact key (capital "ID" acronym): LookupFieldValue's direct obj[key]
	// lookup finds it; the snake->camel normalization would yield "itemId"
	// and miss.
	canonicalID := store.LookupFieldValue(itemInfo, "itemID")
	if canonicalID == nil {
		// No canonical ID available; leave any existing top-level id as-is.
		return item
	}
	// itemInfo.itemID is always the canonical identity for a top-ads row.
	// Always set/overwrite the top-level id from it rather than trusting a
	// pre-existing top-level id field, which could disagree with the
	// canonical value and key the store under the wrong identity (silent
	// duplicate rows or missed updates).
	if existing, ok := obj["id"]; ok && existing == canonicalID {
		return item
	}
	// Build a new object (immutability: never mutate the decoded map in place
	// if it were shared — DecodeJSONObject returns a fresh map each call, but
	// copy to be safe and explicit).
	flat := make(map[string]any, len(obj)+1)
	for k, v := range obj {
		flat[k] = v
	}
	flat["id"] = canonicalID
	data, err := json.Marshal(flat)
	if err != nil {
		return item
	}
	return data
}
