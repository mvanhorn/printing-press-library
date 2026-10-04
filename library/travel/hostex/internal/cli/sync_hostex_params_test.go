// Copyright 2026 bust011r and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/store"
)

type paramRecorder struct {
	calls []map[string]string
}

func (r *paramRecorder) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	cp := map[string]string{}
	for k, v := range params {
		cp[k] = v
	}
	r.calls = append(r.calls, cp)
	return json.RawMessage(`{"error_code":200,"data":{}}`), nil
}

func (r *paramRecorder) RateLimit() float64 { return 0 }

// PATCH(hostex-sync-satisfies-required-list-params): Hostex rejects list calls
// without an explicit offset, and /transactions needs a window of at most 366
// days.
func TestSyncSendsParamsHostexRequires(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	rec := &paramRecorder{}
	syncResource(context.Background(), rec, db, "reservations", "", true, 1, false, false, nil, nil)
	if len(rec.calls) == 0 {
		t.Fatal("sync made no request for reservations")
	}
	if got, ok := rec.calls[0]["offset"]; !ok || got != "0" {
		t.Fatalf("first reservations page offset = %q (present=%v), want explicit 0", got, ok)
	}

	rec = &paramRecorder{}
	syncResource(context.Background(), rec, db, "transactions", "", true, 1, false, false, nil, nil)
	if len(rec.calls) == 0 {
		t.Fatal("sync made no request for transactions")
	}
	first := rec.calls[0]
	start, err := time.Parse("2006-01-02", first["start_date"])
	if err != nil {
		t.Fatalf("start_date %q: %v", first["start_date"], err)
	}
	end, err := time.Parse("2006-01-02", first["end_date"])
	if err != nil {
		t.Fatalf("end_date %q: %v", first["end_date"], err)
	}
	if days := int(end.Sub(start).Hours() / 24); days < 1 || days > 366 {
		t.Fatalf("transactions window = %d days, want 1..366", days)
	}
}

func TestDefaultSyncResourcesSkipsEndpointsNeedingParams(t *testing.T) {
	for _, name := range defaultSyncResources() {
		if name == "automation" || name == "pricing-ratios" {
			t.Fatalf("defaultSyncResources lists %q, which needs a param the generic sync cannot supply", name)
		}
	}
}
