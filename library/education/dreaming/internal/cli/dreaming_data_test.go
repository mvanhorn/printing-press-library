// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"
)

func TestEnsureUserPropagatesFetchErrors(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("DREAMING_BASE_URL", server.URL)
	t.Setenv("DREAMING_TOKEN", "test-token")

	_, ok, err := ensureUser(context.Background(), &rootFlags{}, db)
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("got ok=%v error=%v, want HTTP 503 error", ok, err)
	}
}

func TestStreaksUsesLocalCalendarDay(t *testing.T) {
	loc := time.FixedZone("UTC-8", -8*60*60)
	now := time.Date(2026, time.July, 20, 18, 0, 0, 0, loc)

	current, longest := streaksAt([]daySeconds{
		{Date: "2026-07-18", Seconds: 600},
		{Date: "2026-07-19", Seconds: 600},
	}, now)
	if current != 2 || longest != 2 {
		t.Fatalf("got current=%d longest=%d, want 2 and 2", current, longest)
	}
}
