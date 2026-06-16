package intelcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testApplyPolicy struct {
	ApplyPolicyBase
	AllowedAccounts []string `json:"allowed_accounts"`
}

type testReversal struct {
	ID        string         `json:"id"`
	AccountID string         `json:"account_id"`
	Target    map[string]any `json:"target"`
}

func TestLoadApplyPolicyDefaultsAndAllowlist(t *testing.T) {
	home := t.TempDir()
	defaults := testApplyPolicy{ApplyPolicyBase: ApplyPolicyBase{SchemaVersion: "test/v1", MaxChangesPerRun: 1}}

	missing, err := LoadApplyPolicy(filepath.Join(home, "missing.json"), defaults)
	if err != nil {
		t.Fatal(err)
	}
	if missing.SchemaVersion != "test/v1" || missing.MaxChangesPerRun != 1 {
		t.Fatalf("defaults not preserved for missing policy: %#v", missing)
	}

	path := filepath.Join(home, "policy.json")
	if err := os.WriteFile(path, []byte(`{"max_changes_per_run":3,"allowed_accounts":["acct-1"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadApplyPolicy(path, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MaxChangesPerRun != 3 || !AllowlistSet(loaded.AllowedAccounts, []string{"acct-2"})["acct-2"] {
		t.Fatalf("policy or flag allowlist not loaded: %#v", loaded)
	}
}

func TestApplyCoreLiveGates(t *testing.T) {
	if err := ValidateMaxChanges(0); err == nil || !strings.Contains(err.Error(), "max-changes-per-run") {
		t.Fatalf("expected cap validation refusal, got %v", err)
	}
	if err := EnforceChangeCap(2, 1); err == nil || !strings.Contains(err.Error(), "exceeds max-changes-per-run") {
		t.Fatalf("expected planned cap refusal, got %v", err)
	}
	if err := RequireTypedConfirm("apply", "", "APPLY TEST acct"); err == nil || !strings.Contains(err.Error(), `typed confirmation`) {
		t.Fatalf("expected typed confirmation refusal, got %v", err)
	}
	if err := RequireApplyConfidence(ConfidenceReport{Level: ConfidenceLow}); err == nil || !strings.Contains(err.Error(), "tracking confidence") {
		t.Fatalf("expected confidence refusal, got %v", err)
	}
	if err := RequireApplyConfidence(ConfidenceReport{Level: ConfidenceMedium}); err != nil {
		t.Fatalf("medium confidence should pass apply gate: %v", err)
	}
	if got := ApplyMode(false); got != ApplyModeDryRun {
		t.Fatalf("dry-run mode = %q", got)
	}
	if got := ApplyMode(true); got != ApplyModeLiveApproved {
		t.Fatalf("live-approved mode = %q", got)
	}
}

func TestApplyCoreSnapshotReversalAuditAndLock(t *testing.T) {
	home := t.TempDir()
	unlock, err := AcquireApplyLock(home, "test-platform", "acct/1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireApplyLock(home, "test-platform", "acct/1"); err == nil {
		t.Fatal("expected second lock acquisition to fail")
	}
	unlock()
	if _, err := AcquireApplyLock(home, "test-platform", "acct/1"); err != nil {
		t.Fatalf("expected lock to release, got %v", err)
	}

	snapshotPath, err := WriteApplySnapshot(ApplySnapshot{
		SchemaVersion: "test.snapshot/v1",
		Home:          home,
		Profile:       "demo",
		AccountID:     "acct/1",
		Target:        map[string]any{"id": "target-1"},
		StateKey:      "before",
		State:         []string{"old"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(b, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["schema_version"] != "test.snapshot/v1" || snapshot["before"] == nil {
		t.Fatalf("snapshot missing shared fields: %#v", snapshot)
	}

	rev := testReversal{ID: "rev-1", AccountID: "acct/1", Target: map[string]any{"id": "target-1"}}
	if err := AppendReversal(home, "demo", rev); err != nil {
		t.Fatal(err)
	}
	got, err := FindReversal[testReversal](home, "demo", "rev-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountID != "acct/1" {
		t.Fatalf("wrong reversal: %#v", got)
	}

	if err := AppendApplyAudit(home, ApplyAuditEntry{SchemaVersion: "test.audit/v1", Action: "apply", AccountID: "acct/1", Status: "applied"}); err != nil {
		t.Fatal(err)
	}
	auditPath := filepath.Join(home, "audit", "apply.log")
	auditBytes, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(auditBytes), `"schema_version":"test.audit/v1"`) {
		t.Fatalf("audit entry not written: %s", auditBytes)
	}
}
