package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
)

func TestPlanningCommandsReplaceStalePlanWithEmptyPlan(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"dupes", []string{"dupes"}},
		{"conflicts", []string{"conflicts"}},
		{"mess", []string{"mess"}},
		{"organize", []string{"organize", "--match", "*.jpg", "--under", "/Photos", "--to", "/Sorted/{year}"}},
		{"links audit", []string{"links", "audit"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			planPath := filepath.Join(t.TempDir(), "plan.json")
			if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "old", Ops: []dropbox.Op{{Op: "delete", Path: "/old"}}}); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string{}, tc.args...), "--db", db, "--plan", planPath, "--json")
			var data []byte
			var err error
			if tc.name == "links audit" {
				data, err = runAuditLocal(t, "--db", db, "--plan", planPath)
			} else {
				data, err = runRead(t, args...)
			}
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				PlanPath    string `json:"plan_path"`
				PlanWritten bool   `json:"plan_written"`
				PlanOps     int    `json:"plan_ops"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if result.PlanPath != planPath || !result.PlanWritten || result.PlanOps != 0 {
				t.Fatalf("plan output=%s", data)
			}
			plan, err := dropbox.ReadPlan(planPath)
			if err != nil || len(plan.Ops) != 0 {
				t.Fatalf("empty plan=%+v err=%v", plan, err)
			}
		})
	}
}

func runAuditLocal(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := newNovelLinksAuditCmd(&rootFlags{asJSON: true, dataSource: "local"})
	cmd.SetArgs(args)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil && !json.Valid(out.Bytes()) {
		t.Fatalf("invalid audit JSON %q; stderr=%s", out.String(), stderr.String())
	}
	return out.Bytes(), err
}

func TestPlanOutputRefusesUnrelatedFileUnlessForced(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t)
	file := filepath.Join(t.TempDir(), "other.json")
	if err := os.WriteFile(file, []byte(`{"hello":"world"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runRead(t, "dupes", "--db", db, "--plan", file, "--json"); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("overwrite error=%v", err)
	}
	b, err := os.ReadFile(file)
	if err != nil || string(b) != `{"hello":"world"}` {
		t.Fatalf("file changed=%q err=%v", b, err)
	}
	if _, err := runRead(t, "dupes", "--db", db, "--plan", file, "--force", "--json"); err != nil {
		t.Fatal(err)
	}
	if p, err := dropbox.ReadPlan(file); err != nil || len(p.Ops) != 0 {
		t.Fatalf("forced plan=%+v err=%v", p, err)
	}
}

func TestPrintPlanAndEmptyApplyNoop(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t, fixtureRow("/A/a", "file", "H", "2020-01-01T00:00:00Z", 1), fixtureRow("/B/a", "file", "H", "2021-01-01T00:00:00Z", 1))
	data, err := runRead(t, "dupes", "--db", db, "--print-plan", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		PlanOps int          `json:"plan_ops"`
		Plan    dropbox.Plan `json:"plan"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.PlanOps != 1 || len(got.Plan.Ops) != 1 || got.Plan.Ops[0].Keeper != "/A/a" {
		t.Fatalf("printed plan=%s", data)
	}
	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := dropbox.WritePlan(empty, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "mess", Ops: []dropbox.Op{}}); err != nil {
		t.Fatal(err)
	}
	data, err = runRead(t, "apply", empty, "--yes", "--db", filepath.Join(t.TempDir(), "missing.db"), "--json")
	if err != nil || !strings.Contains(string(data), `"status": "noop"`) {
		t.Fatalf("empty apply=%s err=%v", data, err)
	}
}
