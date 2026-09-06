package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func TestRunAutoRefreshSkipsResourcesWithRequiredParams(t *testing.T) {
	isolateNovelTest(t)
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer srv.Close()
	t.Setenv("SEATS_AERO_BASE_URL", srv.URL)
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stderr := captureStderr(t, func() {
		if _, err := runAutoRefresh(context.Background(), &rootFlags{}, db, []string{"availability"}); err != nil {
			t.Fatal(err)
		}
	})
	if requests.Load() != 0 {
		t.Fatalf("requests=%d, want 0", requests.Load())
	}
	if !strings.Contains(string(stderr), "missing_required_params: source") || !strings.Contains(string(stderr), `"skipped":true`) {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestAutoRefreshIfStaleReportsAllRequiredParamSkips(t *testing.T) {
	isolateNovelTest(t)
	t.Setenv("SEATS_AERO_NO_AUTO_REFRESH", "0")
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1); t.Error("unexpected request") }))
	defer srv.Close()
	t.Setenv("SEATS_AERO_BASE_URL", srv.URL)
	path := defaultDBPath("seats-aero-pp-cli")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncStateAt("availability", "", 1, time.Now().UTC().Add(-2000*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var metaReason string
	var ran bool
	stderr := captureStderr(t, func() {
		meta := autoRefreshIfStale(t.Context(), &rootFlags{dataSource: "auto"}, []string{"availability"})
		metaReason, ran = meta.Reason, meta.Ran
	})
	if requests.Load() != 0 || metaReason != "skipped_missing_required_params" || ran {
		t.Fatalf("requests=%d reason=%q ran=%v", requests.Load(), metaReason, ran)
	}
	if !strings.Contains(string(stderr), "warning: using stale seats-aero-pp-cli cache (auto-refresh skipped: missing_required_params: source)") {
		t.Fatalf("stderr=%q", stderr)
	}
}
