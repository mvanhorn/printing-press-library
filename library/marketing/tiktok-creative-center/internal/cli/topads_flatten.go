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
	if v, ok := obj["id"]; ok && v != nil {
		// Already has a top-level id; leave untouched.
		return item
	}
	itemInfo, ok := obj["itemInfo"].(map[string]any)
	if !ok {
		return item
	}
	// Exact key (capital "ID" acronym): LookupFieldValue's direct obj[key]
	// lookup finds it; the snake->camel normalization would yield "itemId"
	// and miss.
	id := store.LookupFieldValue(itemInfo, "itemID")
	if id == nil {
		return item
	}
	// Build a new object (immutability: never mutate the decoded map in place
	// if it were shared — DecodeJSONObject returns a fresh map each call, but
	// copy to be safe and explicit).
	flat := make(map[string]any, len(obj)+1)
	for k, v := range obj {
		flat[k] = v
	}
	flat["id"] = id
	data, err := json.Marshal(flat)
	if err != nil {
		return item
	}
	return data
}
