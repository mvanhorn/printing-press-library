// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/square/internal/store"
)

type squareSyncFixtureClient struct {
	getCalls  int
	postCalls int
	path      string
	params    map[string]string
	body      map[string]any
	response  json.RawMessage
}

func (c *squareSyncFixtureClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	c.getCalls++
	c.path, c.params = path, params
	return c.response, nil
}

func (c *squareSyncFixtureClient) PostQueryWithParams(_ context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error) {
	c.postCalls++
	c.path, c.params = path, params
	c.body, _ = body.(map[string]any)
	return c.response, 200, nil
}

func (*squareSyncFixtureClient) RateLimit() float64 { return 0 }

func TestSquareSyncResourcePathsAndMethods(t *testing.T) {
	tests := []struct {
		resource string
		path     string
		method   string
	}{
		{"catalog", "/v2/catalog/search", "POST"},
		{"orders", "/v2/orders/search", "POST"},
		{"inventory", "/v2/inventory/counts/batch-retrieve", "POST"},
		{"team-members", "/v2/team-members/search", "POST"},
	}
	for _, tt := range tests {
		path, err := syncResourcePath(tt.resource)
		if err != nil || path != tt.path {
			t.Fatalf("syncResourcePath(%q) = %q, %v; want %q", tt.resource, path, err, tt.path)
		}
		if got := syncResourceMethod(tt.resource); got != tt.method {
			t.Fatalf("syncResourceMethod(%q) = %q, want %q", tt.resource, got, tt.method)
		}
	}
}

func TestSquareSyncPOSTBodiesMoveWireFieldsOutOfQuery(t *testing.T) {
	tests := []struct {
		resource string
		params   map[string]string
		wantBody map[string]any
	}{
		{
			resource: "catalog",
			params:   map[string]string{"cursor": "next", "limit": "100", "include_deleted_objects": "true", "types": "ITEM,ITEM_VARIATION"},
			wantBody: map[string]any{"cursor": "next", "limit": 100, "include_deleted_objects": true, "object_types": []string{"ITEM", "ITEM_VARIATION"}},
		},
		{
			resource: "orders",
			params:   map[string]string{"cursor": "next", "limit": "100", "location_ids": "LOC1,LOC2", "updated_at_begin_time": "2026-08-01T00:00:00Z"},
			wantBody: map[string]any{
				"cursor": "next", "limit": 100, "location_ids": []string{"LOC1", "LOC2"},
				"query": map[string]any{"filter": map[string]any{"date_time_filter": map[string]any{"updated_at": map[string]any{"start_at": "2026-08-01T00:00:00Z"}}}},
			},
		},
		{
			resource: "inventory",
			params:   map[string]string{"cursor": "next", "limit": "100", "catalog_object_ids": "VAR1,VAR2", "location_ids": "LOC1", "states": "IN_STOCK", "updated_after": "2026-08-01T00:00:00Z"},
			wantBody: map[string]any{"cursor": "next", "limit": 100, "catalog_object_ids": []string{"VAR1", "VAR2"}, "location_ids": []string{"LOC1"}, "states": []string{"IN_STOCK"}, "updated_after": "2026-08-01T00:00:00Z"},
		},
		{
			resource: "team-members",
			params:   map[string]string{"cursor": "next", "limit": "100", "location_ids": "LOC1", "status": "ACTIVE", "is_owner": "false"},
			wantBody: map[string]any{"cursor": "next", "limit": 100, "query": map[string]any{"filter": map[string]any{"location_ids": []string{"LOC1"}, "status": "ACTIVE", "is_owner": false}}},
		},
	}
	for _, tt := range tests {
		body, query, err := syncResourcePOSTBody(tt.resource, tt.params)
		if err != nil {
			t.Fatalf("syncResourcePOSTBody(%q): %v", tt.resource, err)
		}
		if len(query) != 0 {
			t.Fatalf("%s query params = %#v, want body-only request", tt.resource, query)
		}
		if !reflect.DeepEqual(body, tt.wantBody) {
			t.Fatalf("%s body = %#v, want %#v", tt.resource, body, tt.wantBody)
		}
	}
}

func TestFetchSquareSyncResourceUsesPOSTQuery(t *testing.T) {
	for _, tt := range []struct {
		resource string
		path     string
		response string
	}{
		{"catalog", "/v2/catalog/search", `{"objects":[]}`},
		{"orders", "/v2/orders/search", `{"orders":[]}`},
	} {
		client := &squareSyncFixtureClient{response: json.RawMessage(tt.response)}
		_, err := fetchSyncResourcePage(context.Background(), client, tt.resource, tt.path, map[string]string{"limit": "100"})
		if err != nil {
			t.Fatal(err)
		}
		if client.postCalls != 1 || client.getCalls != 0 || client.path != tt.path {
			t.Fatalf("%s calls: path=%q GET=%d POST=%d, want POST %s", tt.resource, client.path, client.getCalls, client.postCalls, tt.path)
		}
	}
}

func TestSquareSyncEnvelopeFixturesExtractEntities(t *testing.T) {
	tests := []struct {
		resource string
		path     string
		fixture  string
		wantID   string
	}{
		{"catalog", "/v2/catalog/search", `{"objects":[{"type":"ITEM","id":"ITEM1"}],"cursor":"cat-next"}`, "ITEM1"},
		{"orders", "/v2/orders/search", `{"orders":[{"id":"ORDER1","location_id":"LOC1","state":"COMPLETED"}],"cursor":"order-next"}`, "ORDER1"},
		{"inventory", "/v2/inventory/counts/batch-retrieve", `{"counts":[{"catalog_object_id":"VAR1","location_id":"LOC1","state":"IN_STOCK","quantity":"3"}],"cursor":"inventory-next"}`, "VAR1\x00LOC1\x00IN_STOCK"},
		{"team-members", "/v2/team-members/search", `{"team_members":[{"id":"TM1","given_name":"Ada","family_name":"Lovelace","status":"ACTIVE"}],"cursor":"team-next"}`, "TM1"},
	}
	for _, tt := range tests {
		items, cursor, _ := extractPageItems(json.RawMessage(tt.fixture), "cursor", responsePathForResource(tt.resource, tt.path)...)
		items = normalizeSquareSyncItems(tt.resource, items)
		if len(items) != 1 || cursor == "" {
			t.Fatalf("%s fixture extracted %d items, cursor %q", tt.resource, len(items), cursor)
		}
		var object map[string]any
		if err := json.Unmarshal(items[0], &object); err != nil {
			t.Fatal(err)
		}
		if got := extractID(tt.resource, object); got != tt.wantID {
			t.Fatalf("%s entity id = %q, want %q", tt.resource, got, tt.wantID)
		}
	}
}

func TestCatalogSyncDefaultsIncludeDeletedObjects(t *testing.T) {
	params := map[string]string{}
	applySquareSyncDefaultParams("catalog", params)
	if params["include_deleted_objects"] != "true" {
		t.Fatalf("catalog params = %#v, want include_deleted_objects=true", params)
	}
	applySquareSyncDefaultParams("orders", params)
	if len(params) != 1 {
		t.Fatalf("non-catalog defaults changed params: %#v", params)
	}
}

func TestInventoryCountsUseStableIDsThroughStoreAndFanOutOnce(t *testing.T) {
	fixture := json.RawMessage(`{"counts":[
		{"catalog_object_id":"VAR1","location_id":"LOC1","state":"IN_STOCK","quantity":"3"},
		{"catalog_object_id":"VAR1","location_id":"LOC2","state":"IN_STOCK","quantity":"4"}
	]}`)
	items, _, _ := extractPageItems(fixture, "cursor", responsePathForResource("inventory", "/v2/inventory/counts/batch-retrieve")...)
	items = normalizeSquareSyncItems("inventory", items)
	db, err := store.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stored, failures, err := db.UpsertBatch("inventory", items)
	if err != nil || stored != 2 || failures != 0 {
		t.Fatalf("UpsertBatch stored=%d failures=%d err=%v", stored, failures, err)
	}
	if _, err := db.Get("inventory", "VAR1\x00LOC1\x00IN_STOCK"); err != nil {
		t.Fatalf("stable count ID was not persisted: %v", err)
	}
	rows, err := dependentParentRows(db, "inventory", []dependentPathParamDef{{Param: "catalog_object_id", Field: "catalog_object_id"}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["catalog_object_id"] != "VAR1" {
		t.Fatalf("inventory change fan-out rows = %#v, want one VAR1 row", rows)
	}
}

func TestInventoryChangesNormalizeNestedSquareEnvelopeThroughStore(t *testing.T) {
	fixture := json.RawMessage(`{"changes":[
		{"type":"PHYSICAL_COUNT","physical_count":{"catalog_object_id":"VAR1","location_id":"LOC1","state":"IN_STOCK","quantity":"3","calculated_at":"2026-08-04T12:00:00Z"}},
		{"type":"ADJUSTMENT","adjustment":{"catalog_object_id":"VAR1","location_id":"LOC1","from_state":"IN_STOCK","to_state":"SOLD","quantity":"1","occurred_at":"2026-08-04T13:00:00Z"}}
	]}`)
	items, _, _ := extractPageItems(fixture, "cursor", responsePathForResource("changes", "/v2/inventory/VAR1/changes")...)
	items = normalizeSquareSyncItems("changes", items)
	if len(items) != 2 {
		t.Fatalf("changes extracted %d items, want 2", len(items))
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "changes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ids := map[string]bool{}
	for _, item := range items {
		var object map[string]any
		if err := json.Unmarshal(item, &object); err != nil {
			t.Fatal(err)
		}
		id := extractID("changes", object)
		if id == "" || object["catalog_object_id"] != "VAR1" {
			t.Fatalf("normalized change = %#v", object)
		}
		ids[id] = true
	}
	if len(ids) != 2 {
		t.Fatalf("derived IDs = %#v, want distinct physical-count and adjustment IDs", ids)
	}
	stored, failures, err := db.UpsertBatch("changes", items)
	if err != nil || stored != 2 || failures != 0 {
		t.Fatalf("UpsertBatch stored=%d failures=%d err=%v", stored, failures, err)
	}
}

func TestCatalogDeletedObjectIsStoredAsObservableDrift(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, fixture := range []string{
		`{"objects":[{"type":"ITEM","id":"ITEM1","is_deleted":false,"version":1}]}`,
		`{"objects":[{"type":"ITEM","id":"ITEM1","is_deleted":true,"version":2}]}`,
	} {
		items, _, _ := extractPageItems(json.RawMessage(fixture), "cursor", responsePathForResource("catalog", "/v2/catalog/search")...)
		if stored, failures, err := db.UpsertBatch("catalog", items); err != nil || stored != 1 || failures != 0 {
			t.Fatalf("catalog UpsertBatch stored=%d failures=%d err=%v", stored, failures, err)
		}
	}
	history, err := loadResourceHistory(context.Background(), db, []string{"catalog"}, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("catalog history = %#v, want live object plus tombstone", history)
	}
	changes := meaningfulFieldChanges(history[0].Data, history[1].Data)
	foundDeletion := false
	for _, change := range changes {
		if change.Field == "is_deleted" {
			foundDeletion = true
		}
	}
	if !foundDeletion {
		t.Fatalf("catalog changes = %#v, want observable is_deleted drift", changes)
	}
}

type catalogPaginationFixtureClient struct {
	responses []json.RawMessage
	paths     []string
	bodies    []map[string]any
}

func (c *catalogPaginationFixtureClient) Get(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
	return nil, fmt.Errorf("unexpected GET %s", path)
}

func (c *catalogPaginationFixtureClient) PostQueryWithParams(_ context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error) {
	if len(c.paths) >= len(c.responses) {
		return nil, 0, fmt.Errorf("unexpected extra POST %s", path)
	}
	if len(params) != 0 {
		return nil, 0, fmt.Errorf("catalog query parameters = %#v, want body-only request", params)
	}
	c.paths = append(c.paths, path)
	bodyMap, _ := body.(map[string]any)
	c.bodies = append(c.bodies, bodyMap)
	return c.responses[len(c.paths)-1], 200, nil
}

func (*catalogPaginationFixtureClient) RateLimit() float64 { return 0 }

func TestCatalogSyncPOSTPaginationCarriesCursorTypesAndDeletedFlag(t *testing.T) {
	client := &catalogPaginationFixtureClient{responses: []json.RawMessage{
		json.RawMessage(`{"objects":[{"type":"ITEM","id":"ITEM1","is_deleted":false}],"cursor":"catalog-next"}`),
		json.RawMessage(`{"objects":[{"type":"ITEM","id":"ITEM2","is_deleted":true}]}`),
	}}
	db, err := store.Open(filepath.Join(t.TempDir(), "catalog-pagination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	result := syncResource(context.Background(), client, db, "catalog", "", true, 0, false, false, &syncUserParams{
		perResource: map[string]map[string]string{"catalog": {"types": "ITEM,ITEM_VARIATION"}},
	}, io.Discard)
	if result.Err != nil || result.Warn != nil || result.Count != 2 {
		t.Fatalf("catalog sync result = %#v", result)
	}
	if len(client.bodies) != 2 || client.paths[0] != "/v2/catalog/search" || client.paths[1] != "/v2/catalog/search" {
		t.Fatalf("catalog POST calls paths=%#v bodies=%#v", client.paths, client.bodies)
	}
	for i, body := range client.bodies {
		if body["include_deleted_objects"] != true || !reflect.DeepEqual(body["object_types"], []string{"ITEM", "ITEM_VARIATION"}) {
			t.Fatalf("catalog body %d = %#v", i, body)
		}
	}
	if _, exists := client.bodies[0]["cursor"]; exists {
		t.Fatalf("first catalog body unexpectedly has cursor: %#v", client.bodies[0])
	}
	if client.bodies[1]["cursor"] != "catalog-next" {
		t.Fatalf("second catalog body = %#v, want cursor catalog-next", client.bodies[1])
	}
	if tombstone, err := db.Get("catalog", "ITEM2"); err != nil || !strings.Contains(string(tombstone), `"is_deleted":true`) {
		t.Fatalf("deleted catalog object was not retained: row=%s err=%v", tombstone, err)
	}
}
