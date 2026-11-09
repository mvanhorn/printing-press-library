// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Klaviyo applies a 1/s burst and 15/m steady limit when profile_count is
// requested as an additional field. A 4.2 second interval stays below the
// steady limit with a small scheduling cushion. The HTTP client separately
// honors Retry-After when the API still returns 429.
const profileCountRequestSpacing = 4200 * time.Millisecond

var waitForProfileCountRequest = func(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func enrichProfileCounts(ctx context.Context, c flowClient, items []map[string]any, resource, resourceType string) error {
	for i, item := range items {
		id := fmt.Sprint(item["id"])
		if id == "" || id == "<nil>" {
			return fmt.Errorf("%s response item did not include an id", resourceType)
		}
		if i > 0 {
			if err := waitForProfileCountRequest(ctx, profileCountRequestSpacing); err != nil {
				return err
			}
		}
		resp, err := c.Get("/api/"+resource+"/"+url.PathEscape(id)+"/", map[string]string{
			"additional-fields[" + resourceType + "]": "profile_count",
		})
		if err != nil {
			return classifyAPIError(err)
		}
		profileCount := firstJSONValue(resp, "data.attributes.profile_count")
		if profileCount == nil {
			return fmt.Errorf("%s %s response did not include profile_count", resourceType, id)
		}
		attrs, _ := item["attributes"].(map[string]any)
		if attrs == nil {
			attrs = map[string]any{}
			item["attributes"] = attrs
		}
		attrs["profile_count"] = anyInt(profileCount)
	}
	return nil
}

func enrichSegmentProfileCounts(ctx context.Context, c flowClient, data json.RawMessage) (json.RawMessage, error) {
	var segments []map[string]any
	var envelope map[string]json.RawMessage
	isEnvelope := false
	if err := json.Unmarshal(data, &segments); err != nil {
		if envelopeErr := json.Unmarshal(data, &envelope); envelopeErr != nil {
			return nil, fmt.Errorf("parsing segment list for profile-count enrichment: %w", err)
		}
		items, ok := envelope["data"]
		if !ok {
			return nil, fmt.Errorf("segment list response did not include data")
		}
		if err := json.Unmarshal(items, &segments); err != nil {
			return nil, fmt.Errorf("parsing segment list data for profile-count enrichment: %w", err)
		}
		isEnvelope = true
	}
	if err := enrichProfileCounts(ctx, c, segments, "segments", "segment"); err != nil {
		return nil, err
	}
	if isEnvelope {
		enrichedItems, err := json.Marshal(segments)
		if err != nil {
			return nil, fmt.Errorf("encoding enriched segment list data: %w", err)
		}
		envelope["data"] = enrichedItems
		enriched, err := json.Marshal(envelope)
		if err != nil {
			return nil, fmt.Errorf("encoding enriched segment list response: %w", err)
		}
		return json.RawMessage(enriched), nil
	}
	enriched, err := json.Marshal(segments)
	if err != nil {
		return nil, fmt.Errorf("encoding enriched segment list: %w", err)
	}
	return json.RawMessage(enriched), nil
}

// compactSegmentProfileCounts preserves the documented compact list shape while
// carrying the one field --with-counts adds. Klaviyo's one-page response is a
// JSON:API envelope, whereas --all and local reads are bare arrays; agent output
// must be a results array for both paths.
func compactSegmentProfileCounts(data json.RawMessage) json.RawMessage {
	var segments []map[string]any
	if err := json.Unmarshal(data, &segments); err != nil {
		var envelope map[string]json.RawMessage
		if json.Unmarshal(data, &envelope) != nil || json.Unmarshal(envelope["data"], &segments) != nil {
			return compactFields(data)
		}
	}

	compactRaw := compactListFields(segments)
	var compact []map[string]any
	if err := json.Unmarshal(compactRaw, &compact); err != nil {
		return compactRaw
	}
	for i, segment := range segments {
		attrs, _ := segment["attributes"].(map[string]any)
		if count, ok := attrs["profile_count"]; ok {
			compact[i]["profile_count"] = count
		}
	}
	result, err := json.Marshal(compact)
	if err != nil {
		return compactRaw
	}
	return json.RawMessage(result)
}
