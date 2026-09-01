package sequence

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWorkflowRepositoryLoadsResolvesAndRedacts(t *testing.T) {
	data := []byte(`{"workflows":[{"name":"demo","target_independent":true,"steps":[{"type":"text","value":"secret"},{"type":"assert_screen","contains":"password"}],"max_duration_ms":5000}]}`)
	repo, err := LoadWorkflowRepositoryBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	list := repo.List()
	if len(list) != 1 || list[0].Name != "demo" {
		t.Fatalf("list=%#v", list)
	}
	if list[0].Revision == "" {
		t.Fatal("missing revision")
	}
	plan, err := repo.Resolve("demo", list[0].Revision, "host-a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Target != "host-a" || plan.MaxDuration.Milliseconds() != 5000 {
		t.Fatalf("plan=%#v", plan)
	}
	inspected, err := repo.Inspect("demo", list[0].Revision, "")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(inspected, []byte("secret")) || bytes.Contains(inspected, []byte("password")) {
		t.Fatalf("inspection leaked secret: %s", inspected)
	}
}

func TestWorkflowRepositoryRejectsRevisionAndTargetErrors(t *testing.T) {
	repo, err := LoadWorkflowRepositoryBytes([]byte(`[{"name":"demo","target":"host-a","steps":[{"type":"key","value":"Enter"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	d := repo.List()[0]
	for _, tc := range []struct {
		name, revision, target string
	}{
		{"demo", "sha256:bad", "host-a"},
		{"demo", d.Revision, "host-b"},
	} {
		if _, err := repo.Resolve(tc.name, tc.revision, tc.target); err == nil {
			t.Fatalf("Resolve(%q, %q, %q) unexpectedly succeeded", tc.name, tc.revision, tc.target)
		}
	}
	if _, err := repo.Resolve("demo", d.Revision, ""); err == nil {
		t.Fatal("target-independent missing target unexpectedly succeeded")
	}
}

func TestWorkflowRepositoryRejectsSuppliedRevisionMismatch(t *testing.T) {
	_, err := LoadWorkflowRepositoryBytes([]byte(`[{"name":"demo","target":"host-a","revision":"sha256:bad","steps":[{"type":"release_all"}]}]`))
	if err == nil {
		t.Fatal("revision spoof unexpectedly accepted")
	}
}

func TestWorkflowRepositoryInspectionIsJSON(t *testing.T) {
	repo, err := LoadWorkflowRepositoryBytes([]byte(`[{"name":"demo","target":"host-a","steps":[{"type":"release_all"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := repo.Inspect("demo", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("invalid inspection JSON: %v", err)
	}
}
