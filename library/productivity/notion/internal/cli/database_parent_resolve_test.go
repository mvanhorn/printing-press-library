// Copyright 2026 Nikica Jokic and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// fakeGetter implements databaseParentGetter for resolveDatabaseParent tests.
type fakeGetter struct {
	resp   json.RawMessage
	err    error
	called bool
	path   string
}

func (f *fakeGetter) Get(path string, _ map[string]string) (json.RawMessage, error) {
	f.called = true
	f.path = path
	return f.resp, f.err
}

func TestResolveDatabaseParent_SingleDataSource(t *testing.T) {
	g := &fakeGetter{resp: json.RawMessage(`{"data_sources":[{"id":"ds-1","name":"Tasks"}]}`)}
	parent := map[string]any{"database_id": "db-123"}
	got, err := resolveDatabaseParent(g, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{"type": "data_source_id", "data_source_id": "ds-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if !g.called || g.path != "/v1/databases/db-123" {
		t.Fatalf("expected GET /v1/databases/db-123, got called=%v path=%q", g.called, g.path)
	}
}

func TestResolveDatabaseParent_MultiDataSourceErrors(t *testing.T) {
	g := &fakeGetter{resp: json.RawMessage(`{"data_sources":[{"id":"ds-1","name":"Tasks"},{"id":"ds-2","name":"Notes"}]}`)}
	_, err := resolveDatabaseParent(g, map[string]any{"database_id": "db-123"})
	if err == nil {
		t.Fatal("expected an error for multi-data-source database")
	}
	if !strings.Contains(err.Error(), "ds-1") || !strings.Contains(err.Error(), "ds-2") {
		t.Fatalf("error should list both data source ids, got: %v", err)
	}
	if !strings.Contains(err.Error(), "data_source_id:<id>") {
		t.Fatalf("error should tell the user how to fix it, got: %v", err)
	}
}

func TestResolveDatabaseParent_ZeroDataSourcesErrors(t *testing.T) {
	g := &fakeGetter{resp: json.RawMessage(`{"data_sources":[]}`)}
	_, err := resolveDatabaseParent(g, map[string]any{"database_id": "db-123"})
	if err == nil {
		t.Fatal("expected an error when the database has no data sources")
	}
	if !strings.Contains(err.Error(), "no accessible data sources") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestResolveDatabaseParent_NonDatabaseParentUntouched(t *testing.T) {
	cases := []map[string]any{
		{"type": "data_source_id", "data_source_id": "ds-9"},
		{"type": "page_id", "page_id": "page-9"},
		{"type": "block_id", "block_id": "block-9"},
		{"type": "workspace", "workspace": true},
		{"database_id": ""}, // empty database_id must not trigger a lookup
	}
	for _, parent := range cases {
		g := &fakeGetter{}
		got, err := resolveDatabaseParent(g, parent)
		if err != nil {
			t.Fatalf("parent %#v: unexpected error: %v", parent, err)
		}
		if g.called {
			t.Fatalf("parent %#v: should not have made an API call", parent)
		}
		if !reflect.DeepEqual(got, parent) {
			t.Fatalf("parent %#v: should pass through unchanged, got %#v", parent, got)
		}
	}
}

func TestResolveDatabaseParent_NonMapParentUntouched(t *testing.T) {
	g := &fakeGetter{}
	got, err := resolveDatabaseParent(g, "raw-string-parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.called {
		t.Fatal("non-map parent should not trigger an API call")
	}
	if got != "raw-string-parent" {
		t.Fatalf("non-map parent should pass through unchanged, got %#v", got)
	}
}

func TestResolveDatabaseParent_GetErrorPropagates(t *testing.T) {
	g := &fakeGetter{err: errors.New("boom")}
	_, err := resolveDatabaseParent(g, map[string]any{"database_id": "db-123"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected the underlying Get error to propagate, got: %v", err)
	}
}

func TestResolveDatabaseParent_SkipsBlankDataSourceIDs(t *testing.T) {
	// A data_sources entry with an empty id is ignored; a single valid one remains.
	g := &fakeGetter{resp: json.RawMessage(`{"data_sources":[{"id":"","name":"ghost"},{"id":"ds-real","name":"Real"}]}`)}
	got, err := resolveDatabaseParent(g, map[string]any{"database_id": "db-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{"type": "data_source_id", "data_source_id": "ds-real"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
