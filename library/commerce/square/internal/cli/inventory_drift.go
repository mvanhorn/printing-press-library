// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/square/internal/store"
	"github.com/spf13/cobra"
)

type resourceHistoryRecord struct {
	Sequence     int64
	ResourceType string
	ResourceID   string
	Data         map[string]any
	ObservedAt   time.Time
}

// loadResourceHistory drains and closes its rows before returning. Keeping this
// invariant matters on the store's deliberately small SQLite connection pool.
func loadResourceHistory(ctx context.Context, db *store.Store, resourceTypes []string, cutoff time.Time) ([]resourceHistoryRecord, error) {
	if len(resourceTypes) == 0 {
		return nil, nil
	}
	marks := make([]string, len(resourceTypes))
	args := make([]any, len(resourceTypes))
	for i, resourceType := range resourceTypes {
		marks[i], args[i] = "?", resourceType
	}
	cutoffValue := cutoff.UTC().Format(time.RFC3339Nano)
	queryArgs := append(append([]any{}, args...), cutoffValue)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, cutoffValue)
	rows, err := db.DB().QueryContext(ctx, `WITH before_window AS (
		SELECT sequence, resource_type, resource_id, data, observed_at,
		ROW_NUMBER() OVER (PARTITION BY resource_type, resource_id ORDER BY julianday(observed_at) DESC, sequence DESC) AS rank
		FROM resource_history WHERE resource_type IN (`+strings.Join(marks, ",")+`) AND julianday(observed_at) < julianday(?)
	)
	SELECT sequence, resource_type, resource_id, data, observed_at FROM before_window WHERE rank = 1
	UNION ALL
	SELECT sequence, resource_type, resource_id, data, observed_at FROM resource_history
	WHERE resource_type IN (`+strings.Join(marks, ",")+`) AND julianday(observed_at) >= julianday(?)
	ORDER BY resource_type, resource_id, observed_at, sequence`, queryArgs...)
	if err != nil {
		// A CLI upgrade can inspect a v9 database before the next write-side
		// sync has had a chance to run migrations. Fall back to its current
		// snapshots so the report stays usable and truthfully says no baseline.
		if strings.Contains(err.Error(), "no such table: resource_history") {
			return loadCurrentResourcesAsHistory(ctx, db, marks, args)
		}
		return nil, err
	}
	var history []resourceHistoryRecord
	for rows.Next() {
		var record resourceHistoryRecord
		var raw []byte
		if err := rows.Scan(&record.Sequence, &record.ResourceType, &record.ResourceID, &raw, &record.ObservedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &record.Data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("decoding history %s/%s: %w", record.ResourceType, record.ResourceID, err)
		}
		history = append(history, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return history, nil
}

func loadCurrentResourcesAsHistory(ctx context.Context, db *store.Store, marks []string, args []any) ([]resourceHistoryRecord, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT resource_type, id, data, synced_at
		FROM resources WHERE resource_type IN (`+strings.Join(marks, ",")+`)
		ORDER BY resource_type, id`, args...)
	if err != nil {
		return nil, err
	}
	var history []resourceHistoryRecord
	for rows.Next() {
		var record resourceHistoryRecord
		var raw []byte
		if err := rows.Scan(&record.ResourceType, &record.ResourceID, &raw, &record.ObservedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &record.Data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("decoding current %s/%s: %w", record.ResourceType, record.ResourceID, err)
		}
		history = append(history, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return history, nil
}

type fieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}

var nonBusinessHistoryFields = map[string]bool{
	"created_at": true, "updated_at": true, "synced_at": true, "version": true,
}

func flattenMeaningfulFields(prefix string, value any, out map[string]any) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if nonBusinessHistoryFields[strings.ToLower(key)] {
				continue
			}
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			flattenMeaningfulFields(path, typed[key], out)
		}
	case []any:
		for i, child := range typed {
			flattenMeaningfulFields(prefix+"["+strconv.Itoa(i)+"]", child, out)
		}
	default:
		out[prefix] = typed
	}
}

func meaningfulFieldChanges(before, after map[string]any) []fieldChange {
	left, right := map[string]any{}, map[string]any{}
	flattenMeaningfulFields("", before, left)
	flattenMeaningfulFields("", after, right)
	keys := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		keys[key] = struct{}{}
	}
	for key := range right {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	changes := make([]fieldChange, 0)
	for _, key := range ordered {
		if !reflect.DeepEqual(left[key], right[key]) {
			changes = append(changes, fieldChange{Field: key, Before: left[key], After: right[key]})
		}
	}
	return changes
}

func isRealInventoryHistory(record resourceHistoryRecord) bool {
	if record.Data["_deleted"] == true {
		return true
	}
	switch record.ResourceType {
	case "catalog":
		return stringValue(record.Data, "id") != "" && stringValue(record.Data, "type") != ""
	case "inventory", "changes":
		return firstString(record.Data,
			[]string{"catalog_object_id"}, []string{"catalog_object", "id"},
			[]string{"inventory_count", "catalog_object_id"}, []string{"physical_count", "catalog_object_id"}) != ""
	default:
		return false
	}
}

func newNovelInventoryDriftCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "drift",
		Short:       "See which prices, variations, availability settings, and location counts changed since an earlier snapshot.",
		Example:     "  square-pp-cli inventory drift --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "inventory drift")
			}
			cutoff, err := parseSinceDuration(flagSince)
			if err != nil {
				return fmt.Errorf("invalid value %q for --since: %s", flagSince, err)
			}
			resources := []string{"catalog", "inventory", "changes"}
			db, err := openNovelLocalStore(cmd, flags, resources)
			if err != nil {
				return err
			}
			defer db.Close()
			history, err := loadResourceHistory(cmd.Context(), db, resources, cutoff)
			if err != nil {
				return fmt.Errorf("loading inventory history: %w", err)
			}

			groups := map[string][]resourceHistoryRecord{}
			changeEvents := make([]map[string]any, 0)
			for _, record := range history {
				if !isRealInventoryHistory(record) {
					continue
				}
				if record.ResourceType == "changes" {
					if !record.ObservedAt.Before(cutoff) {
						changeEvents = append(changeEvents, map[string]any{"id": record.ResourceID, "observed_at": record.ObservedAt, "data": record.Data})
					}
					continue
				}
				key := record.ResourceType + "\x00" + record.ResourceID
				groups[key] = append(groups[key], record)
			}

			type drift struct {
				Resource   string        `json:"resource"`
				ID         string        `json:"id"`
				BaselineAt time.Time     `json:"baseline_at"`
				LatestAt   time.Time     `json:"latest_at"`
				Changes    []fieldChange `json:"changes"`
			}
			drifts := make([]drift, 0)
			uncompared := 0
			comparisons := 0
			for _, versions := range groups {
				sort.SliceStable(versions, func(i, j int) bool {
					if versions[i].ObservedAt.Equal(versions[j].ObservedAt) {
						return versions[i].Sequence < versions[j].Sequence
					}
					return versions[i].ObservedAt.Before(versions[j].ObservedAt)
				})
				latest := versions[len(versions)-1]
				if latest.ObservedAt.Before(cutoff) {
					continue
				}
				baseline := -1
				for i := range versions {
					if versions[i].ObservedAt.Before(cutoff) {
						baseline = i // state immediately before the requested window
					}
				}
				if baseline < 0 && len(versions) > 1 {
					baseline = 0 // earliest observation inside the window
				}
				if baseline < 0 || baseline == len(versions)-1 {
					uncompared++
					continue
				}
				comparisons++
				changes := meaningfulFieldChanges(versions[baseline].Data, latest.Data)
				if len(changes) == 0 {
					continue
				}
				drifts = append(drifts, drift{Resource: latest.ResourceType, ID: latest.ResourceID, BaselineAt: versions[baseline].ObservedAt, LatestAt: latest.ObservedAt, Changes: changes})
			}
			sort.Slice(drifts, func(i, j int) bool {
				if drifts[i].Resource == drifts[j].Resource {
					return drifts[i].ID < drifts[j].ID
				}
				return drifts[i].Resource < drifts[j].Resource
			})
			limitations := []string{}
			if comparisons == 0 {
				limitations = append(limitations, "No resource has both a baseline and a later retained version yet. Run sync on separate occasions to build comparison history.")
			}
			if uncompared > 0 {
				limitations = append(limitations, fmt.Sprintf("%d recently observed resources have only one retained version and cannot be compared yet.", uncompared))
			}
			return flags.printJSON(cmd, map[string]any{
				"data_source": "local", "since": flagSince, "cutoff": cutoff,
				"baseline_available": comparisons > 0, "compared_resources": comparisons,
				"uncompared_resources": uncompared, "drifted_resources": drifts,
				"change_events": changeEvents, "limitations": limitations,
			})
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Compare retained versions observed during this period (for example 7d or 24h)")
	return cmd
}
