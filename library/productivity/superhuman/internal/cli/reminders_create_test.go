// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRemindersCreate_BackendCallShape verifies the command issues a
// userdata.read for the thread (to discover its messageIds) followed by
// a userdata.write with the full reminder value shape that the
// Superhuman backend validates: clientCreatedAt + keepOnReply +
// messageIds + onDesktop + reminderId + source + threadId + triggerAt.
// Shape recovered from a live-snoozed thread via /v3/userdata.read --
// see .manuscripts/20260515-165115/discovery/reminders-sniff-report.md.
func TestRemindersCreate_BackendCallShape(t *testing.T) {
	var writePath string
	var writeValue map[string]any
	var sawRead bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		switch {
		case strings.HasSuffix(r.URL.Path, "/v3/userdata.read"):
			sawRead = true
			fmt.Fprint(w, `{"results":[{"value":{"messages":{"msg-a":{},"msg-b":{}}}}]}`)
		case strings.HasSuffix(r.URL.Path, "/v3/userdata.write"):
			if writes, ok := parsed["writes"].([]any); ok && len(writes) > 0 {
				if first, ok := writes[0].(map[string]any); ok {
					if p, ok := first["path"].(string); ok {
						writePath = p
					}
					if v, ok := first["value"].(map[string]any); ok {
						writeValue = v
					}
				}
			}
			fmt.Fprint(w, `{"ok":true}`)
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, 404)
		}
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json",
		"reminders", "create",
		"--thread-id", "19e2dc46a8b281fe",
		"--trigger-at", "2026-05-16T09:00:00Z",
	)
	if err != nil {
		t.Fatalf("reminders create --json: %v", err)
	}

	if !sawRead {
		t.Fatalf("expected userdata.read to fetch thread messageIds")
	}

	wantPath := "users/gid-001/threads/19e2dc46a8b281fe/reminder"
	if writePath != wantPath {
		t.Fatalf("path = %q want %q", writePath, wantPath)
	}

	// Field-by-field shape check
	if writeValue["threadId"] != "19e2dc46a8b281fe" {
		t.Errorf("threadId = %v want 19e2dc46a8b281fe", writeValue["threadId"])
	}
	if writeValue["source"] != "USER" {
		t.Errorf("source = %v want \"USER\"", writeValue["source"])
	}
	if writeValue["onDesktop"] != false {
		t.Errorf("onDesktop = %v want false", writeValue["onDesktop"])
	}
	// triggerAt is the ISO-with-nanos format
	if got, _ := writeValue["triggerAt"].(string); got != "2026-05-16T09:00:00.000000000Z" {
		t.Errorf("triggerAt = %q want \"2026-05-16T09:00:00.000000000Z\"", got)
	}
	if writeValue["keepOnReply"] != true {
		t.Errorf("keepOnReply for default --condition always should be true, got %v", writeValue["keepOnReply"])
	}
	// messageIds comes from the userdata.read response
	ids, ok := writeValue["messageIds"].([]any)
	if !ok || len(ids) != 2 {
		t.Fatalf("messageIds should be a 2-element array from the read response, got %v", writeValue["messageIds"])
	}
	if ids[0] != "msg-a" || ids[1] != "msg-b" {
		t.Errorf("messageIds = %v want [msg-a msg-b] (sorted)", ids)
	}
	// reminderId is a UUID -- just check it's a non-empty string of the right length
	rid, _ := writeValue["reminderId"].(string)
	if len(rid) != 36 {
		t.Errorf("reminderId should be a UUID (36 chars), got %q", rid)
	}
	// clientCreatedAt is an ISO-with-nanos timestamp
	cca, _ := writeValue["clientCreatedAt"].(string)
	if !strings.HasSuffix(cca, "Z") || len(cca) < 24 {
		t.Errorf("clientCreatedAt should be an ISO-with-nanos UTC timestamp, got %q", cca)
	}

	if !strings.Contains(stdout, "reminders") {
		t.Fatalf("envelope missing resource: %s", stdout)
	}
}

// TestRemindersCreate_IfNoReplyCondition flips keepOnReply to false when
// --condition if-no-reply is set.
func TestRemindersCreate_IfNoReplyCondition(t *testing.T) {
	var writeValue map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		switch {
		case strings.HasSuffix(r.URL.Path, "/v3/userdata.read"):
			fmt.Fprint(w, `{"results":[{"value":{"messages":{"m1":{}}}}]}`)
		case strings.HasSuffix(r.URL.Path, "/v3/userdata.write"):
			if writes, ok := parsed["writes"].([]any); ok && len(writes) > 0 {
				if first, ok := writes[0].(map[string]any); ok {
					if v, ok := first["value"].(map[string]any); ok {
						writeValue = v
					}
				}
			}
			fmt.Fprint(w, `{"ok":true}`)
		}
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json",
		"reminders", "create",
		"--thread-id", "abc123",
		"--trigger-at", "2026-05-16T09:00:00Z",
		"--condition", "if-no-reply",
	)
	if err != nil {
		t.Fatalf("reminders create: %v", err)
	}
	if writeValue["keepOnReply"] != false {
		t.Fatalf("keepOnReply for --condition if-no-reply should be false, got %v", writeValue["keepOnReply"])
	}
}

// TestRemindersCreate_TriggerAtAcceptsMsInt confirms --trigger-at also
// accepts integer ms strings and normalizes them to the ISO format.
func TestRemindersCreate_TriggerAtAcceptsMsInt(t *testing.T) {
	var writeValue map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		switch {
		case strings.HasSuffix(r.URL.Path, "/v3/userdata.read"):
			fmt.Fprint(w, `{"results":[{"value":{"messages":{"m1":{}}}}]}`)
		case strings.HasSuffix(r.URL.Path, "/v3/userdata.write"):
			if writes, ok := parsed["writes"].([]any); ok && len(writes) > 0 {
				if first, ok := writes[0].(map[string]any); ok {
					if v, ok := first["value"].(map[string]any); ok {
						writeValue = v
					}
				}
			}
			fmt.Fprint(w, `{"ok":true}`)
		}
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json",
		"reminders", "create",
		"--thread-id", "abc123",
		"--trigger-at", "1778950800000", // 2026-05-16T17:00:00Z
	)
	if err != nil {
		t.Fatalf("reminders create: %v", err)
	}
	if got, _ := writeValue["triggerAt"].(string); got != "2026-05-16T17:00:00.000000000Z" {
		t.Fatalf("triggerAt = %q want \"2026-05-16T17:00:00.000000000Z\"", got)
	}
}

// TestRemindersCreate_InvalidCondition rejects unknown --condition values
// before making any HTTP call.
func TestRemindersCreate_InvalidCondition(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath,
		"reminders", "create",
		"--thread-id", "abc",
		"--trigger-at", "2026-05-16T09:00:00Z",
		"--condition", "maybe",
	)
	if err == nil {
		t.Fatalf("expected error for invalid --condition")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}

// TestRemindersCreate_MissingTriggerAt surfaces the required-flag error.
func TestRemindersCreate_MissingTriggerAt(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath,
		"reminders", "create",
		"--thread-id", "abc",
	)
	if err == nil {
		t.Fatalf("expected error when --trigger-at missing")
	}
}

// TestExtractThreadMessageIDs_Sorts confirms message ids returned in any
// order from userdata.read are sorted before going into the write payload
// (Gmail hex ids sort by send time when alphabetized, which is what the
// web client appears to do).
func TestExtractThreadMessageIDs_Sorts(t *testing.T) {
	raw := json.RawMessage(`{"results":[{"value":{"messages":{"19b":{},"19a":{},"19c":{}}}}]}`)
	got, err := extractThreadMessageIDs(raw)
	if err != nil {
		t.Fatalf("extractThreadMessageIDs: %v", err)
	}
	want := []string{"19a", "19b", "19c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q want %q", i, got[i], want[i])
		}
	}
}
