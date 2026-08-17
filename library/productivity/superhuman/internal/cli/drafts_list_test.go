// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDraftsList_SendsFilterTypeDraft is a regression guard for the live
// HTTP 400 the released CLI returned. /v3/userdata.getThreads rejects an
// empty {} body, so `drafts list` must send {filter:{type:"draft"}} — the
// same shape threads_list.go already carries (PATCH(U3)). The fake backend
// here mirrors that contract (400 unless filter.type=="draft"), so a
// regression to the empty-body shape fails this test instead of only
// failing live against Superhuman.
func TestDraftsList_SendsFilterTypeDraft(t *testing.T) {
	var observedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v3/userdata.getThreads") {
			http.Error(w, "wrong path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &observedBody)
		filter, ok := observedBody["filter"].(map[string]any)
		if !ok || filter["type"] != "draft" {
			// Mirror the real backend: missing filter.type => HTTP 400.
			http.Error(w, `{"code":400}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"threadList":[{"id":"draft007cf1fe328668c3"}]}}`))
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "list")
	if err != nil {
		t.Fatalf("drafts list --json: %v", err)
	}
	filter, ok := observedBody["filter"].(map[string]any)
	if !ok || filter["type"] != "draft" {
		t.Fatalf("request body missing filter.type=draft: %v", observedBody)
	}
	if !strings.Contains(stdout, "draft007cf1fe328668c3") {
		t.Fatalf("thread list not in envelope: %s", stdout)
	}
}
