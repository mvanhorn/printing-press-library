// Copyright 2026 michegz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/teslatracker/internal/store"
)

func intPtr(n int) *int       { return &n }
func int64Ptr(n int64) *int64 { return &n }

func seedVehicleMirror(t *testing.T, vehicles ...Vehicle) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	st, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	for _, v := range vehicles {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %s: %v", v.VIN, err)
		}
		if err := st.Upsert(vehicleResourceType, v.VIN, raw); err != nil {
			t.Fatalf("upsert %s: %v", v.VIN, err)
		}
	}
	return dbPath
}
