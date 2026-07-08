package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCloseCaseUpdatesStatusAndResolution(t *testing.T) {
	opts, err := NewCloseCaseWriteOptions(CloseCaseOptions{
		CaseID:     "500ACME0001",
		Resolution: "Resolved by phone",
	})
	if err != nil {
		t.Fatalf("NewCloseCaseWriteOptions error: %v", err)
	}
	fake := &highLevelWriteClient{
		getBody: json.RawMessage(`{"fields":{"Status":{"value":"Working"},"Resolution__c":{"value":null}},"id":"500ACME0001","lastModifiedDate":"2026-04-18T14:00:00Z"}`),
	}
	audit := &capturingWriteAudit{}
	opts.Client = fake
	opts.Filter = allowAllWriteFilter{}
	opts.Signer = noopWriteSigner{}
	opts.AuditWriter = audit
	opts.Now = fixedWriteNow

	result, err := ExecuteCloseCase(contextWithTestDeadline(t), opts)
	if err != nil {
		t.Fatalf("ExecuteCloseCase error: %v", err)
	}
	if result.RecordID != "500ACME0001" || result.SObject != "Case" {
		t.Fatalf("result target = %s/%s", result.SObject, result.RecordID)
	}
	body := fake.patchBody.(map[string]any)
	fields := body["fields"].(map[string]any)
	if fields["Status"] != "Closed" || fields["Resolution__c"] != "Resolved by phone" {
		t.Fatalf("patch fields = %#v", fields)
	}
	if audit.pending != 1 || audit.executed != 1 {
		t.Fatalf("audit pending/executed = %d/%d", audit.pending, audit.executed)
	}
}

func TestCloseCaseAlreadyClosedSkipsDMLAndAudit(t *testing.T) {
	opts, err := NewCloseCaseWriteOptions(CloseCaseOptions{
		CaseID:     "500CLOSED001",
		Resolution: "Resolved by phone",
	})
	if err != nil {
		t.Fatalf("NewCloseCaseWriteOptions error: %v", err)
	}
	fake := &highLevelWriteClient{
		getBody: json.RawMessage(`{"fields":{"Status":{"value":"Closed"}},"id":"500CLOSED001","lastModifiedDate":"2026-04-18T14:00:00Z"}`),
	}
	audit := &capturingWriteAudit{}
	var warnings []string
	opts.Client = fake
	opts.Filter = allowAllWriteFilter{}
	opts.Signer = noopWriteSigner{}
	opts.AuditWriter = audit
	opts.LogWarn = func(format string, args ...any) {
		warnings = append(warnings, strings.TrimSpace(format))
	}

	result, err := ExecuteCloseCase(contextWithTestDeadline(t), opts)
	if err != nil {
		t.Fatalf("ExecuteCloseCase error: %v", err)
	}
	if !result.NoChange {
		t.Fatalf("expected no-change result: %#v", result)
	}
	if fake.patchPath != "" {
		t.Fatalf("unexpected DML path: %s", fake.patchPath)
	}
	if audit.pending != 0 || audit.executed != 0 {
		t.Fatalf("audit pending/executed = %d/%d", audit.pending, audit.executed)
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "ALREADY_CLOSED") {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestCloseCaseRequiresFlags(t *testing.T) {
	_, err := NewCloseCaseWriteOptions(CloseCaseOptions{Resolution: "Resolved"})
	assertErrContains(t, err, "MISSING_REQUIRED_FLAG")

	_, err = NewCloseCaseWriteOptions(CloseCaseOptions{CaseID: "500ACME0001"})
	assertErrContains(t, err, "MISSING_REQUIRED_FLAG")
}
