// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// coinlocker-navi.com's terms forbid copying beyond private use, so live
// reads must not be written into the local SQLite store.
func TestSourceReadDoesNotWriteLocalStore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>東京駅のコインロッカー</title></head><body><h1>東京駅</h1><a href="/cl/2685">JR東京駅</a></body></html>`))
	}))
	defer srv.Close()
	t.Setenv("COINLOCKER_NAVI_BASE_URL", srv.URL)

	dbPath := defaultDBPath("coinlocker-navi-pp-cli")
	_ = os.Remove(dbPath)
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"source", "search", "--q", "東京駅", "--json", "--no-cache"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("source search: %v\n%s", err, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("東京駅")) {
		t.Fatalf("source search did not return the page: %s", out.String())
	}
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatalf("a live read wrote the local store at %s", dbPath)
	}
}
