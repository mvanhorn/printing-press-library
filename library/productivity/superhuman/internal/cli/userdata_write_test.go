// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// userdataWriteServer records whether it was hit and captures the last
// write body so tests can assert the POST shape.
func userdataWriteServer(t *testing.T, status int) (*httptest.Server, *int32, *map[string]any) {
	t.Helper()
	var hits int32
	captured := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v3/userdata.write") {
			http.Error(w, "unexpected path: "+r.URL.Path, 404)
			return
		}
		atomic.AddInt32(&hits, 1)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		if status != 200 {
			http.Error(w, `{"code":`+fmt.Sprint(status)+`}`, status)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	return srv, &hits, &captured
}

func TestUserdataWrite_DryRunByDefault(t *testing.T) {
	srv, hits, _ := userdataWriteServer(t, 200)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json",
		"userdata", "write", "users/gid-001/settings/probe", `{"hi":true}`)
	if err != nil {
		t.Fatalf("dry-run default: %v", err)
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Errorf("expected NO HTTP call in dry-run-by-default, got %d", atomic.LoadInt32(hits))
	}
	if !strings.Contains(stdout, `"dry_run": true`) {
		t.Errorf("expected dry_run envelope, got %s", stdout)
	}
}

func TestUserdataWrite_ApplyFires(t *testing.T) {
	srv, hits, captured := userdataWriteServer(t, 200)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json",
		"userdata", "write", "users/gid-001/settings/probe", `{"hi":true}`, "--apply")
	if err != nil {
		t.Fatalf("--apply: %v", err)
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Fatalf("expected exactly 1 HTTP call with --apply, got %d", atomic.LoadInt32(hits))
	}
	writes, ok := (*captured)["writes"].([]any)
	if !ok || len(writes) != 1 {
		t.Fatalf("expected one write in body, got %v", (*captured)["writes"])
	}
	first := writes[0].(map[string]any)
	if first["path"] != "users/gid-001/settings/probe" {
		t.Errorf("path = %v", first["path"])
	}
	val, ok := first["value"].(map[string]any)
	if !ok || val["hi"] != true {
		t.Errorf("value = %v want {hi:true}", first["value"])
	}
}

func TestUserdataWrite_RejectsNonUsersPath(t *testing.T) {
	srv, hits, _ := userdataWriteServer(t, 200)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath,
		"userdata", "write", "threads/x", `{}`, "--apply")
	if err == nil {
		t.Fatalf("expected rejection for non-users/ path")
	}
	if !strings.Contains(err.Error(), "must start with") {
		t.Errorf("error = %q want path-prefix message", err.Error())
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Errorf("no HTTP call should fire on a rejected path, got %d", atomic.LoadInt32(hits))
	}
}

func TestUserdataWrite_RejectsMalformedJSON(t *testing.T) {
	srv, hits, _ := userdataWriteServer(t, 200)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath,
		"userdata", "write", "users/gid-001/settings/probe", "not json", "--apply")
	if err == nil {
		t.Fatalf("expected rejection for malformed JSON")
	}
	if !strings.Contains(err.Error(), "well-formed JSON") {
		t.Errorf("error = %q want JSON-validity message", err.Error())
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Errorf("no HTTP call should fire on malformed JSON, got %d", atomic.LoadInt32(hits))
	}
}

func TestUserdataWrite_ApplyServerError(t *testing.T) {
	srv, _, _ := userdataWriteServer(t, 400)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json",
		"userdata", "write", "users/gid-001/settings/probe", `{"hi":true}`, "--apply")
	if err == nil {
		t.Fatalf("expected error on HTTP 400")
	}
}

func TestUserdataWrite_ArrayValueAccepted(t *testing.T) {
	srv, hits, captured := userdataWriteServer(t, 200)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json",
		"userdata", "write", "users/gid-001/labels/x", `["SENT","INBOX"]`, "--apply")
	if err != nil {
		t.Fatalf("array value should be accepted: %v", err)
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", atomic.LoadInt32(hits))
	}
	writes := (*captured)["writes"].([]any)
	first := writes[0].(map[string]any)
	arr, ok := first["value"].([]any)
	if !ok || len(arr) != 2 {
		t.Errorf("value = %v want a 2-element array", first["value"])
	}
}
