// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRedirectHopsCountAgainstBudget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			http.Redirect(w, r, "/c", http.StatusFound)
		default:
			_, _ = w.Write([]byte("<html>ok</html>"))
		}
	}))
	defer srv.Close()

	f := NewFetcher(5*time.Second, MaxPerHostRate, 10)
	f.noPacing = true
	if _, err := f.GetHTML(context.Background(), srv.URL+"/a"); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.Stats(); got != 3 {
		t.Fatalf("requests = %d, want 3 (one request plus two redirect hops)", got)
	}

	tight := NewFetcher(5*time.Second, MaxPerHostRate, 2)
	tight.noPacing = true
	_, err := tight.GetHTML(context.Background(), srv.URL+"/a")
	if err == nil || !strings.Contains(err.Error(), "request budget of 2 reached") {
		t.Fatalf("want budget error on the second hop, got %v", err)
	}
}
