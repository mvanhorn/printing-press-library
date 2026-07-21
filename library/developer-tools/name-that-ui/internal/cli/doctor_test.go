package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicNoAuthHelpAnd401Guidance(t *testing.T) {
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	if !strings.Contains(help, "verify connectivity and local state") || strings.Contains(help, "verify auth") {
		t.Fatalf("root help has stale auth claim: %q", help)
	}
	err := classifyAPIError(errors.New("GET / returned HTTP 401"), &rootFlags{})
	if strings.Contains(err.Error(), "credentials") || !strings.Contains(err.Error(), "public site") {
		t.Fatalf("401 guidance = %q", err)
	}
}

func TestDoctorDryRunDoesNotClaimAPIReachability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("doctor --dry-run must not call API")
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json", "--no-learn", "--dry-run", "doctor"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report["api"] != "not checked (dry run)" {
		t.Fatalf("api = %#v", report["api"])
	}
}

func TestDoctorHelpHasNoCredentialOrTokenGuidance(t *testing.T) {
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"doctor", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := strings.ToLower(out.String())
	for _, forbidden := range []string{"credential", "token verification", "auth warning"} {
		if strings.Contains(help, forbidden) {
			t.Fatalf("doctor help contains %q: %s", forbidden, out.String())
		}
	}
}

func TestPublicCommandTreeHidesImportAndInventedWriteCandidates(t *testing.T) {
	root := RootCmd()
	for _, command := range root.Commands() {
		if command.Name() == "import" {
			t.Fatal("public command tree still exposes generic import/create surface")
		}
	}
	encoded, err := json.Marshal(buildAgentContext(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"create_log", "create_rum", "create_search", "candidate_commands"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("agent context still advertises invented workflow %q: %s", forbidden, encoded)
		}
	}
}

func TestSyncHelpUsesOnlyActualPublicResources(t *testing.T) {
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"sync", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	if !strings.Contains(help, "catalog,styles") || strings.Contains(help, "catalog,styles,reference") || strings.Contains(help, "sync --since 7d") {
		t.Fatalf("sync examples are not public-resource truthful: %s", help)
	}
	if !strings.Contains(help, "has no effect for catalog or styles") {
		t.Fatalf("sync --since help is not explicit: %s", help)
	}
}

func TestSyncDryRunMachineStdoutRemainsStructured(t *testing.T) {
	stdout, stderr, err := runRootArgs(t, "--json", "--no-learn", "--dry-run", "sync", "--resources", "catalog,styles", "--db", filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "NameThatUI domain mirror preview") {
		t.Fatalf("dry-run prose leaked to stdout: %s", stdout)
	}
	if !strings.Contains(stderr, "NameThatUI domain mirror preview") {
		t.Fatalf("dry-run preview diagnostic missing from stderr: %s", stderr)
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("stdout line is not structured JSON %q: %v", line, err)
		}
	}
}
