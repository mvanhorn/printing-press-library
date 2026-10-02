// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/store"
)

func TestVectorSyncDryRunRendersWithoutConfiguredHost(t *testing.T) {
	t.Setenv("PINECONE_API_KEY", "test-key")
	t.Setenv("PINECONE_INDEX_HOST", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var out bytes.Buffer
	cmd := RootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"sync", "--resources", "vectors", "--vector-index", "index-a", "--dry-run", "--db", filepath.Join(t.TempDir(), "sync.db"), "--config", filepath.Join(t.TempDir(), "missing.toml")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vector dry run failed without PINECONE_INDEX_HOST: %v", err)
	}
	if !strings.Contains(out.String(), `"event":"sync_dryrun"`) {
		t.Fatalf("dry run did not complete: %q", out.String())
	}
}

type vectorHydrationClient struct {
	path   string
	params map[string]string
	data   json.RawMessage
}

func (c *vectorHydrationClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	c.path = path
	c.params = params
	return c.data, nil
}

func (c *vectorHydrationClient) RateLimit() float64 { return 0 }

func TestHydrateScopedPineconeVectorsPersistsVerifiedScope(t *testing.T) {
	c := &vectorHydrationClient{data: json.RawMessage(`{
		"vectors": {
			"shared-id": {"id":"shared-id","metadata":{"timestamp":"2020-01-01T00:00:00Z"}}
		}
	}`)}
	items := []json.RawMessage{json.RawMessage(`{"id":"shared-id"}`)}

	got, err := hydrateScopedPineconeVectors(context.Background(), c, items, "target-index", "target-ns")
	if err != nil {
		t.Fatalf("hydrate vectors: %v", err)
	}
	if !strings.Contains(c.path, "/vectors/fetch?") || !strings.Contains(c.path, "ids=shared-id") {
		t.Fatalf("fetch path = %q, want exploded vector id", c.path)
	}
	if c.params["namespace"] != "target-ns" {
		t.Fatalf("namespace param = %q, want target-ns", c.params["namespace"])
	}
	var obj map[string]any
	if err := json.Unmarshal(got[0], &obj); err != nil {
		t.Fatalf("decode hydrated vector: %v", err)
	}
	if obj["index_name"] != "target-index" || obj["namespace"] != "target-ns" {
		t.Fatalf("hydrated scope = %#v", obj)
	}
}

func TestHydrateScopedPineconeVectorsFailsOnPartialFetch(t *testing.T) {
	c := &vectorHydrationClient{data: json.RawMessage(`{"vectors":{}}`)}
	_, err := hydrateScopedPineconeVectors(context.Background(), c, []json.RawMessage{json.RawMessage(`{"id":"missing"}`)}, "target-index", "")
	if err == nil || !strings.Contains(err.Error(), "omitted id") {
		t.Fatalf("error = %v, want omitted-id failure", err)
	}
}

func TestHydrateScopedPineconeVectorsRejectsNullFetch(t *testing.T) {
	c := &vectorHydrationClient{data: json.RawMessage(`{"vectors":{"missing":null}}`)}
	_, err := hydrateScopedPineconeVectors(context.Background(), c, []json.RawMessage{json.RawMessage(`{"id":"missing"}`)}, "target-index", "")
	if err == nil || !strings.Contains(err.Error(), "is null") {
		t.Fatalf("null fetch error = %v, want failure", err)
	}
}

func TestPineconeVectorSyncUsesDataPlaneCursorPagination(t *testing.T) {
	path, err := syncResourcePath("vectors")
	if err != nil {
		t.Fatalf("vector sync path: %v", err)
	}
	if path != "https://{index_host}/vectors/list" {
		t.Fatalf("vector sync path = %q", path)
	}
	page := determinePaginationDefaults("vectors")
	if page.cursorParam != "paginationToken" || page.cursorType != "page_token" || page.nextCursorPath != "pagination.next" {
		t.Fatalf("vector pagination = %#v", page)
	}

	data := json.RawMessage(`{"vectors":[{"id":"a"}],"pagination":{"next":"next-page"}}`)
	items, next, more := extractPageItemsWithPagination(data, page.cursorParam, page.nextCursorPath, responsePathForResource("vectors", path)...)
	if len(items) != 1 || next != "next-page" || !more {
		t.Fatalf("items=%d next=%q more=%v", len(items), next, more)
	}
}

func TestSyncResourcePersistsPruneEligibleVector(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	calls := 0
	client := &sequencedVectorClient{responses: []json.RawMessage{
		json.RawMessage(`{"vectors":[{"id":"stale"}],"pagination":{}}`),
		json.RawMessage(`{"vectors":{"stale":{"metadata":{"timestamp":"2020-01-01T00:00:00Z"}}}}`),
	}, calls: &calls}
	params := &syncUserParams{
		flatGlobal:      map[string]string{},
		trueGlobal:      map[string]string{},
		perResource:     map[string]map[string]string{"vectors": {"namespace": "target-ns"}},
		vectorIndex:     "target-index",
		vectorNamespace: "target-ns",
	}
	result := syncResource(context.Background(), client, db, "vectors", "", true, 0, false, false, params, io.Discard)
	if result.Err != nil {
		t.Fatalf("sync vectors: %v", result.Err)
	}
	if result.Count != 1 || calls != 2 {
		t.Fatalf("count=%d calls=%d, want 1/2", result.Count, calls)
	}
	got, err := loadScopedPruneVectors(context.Background(), db.DB(), "target-index", "target-ns")
	if err != nil {
		t.Fatalf("load prune vectors: %v", err)
	}
	if len(got) != 1 || got[0].ID != "stale" {
		t.Fatalf("prune vectors = %#v", got)
	}
}

type sequencedVectorClient struct {
	responses []json.RawMessage
	calls     *int
	requests  []map[string]string
}

func (c *sequencedVectorClient) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	copyParams := make(map[string]string, len(params))
	for k, v := range params {
		copyParams[k] = v
	}
	c.requests = append(c.requests, copyParams)
	i := *c.calls
	*c.calls = i + 1
	if i >= len(c.responses) {
		return nil, io.EOF
	}
	return c.responses[i], nil
}

func (c *sequencedVectorClient) RateLimit() float64 { return 0 }

func TestVectorSyncCursorIsScopedByIndexAndNamespace(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := scopedSyncStateResource("vectors", &syncUserParams{vectorIndex: "index-a", vectorNamespace: "production"})
	b := scopedSyncStateResource("vectors", &syncUserParams{vectorIndex: "index-a", vectorNamespace: "staging"})
	c := scopedSyncStateResource("vectors", &syncUserParams{vectorIndex: "index-b", vectorNamespace: "production"})
	if a == b || a == c || b == c || a == "vectors" {
		t.Fatalf("vector scope keys collided: %q, %q, %q", a, b, c)
	}
	if err := db.SaveSyncProgress(a, "next-page", 100); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{b, c, "vectors"} {
		cursor, _, _, err := db.GetSyncState(key)
		if err != nil || cursor != "" {
			t.Fatalf("cursor for unrelated scope %q = %q, %v", key, cursor, err)
		}
	}
}

func TestVectorSyncPageCheckpointUsesScopedKey(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	calls := 0
	c := &sequencedVectorClient{responses: []json.RawMessage{
		json.RawMessage(`{"vectors":[{"id":"first"}],"pagination":{"next":"page-two"}}`),
		json.RawMessage(`{"vectors":{"first":{"metadata":{"timestamp":"2020-01-01T00:00:00Z"}}}}`),
	}, calls: &calls}
	params := &syncUserParams{perResource: map[string]map[string]string{"vectors": {"namespace": "production"}}, vectorIndex: "index-a", vectorNamespace: "production"}
	result := syncResource(context.Background(), c, db, "vectors", "", true, 1, false, false, params, io.Discard)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	key := scopedSyncStateResource("vectors", params)
	got, _, _, err := db.GetSyncState(key)
	if err != nil || got != "page-two" {
		t.Fatalf("scoped checkpoint = %q, %v; want page-two", got, err)
	}
	legacy, _, _, err := db.GetSyncState("vectors")
	if err != nil || legacy != "" {
		t.Fatalf("unscoped checkpoint = %q, %v; want empty", legacy, err)
	}
	resumeCalls := 0
	resume := &sequencedVectorClient{responses: []json.RawMessage{json.RawMessage(`{"vectors":[],"pagination":{}}`)}, calls: &resumeCalls}
	if result := syncResource(context.Background(), resume, db, "vectors", "", false, 1, false, false, params, io.Discard); result.Err != nil {
		t.Fatal(result.Err)
	}
	if got := resume.requests[0]["paginationToken"]; got != "page-two" {
		t.Fatalf("resume token = %q, want page-two", got)
	}
	otherScope := &syncUserParams{perResource: map[string]map[string]string{"vectors": {"namespace": "staging"}}, vectorIndex: "index-a", vectorNamespace: "staging"}
	otherCalls := 0
	other := &sequencedVectorClient{responses: []json.RawMessage{json.RawMessage(`{"vectors":[],"pagination":{}}`)}, calls: &otherCalls}
	if result := syncResource(context.Background(), other, db, "vectors", "", false, 1, false, false, otherScope, io.Discard); result.Err != nil {
		t.Fatal(result.Err)
	}
	if got := other.requests[0]["paginationToken"]; got != "" {
		t.Fatalf("different namespace reused token %q", got)
	}
}
