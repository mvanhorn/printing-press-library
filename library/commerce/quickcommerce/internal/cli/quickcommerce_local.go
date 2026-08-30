// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/quickcommerce/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/quickcommerce/internal/store"
)

type qcObservationInput struct {
	Resource   string
	ItemID     string
	Platform   string
	Location   string
	Query      string
	CapturedAt time.Time
	Data       json.RawMessage
	Price      sql.NullFloat64
	MRP        sql.NullFloat64
	Inventory  sql.NullInt64
	Available  sql.NullBool
	Quantity   string
	ETA        string
	Open       sql.NullBool
	StoreID    string
}

func parseQCLocation(raw string) (float64, float64, string, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return 0, 0, "", fmt.Errorf("--location must be latitude,longitude (for example 12.9021,77.6639)")
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil || lat < -90 || lat > 90 {
		return 0, 0, "", fmt.Errorf("invalid --location latitude %q; use decimal degrees between -90 and 90", parts[0])
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil || lon < -180 || lon > 180 {
		return 0, 0, "", fmt.Errorf("invalid --location longitude %q; use decimal degrees between -180 and 180", parts[1])
	}
	return lat, lon, fmt.Sprintf("%.6f,%.6f", lat, lon), nil
}

func parseQCDuration(raw string) (time.Duration, error) {
	d, err := cliutil.ParseDurationLoose(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid duration %q; use values such as 24h, 7d, or 30d", raw)
	}
	return d, nil
}

func qcDBPath(path string) string {
	if strings.TrimSpace(path) != "" {
		return path
	}
	return defaultDBPath("quickcommerce-pp-cli")
}

func openQCLocal(path string) (*store.Store, bool, error) {
	path = qcDBPath(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("checking local mirror %s: %w", path, err)
	}
	db, err := store.OpenReadOnly(path)
	if err != nil {
		return nil, false, fmt.Errorf("opening local mirror: %w", err)
	}
	return db, true, nil
}

func openQCCreatable(ctx context.Context, path string) (*store.Store, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := store.OpenWithContext(ctx, qcDBPath(path))
	if err != nil {
		return nil, fmt.Errorf("opening local mirror: %w", err)
	}
	if err := store.EnsureQuickCommerceHistory(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing local history: %w", err)
	}
	return db, nil
}

func closeQCStore(db *store.Store) error {
	if db == nil {
		return nil
	}
	return db.Close()
}

func qcMissingMirror(cmdOut io.Writer, cmdErr io.Writer, flags *rootFlags, path, resource string) error {
	fmt.Fprintf(cmdErr, "no local mirror at %s\nrun: quickcommerce-pp-cli sync --resources %s --db %s\n", path, resource, path)
	if !wantsHumanTable(cmdOut, flags) {
		return printJSONFiltered(cmdOut, make([]map[string]any, 0), flags)
	}
	return nil
}

func qcPrint(w io.Writer, flags *rootFlags, rows any, humanRows []map[string]any) error {
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, rows, flags)
	}
	if len(humanRows) == 0 {
		_, err := fmt.Fprintln(w, "No matching local observations found.")
		return err
	}
	return printAutoTable(w, humanRows)
}

func qcString(m map[string]json.RawMessage, key string) string {
	var s string
	if raw, ok := m[key]; ok && json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}
func qcFloat(m map[string]json.RawMessage, keys ...string) sql.NullFloat64 {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil && !math.IsNaN(f) {
			return sql.NullFloat64{Float64: f, Valid: true}
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return sql.NullFloat64{Float64: f, Valid: true}
			}
		}
	}
	return sql.NullFloat64{}
}
func qcInt(m map[string]json.RawMessage, keys ...string) sql.NullInt64 {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var i int64
		if json.Unmarshal(raw, &i) == nil {
			return sql.NullInt64{Int64: i, Valid: true}
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return sql.NullInt64{Int64: int64(f), Valid: true}
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if i, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				return sql.NullInt64{Int64: i, Valid: true}
			}
		}
	}
	return sql.NullInt64{}
}
func qcBool(m map[string]json.RawMessage, keys ...string) sql.NullBool {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			return sql.NullBool{Bool: b, Valid: true}
		}
	}
	return sql.NullBool{}
}
func qcTime(raw string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "02-01-2006 03:04:05 PM", "02-01-2006 03:04:05 PM MST"} {
		if t, err := time.Parse(layout, strings.TrimSpace(raw)); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
func qcDBTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t.UTC()
	case string:
		for _, layout := range []string{"2006-01-02 15:04:05-07:00", "2006-01-02 15:04:05", time.RFC3339Nano} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed.UTC()
			}
		}
	}
	return time.Time{}
}

func qcLocationFromMap(m map[string]json.RawMessage) string {
	lat, lon := qcFloat(m, "lat", "latitude"), qcFloat(m, "lon", "longitude")
	if lat.Valid && lon.Valid {
		return fmt.Sprintf("%.6f,%.6f", lat.Float64, lon.Float64)
	}
	return ""
}

func qcID(resource, itemID, platform, location string, at time.Time, data json.RawMessage) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00", resource, itemID, platform, location, at.UTC().Format(time.RFC3339Nano))
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func qcInputToObservation(in qcObservationInput) store.QuickCommerceObservation {
	if in.CapturedAt.IsZero() {
		in.CapturedAt = time.Now().UTC()
	}
	return store.QuickCommerceObservation{ID: qcID(in.Resource, in.ItemID, in.Platform, in.Location, in.CapturedAt, in.Data), Resource: in.Resource, ItemID: in.ItemID, Platform: in.Platform, Location: in.Location, Query: in.Query, CapturedAt: in.CapturedAt, Data: in.Data, Price: in.Price, MRP: in.MRP, Inventory: in.Inventory, Available: in.Available, Quantity: in.Quantity, ETA: in.ETA, Open: in.Open, StoreID: in.StoreID}
}

func qcMap(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	return m, json.Unmarshal(raw, &m) == nil && m != nil
}

func qcExtractObservations(raw json.RawMessage) []qcObservationInput {
	root, ok := qcMap(raw)
	if !ok {
		return nil
	}
	data := root
	if nested, exists := root["data"]; exists {
		if m, ok := qcMap(nested); ok {
			data = m
		}
	} else if _, agentEnvelope := root["meta"]; agentEnvelope {
		if nested, exists := root["results"]; exists {
			if m, ok := qcMap(nested); ok {
				data = m
			} else {
				data = map[string]json.RawMessage{"results": nested}
			}
		}
	}
	location := qcLocationFromMap(data)
	query := qcString(data, "query")
	platform := qcString(data, "platform")
	at := qcTime(qcString(data, "fetched_at"))
	if at.IsZero() {
		at = time.Now().UTC()
	}
	out := make([]qcObservationInput, 0)
	if products, ok := data["products"]; ok {
		out = append(out, qcExtractArray(products, "products", platform, location, query, at, false)...)
	}
	if items, ok := data["items"]; ok {
		out = append(out, qcExtractArray(items, "items", platform, location, query, at, false)...)
	}
	if eta := qcString(data, "eta"); eta != "" && platform != "" {
		out = append(out, qcObservationInput{Resource: "delivery", Platform: platform, Location: location, CapturedAt: at, Data: raw, ETA: eta, Open: qcBool(data, "open"), StoreID: qcString(data, "store_id")})
	}
	if results, ok := data["results"]; ok {
		var list []json.RawMessage
		if json.Unmarshal(results, &list) == nil {
			out = append(out, qcResultListObservations(list, location, query, at)...)
		} else {
			var grouped map[string]json.RawMessage
			if json.Unmarshal(results, &grouped) == nil {
				if nested, exists := grouped["results"]; exists {
					var nestedList []json.RawMessage
					if json.Unmarshal(nested, &nestedList) == nil {
						out = append(out, qcResultListObservations(nestedList, location, query, at)...)
					} else {
						var nestedMap map[string]json.RawMessage
						if json.Unmarshal(nested, &nestedMap) == nil {
							for name, group := range nestedMap {
								out = append(out, qcExtractArray(group, "comparison", name, location, query, at, true)...)
							}
						}
					}
				} else {
					for name, group := range grouped {
						out = append(out, qcExtractArray(group, "comparison", name, location, query, at, true)...)
					}
				}
			}
		}
	}
	if _, hasSummary := data["summary"]; hasSummary {
		out = append(out, qcObservationInput{Resource: "credits", Location: location, CapturedAt: at, Data: raw})
	}
	if _, hasPlatforms := data["platforms"]; hasPlatforms && data["count"] != nil {
		out = append(out, qcObservationInput{Resource: "platforms", Location: location, CapturedAt: at, Data: raw})
	}
	return out
}

func qcResultListObservations(list []json.RawMessage, location, query string, at time.Time) []qcObservationInput {
	out := make([]qcObservationInput, 0, len(list))
	for _, item := range list {
		m, ok := qcMap(item)
		if !ok {
			continue
		}
		resource := "products"
		if qcString(m, "item_id") != "" {
			resource = "items"
		}
		if qcString(m, "eta") != "" {
			resource = "delivery"
		}
		out = append(out, qcObservationFromMap(resource, m, location, query, at, item))
	}
	return out
}

func qcExtractArray(raw json.RawMessage, resource, platform, location, query string, at time.Time, comparison bool) []qcObservationInput {
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	out := make([]qcObservationInput, 0, len(list))
	for _, item := range list {
		if m, ok := qcMap(item); ok {
			observation := qcObservationFromMap(resource, m, location, query, at, item)
			if observation.Platform == "" {
				observation.Platform = platform
			}
			out = append(out, observation)
		}
	}
	return out
}

func qcObservationFromMap(resource string, m map[string]json.RawMessage, location, query string, at time.Time, raw json.RawMessage) qcObservationInput {
	itemID := qcString(m, "item_id")
	if itemID == "" {
		itemID = qcString(m, "id")
	}
	platform := qcString(m, "platform")
	if nested, ok := m["platform"]; ok {
		if pm, ok := qcMap(nested); ok {
			platform = qcString(pm, "name")
		}
	}
	return qcObservationInput{Resource: resource, ItemID: itemID, Platform: platform, Location: location, Query: query, CapturedAt: at, Data: raw, Price: qcFloat(m, "price", "offer_price"), MRP: qcFloat(m, "mrp"), Inventory: qcInt(m, "inventory"), Available: qcBool(m, "available"), Quantity: qcString(m, "quantity"), ETA: qcString(m, "eta"), Open: qcBool(m, "open"), StoreID: qcString(m, "store_id")}
}

func qcSaveInput(ctx context.Context, db *store.Store, in qcObservationInput) error {
	return store.InsertQuickCommerceObservation(ctx, db, qcInputToObservation(in))
}
