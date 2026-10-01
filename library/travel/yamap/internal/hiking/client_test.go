package hiking

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/yamap/internal/cliutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnonymousCacheRefreshOffline(t *testing.T) {
	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != "GET" || r.Header.Get("Accept-Language") != "ja" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("unexpected request", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"activities":[],"meta":{"next_page":-1}}`))
	}))
	defer s.Close()
	c := NewClient(t.TempDir(), 2)
	c.Base = s.URL
	c.limiter = nil
	_, m, e := c.Fetch(context.Background(), "/activities/search", nil)
	if e != nil || m.CacheHit || hits != 1 {
		t.Fatal(m, e)
	}
	_, m, e = c.Fetch(context.Background(), "/activities/search", nil)
	if e != nil || !m.CacheHit || hits != 1 {
		t.Fatal(m, e)
	}
	c.Refresh = true
	_, m, e = c.Fetch(context.Background(), "/activities/search", nil)
	if e != nil || m.CacheHit || hits != 2 {
		t.Fatal(m, e)
	}
	c.Refresh = false
	c.Offline = true
	_, m, e = c.Fetch(context.Background(), "/activities/search", nil)
	if e != nil || !m.CacheHit || hits != 2 {
		t.Fatal(m, e)
	}
	if _, _, e = c.Fetch(context.Background(), "/missing", nil); e == nil {
		t.Fatal("offline invented result")
	}
	inv, e := c.Inventory()
	if e != nil || inv["entry_count"] != 1 {
		t.Fatal(inv, e)
	}
}
func TestStaleAndCorruptCache(t *testing.T) {
	dir := t.TempDir()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ok":true}`)) }))
	defer s.Close()
	c := NewClient(dir, 2)
	c.Base = s.URL
	c.limiter = nil
	c.Fetch(context.Background(), "/x", nil)
	entries, _ := os.ReadDir(dir)
	path := filepath.Join(dir, entries[0].Name())
	b, _ := os.ReadFile(path)
	var entry cacheEntry
	json.Unmarshal(b, &entry)
	entry.FetchedAt = time.Now().Add(-time.Hour)
	b, _ = json.Marshal(entry)
	os.WriteFile(path, b, 0600)
	c.Offline = true
	_, m, e := c.Fetch(context.Background(), "/x", nil)
	if e != nil || !m.Stale {
		t.Fatal(m, e)
	}
	os.WriteFile(path, []byte(`{invalid`), 0600)
	if _, _, e = c.Fetch(context.Background(), "/x", nil); e == nil {
		t.Fatal("corrupt cache accepted")
	}
}
func TestRateLimitZeroDisables(t *testing.T) {
	disabled := NewClient("", 0)
	if disabled.limiter != nil {
		t.Fatal("rate 0 still paces")
	}
	paced := NewClient("", -1)
	if paced.limiter == nil || paced.limiter.Rate() != 2 {
		t.Fatalf("default rate = %v", paced.limiter)
	}
}

func TestRateLimitAndFailureAreNotEmpty(t *testing.T) {
	for _, status := range []int{429, 404, 403, 202, 500} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(status)
		}))
		c := NewClient("", 2)
		c.Base = s.URL
		c.limiter = nil
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		obj, _, e := c.Fetch(ctx, "/x", nil)
		cancel()
		s.Close()
		if e == nil || obj != nil {
			t.Fatal(status, obj, e)
		}
		if status == 429 {
			var rate *cliutil.RateLimitError
			if !errors.As(e, &rate) {
				t.Fatal("lost typed rate limit", e)
			}
		}
	}
}
func TestChallengeJSONAndDeadline(t *testing.T) {
	for _, body := range []string{`<html>challenge</html>`, `[]`, `{"ok":true} trailing`, `{"error":{"reason":"restricted"}}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		c := NewClient("", 2)
		c.Base = s.URL
		c.limiter = nil
		if _, _, e := c.Fetch(context.Background(), "/x", nil); e == nil {
			t.Fatal(body)
		}
		s.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient("", 2)
	if _, _, e := c.Fetch(ctx, "/x", nil); e == nil {
		t.Fatal("cancelled request passed")
	}
}
