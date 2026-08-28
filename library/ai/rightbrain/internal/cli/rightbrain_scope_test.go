// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// scopeTestConfig points the CLI at a config file inside a fresh temp
// directory, so the scope file this suite reads and writes never touches the
// real one. The relocation goes through RIGHTBRAIN_CONFIG rather than --config
// because several of these cases depend on the invocation carrying no flags at
// all: `scope use` prints help only when it was given neither arguments nor
// flags, and a --config on the command line would defeat that.
func scopeTestConfig(t *testing.T) (configPath, scopePath string) {
	t.Helper()
	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.toml")
	t.Setenv("RIGHTBRAIN_CONFIG", configPath)
	t.Setenv("RIGHTBRAIN_NO_LEARN", "1")
	return configPath, filepath.Join(dir, "scope.json")
}

// runScopeCmd drives the real command tree so command wiring — including the
// scope-injection wrapper applied at init — is part of what is under test.
func runScopeCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

// TestScopeUseWithNoArgsPrintsHelpAndWritesNothing is the injection-scope
// regression. `scope use <org_id> <project_id>` matches the injection heuristic
// exactly — leading positionals org_id and project_id — so the wrapper used to
// hand `scope use` the scope it had just resolved from the environment. Calling
// it with no arguments then silently pinned RB_ORG_ID/RB_PROJECT_ID to
// scope.json, where they kept applying long after the env vars were unset,
// instead of printing help.
func TestScopeUseWithNoArgsPrintsHelpAndWritesNothing(t *testing.T) {
	_, scopePath := scopeTestConfig(t)
	t.Setenv("RB_ORG_ID", "org-from-env")
	t.Setenv("RB_PROJECT_ID", "project-from-env")

	stdout, _, err := runScopeCmd(t, "scope", "use")
	if err != nil {
		t.Fatalf("scope use with no args returned %v, want nil (it must print help)", err)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Fatalf("scope use with no args printed no help:\n%s", stdout)
	}
	if _, statErr := os.Stat(scopePath); statErr == nil {
		data, _ := os.ReadFile(scopePath)
		t.Fatalf("scope use with no args wrote %s (%s); an env-derived scope must never be pinned to disk behind the user's back",
			scopePath, data)
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("stat %s: %v", scopePath, statErr)
	}
}

// TestScopeUseDryRunSucceedsWithoutArgs pins the shared dryRunOK ordering: a
// dry run is answered before argument validation, so `scope use --dry-run`
// exits 0 like every one of its siblings instead of being the only command in
// the tree that fails a dry run with exit 2.
func TestScopeUseDryRunSucceedsWithoutArgs(t *testing.T) {
	_, scopePath := scopeTestConfig(t)

	stdout, _, err := runScopeCmd(t, "scope", "use", "--dry-run")
	if err != nil {
		t.Fatalf("scope use --dry-run returned %v (exit %d), want nil", err, ExitCode(err))
	}
	if !strings.Contains(stdout, "would set scope") {
		t.Fatalf("scope use --dry-run printed %q, want a description of the write it would make", stdout)
	}
	if _, statErr := os.Stat(scopePath); !os.IsNotExist(statErr) {
		t.Fatalf("scope use --dry-run wrote %s; a dry run must not touch disk", scopePath)
	}

	// With arguments the dry run still echoes them, and still writes nothing.
	stdout, _, err = runScopeCmd(t, "scope", "use", "org-1", "project-1", "--dry-run")
	if err != nil {
		t.Fatalf("scope use org project --dry-run returned %v, want nil", err)
	}
	if !strings.Contains(stdout, "org-1") || !strings.Contains(stdout, "project-1") {
		t.Fatalf("scope use --dry-run printed %q, want both arguments echoed", stdout)
	}
	if _, statErr := os.Stat(scopePath); !os.IsNotExist(statErr) {
		t.Fatalf("scope use --dry-run wrote %s; a dry run must not touch disk", scopePath)
	}

	// Without --dry-run the arity check is still enforced, and it is still a
	// usage error rather than a write.
	_, _, err = runScopeCmd(t, "scope", "use", "org-only")
	if err == nil {
		t.Fatal("scope use with one argument returned nil, want a usage error")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("exit code = %d, want 2 for a usage error", ExitCode(err))
	}
}

// TestScopeUseWritesAndReadsBack keeps the happy path honest: the command that
// is excluded from injection must still do its actual job.
func TestScopeUseWritesAndReadsBack(t *testing.T) {
	configPath, scopePath := scopeTestConfig(t)
	t.Setenv("RB_ORG_ID", "")
	t.Setenv("RB_PROJECT_ID", "")

	if _, _, err := runScopeCmd(t, "scope", "use", "org-9", "project-9"); err != nil {
		t.Fatalf("scope use: %v", err)
	}
	if _, statErr := os.Stat(scopePath); statErr != nil {
		t.Fatalf("scope use did not write %s: %v", scopePath, statErr)
	}
	flags := &rootFlags{configPath: configPath}
	saved := loadSavedScope(flags)
	if saved.OrgID != "org-9" || saved.ProjectID != "project-9" {
		t.Fatalf("saved scope = %+v, want org-9/project-9", saved)
	}
}

// TestScopeHelpHasNoCircularNote covers the help text the injection wrapper
// appended: `scope use --help` used to tell the reader that org_id and
// project_id "may be omitted ... set one with 'rightbrain-pp-cli scope use'",
// pointing the command at itself.
func TestScopeHelpHasNoCircularNote(t *testing.T) {
	scopeTestConfig(t)
	for _, args := range [][]string{
		{"scope", "--help"},
		{"scope", "use", "--help"},
		{"scope", "show", "--help"},
		{"scope", "clear", "--help"},
	} {
		stdout, _, err := runScopeCmd(t, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if strings.Contains(stdout, "may be omitted") {
			t.Errorf("%v help carries the scope-injection note, which is circular on the command that sets the scope:\n%s", args, stdout)
		}
	}
}

// TestScopeInjectionStillAppliesElsewhere is the other half of the exclusion:
// skipping the `scope` subtree must not have switched injection off for the
// generated endpoint commands it exists for.
func TestScopeInjectionStillAppliesElsewhere(t *testing.T) {
	scopeTestConfig(t)
	root := RootCmd()

	// The appended long-help note is the observable side effect of the
	// injection wrapper, so it stands in for "this command was wrapped".
	noted, scoped, skippedInScope := 0, 0, 0
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
		if cmd.RunE == nil || scopeLeadingCount(usePositionalNames(cmd.Use)) == 0 {
			return
		}
		if isScopeManagementCommand(cmd) {
			skippedInScope++
			if strings.Contains(cmd.Long, scopeLongNote) {
				t.Errorf("%q is inside the scope subtree but was wrapped for injection", cmd.CommandPath())
			}
			return
		}
		scoped++
		if strings.Contains(cmd.Long, scopeLongNote) {
			noted++
		}
	}
	walk(root)

	if scoped == 0 {
		t.Fatal("no scoped commands found; the walk is not exercising the tree")
	}
	if noted != scoped {
		t.Fatalf("%d of %d scoped commands carry the injection note; injection must still apply outside the scope subtree", noted, scoped)
	}
	// `scope use <org_id> <project_id>` is the command the exclusion exists
	// for; if it stops matching the heuristic this test has gone quiet.
	if skippedInScope == 0 {
		t.Fatal("no scope-subtree command matched the injection heuristic; this test is no longer proving the exclusion")
	}
}

// TestIsScopeManagementCommand pins the predicate itself: `scope` and its
// descendants are excluded, and a command that merely mentions scope in its
// name is not.
func TestIsScopeManagementCommand(t *testing.T) {
	root := &cobra.Command{Use: "rightbrain-pp-cli"}
	scope := &cobra.Command{Use: "scope"}
	use := &cobra.Command{Use: "use <org_id> <project_id>"}
	scope.AddCommand(use)
	other := &cobra.Command{Use: "task <org_id> <project_id>"}
	root.AddCommand(scope, other)

	if isScopeManagementCommand(root) {
		t.Error("root reported as a scope management command")
	}
	if !isScopeManagementCommand(scope) {
		t.Error("scope not reported as a scope management command")
	}
	if !isScopeManagementCommand(use) {
		t.Error("scope use not reported as a scope management command")
	}
	if isScopeManagementCommand(other) {
		t.Error("an ordinary scoped command was reported as scope management; injection would be lost for it")
	}
}

// TestLoadSavedScopeWarnsOnCorruptFile covers the discarded unmarshal error. A
// truncated scope.json used to yield an empty scope in silence, so every scoped
// command reported "no org or project selected" while the broken file sat on
// disk unmentioned.
func TestLoadSavedScopeWarnsOnCorruptFile(t *testing.T) {
	configPath, scopePath := scopeTestConfig(t)
	if err := os.WriteFile(scopePath, []byte(`{"org_id": "org-1", "proj`), 0o600); err != nil {
		t.Fatalf("writing corrupt scope file: %v", err)
	}

	var warn bytes.Buffer
	prev := scopeWarnOut
	scopeWarnOut = &warn
	t.Cleanup(func() { scopeWarnOut = prev })

	flags := &rootFlags{configPath: configPath}
	got := loadSavedScope(flags)

	if got.OrgID != "" || got.ProjectID != "" {
		t.Errorf("scope = %+v, want the zero value: a half-parsed file must not partially apply", got)
	}
	if warn.Len() == 0 {
		t.Fatal("no warning emitted for an unreadable scope file; the failure is invisible to the user")
	}
	if !strings.Contains(warn.String(), scopePath) {
		t.Errorf("warning = %q, want it to name %s", warn.String(), scopePath)
	}
	if !strings.Contains(strings.ToLower(warn.String()), "scope") {
		t.Errorf("warning = %q, want it to say what could not be read", warn.String())
	}

	// A well-formed file is read without any warning at all.
	warn.Reset()
	if err := os.WriteFile(scopePath, []byte(`{"org_id":"org-2","project_id":"project-2"}`), 0o600); err != nil {
		t.Fatalf("writing scope file: %v", err)
	}
	good := loadSavedScope(flags)
	if good.OrgID != "org-2" || good.ProjectID != "project-2" {
		t.Fatalf("scope = %+v, want org-2/project-2", good)
	}
	if warn.Len() != 0 {
		t.Errorf("warning = %q, want none for a readable scope file", warn.String())
	}

	// A missing file is not an error either, and says nothing.
	warn.Reset()
	if err := os.Remove(scopePath); err != nil {
		t.Fatalf("removing scope file: %v", err)
	}
	if missing := loadSavedScope(flags); missing.OrgID != "" || missing.ProjectID != "" {
		t.Fatalf("scope = %+v, want the zero value when no file exists", missing)
	}
	if warn.Len() != 0 {
		t.Errorf("warning = %q, want none when the file simply does not exist", warn.String())
	}
}
