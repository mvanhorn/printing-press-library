package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

func isolateEtsyQuota(t *testing.T) string {
	t.Helper()
	etsyQuotaState.Lock()
	etsyQuotaState.quotas = nil
	etsyQuotaState.persistErrors = nil
	etsyQuotaState.Unlock()
	restore, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	_, err = cliutil.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	return quotaPath("testkey")
}
func quotaHeaders() http.Header {
	h := make(http.Header)
	h.Set("x-limit-per-day", "5000")
	h.Set("x-remaining-today", "4210")
	h.Set("x-limit-per-second", "5")
	h.Set("x-remaining-this-second", "4")
	return h
}
func TestEtsyQuotaObservation(t *testing.T) {
	path := isolateEtsyQuota(t)
	ObserveEtsyQuota(http.Header{}, "testkey")
	ObserveEtsyQuota(http.Header{"X-Remaining-Today": {"bad"}}, "testkey")
	if _, ok := LoadEtsyQuota("testkey"); ok {
		t.Fatal("invalid headers recorded quota")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected state file: %v", err)
	}
	ObserveEtsyQuota(quotaHeaders(), "testkey")
	q, ok := LoadEtsyQuota("testkey")
	if !ok || q.RemainingToday != 4210 || q.LimitPerDay != 5000 || q.LimitPerSecond != 5 || q.ObservedAt.Location() != time.UTC {
		t.Fatalf("quota = %+v, known=%v", q, ok)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions = %v", info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored EtsyQuota
	if err := json.Unmarshal(data, &stored); err != nil || stored.RemainingToday != 4210 {
		t.Fatalf("persisted quota = %+v, %v", stored, err)
	}
	h := quotaHeaders()
	h.Set("x-limit-per-day", "bad")
	h.Set("x-remaining-today", "4200")
	ObserveEtsyQuota(h, "testkey")
	q, _ = LoadEtsyQuota("testkey")
	if q.LimitPerDay != 5000 || q.RemainingToday != 4200 {
		t.Fatalf("partial update = %+v", q)
	}
	h.Set("x-remaining-today", "bad")
	ObserveEtsyQuota(h, "testkey")
	q, _ = LoadEtsyQuota("testkey")
	if q.RemainingToday != 4200 {
		t.Fatalf("invalid remainder replaced quota: %+v", q)
	}
}

// etsy-ext: expiry is rolling and future timestamps cannot poison merges.
func TestEtsyQuotaStaleFile(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	oldNow := etsyQuotaNow
	etsyQuotaNow = func() time.Time { return now }
	t.Cleanup(func() { etsyQuotaNow = oldNow })
	for _, tc := range []struct {
		name  string
		at    time.Time
		known bool
	}{
		{"23h59", now.Add(-23*time.Hour - 59*time.Minute), true},
		{"24h01", now.Add(-24*time.Hour - time.Minute), false},
		{"previous day", time.Date(2026, 11, 18, 23, 59, 0, 0, time.UTC), true},
		{"future", now.Add(2 * time.Minute), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := isolateEtsyQuota(t)
			q := EtsyQuota{RemainingToday: 10, ObservedAt: tc.at}
			data, _ := json.Marshal(q)
			if err := cliutil.AtomicWritePrivateFile(path, data, 0600, 0700); err != nil {
				t.Fatal(err)
			}
			if _, ok := LoadEtsyQuota("testkey"); ok != tc.known {
				t.Fatalf("known=%v", ok)
			}
			if got := mergeEtsyQuota(EtsyQuota{}, q); !tc.known && !got.ObservedAt.IsZero() {
				t.Fatal("untrusted merge")
			}
		})
	}
}

func TestEtsyQuotaRawMerge(t *testing.T) {
	isolateEtsyQuota(t)
	now := time.Now().Add(-time.Minute)
	q := EtsyQuota{}
	for i := 0; i < 4; i++ {
		next := EtsyQuota{RemainingToday: 100 + i, ObservedAt: now.Add(time.Duration(i) * 3 * time.Second)}
		if i == 0 {
			next.LimitPerDay = 5000
			next.LimitPerSecond = 5
		}
		q = mergeEtsyQuota(q, next)
	}
	if q.RemainingToday != 102 || q.LimitPerDay != 5000 || q.LimitPerSecond != 5 {
		t.Fatalf("merge: %+v", q)
	}
}
func TestEtsyQuotaUnwritableCache(t *testing.T) {
	path := isolateEtsyQuota(t)
	// A regular file used as a directory fails even under privileged test runners.
	if err := os.MkdirAll(filepath.Dir(filepath.Dir(path)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Dir(path), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	ObserveEtsyQuota(quotaHeaders(), "testkey")
	if q, ok := LoadEtsyQuota("testkey"); !ok || q.RemainingToday != 4210 {
		t.Fatalf("cache %+v %v", q, ok)
	}
	if EtsyQuotaPersistError("testkey") == nil {
		t.Fatal("missing persistence error")
	}
}

func TestEtsyQuotaClientResponse(t *testing.T) {
	isolateEtsyQuota(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range quotaHeaders() {
			w.Header()[k] = v
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(&config.Config{BaseURL: srv.URL, EtsyApiKey: "testkey"}, 5*time.Second, 0)
	c.HTTPClient = srv.Client()
	if _, err := c.Get(context.Background(), "/quota", nil); err != nil {
		t.Fatal(err)
	}
	q, ok := LoadEtsyQuota("testkey")
	if !ok || q.RemainingToday != 4210 || q.LimitPerDay != 5000 || q.LimitPerSecond != 5 {
		t.Fatalf("response quota = %+v, known=%v", q, ok)
	}
}

// etsy-ext: filesystem permissions cannot disable a long-lived process's quota guard.
func TestEtsyQuotaReadOnlyState(t *testing.T) {
	path := isolateEtsyQuota(t)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	probe := filepath.Join(dir, "permission-probe")
	if err := os.WriteFile(probe, nil, 0600); err == nil {
		_ = os.Remove(probe)
		t.Skip("runner bypasses directory write permissions")
	}
	ObserveEtsyQuota(quotaHeaders(), "testkey")
	if q, ok := LoadEtsyQuota("testkey"); !ok || q.RemainingToday != 4210 {
		t.Fatalf("lost in-process quota: %+v %v", q, ok)
	}
	if EtsyQuotaPersistError("testkey") == nil {
		t.Fatal("missing persistence error")
	}
}

func TestEtsyQuotaMergeOrdering(t *testing.T) {
	isolateEtsyQuota(t)
	now := time.Now()
	for _, gap := range []time.Duration{4 * time.Second, 5 * time.Second, 6 * time.Second} {
		a := EtsyQuota{ObservedAt: now.Add(-gap), RemainingToday: 10, LimitPerDay: 100, LimitPerSecond: 2}
		b := EtsyQuota{ObservedAt: now, RemainingToday: 50, LimitPerDay: 5000, LimitPerSecond: 5}
		want := 50
		if gap <= 5*time.Second {
			want = 10
		}
		for _, q := range []EtsyQuota{mergeEtsyQuota(a, b), mergeEtsyQuota(b, a)} {
			if q.RemainingToday != want || q.LimitPerDay != 5000 || q.LimitPerSecond != 5 || !q.ObservedAt.Equal(now) {
				t.Fatalf("gap %s: %+v", gap, q)
			}
		}
	}
}
