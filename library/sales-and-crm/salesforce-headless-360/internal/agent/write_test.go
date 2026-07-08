package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

func TestComputeFieldDiffPreservesRequestedTypes(t *testing.T) {
	before := map[string]any{
		"Industry":          "Tech",
		"AnnualRevenue":     float64(5),
		"Active__c":         true,
		"Description":       "old",
		"NumberOfEmployees": float64(850),
	}
	requested := map[string]any{
		"Industry":          "Fintech",
		"AnnualRevenue":     float64(5),
		"Active__c":         false,
		"Description":       nil,
		"NumberOfEmployees": float64(900),
	}

	got := ComputeFieldDiff(before, requested)
	want := map[string]map[string]any{
		"Industry":          {"before": "Tech", "after": "Fintech"},
		"Active__c":         {"before": true, "after": false},
		"Description":       {"before": "old", "after": nil},
		"NumberOfEmployees": {"before": float64(850), "after": float64(900)},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diff mismatch:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestParseFieldAssignmentsCoercesJSONScalarSyntax(t *testing.T) {
	got, err := ParseFieldAssignments([]string{
		"Subject=Call",
		"Count__c=5",
		"Amount__c=10.25",
		"Active__c=false",
		"Done__c=true",
		"Description=null",
		"WhatId=001ACME0001",
	})
	if err != nil {
		t.Fatalf("ParseFieldAssignments error: %v", err)
	}

	want := map[string]any{
		"Subject":     "Call",
		"Count__c":    float64(5),
		"Amount__c":   float64(10.25),
		"Active__c":   false,
		"Done__c":     true,
		"Description": nil,
		"WhatId":      "001ACME0001",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields mismatch:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestParseFieldAssignmentsRejectsMalformedField(t *testing.T) {
	if _, err := ParseFieldAssignments([]string{"Subject"}); err == nil {
		t.Fatal("expected malformed field assignment to fail")
	}
}

func TestExecuteWriteUpdateFetchesSignsAuditsAndPatches(t *testing.T) {
	audit := &recordingWriteAudit{}
	fake := &recordingWriteClient{
		getBody:      json.RawMessage(`{"envelope":{"fields":{"Industry":{"value":"Tech"}},"id":"001ACME0001","lastModifiedDate":"2026-04-18T14:00:00.000Z"}}`),
		patchBody:    json.RawMessage(`{"envelope":{"id":"001ACME0001","success":true,"lastModifiedDate":"2026-04-22T18:42:00.000Z"}}`),
		patchStatus:  http.StatusOK,
		patchHeaders: http.Header{"LastModifiedDate": []string{"2026-04-22T18:42:00.000Z"}},
	}

	result, err := ExecuteWrite(contextWithTestDeadline(t), WriteOptions{
		Operation:   WriteOperationUpdate,
		SObject:     "Account",
		RecordID:    "001ACME0001",
		Fields:      map[string]any{"Industry": "Fintech"},
		Client:      fake,
		Filter:      allowAllWriteFilter{},
		Signer:      noopWriteSigner{},
		AuditWriter: audit,
		OrgAlias:    "prod",
		ActingUser:  "005USER",
		Now:         fixedWriteNow,
	})
	if err != nil {
		t.Fatalf("ExecuteWrite error: %v", err)
	}
	if result.Diff["Industry"]["before"] != "Tech" || result.Diff["Industry"]["after"] != "Fintech" {
		t.Fatalf("diff = %#v", result.Diff)
	}
	if fake.patchPath != "/services/data/v63.0/ui-api/records/001ACME0001" {
		t.Fatalf("patch path = %s", fake.patchPath)
	}
	if audit.pending != 1 || audit.executed != 1 {
		t.Fatalf("audit pending/executed = %d/%d", audit.pending, audit.executed)
	}
}

func TestExecuteWriteDryRunSkipsDMLAndAudit(t *testing.T) {
	audit := &recordingWriteAudit{}
	fake := &recordingWriteClient{
		getBody: json.RawMessage(`{"envelope":{"fields":{"Industry":{"value":"Tech"}},"id":"001ACME0001","lastModifiedDate":"2026-04-18T14:00:00.000Z"}}`),
	}

	result, err := ExecuteWrite(contextWithTestDeadline(t), WriteOptions{
		Operation:   WriteOperationUpdate,
		SObject:     "Account",
		RecordID:    "001ACME0001",
		Fields:      map[string]any{"Industry": "Fintech"},
		DryRun:      true,
		Client:      fake,
		Filter:      allowAllWriteFilter{},
		Signer:      noopWriteSigner{},
		AuditWriter: audit,
		OrgAlias:    "prod",
		ActingUser:  "005USER",
		Now:         fixedWriteNow,
	})
	if err != nil {
		t.Fatalf("ExecuteWrite error: %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected dry run result")
	}
	if fake.patchPath != "" {
		t.Fatalf("unexpected DML path: %s", fake.patchPath)
	}
	if audit.pending != 0 || audit.executed != 0 {
		t.Fatalf("dry-run wrote audit pending/executed = %d/%d", audit.pending, audit.executed)
	}
}

func TestExecuteWriteBulkGateStopsBeforeDMLAndAudit(t *testing.T) {
	audit := &recordingWriteAudit{}
	fake := &recordingWriteClient{}

	_, err := ExecuteWrite(contextWithTestDeadline(t), WriteOptions{
		Operation:      WriteOperationCreate,
		SObject:        "Account",
		Fields:         map[string]any{"Name": "Acme"},
		IdempotencyKey: "bulk-test",
		RecordCount:    10,
		Client:         fake,
		Filter:         allowAllWriteFilter{},
		Signer:         noopWriteSigner{},
		AuditWriter:    audit,
		OrgAlias:       "prod",
		ActingUser:     "005USER",
		Now:            fixedWriteNow,
	})
	var writeErr *WriteError
	if !errors.As(err, &writeErr) || writeErr.Envelope.Code != "BULK_OPERATIONS_DEFERRED" {
		t.Fatalf("error = %#v, want BULK_OPERATIONS_DEFERRED", err)
	}
	if fake.postPath != "" || fake.patchPath != "" {
		t.Fatalf("unexpected DML post=%s patch=%s", fake.postPath, fake.patchPath)
	}
	if audit.pending != 0 || audit.executed != 0 || audit.rejected != 0 || audit.conflict != 0 {
		t.Fatalf("bulk gate wrote audit pending/executed/rejected/conflict = %d/%d/%d/%d", audit.pending, audit.executed, audit.rejected, audit.conflict)
	}
}

func TestExecuteWriteUpsertNoContentIsNoChange(t *testing.T) {
	fake := &recordingWriteClient{patchStatus: http.StatusNoContent}
	result, err := ExecuteWrite(contextWithTestDeadline(t), WriteOptions{
		Operation:      WriteOperationUpsert,
		SObject:        "Task",
		IdempotencyKey: "abc",
		Fields:         map[string]any{"Subject": "Call"},
		Client:         fake,
		Filter:         allowAllWriteFilter{},
		Signer:         noopWriteSigner{},
		AuditWriter:    &recordingWriteAudit{},
		OrgAlias:       "prod",
		ActingUser:     "005USER",
		Now:            fixedWriteNow,
	})
	if err != nil {
		t.Fatalf("ExecuteWrite error: %v", err)
	}
	if !result.NoChange {
		t.Fatal("expected NoChange for 204 upsert")
	}
	if fake.patchPath != "/services/data/v63.0/sobjects/Task/SF360_Idempotency_Key__c/abc" {
		t.Fatalf("patch path = %s", fake.patchPath)
	}
}

func TestExecuteWriteJWTRequiresRunAsUser(t *testing.T) {
	_, err := ExecuteWrite(contextWithTestDeadline(t), WriteOptions{
		Operation:  WriteOperationUpdate,
		SObject:    "Account",
		RecordID:   "001ACME0001",
		Fields:     map[string]any{"Industry": "Fintech"},
		Client:     noopWriteClient{},
		Filter:     allowAllWriteFilter{},
		Signer:     noopWriteSigner{},
		AuthMethod: AuthMethodJWT,
	})
	var writeErr *WriteError
	if !errors.As(err, &writeErr) || writeErr.Envelope.Code != "JWT_REQUIRES_RUN_AS_USER" {
		t.Fatalf("error = %#v, want JWT_REQUIRES_RUN_AS_USER", err)
	}
}

type noopWriteClient struct{}

func (noopWriteClient) GetWithResponseHeaders(string, map[string]string) (json.RawMessage, http.Header, error) {
	return nil, nil, nil
}

func (noopWriteClient) Post(string, any) (json.RawMessage, int, error) {
	return nil, 0, nil
}

func (noopWriteClient) PatchWithResponseHeaders(string, any, map[string]string) (json.RawMessage, int, http.Header, error) {
	return nil, 0, nil, nil
}

type noopWriteSigner struct{}

func (noopWriteSigner) SignWriteIntent(trust.WriteIntentClaims) ([]byte, error) {
	return []byte("header.payload.signature"), nil
}

type allowAllWriteFilter struct{}

func (allowAllWriteFilter) AllowFieldWrite(string, string, string) bool { return true }

type recordingWriteClient struct {
	getBody      json.RawMessage
	patchBody    json.RawMessage
	patchStatus  int
	patchHeaders http.Header
	patchPath    string
	postPath     string
}

func (c *recordingWriteClient) GetWithResponseHeaders(string, map[string]string) (json.RawMessage, http.Header, error) {
	return c.getBody, nil, nil
}

func (c *recordingWriteClient) Post(path string, body any) (json.RawMessage, int, error) {
	c.postPath = path
	return json.RawMessage(`{"id":"001NEW","success":true}`), http.StatusCreated, nil
}

func (c *recordingWriteClient) PatchWithResponseHeaders(path string, body any, headers map[string]string) (json.RawMessage, int, http.Header, error) {
	c.patchPath = path
	status := c.patchStatus
	if status == 0 {
		status = http.StatusOK
	}
	return c.patchBody, status, c.patchHeaders, nil
}

type recordingWriteAudit struct {
	pending    int
	executed   int
	rejected   int
	conflict   int
	pendingJTI string
	fieldDiff  map[string]any
}

func (a *recordingWriteAudit) WritePending(intent trust.WriteIntentClaims, _ []byte, fieldDiff map[string]any) error {
	a.pending++
	a.pendingJTI = intent.Jti
	a.fieldDiff = fieldDiff
	return nil
}

func (a *recordingWriteAudit) UpdateExecuted(string, map[string]any) error {
	a.executed++
	return nil
}

func (a *recordingWriteAudit) UpdateRejected(string, string, string) error {
	a.rejected++
	return nil
}

func (a *recordingWriteAudit) UpdateConflict(string, time.Time, time.Time) error {
	a.conflict++
	return nil
}

func fixedWriteNow() time.Time {
	return time.Date(2026, 4, 22, 18, 0, 0, 0, time.UTC)
}

func contextWithTestDeadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}
