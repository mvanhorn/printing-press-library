// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/sculptok"
	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/store"
)

// The --restore-first pre-pass spends credits under its own promptId. If it is
// not persisted, reconcile (which joins credit-event remarks against stored
// prompt IDs) reports every pre-pass charge as unmatched.
func TestRunGeneratePersistsRestorePrePass(t *testing.T) {
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/draw/hd/prompt":
			w.Write([]byte(`{"code":0,"data":{"promptId":"restore-1"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/draw/prompt":
			w.Write([]byte(`{"code":0,"data":{"promptId":"main-1"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/draw/prompt":
			uuid := r.URL.Query().Get("uuid")
			w.Write([]byte(`{"code":0,"data":{"promptId":"` + uuid + `","status":2,"imgRecords":["https://cdn/` + uuid + `.png"]}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := sculptok.New(&config.Config{BaseURL: srv.URL, SculptokApiKey: "testkey"}, 5*time.Second, 0)
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	res, err := runGenerate(ctx, c, st, generateOpts{
		kind:         "depthmap",
		image:        "https://example.com/in.png",
		body:         map[string]any{},
		restoreFirst: true,
		pollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("runGenerate: %v", err)
	}
	if res.RestoredURL != "https://cdn/restore-1.png" {
		t.Fatalf("RestoredURL = %q; want the restore result", res.RestoredURL)
	}

	jobs, err := st.ListJobs(ctx, 10)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	byID := map[string]store.Job{}
	for _, j := range jobs {
		byID[j.PromptID] = j
	}
	if _, ok := byID["main-1"]; !ok {
		t.Fatalf("main draw not persisted; jobs = %v", jobs)
	}
	pre, ok := byID["restore-1"]
	if !ok {
		t.Fatalf("restore pre-pass not persisted; jobs = %v", jobs)
	}
	if pre.Kind != "restore" || pre.Status != "completed" || pre.CreditCost != drawCost("restore", "", "") {
		t.Fatalf("restore job = %+v; want kind=restore status=completed cost=%d", pre, drawCost("restore", "", ""))
	}
}
