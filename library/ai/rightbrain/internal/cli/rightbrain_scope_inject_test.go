// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// Coverage for the argument injection itself.
//
// injectScopeArgs is the load-bearing part of scope resolution: it is what lets
// roughly 290 generated endpoint commands be invoked without pasting two UUIDs,
// and applyScopeInjection wraps every one of their RunE functions with it. Its
// helpers were already well covered individually — usePositionalNames,
// scopeLeadingCount, optionalizeScopePositionals — but nothing executed the
// composition, so the behaviour users actually get was resting on the live
// matrix alone. These cases pin it down offline.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// scopeInjectEnv points config at a throwaway directory and sets an env scope,
// so injection has something to resolve without touching the real scope file.
func scopeInjectEnv(t *testing.T, orgID, projectID string) {
	t.Helper()
	_, _ = scopeTestConfig(t)
	if orgID != "" {
		t.Setenv("RB_ORG_ID", orgID)
	} else {
		t.Setenv("RB_ORG_ID", "")
	}
	if projectID != "" {
		t.Setenv("RB_PROJECT_ID", projectID)
	} else {
		t.Setenv("RB_PROJECT_ID", "")
	}
}

func TestInjectScopeArgs(t *testing.T) {
	cases := []struct {
		name    string
		use     string
		org     string
		project string
		args    []string
		want    []string
		wantWhy string
	}{
		{
			name: "project-scoped command gains both ids",
			use:  "get <org_id> <project_id> <task_id>",
			org:  "org-1", project: "proj-1",
			args: []string{"task-9"},
			want: []string{"org-1", "proj-1", "task-9"},
		},
		{
			name: "org-only command gains one id",
			use:  "get <org_id> <member_id>",
			org:  "org-1", project: "proj-1",
			args: []string{"member-3"},
			want: []string{"org-1", "member-3"},
		},
		{
			name: "no positionals beyond scope",
			use:  "list <org_id> <project_id>",
			org:  "org-1", project: "proj-1",
			args: []string{},
			want: []string{"org-1", "proj-1"},
		},
		{
			name: "fully specified invocation is untouched",
			use:  "get <org_id> <project_id> <task_id>",
			org:  "org-1", project: "proj-1",
			args:    []string{"explicit-org", "explicit-proj", "task-9"},
			want:    []string{"explicit-org", "explicit-proj", "task-9"},
			wantWhy: "explicit arguments must always win over a configured scope",
		},
		{
			name: "deeper command gains both ids",
			use:  "get <org_id> <project_id> <task_id> <run_id>",
			org:  "org-1", project: "proj-1",
			args:    []string{"task-9", "run-3"},
			want:    []string{"org-1", "proj-1", "task-9", "run-3"},
			wantWhy: "supplying exactly the non-scope positionals is the case injection exists for",
		},
		{
			name: "ambiguous partial invocation is untouched",
			use:  "get <org_id> <project_id> <task_id> <run_id>",
			org:  "org-1", project: "proj-1",
			args:    []string{"a", "b", "c"},
			want:    []string{"a", "b", "c"},
			wantWhy: "three of four positionals does not say which one was omitted, so guessing would corrupt the call",
		},
		{
			name: "unscoped command is untouched",
			use:  "whoami",
			org:  "org-1", project: "proj-1",
			args: []string{"x"},
			want: []string{"x"},
		},
		{
			name: "no scope configured leaves args alone",
			use:  "get <org_id> <project_id> <task_id>",
			org:  "", project: "",
			args:    []string{"task-9"},
			want:    []string{"task-9"},
			wantWhy: "the command must report the missing scope itself rather than receive a blank id",
		},
		{
			name: "org without project does not half-inject",
			use:  "get <org_id> <project_id> <task_id>",
			org:  "org-1", project: "",
			args:    []string{"task-9"},
			want:    []string{"task-9"},
			wantWhy: "injecting only the org would shift task-9 into the project position",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scopeInjectEnv(t, tc.org, tc.project)
			cmd := &cobra.Command{Use: tc.use}
			got := injectScopeArgs(cmd, tc.args, &rootFlags{})
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				msg := ""
				if tc.wantWhy != "" {
					msg = " — " + tc.wantWhy
				}
				t.Errorf("injectScopeArgs(%q, %v) = %v, want %v%s", tc.use, tc.args, got, tc.want, msg)
			}
		})
	}
}

// TestApplyScopeInjectionWrapsRunE covers the composition rather than the
// helper: applyScopeInjection has to actually replace RunE, and the replacement
// has to pass the widened argument list through to the original.
func TestApplyScopeInjectionWrapsRunE(t *testing.T) {
	scopeInjectEnv(t, "org-77", "proj-88")

	var seen []string
	leaf := &cobra.Command{
		Use:         "show <org_id> <project_id> <task_id>",
		Short:       "demo",
		Annotations: map[string]string{"pp:path": "/org/{org_id}/project/{project_id}/task/{task_id}"},
		RunE: func(c *cobra.Command, args []string) error {
			seen = args
			return nil
		},
	}
	root := &cobra.Command{Use: "root"}
	root.AddCommand(leaf)

	applyScopeInjection(root, &rootFlags{})

	if err := leaf.RunE(leaf, []string{"task-5"}); err != nil {
		t.Fatalf("wrapped RunE returned %v, want nil", err)
	}
	want := []string{"org-77", "proj-88", "task-5"}
	if strings.Join(seen, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("wrapped RunE received %v, want %v", seen, want)
	}

	// The usage line has to agree with the runtime, or --help contradicts the
	// behaviour and every scope-resolved example reads as missing arguments.
	if !strings.Contains(leaf.Use, "[org_id]") || !strings.Contains(leaf.Use, "[project_id]") {
		t.Errorf("Use = %q, want the scope positionals rewritten to optional form", leaf.Use)
	}
	if !strings.Contains(leaf.Long, "may be omitted") {
		t.Errorf("Long = %q, want the scope note appended", leaf.Long)
	}
}

// TestApplyScopeInjectionIsIdempotent guards the wrapper against being applied
// twice, which would otherwise inject a second copy of the scope.
func TestApplyScopeInjectionIsIdempotent(t *testing.T) {
	scopeInjectEnv(t, "org-77", "proj-88")

	var seen []string
	leaf := &cobra.Command{
		Use:   "show <org_id> <project_id> <task_id>",
		Short: "demo",
		RunE: func(c *cobra.Command, args []string) error {
			seen = args
			return nil
		},
	}
	root := &cobra.Command{Use: "root"}
	root.AddCommand(leaf)

	applyScopeInjection(root, &rootFlags{})
	applyScopeInjection(root, &rootFlags{})

	if err := leaf.RunE(leaf, []string{"task-5"}); err != nil {
		t.Fatalf("wrapped RunE returned %v, want nil", err)
	}
	want := []string{"org-77", "proj-88", "task-5"}
	if strings.Join(seen, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("after a double apply, RunE received %v, want %v (the scope must not be injected twice)", seen, want)
	}
}

func TestRequireScopeNamesWhatIsMissing(t *testing.T) {
	t.Run("resolved", func(t *testing.T) {
		scopeInjectEnv(t, "org-1", "proj-1")
		org, proj, err := requireScope(&rootFlags{})
		if err != nil {
			t.Fatalf("requireScope returned %v, want nil", err)
		}
		if org != "org-1" || proj != "proj-1" {
			t.Errorf("requireScope = (%q, %q), want (org-1, proj-1)", org, proj)
		}
	})

	for _, tc := range []struct {
		name         string
		org, project string
		wantSub      string
	}{
		{"neither", "", "", "org or project"},
		{"project only", "", "proj-1", "no org selected"},
		{"org only", "org-1", "", "no project selected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scopeInjectEnv(t, tc.org, tc.project)
			_, _, err := requireScope(&rootFlags{})
			if err == nil {
				t.Fatal("requireScope returned nil, want a usage error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestResolvedScopePrecedence pins the documented order: flag, then env, then
// the saved file. The flag variables are package-level (the generated rootFlags
// type cannot be extended), so this also documents that they win.
func TestResolvedScopePrecedence(t *testing.T) {
	scopeInjectEnv(t, "org-from-env", "proj-from-env")

	org, proj, orgSrc, projSrc := resolvedScope(&rootFlags{})
	if org != "org-from-env" || proj != "proj-from-env" {
		t.Fatalf("resolvedScope = (%q, %q), want the env values", org, proj)
	}
	if orgSrc != "env:RB_ORG_ID" || projSrc != "env:RB_PROJECT_ID" {
		t.Errorf("sources = (%q, %q), want env sources", orgSrc, projSrc)
	}

	prevOrg, prevProject := scopeFlagOrgID, scopeFlagProjectID
	t.Cleanup(func() { scopeFlagOrgID, scopeFlagProjectID = prevOrg, prevProject })
	scopeFlagOrgID, scopeFlagProjectID = "org-from-flag", "proj-from-flag"

	org, proj, orgSrc, projSrc = resolvedScope(&rootFlags{})
	if org != "org-from-flag" || proj != "proj-from-flag" {
		t.Errorf("resolvedScope = (%q, %q), want the flag values to override the env", org, proj)
	}
	if orgSrc != "flag:--org-id" || projSrc != "flag:--project-id" {
		t.Errorf("sources = (%q, %q), want flag sources", orgSrc, projSrc)
	}
}
