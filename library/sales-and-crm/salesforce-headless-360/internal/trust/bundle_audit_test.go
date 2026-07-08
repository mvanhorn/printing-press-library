package trust

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
)

type fakeBundleAuditClient struct {
	status int
	err    error
	path   string
	body   map[string]any
}

func (f *fakeBundleAuditClient) Post(path string, body any) (json.RawMessage, int, error) {
	f.path = path
	f.body, _ = body.(map[string]any)
	if f.status == 0 {
		f.status = 201
	}
	if f.err != nil {
		return nil, f.status, f.err
	}
	return json.RawMessage(`{"id":"a00AUDIT","success":true,"errors":[]}`), f.status, nil
}

func TestRecordBundleAuditSyncPostsAndMirrorsOK(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	client := &fakeBundleAuditClient{}
	req := fixedBundleAuditRequest()

	if err := RecordBundleAudit(context.Background(), req, BundleAuditOptions{Client: client, DBPath: dbPath, Sync: true}); err != nil {
		t.Fatalf("RecordBundleAudit: %v", err)
	}
	if client.path != BundleAuditPath {
		t.Fatalf("path = %s, want %s", client.path, BundleAuditPath)
	}
	if client.body["BundleJti__c"] != "trace-123" {
		t.Fatalf("BundleJti__c = %#v", client.body["BundleJti__c"])
	}
	if client.body["HipaaMode__c"] != false {
		t.Fatalf("HipaaMode__c = %#v", client.body["HipaaMode__c"])
	}
	assertBundleAuditStatus(t, dbPath, "trace-123", "ok", "")
}

func TestRecordBundleAuditAsyncFailureMirrorsFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	client := &fakeBundleAuditClient{err: errors.New("remote denied")}
	warnings := make(chan string, 1)

	if err := RecordBundleAudit(context.Background(), fixedBundleAuditRequest(), BundleAuditOptions{
		Client: client,
		DBPath: dbPath,
		LogWarn: func(format string, args ...any) {
			warnings <- format
		},
	}); err != nil {
		t.Fatalf("RecordBundleAudit returned error for async path: %v", err)
	}

	select {
	case <-warnings:
	case <-time.After(2 * time.Second):
		status, remoteErr := bundleAuditStatus(t, dbPath, "trace-123")
		t.Fatalf("timed out waiting for async warning, last status=%q err=%q", status, remoteErr)
	}
	status, remoteErr := bundleAuditStatus(t, dbPath, "trace-123")
	if status != "failed" {
		t.Fatalf("write_status = %q, want failed (remote_error=%q)", status, remoteErr)
	}
	if !strings.Contains(remoteErr, "remote denied") {
		t.Fatalf("remote_error = %q", remoteErr)
	}
}

func TestRecordBundleAuditHIPAAFailureAborts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	req := fixedBundleAuditRequest()
	req.HIPAAMode = true

	err := RecordBundleAudit(context.Background(), req, BundleAuditOptions{
		Client: &fakeBundleAuditClient{err: errors.New("validation failed")},
		DBPath: dbPath,
	})
	if err == nil || !strings.Contains(err.Error(), "BUNDLE_AUDIT_WRITE_FAILED") {
		t.Fatalf("expected specific audit failure, got %v", err)
	}
	assertBundleAuditStatus(t, dbPath, "trace-123", "failed", "validation failed")
}

func fixedBundleAuditRequest() BundleAuditRequest {
	return BundleAuditRequest{
		KID:             "kid-123",
		GeneratedBy:     "005USER",
		GeneratedAt:     time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
		AccountID:       "001ACME",
		BundleJTI:       "trace-123",
		SourcesUsed:     []string{"local"},
		RedactionCounts: map[string]int{"HIPAA": 2},
		TraceID:         "trace-123",
	}
}

func assertBundleAuditStatus(t *testing.T, dbPath, jti, wantStatus, wantErrPart string) {
	t.Helper()
	status, remoteErr := bundleAuditStatus(t, dbPath, jti)
	if status != wantStatus {
		t.Fatalf("write_status = %q, want %q", status, wantStatus)
	}
	if wantErrPart != "" && !strings.Contains(remoteErr, wantErrPart) {
		t.Fatalf("remote_error = %q, want containing %q", remoteErr, wantErrPart)
	}
}

func bundleAuditStatus(t *testing.T, dbPath, jti string) (string, string) {
	t.Helper()
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	var status, remoteErr string
	if err := s.DB().QueryRow(`SELECT write_status, COALESCE(remote_error, '') FROM bundle_audit_local WHERE bundle_jti = ?`, jti).Scan(&status, &remoteErr); err != nil {
		return "", ""
	}
	return status, remoteErr
}
