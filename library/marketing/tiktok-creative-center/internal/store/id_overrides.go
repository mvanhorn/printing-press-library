// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored: resource ID field overrides for the hand-authored TikTok
// Creative Center spec.
//
// The spec's types use camelCase primary keys (hashtagID) and, for top-ads, a
// nested primary key (itemInfo.itemID) that the generated generic ID-extraction
// fallbacks do not recognize. Without these overrides, sync's UpsertBatch
// dropped every row with the warning "no extractable ID field", leaving the
// local store empty and every transcendence command dead.
//
// store.LookupFieldValue normalizes snake_case -> camelCase, so the override
// values are the snake_case forms.
//
// top-ads keys on a NESTED field (itemInfo.itemID); LookupFieldValue is
// top-level only, so sync.go (internal/cli/topads_flatten.go) hoists
// itemInfo.itemID onto a top-level "id" before UpsertBatch, and the "id"
// override here resolves that flattened value.

package store

func init() {
	// Exact camelCase keys: LookupFieldValue does a direct obj[key] lookup
	// first, so the exact key wins. Note these use capital "ID" (an acronym),
	// NOT the snake_case->camelCase normalization (which would yield
	// "hashtagId"/"itemId" and miss). top-ads keys on nested itemInfo.itemID;
	// sync.go flattens that to a top-level "id" (see topads_flatten.go).
	resourceIDFieldOverrides["hashtags"] = "hashtagID"
	resourceIDFieldOverrides["top-ads"] = "id"
}
