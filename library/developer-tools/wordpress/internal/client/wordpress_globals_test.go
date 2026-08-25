package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/config"
)

func TestGlobalQueryParamsSnapshotIsIndependent(t *testing.T) {
	SetGlobalQueryParam("_fields", "id,title")
	t.Cleanup(func() { SetGlobalQueryParam("_fields", "") })

	snapshot := globalQueryParamsSnapshot()
	snapshot["_fields"] = "changed"
	if got := globalQueryParamsSnapshot()["_fields"]; got != "id,title" {
		t.Fatalf("global value changed through snapshot: %q", got)
	}
}

func TestGlobalQueryParamsMergeAndExplicitParamsWin(t *testing.T) {
	SetGlobalQueryParam("_fields", "id,title")
	SetGlobalQueryParam("_embed", "1")
	t.Cleanup(func() {
		SetGlobalQueryParam("_fields", "")
		SetGlobalQueryParam("_embed", "")
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("_fields"); got != "slug" {
			t.Fatalf("_fields = %q, want explicit value slug", got)
		}
		if got := r.URL.Query().Get("_embed"); got != "1" {
			t.Fatalf("_embed = %q, want 1", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true
	if _, err := c.Get(context.Background(), "/posts", map[string]string{"_fields": "slug"}); err != nil {
		t.Fatal(err)
	}
}
