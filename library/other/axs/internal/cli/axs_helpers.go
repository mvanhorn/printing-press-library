// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature helpers for the SRAM AXS CLI. Not generated.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/axs/internal/store"
)

var errNoLocalMirror = errors.New("no local mirror")

// fetchList GETs a path that returns either a bare JSON array or a DRF-paginated
// envelope ({"results":[...]}) and returns the rows as generic maps. AXS hosts
// mix both shapes, so decode defensively.
func fetchList(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, path string, params map[string]string) ([]map[string]any, error) {
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	return decodeList(data), nil
}

// decodeList unwraps a bare array or a {"results":[...]} / {"data":[...]} envelope.
func decodeList(data json.RawMessage) []map[string]any {
	var arr []map[string]any
	if json.Unmarshal(data, &arr) == nil {
		return arr
	}
	var env struct {
		Results []map[string]any `json:"results"`
		Data    []map[string]any `json:"data"`
	}
	if json.Unmarshal(data, &env) == nil {
		if len(env.Results) > 0 {
			return env.Results
		}
		if len(env.Data) > 0 {
			return env.Data
		}
	}
	return nil
}

// gstr returns the first non-empty string value among the candidate keys.
// Values are coerced from numbers/bools so a stringified id still resolves.
func gstr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if t != "" {
				return t
			}
		case float64:
			return trimFloat(t)
		case bool:
			if t {
				return "true"
			}
			return "false"
		}
	}
	return ""
}

func trimFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

func trimFloatPtr(f *float64) string {
	if f == nil {
		return ""
	}
	return trimFloat(*f)
}

func gnum(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case float64:
			if !math.IsNaN(t) {
				return t, true
			}
		case int:
			return float64(t), true
		case int64:
			return float64(t), true
		case json.Number:
			if f, err := t.Float64(); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

type summaryResource struct {
	ID   string
	Item map[string]any
	Data map[string]any
}

func loadLocalSummaryResources(ctx context.Context, dbPath string) ([]summaryResource, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("axs-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return nil, fmt.Errorf("%w at %s\nrun: axs-pp-cli sync --resources summaries --db %s", errNoLocalMirror, dbPath, dbPath)
	}
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	rowsSQL, err := db.DB().QueryContext(ctx, `SELECT id, data FROM resources WHERE resource_type = 'summaries'`)
	if err != nil {
		return nil, fmt.Errorf("query summaries: %w", err)
	}
	defer rowsSQL.Close()
	rows := []summaryResource{}
	for rowsSQL.Next() {
		var id string
		var raw sql.NullString
		if err := rowsSQL.Scan(&id, &raw); err != nil {
			continue
		}
		var item map[string]any
		if raw.String == "" || json.Unmarshal([]byte(raw.String), &item) != nil {
			continue
		}
		rows = append(rows, summaryResource{ID: id, Item: item, Data: nestedMap(item, "data")})
	}
	if err := rowsSQL.Err(); err != nil {
		return nil, fmt.Errorf("read summaries: %w", err)
	}
	return rows, nil
}

func deviceTypeLabel(deviceType string) string {
	switch deviceType {
	case "11":
		return "power meter"
	case "34":
		return "drivetrain"
	default:
		return deviceType
	}
}

func nestedMap(m map[string]any, key string) map[string]any {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if nested, ok := v.(map[string]any); ok {
		return nested
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func fetchAXSBLEServices(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, componentID string) ([]map[string]any, error) {
	if componentID == "" {
		return nil, nil
	}
	return fetchList(ctx, c, "https://api.axs.sram.com/ble-service/api/v1/bleservices/3_1/"+componentID+"/", nil)
}

func findAXSService(services []map[string]any, names ...string) map[string]any {
	for _, service := range services {
		serviceName := gstr(service, "service_name", "name")
		for _, name := range names {
			if serviceName == name {
				return service
			}
		}
	}
	return nil
}
