// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// Scope resolution for Rightbrain's org/project-keyed API.
//
// Every Rightbrain resource path is /org/{org_id}/project/{project_id}/...,
// so the generated endpoint commands each take org_id and project_id as their
// first two positional arguments. Typing two UUIDs on every invocation is the
// single biggest ergonomic cost of this API, so this file resolves them once
// from (in priority order) the --org-id/--project-id flags, the RB_ORG_ID and
// RB_PROJECT_ID environment variables, or a saved scope file, and injects them
// into any scoped command invoked without them.
//
// Injection is additive: a command called with its full positional list is
// passed through untouched, so nothing that worked before this file existed
// behaves differently.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/config"
)

// scopeFlagOrgID and scopeFlagProjectID hold the --org-id / --project-id
// persistent flag values. Package-level because the generated rootFlags struct
// is a generated type this file must not modify.
var (
	scopeFlagOrgID     string
	scopeFlagProjectID string
)

// scopeWarnOut is where loadSavedScope reports a scope file it could not read.
// A package-level writer (rather than a cobra command's stderr) because scope
// resolution happens far below any command plumbing; tests swap it out.
var scopeWarnOut io.Writer = os.Stderr

// rightbrainScope is the persisted org/project selection.
type rightbrainScope struct {
	OrgID     string `json:"org_id"`
	ProjectID string `json:"project_id"`
}

// scopeFilePath returns the path of the saved scope file, which lives beside
// the resolved config file so --config and --home are both honored.
func scopeFilePath(flags *rootFlags) (string, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return "", err
	}
	if cfg.Path == "" {
		return "", fmt.Errorf("could not resolve a config directory for the scope file")
	}
	return filepath.Join(filepath.Dir(cfg.Path), "scope.json"), nil
}

// loadSavedScope reads the saved scope file. A missing file is not an error.
//
// A file that exists but does not parse is: swallowing that error leaves an
// empty scope behind, and every scoped command then reports "no org or project
// selected" while the file sits on disk, corrupt and unmentioned. The value
// still degrades to the zero scope — one truncated file must not make the CLI
// unusable — but the reason is named on stderr.
func loadSavedScope(flags *rootFlags) rightbrainScope {
	var sc rightbrainScope
	path, err := scopeFilePath(flags)
	if err != nil {
		return sc
	}
	data, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- app-derived config path: scope.json beside the resolved config file.
	if err != nil {
		return sc
	}
	if err := json.Unmarshal(data, &sc); err != nil {
		fmt.Fprintf(scopeWarnOut, "warning: ignoring unreadable scope file %s: %v\n", path, err)
		fmt.Fprintf(scopeWarnOut, "fix it by hand, or run: rightbrain-pp-cli scope use <org_id> <project_id>\n")
		return rightbrainScope{}
	}
	return sc
}

// saveScope persists the org/project selection with owner-only permissions.
func saveScope(flags *rootFlags, sc rightbrainScope) (string, error) {
	path, err := scopeFilePath(flags)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("writing scope file: %w", err)
	}
	return path, nil
}

// resolvedScope reports the effective org/project and where each value came
// from. Empty values mean unresolved; callers decide whether that is fatal.
func resolvedScope(flags *rootFlags) (orgID, projectID, orgSource, projectSource string) {
	saved := loadSavedScope(flags)

	switch {
	case scopeFlagOrgID != "":
		orgID, orgSource = scopeFlagOrgID, "flag:--org-id"
	case os.Getenv("RB_ORG_ID") != "":
		orgID, orgSource = os.Getenv("RB_ORG_ID"), "env:RB_ORG_ID"
	case saved.OrgID != "":
		orgID, orgSource = saved.OrgID, "config:scope.json"
	}

	switch {
	case scopeFlagProjectID != "":
		projectID, projectSource = scopeFlagProjectID, "flag:--project-id"
	case os.Getenv("RB_PROJECT_ID") != "":
		projectID, projectSource = os.Getenv("RB_PROJECT_ID"), "env:RB_PROJECT_ID"
	case saved.ProjectID != "":
		projectID, projectSource = saved.ProjectID, "config:scope.json"
	}
	return orgID, projectID, orgSource, projectSource
}

// requireScope returns the effective org and project, or a usage error naming
// every way the caller can supply the missing one. Hand-written commands in
// this package call it instead of taking org/project positionally.
func requireScope(flags *rootFlags) (orgID, projectID string, err error) {
	orgID, projectID, _, _ = resolvedScope(flags)
	var missing []string
	if orgID == "" {
		missing = append(missing, "org")
	}
	if projectID == "" {
		missing = append(missing, "project")
	}
	if len(missing) > 0 {
		return "", "", usageErr(fmt.Errorf(
			"no %s selected\nSet one with any of:\n  rightbrain-pp-cli scope use <org_id> <project_id>\n  export RB_ORG_ID=... RB_PROJECT_ID=...\n  --org-id <id> --project-id <id>",
			strings.Join(missing, " or ")))
	}
	return orgID, projectID, nil
}

// usePositionalNames extracts the positional argument names from a Cobra Use
// string, e.g. "run <org_id> <project_id> <task_id>" -> [org_id project_id
// task_id]. Both angle-bracket (required) and square-bracket (optional) forms
// are recognized; "[flags]" is skipped.
func usePositionalNames(use string) []string {
	var names []string
	for _, tok := range strings.Fields(use) {
		if len(tok) < 3 {
			continue
		}
		open, close := tok[0], tok[len(tok)-1]
		if !((open == '<' && close == '>') || (open == '[' && close == ']')) {
			continue
		}
		name := tok[1 : len(tok)-1]
		if name == "" || name == "flags" || strings.HasPrefix(name, "-") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// scopeLeadingCount reports how many leading positionals of a command are the
// org/project scope pair this file can supply: 2 for a project-scoped path, 1
// for an org-only path, 0 for anything else.
func scopeLeadingCount(names []string) int {
	if len(names) >= 2 && names[0] == "org_id" && names[1] == "project_id" {
		return 2
	}
	if len(names) >= 1 && names[0] == "org_id" {
		return 1
	}
	return 0
}

// injectScopeArgs prepends the resolved org/project when a scoped command was
// invoked without them. It is deliberately conservative: it acts only when the
// supplied argument count is exactly the command's arity minus the scope
// prefix, so a fully-specified invocation, or an ambiguous partial one, is
// always passed through unchanged.
func injectScopeArgs(cmd *cobra.Command, args []string, flags *rootFlags) []string {
	names := usePositionalNames(cmd.Use)
	need := scopeLeadingCount(names)
	if need == 0 {
		return args
	}
	if len(args) != len(names)-need {
		return args
	}
	orgID, projectID, _, _ := resolvedScope(flags)
	if orgID == "" {
		return args
	}
	if need == 2 {
		if projectID == "" {
			return args
		}
		return append([]string{orgID, projectID}, args...)
	}
	return append([]string{orgID}, args...)
}

// isScopedCommand reports whether a command targets an org/project-keyed API
// path and therefore participates in scope injection.
func isScopedCommand(cmd *cobra.Command) bool {
	if cmd.Annotations != nil {
		if p, ok := cmd.Annotations["pp:path"]; ok && strings.Contains(p, "{org_id}") {
			return true
		}
	}
	return scopeLeadingCount(usePositionalNames(cmd.Use)) > 0
}

const scopeLongNote = "\n\norg_id and project_id may be omitted when a scope is configured — " +
	"set one with 'rightbrain-pp-cli scope use <org_id> <project_id>', the RB_ORG_ID and " +
	"RB_PROJECT_ID environment variables, or the --org-id and --project-id flags."

// isScopeManagementCommand reports whether a command is `scope` itself or one
// of its descendants.
func isScopeManagementCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "scope" && c.Parent() != nil {
			return true
		}
	}
	return false
}

// applyScopeInjection walks the command tree and wraps every scoped command so
// a configured scope is supplied automatically.
//
// The `scope` subtree is excluded outright. Its `use` subcommand takes
// <org_id> <project_id> positionally, so it matches the injection heuristic
// exactly — and injecting there means `scope use` with no arguments quietly
// re-saves the scope it just resolved from the environment (pinning env values
// to disk, where they keep applying after the env vars are gone) instead of
// printing help. It also stops the "may be omitted, set one with 'scope use'"
// note from being appended to the help of the very command that sets it.
func applyScopeInjection(cmd *cobra.Command, flags *rootFlags) {
	if isScopeManagementCommand(cmd) {
		return
	}
	for _, sub := range cmd.Commands() {
		applyScopeInjection(sub, flags)
	}
	if cmd.RunE == nil || !isScopedCommand(cmd) {
		return
	}
	if scopeLeadingCount(usePositionalNames(cmd.Use)) == 0 {
		return
	}
	if !strings.Contains(cmd.Long, scopeLongNote) {
		if cmd.Long == "" {
			cmd.Long = cmd.Short + scopeLongNote
		} else {
			cmd.Long += scopeLongNote
		}
	}
	// With injection in place these two positionals really are optional, so the
	// usage line has to say so. Leaving them as <org_id> <project_id> makes
	// --help contradict the runtime, and makes every scope-resolved example in
	// the README and SKILL look like it is missing required arguments.
	cmd.Use = optionalizeScopePositionals(cmd.Use)
	orig := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		return orig(c, injectScopeArgs(c, args, flags))
	}
}

// optionalizeScopePositionals rewrites the leading <org_id> / <project_id>
// tokens of a Use string to their optional bracket form, leaving every other
// token untouched.
func optionalizeScopePositionals(use string) string {
	fields := strings.Fields(use)
	converted := 0
	for i, tok := range fields {
		if converted >= 2 {
			break
		}
		if tok == "<org_id>" || tok == "<project_id>" {
			fields[i] = "[" + tok[1:len(tok)-1] + "]"
			converted++
			continue
		}
		// Only the leading run of scope positionals is optional; stop at the
		// first token that is not one of them (past the command name itself).
		if i > 0 && strings.HasPrefix(tok, "<") {
			break
		}
	}
	return strings.Join(fields, " ")
}

func newScopeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "Show or set the default organization and project",
		Long: "Rightbrain keys every API path by organization and project. Setting a scope " +
			"once lets every command omit both UUIDs." + "\n\nResolution order: --org-id/--project-id flags, " +
			"then RB_ORG_ID/RB_PROJECT_ID, then the saved scope file.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        func(c *cobra.Command, args []string) error { return c.Help() },
	}
	cmd.AddCommand(newScopeShowCmd(flags), newScopeUseCmd(flags), newScopeClearCmd(flags))
	return cmd
}

func newScopeShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Short:       "Show the organization and project every command will use",
		Example:     "  rightbrain-pp-cli scope show --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			orgID, projectID, orgSrc, projSrc := resolvedScope(flags)
			view := map[string]any{
				"org_id":         orgID,
				"project_id":     projectID,
				"org_source":     orgSrc,
				"project_source": projSrc,
				"resolved":       orgID != "" && projectID != "",
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if orgID == "" && projectID == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "no scope configured")
					fmt.Fprintln(cmd.OutOrStdout(), "set one with: rightbrain-pp-cli scope use <org_id> <project_id>")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "org      %s\t(%s)\n", orDash(orgID), orDash(orgSrc))
				fmt.Fprintf(cmd.OutOrStdout(), "project  %s\t(%s)\n", orDash(projectID), orDash(projSrc))
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func newScopeUseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <org_id> <project_id>",
		Short: "Save the organization and project every command should default to",
		Long: "Save an organization and project so generated commands can be called without " +
			"repeating both UUIDs. Values are written to a scope file beside your config with " +
			"owner-only permissions.",
		Example:     "  rightbrain-pp-cli scope use 018936f0-03ea-e8be-a1a1-00000012b0a1 0195d1ff-1f05-437a-95ac-6de8969cb47b",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			// --dry-run is answered before the arity check so this command
			// behaves like every other one under it: a dry run never fails on
			// missing arguments, it describes the write it would have made.
			if dryRunOK(flags) {
				if len(args) < 2 {
					fmt.Fprintln(cmd.OutOrStdout(), "would set scope to the org and project given as arguments")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would set scope to org %s project %s\n", args[0], args[1])
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both <org_id> and <project_id> are required"))
			}
			sc := rightbrainScope{OrgID: args[0], ProjectID: args[1]}
			path, err := saveScope(flags, sc)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "scope set: org %s, project %s\nsaved to %s\n", sc.OrgID, sc.ProjectID, path)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"org_id": sc.OrgID, "project_id": sc.ProjectID, "saved_to": path,
			}, flags)
		},
	}
}

func newScopeClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "clear",
		Short:       "Forget the saved organization and project",
		Example:     "  rightbrain-pp-cli scope clear",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would clear the saved scope")
				return nil
			}
			path, err := scopeFilePath(flags)
			if err != nil {
				return err
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing scope file: %w", err)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "scope cleared")
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"cleared": true, "path": path}, flags)
		},
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.PersistentFlags().StringVar(&scopeFlagOrgID, "org-id",
			"", "Rightbrain organization ID (overrides RB_ORG_ID and the saved scope)")
		root.PersistentFlags().StringVar(&scopeFlagProjectID, "project-id",
			"", "Rightbrain project ID (overrides RB_PROJECT_ID and the saved scope)")
		root.AddCommand(newScopeCmd(flags))
		applyScopeInjection(root, flags)
	})
}
