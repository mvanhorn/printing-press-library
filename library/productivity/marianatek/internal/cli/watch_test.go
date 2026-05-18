// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchAutoBookRetriesAfterFailure(t *testing.T) {
	var getCount int32
	var postCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/classes/class-1":
			spots := 1
			if atomic.AddInt32(&getCount, 1) == 1 {
				spots = 0
			}
			fmt.Fprintf(w, `{"data":{"attributes":{"remaining_spots":%d}}}`, spots)
		case r.Method == http.MethodPost && r.URL.Path == "/me/reservations":
			if atomic.AddInt32(&postCount, 1) == 1 {
				http.Error(w, `{"error":"try again"}`, http.StatusConflict)
				return
			}
			fmt.Fprint(w, `{"data":{"id":"reservation-1"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	config := fmt.Sprintf("base_url = %q\nbase_path = \"\"\n", server.URL)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	flags := &rootFlags{configPath: configPath, noCache: true, timeout: time.Second}
	cmd := newWatchCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"class-1", "--interval", "1ms", "--max-duration", "250ms", "--auto-book"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("watch returned error: %v", err)
	}
	if got := atomic.LoadInt32(&postCount); got != 2 {
		t.Fatalf("POST attempts = %d, want 2; output:\n%s", got, out.String())
	}
	for _, want := range []string{`"event":"auto_book_failed"`, `"event":"booked"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %s:\n%s", want, out.String())
		}
	}
}
