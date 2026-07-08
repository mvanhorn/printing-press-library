package trust

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"

	_ "modernc.org/sqlite"
)

type fakeWriteAuditClient struct {
	status int
	err    error
	path   string
	body   map[string]any
}

func (f *fakeWriteAuditClient) Post(path string, body any) (json.RawMessage, int, error) {
	f.path = path
	f.body, _ = body.(map[string]any)
	if f.status == 0 {
		f.status = 201
	}
	if f.err != nil {
		return nil, f.status, f.err
	}
	if f.status >= 400 {
		return json.RawMessage(`{"error":"mock failed"}`), f.status, nil
	}
	return json.RawMessage(`{"id":"a01WRITEAUDIT","success":true,"errors":[]}`), f.status, nil
}

func TestWriteAuditWriterWritePendingMirrorsPending(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	writer := newTestWriteAuditWriter(t, dbPath, &fakeWriteAuditClient{}, false)
	intent := fixedWriteIntentClaims()

	if err := writer.WritePending(intent, []byte("jws.payload.sig"), map[string]any{"Name": map[string]any{"after": "Acme"}}); err != nil {
		t.Fatalf("WritePending: %v", err)
	}

	row := getWriteAuditRow(t, dbPath, intent.Jti)
	if row.ExecutionStatus != "pending" {
		t.Fatalf("execution_status = %q, want pending", row.ExecutionStatus)
	}
	if !strings.Contains(row.FieldDiff, "Acme") {
		t.Fatalf("field_diff = %q", row.FieldDiff)
	}
}

func TestWriteAuditWriterUpdateExecuted(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	writer := newTestWriteAuditWriter(t, dbPath, &fakeWriteAuditClient{}, true)
	intent := fixedWriteIntentClaims()

	if err := writer.WritePending(intent, []byte("jws.payload.sig"), map[string]any{}); err != nil {
		t.Fatalf("WritePending: %v", err)
	}
	if err := writer.UpdateExecuted(intent.Jti, map[string]any{"Name": "Acme"}); err != nil {
		t.Fatalf("UpdateExecuted: %v", err)
	}

	row := getWriteAuditRow(t, dbPath, intent.Jti)
	if row.ExecutionStatus != "executed" {
		t.Fatalf("execution_status = %q, want executed", row.ExecutionStatus)
	}
	if row.ExecutedAt == "" {
		t.Fatal("expected executed_at to be populated")
	}
}

func TestWriteAuditWriterHIPAAFailureAborts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	writer := newTestWriteAuditWriter(t, dbPath, &fakeWriteAuditClient{status: 500}, true)

	err := writer.WritePending(fixedWriteIntentClaims(), []byte("jws.payload.sig"), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "WRITE_INTENT_AUDIT_FAILED") {
		t.Fatalf("expected sync audit failure, got %v", err)
	}

	row := getWriteAuditRow(t, dbPath, "01JWRITEINTENT")
	if row.WriteStatus != "failed" {
		t.Fatalf("write_status = %q, want failed", row.WriteStatus)
	}
	if !strings.Contains(row.RemoteError, "HTTP 500") {
		t.Fatalf("remote_error = %q", row.RemoteError)
	}
}

func TestWriteAuditWriterAsyncFailureMirrorsFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	warnings := make(chan string, 1)
	writer := NewWriteAuditWriter(WriteAuditOptions{
		Client:    &fakeWriteAuditClient{err: errors.New("remote denied")},
		DBPath:    dbPath,
		HIPAAMode: false,
		LogWarn: func(format string, args ...any) {
			warnings <- format
		},
	})

	if err := writer.WritePending(fixedWriteIntentClaims(), []byte("jws.payload.sig"), map[string]any{}); err != nil {
		t.Fatalf("WritePending returned error for async path: %v", err)
	}
	select {
	case <-warnings:
	case <-time.After(2 * time.Second):
		row := getWriteAuditRow(t, dbPath, "01JWRITEINTENT")
		t.Fatalf("timed out waiting for async warning, last status=%q err=%q", row.WriteStatus, row.RemoteError)
	}

	row := getWriteAuditRow(t, dbPath, "01JWRITEINTENT")
	if row.WriteStatus != "failed" {
		t.Fatalf("write_status = %q, want failed", row.WriteStatus)
	}
	if !strings.Contains(row.RemoteError, "remote denied") {
		t.Fatalf("remote_error = %q", row.RemoteError)
	}
}

func TestWriteAuditRoundTripSignPendingExecutedList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "data.db")
	signer := writeIntentTestSigner(t)
	intent := fixedWriteIntentClaims()
	jws, err := signer.SignWriteIntent(intent)
	if err != nil {
		t.Fatalf("SignWriteIntent: %v", err)
	}
	writer := newTestWriteAuditWriter(t, dbPath, &fakeWriteAuditClient{}, true)

	if err := writer.WritePending(intent, jws, map[string]any{"Name": map[string]any{"after": "Acme"}}); err != nil {
		t.Fatalf("WritePending: %v", err)
	}
	if err := writer.UpdateExecuted(intent.Jti, map[string]any{"Name": "Acme"}); err != nil {
		t.Fatalf("UpdateExecuted: %v", err)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	rows, err := s.ListWriteAudit(store.WriteAuditFilter{TargetSObject: "Account"})
	if err != nil {
		t.Fatalf("ListWriteAudit: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListWriteAudit returned %d rows, want 1", len(rows))
	}
	if rows[0].IntentJWS != string(jws) || rows[0].ExecutionStatus != "executed" {
		t.Fatalf("unexpected row: %#v", rows[0])
	}
}

func TestWriteAuditMigrationFromVersion4IsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(`PRAGMA user_version = 4`); err != nil {
		t.Fatalf("stamp v4: %v", err)
	}
	raw.Close()

	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open v4 db: %v", err)
	}
	s1.Close()
	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen migrated db: %v", err)
	}
	defer s2.Close()

	if err := s2.InsertWriteAudit(store.WriteAuditRow{JTI: "jti-1", ExecutionStatus: "pending"}); err != nil {
		t.Fatalf("InsertWriteAudit: %v", err)
	}
	row, err := s2.GetWriteAudit("jti-1")
	if err != nil {
		t.Fatalf("GetWriteAudit: %v", err)
	}
	if row.JTI != "jti-1" {
		t.Fatalf("row jti = %q", row.JTI)
	}
}

func newTestWriteAuditWriter(t *testing.T, dbPath string, client SFClient, hipaa bool) *WriteAuditWriter {
	t.Helper()
	w := NewWriteAuditWriter(WriteAuditOptions{
		Client:     client,
		DBPath:     dbPath,
		HIPAAMode:  hipaa,
		ClientHost: "test-host",
		Now: func() time.Time {
			return time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
		},
	})
	// Drain async goroutines before the test's TempDir cleanup runs.
	t.Cleanup(w.Wait)
	return w
}

func getWriteAuditRow(t *testing.T, dbPath, jti string) store.WriteAuditRow {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		s, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		row, getErr := s.GetWriteAudit(jti)
		s.Close()
		if getErr == nil {
			return row
		}
		if time.Now().After(deadline) {
			t.Fatalf("GetWriteAudit(%s): %v", jti, getErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
