// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// queuePathForTest returns the queue file path the CLI would resolve when
// XDG_CONFIG_HOME points at the test temp dir. The send_queue.go path
// resolver uses XDG_CONFIG_HOME with the "superhuman-pp-cli" subdir.
func queuePathForTest(t *testing.T, xdgDir string) string {
	t.Helper()
	return filepath.Join(xdgDir, "superhuman-pp-cli", sendQueueFilename)
}

// TestSendQueue_PersistAndRestore covers the basic round-trip: write a queue
// file, read it back, assert the entry shapes survive serialization.
func TestSendQueue_PersistAndRestore(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	path, err := sendQueuePath()
	if err != nil {
		t.Fatalf("sendQueuePath: %v", err)
	}
	if want := queuePathForTest(t, xdg); path != want {
		t.Fatalf("queue path: want %s, got %s", want, path)
	}

	now := time.Now().UnixMilli()
	q := &sendQueueFile{
		Entries: []sendQueueEntry{
			{
				QueueID:       "queue-1",
				Account:       "user@example.com",
				FireAtEpochMs: now + 30000,
				EnqueuedAtMs:  now,
				Status:        queueStatusPending,
				OutgoingMessage: outgoingMessage{
					MessageID:           "draft0001",
					Subject:             "test",
					To:                  []addressObject{{Email: "a@x.com"}},
					Headers:             []recipientHeader{},
					Attachments:         []any{},
					MailMergeRecipients: []any{},
				},
			},
		},
	}
	if err := saveSendQueue(path, q); err != nil {
		t.Fatalf("saveSendQueue: %v", err)
	}
	loaded, err := loadSendQueue(path)
	if err != nil {
		t.Fatalf("loadSendQueue: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].QueueID != "queue-1" {
		t.Fatalf("queue id mismatch: %s", loaded.Entries[0].QueueID)
	}
	if loaded.Entries[0].Status != queueStatusPending {
		t.Fatalf("status: want pending, got %s", loaded.Entries[0].Status)
	}
}

// TestSendQueue_LoadMissingReturnsEmpty: missing queue file is not an error,
// so first-run enqueues can proceed without an explicit init step.
func TestSendQueue_LoadMissingReturnsEmpty(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	path, err := sendQueuePath()
	if err != nil {
		t.Fatalf("sendQueuePath: %v", err)
	}
	q, err := loadSendQueue(path)
	if err != nil {
		t.Fatalf("loadSendQueue empty: %v", err)
	}
	if q == nil || len(q.Entries) != 0 {
		t.Fatalf("missing file should yield empty queue, got %+v", q)
	}
}

// TestSendQueue_UpdateStatusTransitions verifies the terminal-status guard:
// fired/cancelled cannot be overwritten.
func TestSendQueue_UpdateStatusTransitions(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	path, err := sendQueuePath()
	if err != nil {
		t.Fatalf("sendQueuePath: %v", err)
	}
	now := time.Now().UnixMilli()
	q := &sendQueueFile{
		Entries: []sendQueueEntry{
			{QueueID: "q-pending", Status: queueStatusPending, EnqueuedAtMs: now},
			{QueueID: "q-fired", Status: queueStatusFired, EnqueuedAtMs: now},
			{QueueID: "q-cancelled", Status: queueStatusCancelled, EnqueuedAtMs: now},
		},
	}
	if err := saveSendQueue(path, q); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// pending -> cancelled OK
	if err := updateQueueStatus(path, "q-pending", queueStatusCancelled); err != nil {
		t.Fatalf("pending->cancelled: %v", err)
	}
	// fired -> cancelled: no-op (first-writer-wins)
	if err := updateQueueStatus(path, "q-fired", queueStatusCancelled); err != nil {
		t.Fatalf("fired->cancelled: %v", err)
	}
	// cancelled -> fired: no-op
	if err := updateQueueStatus(path, "q-cancelled", queueStatusFired); err != nil {
		t.Fatalf("cancelled->fired: %v", err)
	}

	loaded, err := loadSendQueue(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	statuses := map[string]string{}
	for _, e := range loaded.Entries {
		statuses[e.QueueID] = e.Status
	}
	if statuses["q-pending"] != queueStatusCancelled {
		t.Fatalf("q-pending: want cancelled, got %s", statuses["q-pending"])
	}
	if statuses["q-fired"] != queueStatusFired {
		t.Fatalf("q-fired must stay fired, got %s", statuses["q-fired"])
	}
	if statuses["q-cancelled"] != queueStatusCancelled {
		t.Fatalf("q-cancelled must stay cancelled, got %s", statuses["q-cancelled"])
	}
}

// TestSendQueue_UpdateStatusNotFound returns an error so the caller doesn't
// silently no-op on a typo'd queue id.
func TestSendQueue_UpdateStatusNotFound(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	path, err := sendQueuePath()
	if err != nil {
		t.Fatalf("sendQueuePath: %v", err)
	}
	if err := saveSendQueue(path, &sendQueueFile{Entries: []sendQueueEntry{}}); err != nil {
		t.Fatalf("seed empty: %v", err)
	}
	if err := updateQueueStatus(path, "missing-id", queueStatusCancelled); err == nil {
		t.Fatalf("expected not-found error")
	}
}

// TestUnsend_NoQueue returns not-found-coded error (exit 3).
func TestUnsend_NoQueue(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "unsend")
	if err == nil {
		t.Fatalf("expected error with empty queue")
	}
	if !strings.Contains(err.Error(), "no entries") && !strings.Contains(err.Error(), "no pending") {
		t.Fatalf("expected empty-queue error, got: %v", err)
	}
}

// TestUnsend_CancelsMostRecentPending: with no arg, picks the most-recent.
func TestUnsend_CancelsMostRecentPending(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	queuePath, err := sendQueuePath()
	if err != nil {
		t.Fatalf("sendQueuePath: %v", err)
	}
	now := time.Now().UnixMilli()
	q := &sendQueueFile{
		Entries: []sendQueueEntry{
			{QueueID: "q-older", Status: queueStatusPending, EnqueuedAtMs: now - 1000},
			{QueueID: "q-newer", Status: queueStatusPending, EnqueuedAtMs: now},
		},
	}
	if err := saveSendQueue(queuePath, q); err != nil {
		t.Fatalf("seed: %v", err)
	}
	configPath, _ := withConfigPath(t)
	stdout, _, err := executeCmd(t, "--config", configPath, "unsend")
	if err != nil {
		t.Fatalf("unsend: %v", err)
	}
	if !strings.Contains(stdout, "q-newer") {
		t.Fatalf("expected most-recent (q-newer) to be cancelled, got: %s", stdout)
	}
	loaded, _ := loadSendQueue(queuePath)
	for _, e := range loaded.Entries {
		switch e.QueueID {
		case "q-newer":
			if e.Status != queueStatusCancelled {
				t.Fatalf("q-newer: want cancelled, got %s", e.Status)
			}
		case "q-older":
			if e.Status != queueStatusPending {
				t.Fatalf("q-older must stay pending, got %s", e.Status)
			}
		}
	}
}

// TestUnsend_CancelsSpecificID: with a queue-id arg, cancels that one.
func TestUnsend_CancelsSpecificID(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	queuePath, err := sendQueuePath()
	if err != nil {
		t.Fatalf("sendQueuePath: %v", err)
	}
	now := time.Now().UnixMilli()
	if err := saveSendQueue(queuePath, &sendQueueFile{
		Entries: []sendQueueEntry{
			{QueueID: "q-a", Status: queueStatusPending, EnqueuedAtMs: now},
			{QueueID: "q-b", Status: queueStatusPending, EnqueuedAtMs: now + 1},
		},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	configPath, _ := withConfigPath(t)
	stdout, _, err := executeCmd(t, "--config", configPath, "unsend", "q-a")
	if err != nil {
		t.Fatalf("unsend q-a: %v", err)
	}
	if !strings.Contains(stdout, "q-a") {
		t.Fatalf("expected q-a in stdout, got: %s", stdout)
	}
	loaded, _ := loadSendQueue(queuePath)
	for _, e := range loaded.Entries {
		want := queueStatusPending
		if e.QueueID == "q-a" {
			want = queueStatusCancelled
		}
		if e.Status != want {
			t.Fatalf("%s: want %s, got %s", e.QueueID, want, e.Status)
		}
	}
}

// TestSend_UndoQueueGetsPopulated: send --undo 200ms enqueues an entry, the
// queue file is written and "Queued for send" prints to stderr. The actual
// delivery (Gmail API) will fail with a fake access token — same trade-off as
// TestSend_Pipeline_HappyPath — but the queue lifecycle is what this test
// asserts: a pending entry appears, the timer expires, the entry transitions
// to fired (even if the Gmail step then errors).
func TestSend_UndoQueueGetsPopulated(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	configPath := filepath.Join(xdg, "superhuman-pp-cli", "config.toml")
	tokenStorePath := filepath.Join(xdg, "superhuman-pp-cli", "tokens.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	seedSendStore(t, tokenStorePath, "user@example.com", "1234567890123456789")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	// undo 200ms — keeps the test fast.
	_, stderr, _ := executeCmd(t,
		"--config", configPath,
		"send",
		"--to", "alice@example.com",
		"--subject", "undo test",
		"--body", "hello",
		"--from", "user@example.com",
		"--undo", "200ms",
	)
	if !strings.Contains(stderr, "Queued for send") {
		t.Fatalf("expected 'Queued for send' in stderr, got: %s", stderr)
	}
	// The queue entry should have been written; even if the Gmail step then
	// fails, the entry should be marked fired (we transition status before
	// firing, see send_queue.go's `updateQueueStatus` call order).
	queuePath := filepath.Join(xdg, "superhuman-pp-cli", sendQueueFilename)
	q, lerr := loadSendQueue(queuePath)
	if lerr != nil {
		t.Fatalf("load queue: %v", lerr)
	}
	if len(q.Entries) < 1 {
		t.Fatalf("queue should have entry, got %d", len(q.Entries))
	}
	last := q.Entries[len(q.Entries)-1]
	if last.Status != queueStatusFired {
		t.Fatalf("last entry status: want fired, got %s", last.Status)
	}
}

// allow `net/http` and `httptest` imports to remain in use after the queue
// test relaxation above.
var _ = http.MethodGet
var _ = httptest.NewServer
