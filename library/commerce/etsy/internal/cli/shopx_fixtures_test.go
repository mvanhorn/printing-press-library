// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// shopxFixedNow is the deterministic clock for every Slice 3 test.
var shopxFixedNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// shopxAgo / shopxIn return unix seconds relative to the fixed clock.
func shopxAgo(d time.Duration) int64 { return shopxFixedNow.Add(-d).Unix() }
func shopxIn(d time.Duration) int64  { return shopxFixedNow.Add(d).Unix() }

const shopxTestShopID = "555001"

const shopxDay = 24 * time.Hour

// shopxFixture is a synthetic SQLite store built through the real schema and
// the real sync write path (store.UpsertBatchDetailed, the function sync.go's
// upsertResourceBatch calls), so tables and columns are the real ones.
type shopxFixture struct {
	t    *testing.T
	db   *store.Store
	path string
}

func shopxNewFixture(t *testing.T) *shopxFixture {
	t.Helper()
	testenv.Isolate(t)
	t.Setenv("ETSY_SHOP_ID", shopxTestShopID)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open fixture store: %v", err)
	}
	f := &shopxFixture{t: t, db: db, path: path}
	t.Cleanup(func() { f.close() })
	return f
}

func (f *shopxFixture) close() {
	if f.db != nil {
		_ = f.db.Close()
		f.db = nil
	}
}

// put writes rows under a sync resource name and fails the test if the real
// upsert path silently drops any of them.
func (f *shopxFixture) put(resource string, rows ...map[string]any) {
	f.t.Helper()
	items := make([]json.RawMessage, 0, len(rows))
	for _, r := range rows {
		for _, typ := range shopxListingTypes {
			if resource == typ {
				if _, exists := r["shop_id"]; !exists {
					r["shop_id"] = 555001
				}
			}
		}
		b, err := json.Marshal(r)
		if err != nil {
			f.t.Fatalf("marshal %s row: %v", resource, err)
		}
		items = append(items, b)
	}
	stored, extractFailures, _, err := f.db.UpsertBatchDetailed(resource, items)
	if err != nil {
		f.t.Fatalf("upsert %s: %v", resource, err)
	}
	if stored != len(rows) || extractFailures != 0 {
		f.t.Fatalf("upsert %s: stored %d of %d rows (extract failures %d)", resource, stored, len(rows), extractFailures)
	}
}

// synced records a completed sync of resource `ago` before the fixed clock.
func (f *shopxFixture) synced(resource string, ago time.Duration) {
	f.t.Helper()
	if err := f.db.SaveSyncStateAt(resource, "", 1, shopxFixedNow.Add(-ago)); err != nil {
		f.t.Fatalf("sync state %s: %v", resource, err)
	}
}

func shopxMoneyObj(amount int64, currency string) map[string]any {
	return map[string]any{"amount": amount, "divisor": 100, "currency_code": currency}
}

// shopxRun executes `etsy-pp-cli <args> --db <fixture> --json` with the fixed
// clock and returns stdout/stderr. The fixture is closed first so the command
// opens it read-only exactly as in production.
func shopxRun(t *testing.T, f *shopxFixture, args ...string) (string, string, error) {
	t.Helper()
	prev := shopxNow
	shopxNow = func() time.Time { return shopxFixedNow }
	t.Cleanup(func() { shopxNow = prev })
	path := filepath.Join(t.TempDir(), "absent.db")
	if f != nil {
		f.close()
		path = f.path
	}
	root := RootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append(append([]string{}, args...), "--db", path, "--json"))
	err := root.Execute()
	return out.String(), errb.String(), err
}

func shopxMustEnvelope(t *testing.T, stdout string) (any, []string) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatal(err, stdout)
	}
	if _, ok := obj["meta"]; ok {
		var wrapped map[string]json.RawMessage
		if err := json.Unmarshal(obj["results"], &wrapped); err != nil {
			t.Fatal(err)
		}
		obj = wrapped
	}
	var notes []string
	if err := json.Unmarshal(obj["notes"], &notes); err != nil || notes == nil {
		t.Fatalf("notes must be an array: %s", stdout)
	}
	var results any
	if err := json.Unmarshal(obj["results"], &results); err != nil {
		t.Fatal(err)
	}
	return results, notes
}

func shopxMustList(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	result, _ := shopxMustEnvelope(t, stdout)
	b, _ := json.Marshal(result)
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil || rows == nil {
		t.Fatalf("results must be array: %s", stdout)
	}
	return rows
}

func shopxMustObject(t *testing.T, stdout string) map[string]any {
	t.Helper()
	result, _ := shopxMustEnvelope(t, stdout)
	obj, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("results must be object: %s", stdout)
	}
	return obj
}

func shopxDecodeResults(t *testing.T, stdout string, v any) {
	t.Helper()
	results, _ := shopxMustEnvelope(t, stdout)
	b, _ := json.Marshal(results)
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}
