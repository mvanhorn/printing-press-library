// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(import-review-safety): hydrate and scope vector sync rows before they can drive prune.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Vector continuation tokens belong to one index and namespace. A token from
// another scope must never be reused when the operator switches targets.
func scopedSyncStateResource(resource string, params *syncUserParams) string {
	if resource != "vectors" || params == nil || params.vectorIndex == "" {
		return resource
	}
	return resource + ":" + strconv.Quote(params.vectorIndex) + ":" + strconv.Quote(params.vectorNamespace)
}

// hydrateScopedPineconeVectors turns the ID-only /vectors/list response into
// full vector records and stamps the verified index/namespace provenance into
// every payload. Prune reads only these explicitly scoped rows. Missing fetch
// results fail the page rather than leaving an apparently complete mirror.
func hydrateScopedPineconeVectors(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, items []json.RawMessage, indexName, namespace string) ([]json.RawMessage, error) {
	if strings.TrimSpace(indexName) == "" {
		return nil, fmt.Errorf("hydrating vectors requires an index name")
	}
	if len(items) == 0 {
		return items, nil
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		var listed struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(item, &listed); err != nil || strings.TrimSpace(listed.ID) == "" {
			return nil, fmt.Errorf("vectors list returned an item without an id")
		}
		ids = append(ids, listed.ID)
	}
	encodedIDs, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("encoding vector ids: %w", err)
	}
	path := appendArrayQueryParam("https://{index_host}/vectors/fetch", "ids", string(encodedIDs), "form", true)
	data, err := c.Get(ctx, path, map[string]string{"namespace": namespace})
	if err != nil {
		return nil, fmt.Errorf("hydrating vectors for index %q namespace %q: %w", indexName, namespace, err)
	}

	var response struct {
		Vectors map[string]json.RawMessage `json:"vectors"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("parsing vector hydration response: %w", err)
	}
	if response.Vectors == nil {
		return nil, fmt.Errorf("vector hydration response did not contain a vectors object")
	}

	hydrated := make([]json.RawMessage, 0, len(ids))
	for _, id := range ids {
		raw, ok := response.Vectors[id]
		if !ok {
			return nil, fmt.Errorf("vector hydration response omitted id %q", id)
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("parsing hydrated vector %q: %w", id, err)
		}
		if obj == nil {
			return nil, fmt.Errorf("hydrated vector %q is null", id)
		}
		obj["id"] = id
		obj["index_name"] = indexName
		obj["namespace"] = namespace
		encoded, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("encoding hydrated vector %q: %w", id, err)
		}
		hydrated = append(hydrated, encoded)
	}
	return hydrated, nil
}
