package cli

import (
	"encoding/json"
	"testing"
)

func TestSpendAndAuditApplySinceWindow(t *testing.T) {
	p := seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user": {"1": `{"user":{"id":10}}`},
		"get-expenses":     {"1": `{"id":1,"date":"2026-01-15","cost":"10.00","currency_code":"USD","users":[{"user_id":10,"owed_share":"10.00"}]}`, "2": `{"id":2,"date":"2026-08-15","cost":"20.00","currency_code":"USD","users":[{"user_id":10,"owed_share":"20.00"}]}`},
	})
	out, _, err := runRootArgs(t, "spend", "--group-by", "month", "--since", "2026-06-01", "--json", "--db", p)
	if err != nil {
		t.Fatal(err)
	}
	var spend []map[string]any
	if json.Unmarshal([]byte(out), &spend) != nil || len(spend) != 1 || spend[0]["bucket"] != "2026-08" {
		t.Fatalf("spend=%s", out)
	}
	out, _, err = runRootArgs(t, "audit", "--since", "2026-06-01", "--json", "--db", p)
	if err != nil {
		t.Fatal(err)
	}
	var audit auditResult
	if json.Unmarshal([]byte(out), &audit) != nil || audit.ScannedExpenses != 1 {
		t.Fatalf("audit=%s", out)
	}
	for _, cmd := range []string{"spend", "audit"} {
		_, _, err := runRootArgs(t, cmd, "--since", "nonsense", "--json", "--db", p)
		if ExitCode(err) != 2 {
			t.Errorf("%s exit=%d err=%v", cmd, ExitCode(err), err)
		}
	}
}
