package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

func executeForReviewTest(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestGenericSearchUsesDesignSearchContract(t *testing.T) {
	var gotPath string
	var gotQuery map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = make(map[string]string)
		for key := range r.URL.Query() {
			gotQuery[key] = r.URL.Query().Get(key)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"hits":[{"id":1,"title":"dragon"}]}`)
	}))
	defer srv.Close()
	t.Setenv("MAKERWORLD_BASE_URL", srv.URL)
	t.Setenv("HOME", t.TempDir())

	stdout, _, err := executeForReviewTest(t, "search", "articulated dragon", "--data-source", "live", "--json", "--limit", "2")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if gotPath != "/search-service/select/design2" {
		t.Fatalf("path = %q, want design search endpoint", gotPath)
	}
	for key, want := range map[string]string{"keyword": "articulated dragon", "orderBy": "score", "offset": "0", "limit": "2"} {
		if gotQuery[key] != want {
			t.Errorf("query[%q] = %q, want %q", key, gotQuery[key], want)
		}
	}
	if !bytes.Contains([]byte(stdout), []byte(`"title": "dragon"`)) {
		t.Fatalf("search output omitted matched design: %s", stdout)
	}
}

func TestDesignsListAllAdvancesOffset(t *testing.T) {
	var mu sync.Mutex
	var offsets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		mu.Lock()
		offsets = append(offsets, offset)
		mu.Unlock()
		count := 2
		if offset == 4 {
			count = 1
		}
		items := make([]map[string]any, 0, count)
		for i := 0; i < count; i++ {
			items = append(items, map[string]any{"id": offset + i + 1})
		}
		_ = json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()
	t.Setenv("MAKERWORLD_BASE_URL", srv.URL)
	t.Setenv("HOME", t.TempDir())

	if _, _, err := executeForReviewTest(t, "designs", "list", "--all", "--limit", "2", "--data-source", "live", "--json"); err != nil {
		t.Fatalf("designs list --all: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []int{0, 2, 4}
	if fmt.Sprint(offsets) != fmt.Sprint(want) {
		t.Fatalf("offsets = %v, want %v", offsets, want)
	}
}

func TestFavoritesNeverSharesCachedAccountData(t *testing.T) {
	var mu sync.Mutex
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": []map[string]string{{"token": r.Header.Get("Authorization")}}})
	}))
	defer srv.Close()
	t.Setenv("MAKERWORLD_BASE_URL", srv.URL)
	t.Setenv("HOME", t.TempDir())

	t.Setenv("MAKERWORLD_TOKEN", "alice")
	alice, _, err := executeForReviewTest(t, "favorites", "--json")
	if err != nil {
		t.Fatalf("alice favorites: %v", err)
	}
	t.Setenv("MAKERWORLD_TOKEN", "bob")
	bob, _, err := executeForReviewTest(t, "favorites", "--json")
	if err != nil {
		t.Fatalf("bob favorites: %v", err)
	}
	if !bytes.Contains([]byte(alice), []byte("Bearer alice")) || !bytes.Contains([]byte(bob), []byte("Bearer bob")) {
		t.Fatalf("account outputs not isolated: alice=%q bob=%q", alice, bob)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests != 2 {
		t.Fatalf("endpoint requests = %d, want 2 (no account-scoped cache reuse)", requests)
	}
}
