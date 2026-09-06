// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-tag-manager/internal/store"
)

type fakeGetter struct{ responses map[string]string }

func (f fakeGetter) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	if r, ok := f.responses[path]; ok {
		return json.RawMessage(r), nil
	}
	return nil, fmt.Errorf("no fake response for %s", path)
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := ensureGTMSchema(context.Background(), s.DB()); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s.DB()
}

func testCtx() context.Context { return context.Background() }

func fixedNow() time.Time { return time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC) }
