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

// modifyFake records add/remove label payloads and responds with the
// post-modify message envelope so the helper can read the resulting label
// set.
type modifyFake struct {
	srv          *httptest.Server
	addPayloads  []map[string]any
	rmPayloads   []map[string]any
	receivedPath string
}

func newModifyFake(t *testing.T, respMsgLabels []string) *modifyFake {
	t.Helper()
	f := &modifyFake{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.receivedPath = r.URL.Path
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		f.addPayloads = append(f.addPayloads, parsed)
		// Response shape: thread modify returns {messages:[{labelIds:[...]}, ...]}.
		resp := map[string]any{
			"messages": []map[string]any{
				{"labelIds": respMsgLabels},
			},
		}
		out, _ := json.Marshal(resp)
		_, _ = w.Write(out)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// TestThreadsUpdate_Archive_RemovesInbox verifies the archive verb maps
// to remove-INBOX with no adds.
func TestThreadsUpdate_Archive_RemovesInbox(t *testing.T) {
	fake := newModifyFake(t, []string{"UNREAD"})
	withGmailBaseURL(t, fake.srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "threads", "update", "19abc", "--action", "archive")
	if err != nil {
		t.Fatalf("threads update --action archive: %v", err)
	}
	if !strings.Contains(fake.receivedPath, "/threads/19abc/modify") {
		t.Fatalf("path = %q want /threads/19abc/modify", fake.receivedPath)
	}
	payload := fake.addPayloads[0]
	if rm, _ := payload["removeLabelIds"].([]any); len(rm) != 1 || rm[0] != "INBOX" {
		t.Fatalf("archive should removeLabelIds=[INBOX], got %v", payload["removeLabelIds"])
	}
	if add, _ := payload["addLabelIds"].([]any); len(add) != 0 {
		t.Fatalf("archive should have no addLabelIds, got %v", add)
	}
	var env map[string]any
	_ = json.Unmarshal([]byte(stdout), &env)
	if env["verb"] != "archive" {
		t.Fatalf("envelope verb = %v want archive", env["verb"])
	}
}

// TestThreadsUpdate_Star_AddsStarred verifies the star verb adds STARRED.
func TestThreadsUpdate_Star_AddsStarred(t *testing.T) {
	fake := newModifyFake(t, []string{"INBOX", "STARRED"})
	withGmailBaseURL(t, fake.srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "threads", "update", "19abc", "--action", "star")
	if err != nil {
		t.Fatalf("threads update --action star: %v", err)
	}
	payload := fake.addPayloads[0]
	if add, _ := payload["addLabelIds"].([]any); len(add) != 1 || add[0] != "STARRED" {
		t.Fatalf("star should addLabelIds=[STARRED], got %v", add)
	}
}

// TestThreadsUpdate_UnsupportedAction surfaces a clean usage error that
// names the supported verbs.
func TestThreadsUpdate_UnsupportedAction(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "threads", "update", "19abc", "--action", "encrypt")
	if err == nil {
		t.Fatalf("expected error for --action encrypt")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
	for _, verb := range []string{"archive", "read", "unread", "star", "unstar"} {
		if !strings.Contains(err.Error(), verb) {
			t.Fatalf("error %q should list %s as a supported verb", err.Error(), verb)
		}
	}
}

// TestThreadsUpdate_MissingAction surfaces a usage error.
func TestThreadsUpdate_MissingAction(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "threads", "update", "19abc")
	if err == nil {
		t.Fatalf("expected error for missing --action")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}
