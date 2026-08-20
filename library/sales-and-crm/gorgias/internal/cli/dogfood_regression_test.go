// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/store"
)

type recordingSyncClient struct {
	pages    []json.RawMessage
	requests []map[string]string
}

func (c *recordingSyncClient) Get(_ string, params map[string]string) (json.RawMessage, error) {
	cp := make(map[string]string, len(params))
	for k, v := range params {
		cp[k] = v
	}
	c.requests = append(c.requests, cp)
	if len(c.requests) <= len(c.pages) {
		return c.pages[len(c.requests)-1], nil
	}
	return json.RawMessage(`{"data":[]}`), nil
}

func (c *recordingSyncClient) RateLimit() float64 {
	return 0
}

func TestSyncTicketsSinceUsesLocalCutoff(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &recordingSyncClient{pages: []json.RawMessage{json.RawMessage(`{
		"data": [
			{"id": 1, "updated_datetime": "2026-05-14T00:00:00Z", "subject": "fresh ticket"},
			{"id": 2, "updated_datetime": "2026-05-12T00:00:00Z", "subject": "old ticket"}
		],
		"meta": {"next_cursor": "next"}
	}`)}}

	res := syncResource(client, db, "tickets", "2026-05-13T00:00:00Z", false, 100, false, nil)
	if res.Err != nil {
		t.Fatalf("syncResource returned error: %v", res.Err)
	}
	if res.Count != 1 {
		t.Fatalf("synced count = %d, want 1", res.Count)
	}
	if len(client.requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(client.requests))
	}
	if got := client.requests[0]["order_by"]; got != "updated_datetime:desc" {
		t.Fatalf("order_by = %q, want updated_datetime:desc", got)
	}

	count, err := db.Count("tickets")
	if err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if count != 1 {
		t.Fatalf("stored ticket count = %d, want 1", count)
	}
	if _, err := db.Get("tickets", "2"); err == nil {
		t.Fatalf("old ticket was stored despite since cutoff")
	}
}

func TestFilterSyncItemsByLocalSinceKeepsOutOfOrderFreshItems(t *testing.T) {
	cutoff := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	items := []json.RawMessage{
		json.RawMessage(`{"id":1,"updated_datetime":"2026-05-14T00:00:00Z","subject":"fresh"}`),
		json.RawMessage(`{"id":2,"updated_datetime":"2026-05-12T00:00:00Z","subject":"old"}`),
		json.RawMessage(`{"id":3,"updated_datetime":"2026-05-13T12:00:00Z","subject":"fresh but out of order"}`),
	}

	result := filterSyncItemsByLocalSince(items, localSinceFilter{
		Cutoff: cutoff,
		Fields: []string{"updated_datetime"},
	}, true, time.Time{}, false)

	if !result.OrderingBroken {
		t.Fatalf("OrderingBroken = false, want true")
	}
	if result.HitCutoff {
		t.Fatalf("HitCutoff = true, want false when the page proves ordering is not newest-first")
	}
	if len(result.Items) != 2 {
		t.Fatalf("kept item count = %d, want 2", len(result.Items))
	}
	var kept []map[string]any
	for _, raw := range result.Items {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("decode kept item: %v", err)
		}
		kept = append(kept, obj)
	}
	if kept[0]["id"] != float64(1) || kept[1]["id"] != float64(3) {
		t.Fatalf("kept ids = %#v, want 1 and 3", kept)
	}
}

func TestSearchLocalDefaultQueriesResourcesFTS(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.Upsert("tickets", "1", []byte(`{"id":1,"subject":"Cancel order","status":"open","tags":[{"name":"cancel/refund"}]}`)); err != nil {
		t.Fatalf("upsert ticket: %v", err)
	}
	db.Close()

	cmd := RootCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--data-source", "local", "--json", "search", "cancel", "--db", dbPath, "--limit", "5"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v\nstderr: %s", err, stderr.String())
	}

	var envelope struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode search output %q: %v", out.String(), err)
	}
	if len(envelope.Results) != 1 {
		t.Fatalf("search result count = %d, want 1; output=%s", len(envelope.Results), out.String())
	}
	if got := envelope.Results[0]["subject"]; got != "Cancel order" {
		t.Fatalf("subject = %v, want Cancel order", got)
	}
}

func TestSearchResourceSplitsGlobalAndTypedFTS(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("tickets", "1", []byte(`{"id":1,"subject":"Cancel order"}`)); err != nil {
		t.Fatalf("upsert ticket: %v", err)
	}
	if err := db.Upsert("customers", "2", []byte(`{"id":2,"name":"Cancel Customer"}`)); err != nil {
		t.Fatalf("upsert customer: %v", err)
	}

	global, err := db.Search("cancel", 10)
	if err != nil {
		t.Fatalf("global search: %v", err)
	}
	if len(global) != 2 {
		t.Fatalf("global search count = %d, want 2", len(global))
	}
	tickets, err := db.SearchResource("tickets", "cancel", 10)
	if err != nil {
		t.Fatalf("typed search: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("typed search count = %d, want 1", len(tickets))
	}
	var ticket map[string]any
	if err := json.Unmarshal(tickets[0], &ticket); err != nil {
		t.Fatalf("decode typed search result: %v", err)
	}
	if ticket["subject"] != "Cancel order" {
		t.Fatalf("typed search subject = %v, want Cancel order", ticket["subject"])
	}
}

func TestAnalyticsGroupByUsesGenericResourcesTable(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	for i := 0; i < 225; i++ {
		id := fmt.Sprintf("closed-%03d", i)
		payload := fmt.Sprintf(`{"id":%q,"status":"closed"}`, id)
		if err := db.Upsert("tickets", id, []byte(payload)); err != nil {
			t.Fatalf("upsert ticket %s: %v", id, err)
		}
	}
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("open-%03d", i)
		payload := fmt.Sprintf(`{"id":%q,"status":"open"}`, id)
		if err := db.Upsert("tickets", id, []byte(payload)); err != nil {
			t.Fatalf("upsert ticket %s: %v", id, err)
		}
	}

	output := captureStdout(t, func() error {
		return runGroupBy(db, "tickets", "status", 10, &rootFlags{})
	})
	if !bytes.Contains([]byte(output), []byte("closed\t225")) {
		t.Fatalf("analytics output missing closed count: %q", output)
	}
	if !bytes.Contains([]byte(output), []byte("open\t25")) {
		t.Fatalf("analytics output missing open count: %q", output)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	callErr := fn()
	if closeErr := w.Close(); closeErr != nil {
		os.Stdout = old
		t.Fatalf("close stdout pipe: %v", closeErr)
	}
	os.Stdout = old
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout pipe: %v", readErr)
	}
	if callErr != nil {
		t.Fatalf("captured function failed: %v", callErr)
	}
	return string(out)
}
