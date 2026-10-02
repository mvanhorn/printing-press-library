// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type pruneVerificationClient interface {
	GetWithHeadersNoCache(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error)
}

func pruneTimestamp(metadata map[string]any) (time.Time, bool) {
	raw, ok := metadata["timestamp"].(string)
	if !ok {
		return time.Time{}, false
	}
	if parsed, err := time.Parse("02/01/06 3:04:05 PM", raw); err == nil {
		return parsed, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	return parsed, err == nil
}

// verifyPruneCandidates re-reads every proposed deletion from the verified
// index host and namespace. No delete is sent unless all pages were checked.
func verifyPruneCandidates(ctx context.Context, c pruneVerificationClient, fetchPath, indexName, namespace string, ids []string, cutoff time.Time) ([]string, []string, error) {
	verified := make([]string, 0, len(ids))
	var missing []string
	for start := 0; start < len(ids); start += 100 {
		end := start + 100
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		encodedIDs, err := json.Marshal(batch)
		if err != nil {
			return nil, nil, fmt.Errorf("encoding prune candidate IDs: %w", err)
		}
		path := appendArrayQueryParam(fetchPath, "ids", string(encodedIDs), "form", true)
		data, err := c.GetWithHeadersNoCache(ctx, path, map[string]string{"namespace": namespace}, apiVersionHeaders())
		if err != nil {
			return nil, nil, fmt.Errorf("rechecking vectors before delete: %w", err)
		}
		var response struct {
			Vectors map[string]json.RawMessage `json:"vectors"`
		}
		if err := json.Unmarshal(data, &response); err != nil || response.Vectors == nil {
			return nil, nil, fmt.Errorf("rechecking vectors before delete: invalid fetch response")
		}
		for _, id := range batch {
			raw, ok := response.Vectors[id]
			if !ok {
				// Pinecone omits IDs that no longer exist. Every returned ID
				// still receives its own live scope and timestamp check.
				missing = append(missing, id)
				continue
			}
			if string(raw) == "null" {
				return nil, nil, fmt.Errorf("rechecking vectors before delete: invalid vector %q", id)
			}
			var vector struct {
				ID        string         `json:"id"`
				IndexName string         `json:"index_name"`
				Namespace string         `json:"namespace"`
				Metadata  map[string]any `json:"metadata"`
			}
			if err := json.Unmarshal(raw, &vector); err != nil || (vector.ID != "" && vector.ID != id) || (vector.IndexName != "" && vector.IndexName != indexName) || (vector.Namespace != "" && vector.Namespace != namespace) {
				return nil, nil, fmt.Errorf("rechecking vectors before delete: vector %q has invalid identity or scope", id)
			}
			stamp, ok := pruneTimestamp(vector.Metadata)
			if !ok {
				return nil, nil, fmt.Errorf("rechecking vectors before delete: vector %q has no valid timestamp", id)
			}
			if stamp.Before(cutoff) {
				verified = append(verified, id)
			}
		}
	}
	return verified, missing, nil
}
