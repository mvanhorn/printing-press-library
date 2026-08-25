// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/square/internal/store"
	"github.com/spf13/cobra"
)

type localSquareRecord struct {
	ID           string         `json:"id"`
	ResourceType string         `json:"resource_type"`
	Data         map[string]any `json:"data,omitempty"`
	SyncedAt     time.Time      `json:"synced_at"`
}

func openNovelLocalStore(cmd *cobra.Command, flags *rootFlags, resourceTypes []string) (*store.Store, error) {
	if err := validateDataSourceStrategy(flags, "local"); err != nil {
		return nil, err
	}
	db, err := openStoreForRead(cmd.Context(), "square-pp-cli")
	if err != nil {
		return nil, fmt.Errorf("opening local Square data: %w", err)
	}
	if db == nil {
		return nil, fmt.Errorf("no local Square data found; run 'square-pp-cli sync --resource %s' first", strings.Join(resourceTypes, ","))
	}
	for _, resourceType := range resourceTypes {
		if !hintIfUnsynced(cmd, db, resourceType) {
			hintIfStale(cmd, db, resourceType, flags.maxAge)
		}
	}
	return db, nil
}

// loadLocalSquareRecords fully drains and closes its SQL rows before returning.
// Follow-up queries are therefore safe even when the store has a small pool.
func loadLocalSquareRecords(ctx context.Context, db *store.Store, resourceTypes []string) ([]localSquareRecord, error) {
	if len(resourceTypes) == 0 {
		return nil, nil
	}
	marks := make([]string, len(resourceTypes))
	args := make([]any, len(resourceTypes))
	for i, resourceType := range resourceTypes {
		marks[i], args[i] = "?", resourceType
	}
	query := `SELECT id, resource_type, data, synced_at FROM resources WHERE resource_type IN (` + strings.Join(marks, ",") + `)`
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var records []localSquareRecord
	for rows.Next() {
		var record localSquareRecord
		var raw []byte
		if err := rows.Scan(&record.ID, &record.ResourceType, &raw, &record.SyncedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &record.Data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("decoding %s %s: %w", record.ResourceType, record.ID, err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return records, nil
}

func recordTime(record localSquareRecord) time.Time {
	for _, key := range []string{"created_at", "updated_at", "occurred_at", "start_at", "requested_at", "published_at"} {
		if value := stringValue(record.Data, key); value != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
				return parsed
			}
		}
	}
	return record.SyncedAt
}

func recordsSince(records []localSquareRecord, cutoff time.Time) []localSquareRecord {
	filtered := records[:0]
	for _, record := range records {
		if !recordTime(record).Before(cutoff) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func stringValue(data map[string]any, path ...string) string {
	var current any = data
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	switch value := current.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

func intValue(data map[string]any, path ...string) int64 {
	var current any = data
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return 0
		}
		current = object[key]
	}
	switch value := current.(type) {
	case float64:
		return int64(value)
	case json.Number:
		n, _ := value.Int64()
		return n
	default:
		return 0
	}
}

func firstString(data map[string]any, paths ...[]string) string {
	for _, path := range paths {
		if value := stringValue(data, path...); value != "" {
			return value
		}
	}
	return ""
}

func referencesCustomer(value any, customerID string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
			if (strings.Contains(normalized, "customer") || normalized == "buyer_id") && fmt.Sprint(child) == customerID {
				return true
			}
			if referencesCustomer(child, customerID) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if referencesCustomer(child, customerID) {
				return true
			}
		}
	}
	return false
}

func sortRecordsChronologically(records []localSquareRecord) {
	sort.SliceStable(records, func(i, j int) bool { return recordTime(records[i]).Before(recordTime(records[j])) })
}
