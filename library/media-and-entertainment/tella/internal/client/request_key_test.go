// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWriteRequestIdentity(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	var keys []string
	attempt := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		attempt++
		w.Header().Set("Content-Type", "application/json")
		if attempt == 1 {
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":"retry"}`)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer s.Close()
	c := New(&config.Config{BaseURL: s.URL}, 5*time.Second, 1000)
	c.NoCache = true
	for _, verb := range []string{"POST", "PUT", "PATCH", "DELETE", "GET"} {
		if _, _, err := c.do(context.Background(), verb, "/v1/test", nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(keys) != 6 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("retry keys: %v", keys)
	}
	seen := map[string]bool{}
	for _, k := range keys[1:5] {
		if k == "" || seen[k] {
			t.Fatalf("distinct writes share identity: %v", keys)
		}
		seen[k] = true
	}
	if keys[5] != "" {
		t.Fatal("read received key")
	}
	if _, _, err := c.PostWithHeaders(context.Background(), "/v1/test", nil, map[string]string{"Idempotency-Key": "explicit-test-key"}); err != nil {
		t.Fatal(err)
	}
	if keys[6] != "explicit-test-key" {
		t.Fatal("override lost")
	}
}
