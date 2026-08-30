// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package analysis

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Snapshot is one locally persisted Etsy observation.
type Snapshot struct {
	Resource   string
	ObservedAt time.Time
	Value      map[string]any
}

// DecodeSnapshots converts store rows into typed observations.
func DecodeSnapshots(resource string, rows []json.RawMessage) ([]Snapshot, error) {
	snapshots := make([]Snapshot, 0, len(rows))
	for index, row := range rows {
		var value map[string]any
		if err := json.Unmarshal(row, &value); err != nil {
			return nil, fmt.Errorf("decoding %s row %d: %w", resource, index, err)
		}
		observedAt, _ := time.Parse(time.RFC3339Nano, String(value, "_observed_at"))
		snapshots = append(snapshots, Snapshot{
			Resource:   resource,
			ObservedAt: observedAt,
			Value:      value,
		})
	}
	return snapshots, nil
}

// String returns the first non-empty string found at the requested keys.
func String(value map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := value[key]
		if !ok || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case string:
			if typed != "" {
				return typed
			}
		case json.Number:
			return typed.String()
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		}
	}
	return ""
}

// Number returns the first numeric value found at the requested keys.
func Number(value map[string]any, keys ...string) float64 {
	for _, key := range keys {
		raw, ok := value[key]
		if !ok || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int64:
			return float64(typed)
		case json.Number:
			number, _ := typed.Float64()
			return number
		case string:
			cleaned := strings.NewReplacer("$", "", ",", "", "%", "").Replace(typed)
			number, _ := strconv.ParseFloat(strings.TrimSpace(cleaned), 64)
			return number
		}
	}
	return 0
}

// NestedMap returns a nested object when present.
func NestedMap(value map[string]any, key string) map[string]any {
	nested, _ := value[key].(map[string]any)
	return nested
}

// ListingID extracts Etsy's listing identifier from observed response shapes.
func ListingID(value map[string]any) string {
	if identifier := String(value, "listingId", "listing_id"); identifier != "" {
		return identifier
	}
	if listing := NestedMap(value, "listing"); listing != nil {
		return String(listing, "listingId", "listing_id", "id")
	}
	return ""
}

// ObservedAt returns the snapshot time, falling back to Unix timestamps in data.
func ObservedAt(snapshot Snapshot) time.Time {
	if !snapshot.ObservedAt.IsZero() {
		return snapshot.ObservedAt
	}
	for _, key := range []string{"timestamp", "start_date", "create_date"} {
		seconds := Number(snapshot.Value, key)
		if seconds <= 0 {
			continue
		}
		if seconds > 1e12 {
			seconds /= 1000
		}
		return time.Unix(int64(seconds), 0).UTC()
	}
	return time.Time{}
}

// LatestByListing drains snapshots into the newest row per listing.
func LatestByListing(snapshots []Snapshot) map[string]Snapshot {
	latest := make(map[string]Snapshot)
	for _, snapshot := range snapshots {
		identifier := ListingID(snapshot.Value)
		if identifier == "" {
			continue
		}
		current, exists := latest[identifier]
		if !exists || ObservedAt(snapshot).After(ObservedAt(current)) {
			latest[identifier] = snapshot
		}
	}
	return latest
}

// Latest returns the most recent observation.
func Latest(snapshots []Snapshot) (Snapshot, bool) {
	if len(snapshots) == 0 {
		return Snapshot{}, false
	}
	latest := snapshots[0]
	for _, snapshot := range snapshots[1:] {
		if ObservedAt(snapshot).After(ObservedAt(latest)) {
			latest = snapshot
		}
	}
	return latest, true
}

// PercentDelta computes a safe relative change.
func PercentDelta(current, baseline float64) *float64 {
	if baseline == 0 {
		return nil
	}
	delta := (current - baseline) / math.Abs(baseline)
	return &delta
}

func nestedNumber(value map[string]any, objectKey string, keys ...string) float64 {
	nested := NestedMap(value, objectKey)
	if nested == nil {
		return 0
	}
	return Number(nested, keys...)
}

func listingIDs(value any) []string {
	rawValues, ok := value.([]any)
	if !ok {
		return nil
	}
	identifiers := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		switch typed := rawValue.(type) {
		case string:
			identifiers = append(identifiers, typed)
		case float64:
			identifiers = append(identifiers, strconv.FormatInt(int64(typed), 10))
		}
	}
	return identifiers
}
