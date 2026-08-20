// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// TestMessagesReadStatus_BackendCallShape verifies the request path uses
// users/<gid>/read_status/<thread-id> shape (the U8 path hypothesis).
func TestMessagesReadStatus_BackendCallShape(t *testing.T) {
	backend := newSuperhumanBackendFake(t, `[
		{"recipient":"alice@example.com","opened_at":1700000000,"device":"iOS"}
	]`)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, backend.srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "messages", "read-status", "19abc")
	if err != nil {
		t.Fatalf("messages read-status: %v", err)
	}
	if len(backend.requestPaths) != 1 {
		t.Fatalf("expected 1 backend call, got %d", len(backend.requestPaths))
	}
	want := "users/gid-001/read_status/19abc"
	if backend.requestPaths[0] != want {
		t.Fatalf("path = %q want %q", backend.requestPaths[0], want)
	}
}

// TestMessagesReadStatus_MissingArg surfaces a usage error.
func TestMessagesReadStatus_MissingArg(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "messages", "read-status")
	if err == nil {
		t.Fatalf("expected usage error without thread id")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}

// TestMessagesReadStatus_EmptyFeed_HumanFriendly asserts the no-opens
// case prints a friendly message.
func TestMessagesReadStatus_EmptyFeed_HumanFriendly(t *testing.T) {
	backend := newSuperhumanBackendFake(t, `[]`)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, backend.srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--plain", "messages", "read-status", "19abc")
	if err != nil {
		t.Fatalf("messages read-status (empty): %v", err)
	}
	if !strings.Contains(stdout, "No read events") {
		t.Fatalf("expected 'No read events', got: %s", stdout)
	}
}
