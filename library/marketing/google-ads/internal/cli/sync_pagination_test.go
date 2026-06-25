// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ads/internal/store"
)

// TestNonPaginatedResourcesSuppressesParams guards the F0 sync fix: the
// listAccessibleCustomers endpoint (resource "customers") takes no pagination,
// cursor, or filter query parameters and rejects unknown params with HTTP 400
// ("Invalid JSON payload received. Unknown name \"limit\""). The sync loop must
// therefore send NO generated query params for that resource. This test pins
// the data that drives that suppression so a regen or refactor cannot silently
// re-introduce the limit= param that left the mirror unhydrated.
func TestNonPaginatedResourcesSuppressesParams(t *testing.T) {
	if !nonPaginatedResources["customers"] {
		t.Fatalf("customers must be in nonPaginatedResources: listAccessibleCustomers 400s on any query param (limit/after/since)")
	}
}

// TestDefaultSyncRootIsNonPaginated asserts that every resource sync would run
// by default is recognized as non-paginated, since "customers" is both the only
// default resource and the cascade root. If the default set grows to include a
// genuinely paginated resource, that's fine — this test only fails if a default
// resource maps to a custom (:method) endpoint that still received pagination
// params, which is the exact failure mode F0 fixed.
func TestDefaultSyncRootIsNonPaginated(t *testing.T) {
	for _, resource := range defaultSyncResources() {
		path, err := syncResourcePath(resource)
		if err != nil {
			t.Fatalf("syncResourcePath(%q) returned error: %v", resource, err)
		}
		// Google Ads custom methods use a ":verb" suffix on the collection
		// path (e.g. /v22/customers:listAccessibleCustomers) and bind query
		// params strictly. Any such default resource must be non-paginated.
		if containsColonMethod(path) && !nonPaginatedResources[resource] {
			t.Fatalf("default resource %q maps to custom-method path %q but is not marked non-paginated; sync will send pagination params the API rejects with HTTP 400", resource, path)
		}
	}
}

// containsColonMethod reports whether a path uses the REST custom-method
// shape "<collection>:<verb>" in its final segment.
func containsColonMethod(path string) bool {
	lastSlash := -1
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			lastSlash = i
		}
	}
	for i := lastSlash + 1; i < len(path); i++ {
		if path[i] == ':' {
			return true
		}
	}
	return false
}

// fakeSyncClient satisfies syncResource's client interface and records the
// query params of the last request so tests can pin the request shape.
type fakeSyncClient struct {
	response  string
	gotParams map[string]string
	calls     int
}

func (f *fakeSyncClient) Get(path string, params map[string]string) (json.RawMessage, error) {
	f.calls++
	f.gotParams = params
	return json.RawMessage(f.response), nil
}

func (f *fakeSyncClient) RateLimit() float64 { return 0 }

// TestWrapScalarResourceNameItems guards the scalar-to-object transform: bare
// resource-name strings become ID-bearing objects; pages containing anything
// other than non-empty JSON strings pass through untouched (ok=false).
func TestWrapScalarResourceNameItems(t *testing.T) {
	wrapped, ok := wrapScalarResourceNameItems([]json.RawMessage{
		json.RawMessage(`"customers/1112223334"`),
		json.RawMessage(`"customers/9998887776"`),
	})
	if !ok || len(wrapped) != 2 {
		t.Fatalf("wrap of 2 scalar items: got ok=%v len=%d, want ok=true len=2", ok, len(wrapped))
	}
	var obj map[string]string
	if err := json.Unmarshal(wrapped[0], &obj); err != nil {
		t.Fatalf("unmarshaling wrapped item: %v", err)
	}
	if obj["resourceName"] != "customers/1112223334" || obj["id"] != "1112223334" || obj["customerId"] != "1112223334" {
		t.Fatalf("wrapped item fields wrong: %v", obj)
	}

	// Object items must NOT be wrapped — only all-scalar pages transform.
	if _, ok := wrapScalarResourceNameItems([]json.RawMessage{json.RawMessage(`{"id":"x"}`)}); ok {
		t.Fatal("object items must pass through untouched (ok=false)")
	}
	// Mixed pages must not transform either.
	if _, ok := wrapScalarResourceNameItems([]json.RawMessage{
		json.RawMessage(`"customers/1"`),
		json.RawMessage(`{"id":"x"}`),
	}); ok {
		t.Fatal("mixed scalar/object pages must pass through untouched (ok=false)")
	}
}

// TestSyncCustomersHydratesFromResourceNameStrings is the data-storage guard
// for the F0 fix: suppressing the pagination params removes the HTTP 400, but
// the mirror only hydrates if the {"resourceNames": [...]} response — an array
// of strings, not objects — actually lands as rows. Before the scalar wrap,
// UpsertBatch skipped every string item (no extractable primary key) and the
// customers mirror stayed at 0 stored records with an
// all_items_failed_id_extraction anomaly per run.
func TestSyncCustomersHydratesFromResourceNameStrings(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()

	fake := &fakeSyncClient{response: `{"resourceNames":["customers/1112223334","customers/9998887776"]}`}
	res := syncResource(fake, db, "customers", "", false, 100)
	if res.Err != nil {
		t.Fatalf("syncResource returned error: %v", res.Err)
	}
	if res.Count != 2 {
		t.Fatalf("stored %d rows, want 2 — scalar resourceNames items must hydrate the customers mirror", res.Count)
	}

	// Request-shape guard: the non-paginated endpoint must receive no params.
	if len(fake.gotParams) != 0 {
		t.Fatalf("listAccessibleCustomers received query params %v, want none", fake.gotParams)
	}

	// One row per accessible customer, keyed by the trailing customer ID.
	row, err := db.Get("customers", "1112223334")
	if err != nil {
		t.Fatalf("reading stored customer row: %v", err)
	}
	if row == nil || !strings.Contains(string(row), "customers/1112223334") {
		t.Fatalf("customer row missing or lacks resourceName: %s", row)
	}

	// The raw envelope must be preserved under the endpoint's path-derived ID
	// so `customers list-accessible-customers --data-source local` (a
	// single-object read of db.Get("customers", "customers:listAccessibleCustomers"))
	// serves the original response shape offline.
	env, err := db.Get("customers", "customers:listAccessibleCustomers")
	if err != nil {
		t.Fatalf("reading stored envelope: %v", err)
	}
	if env == nil || !strings.Contains(string(env), `"resourceNames"`) {
		t.Fatalf("raw envelope missing or malformed: %s", env)
	}
}
