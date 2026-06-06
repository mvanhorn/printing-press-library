// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopping/internal/store"
	"github.com/spf13/cobra"
)

// newTestStore opens a temp store with novel-feature tables ensured.
func newTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "shop.db")
	db, err := openShopStore(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("openShopStore: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, dbPath
}

// insertProduct writes one product row whose data JSON carries the given
// fields. retailers_id is stored both as a column and inside the JSON.
func insertProduct(t *testing.T, db *store.Store, id, retailerID string, data map[string]any) {
	t.Helper()
	data["id"] = id
	data["retailers_id"] = retailerID
	blob, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal product %s: %v", id, err)
	}
	if _, err := db.DB().Exec(
		`INSERT INTO products (id, retailers_id, data, parent_id) VALUES (?, ?, ?, ?)`,
		id, retailerID, string(blob), retailerID,
	); err != nil {
		t.Fatalf("insert product %s: %v", id, err)
	}
}

// insertPricePoint writes one price_points row.
func insertPricePoint(t *testing.T, db *store.Store, retailerID, productID, ts string, price float64) {
	t.Helper()
	if _, err := db.DB().Exec(
		`INSERT OR REPLACE INTO price_points (retailers_id, product_id, ts, price, source) VALUES (?, ?, ?, ?, ?)`,
		retailerID, productID, ts, price, "test",
	); err != nil {
		t.Fatalf("insert price point %s/%s@%s: %v", retailerID, productID, ts, err)
	}
}

// runNovelCmd builds the command, wires --db at the given path, runs it, and
// returns trimmed stdout.
func runNovelCmd(t *testing.T, build func(*rootFlags) *cobra.Command, dbPath string, args ...string) string {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	cmd := build(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	fullArgs := append([]string{"--db", dbPath}, args...)
	cmd.SetArgs(fullArgs)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %s: %v\noutput: %s", cmd.Name(), err, out.String())
	}
	return out.String()
}
