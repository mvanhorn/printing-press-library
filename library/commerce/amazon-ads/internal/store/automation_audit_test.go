package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestAppendAndListAutomationAudit(t *testing.T) {
	t.Parallel()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	payload := json.RawMessage(`{"plans":[{"action":"decrease"}]}`)
	audit, err := s.AppendAutomationAudit(context.Background(), "bid-rules apply", "dry_run", "kw.csv", 1, payload)
	if err != nil {
		t.Fatalf("AppendAutomationAudit returned error: %v", err)
	}
	if audit.ID == "" || audit.PlanCount != 1 {
		t.Fatalf("audit = %+v", audit)
	}
	got, err := s.ListAutomationAudits(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListAutomationAudits returned error: %v", err)
	}
	if len(got) != 1 || got[0].Command != "bid-rules apply" {
		t.Fatalf("got = %+v", got)
	}
}
