// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gmailAttachmentFixture builds the response body for
// users.messages.attachments.get with the supplied raw bytes.
func gmailAttachmentFixture(raw []byte) string {
	encoded := strings.TrimRight(base64.URLEncoding.EncodeToString(raw), "=")
	out, _ := json.Marshal(map[string]any{
		"size": len(raw),
		"data": encoded,
	})
	return string(out)
}

func newAttachmentFakeServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/attachments/") {
			_, _ = w.Write([]byte(body))
			return
		}
		http.Error(w, "wrong path", 404)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestMessagesGetAttachment_HappyPath_WritesFile rounds-trips a tiny payload
// to disk and asserts the on-disk bytes match.
func TestMessagesGetAttachment_HappyPath_WritesFile(t *testing.T) {
	want := []byte("hello U5 attachment bytes")
	srv := newAttachmentFakeServer(t, gmailAttachmentFixture(want))
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	outPath := filepath.Join(t.TempDir(), "logo.png")
	_, _, err := executeCmd(t, "--config", configPath, "messages", "get-attachment", "msg-1", "att-1", "--output", outPath)
	if err != nil {
		t.Fatalf("messages get-attachment: %v", err)
	}
	got, rerr := os.ReadFile(outPath)
	if rerr != nil {
		t.Fatalf("read output: %v", rerr)
	}
	if string(got) != string(want) {
		t.Fatalf("on-disk bytes = %q want %q", got, want)
	}
}

// TestMessagesGetAttachment_ExistingFileRequiresForce asserts the safety
// gate fires before any network call.
func TestMessagesGetAttachment_ExistingFileRequiresForce(t *testing.T) {
	srv := newAttachmentFakeServer(t, gmailAttachmentFixture([]byte("x")))
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	outPath := filepath.Join(t.TempDir(), "existing.bin")
	if werr := os.WriteFile(outPath, []byte("preexisting"), 0o600); werr != nil {
		t.Fatalf("seed pre-existing file: %v", werr)
	}
	_, _, err := executeCmd(t, "--config", configPath, "messages", "get-attachment", "msg-1", "att-1", "--output", outPath)
	if err == nil {
		t.Fatalf("expected error when output already exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error %q missing 'already exists'", err.Error())
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error %q missing --force hint", err.Error())
	}
	// File contents unchanged.
	got, _ := os.ReadFile(outPath)
	if string(got) != "preexisting" {
		t.Fatalf("pre-existing file should not be overwritten without --force, got %q", got)
	}
}

// TestMessagesGetAttachment_ForceOverwrites confirms --force lets the
// download proceed and clobber.
func TestMessagesGetAttachment_ForceOverwrites(t *testing.T) {
	want := []byte("force overwrite")
	srv := newAttachmentFakeServer(t, gmailAttachmentFixture(want))
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	outPath := filepath.Join(t.TempDir(), "existing.bin")
	if werr := os.WriteFile(outPath, []byte("preexisting"), 0o600); werr != nil {
		t.Fatalf("seed: %v", werr)
	}
	_, _, err := executeCmd(t, "--config", configPath, "messages", "get-attachment", "msg-1", "att-1", "--output", outPath, "--force")
	if err != nil {
		t.Fatalf("--force should overwrite: %v", err)
	}
	got, _ := os.ReadFile(outPath)
	if string(got) != string(want) {
		t.Fatalf("after --force: got %q want %q", got, want)
	}
}

// TestMessagesGetAttachment_RequiresOutput asserts the missing --output
// flag fires a usage error.
func TestMessagesGetAttachment_RequiresOutput(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "messages", "get-attachment", "msg-1", "att-1")
	if err == nil {
		t.Fatalf("expected usage error without --output")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}

// TestMessagesGetAttachment_MissingArgs surfaces a clean usage error.
func TestMessagesGetAttachment_MissingArgs(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "messages", "get-attachment", "only-one")
	if err == nil {
		t.Fatalf("expected usage error for single arg")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}
