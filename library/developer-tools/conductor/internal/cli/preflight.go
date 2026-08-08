// Copyright 2026 Cole Grolmus and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored ENG-549 launch preflight.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/conductor/internal/client"
	"github.com/spf13/cobra"
)

type preflightOptions struct {
	Issue        string
	ProjectID    string
	Repository   string
	RepoDir      string
	Branch       string
	Agent        string
	Model        string
	Effort       string
	RequireEnv   []string
	RequireTool  []string
	RequireVault []string
	RequireFile  []string
}

type preflightGate struct {
	Gate         string `json:"gate"`
	Status       string `json:"status"`
	Evidence     string `json:"evidence"`
	FailureClass string `json:"failure_class,omitempty"`
}

type preflightReceipt struct {
	SchemaVersion int             `json:"schema_version"`
	Action        string          `json:"action"`
	Outcome       string          `json:"outcome"`
	Issue         string          `json:"issue"`
	ProjectID     string          `json:"project_id"`
	Repository    string          `json:"repository"`
	Branch        string          `json:"branch"`
	Agent         string          `json:"agent"`
	Model         string          `json:"model,omitempty"`
	Effort        string          `json:"effort,omitempty"`
	Gates         []preflightGate `json:"gates"`
}

type preflightCommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
	LookPath(string) error
}

type osPreflightRunner struct{}

func (osPreflightRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (osPreflightRunner) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func newNovelPreflightCmd(flags *rootFlags) *cobra.Command {
	var opts preflightOptions
	cmd := &cobra.Command{
		Use:         "preflight",
		Short:       "Verify a Conductor launch without creating a workspace.",
		Long:        "Run deterministic CLI/auth, model-contract, Conductor project, GitHub repository, duplicate-work, environment, tool, vault, and repository-file checks. Secret values and command output are never returned. This command never creates a workspace.",
		Example:     "  conductor-pp-cli preflight --issue ENG-549 --project-id proj_123 --repository owner/repo --branch main --harness codex --model gpt-5.4 --require-vault 'Coding Agents' --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,1"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validatePreflightOptions(opts); err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{
					"dry_run": true, "action": "preflight", "issue": opts.Issue,
					"project_id": opts.ProjectID, "repository": opts.Repository,
					"branch": opts.Branch, "agent": opts.Agent, "model": opts.Model,
					"effort": opts.Effort, "require_env": opts.RequireEnv,
					"require_tool": opts.RequireTool, "require_vault": opts.RequireVault,
					"require_file": opts.RequireFile,
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			receipt := runConductorPreflight(cmd.Context(), c, opts, osPreflightRunner{})
			if err := flags.printJSON(cmd, receipt); err != nil {
				return err
			}
			if receipt.Outcome == "BLOCKED" {
				return errors.New("preflight blocked; inspect the receipt gates")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Issue, "issue", "", "Linear issue identifier used for duplicate detection")
	cmd.Flags().StringVar(&opts.ProjectID, "project-id", "", "Existing Conductor project id")
	cmd.Flags().StringVar(&opts.Repository, "repository", "", "GitHub repository in owner/name form")
	cmd.Flags().StringVar(&opts.RepoDir, "repo-dir", "", "Verified local checkout used only for --require-file checks")
	cmd.Flags().StringVar(&opts.Branch, "branch", "", "Authorized base branch")
	cmd.Flags().StringVar(&opts.Agent, "harness", "", "Agent harness: claude, codex, cursor, or acp")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Model id from the pinned Conductor contract")
	cmd.Flags().StringVar(&opts.Effort, "effort", "", "Effort from the pinned Conductor contract")
	cmd.Flags().StringSliceVar(&opts.RequireEnv, "require-env", nil, "Required environment variable name (repeatable; values are never read or printed)")
	cmd.Flags().StringSliceVar(&opts.RequireTool, "require-tool", nil, "Required executable name (repeatable)")
	cmd.Flags().StringSliceVar(&opts.RequireVault, "require-vault", nil, "Required 1Password vault name (repeatable; metadata access only)")
	cmd.Flags().StringSliceVar(&opts.RequireFile, "require-file", nil, "Required path relative to --repo-dir (repeatable; never executed)")
	return cmd
}

func validatePreflightOptions(opts preflightOptions) error {
	for name, value := range map[string]string{
		"--issue": opts.Issue, "--project-id": opts.ProjectID, "--repository": opts.Repository,
		"--branch": opts.Branch, "--harness": opts.Agent,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if strings.Count(opts.Repository, "/") != 1 {
		return errors.New("--repository must use owner/name form")
	}
	if err := validateAgentModelEffort(opts.Agent, opts.Model, opts.Effort); err != nil {
		return err
	}
	if len(opts.RequireFile) > 0 && strings.TrimSpace(opts.RepoDir) == "" {
		return errors.New("--repo-dir is required with --require-file")
	}
	for _, path := range opts.RequireFile {
		clean := filepath.Clean(path)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("--require-file %q must stay within --repo-dir", path)
		}
	}
	return nil
}

func runConductorPreflight(ctx context.Context, c *client.Client, opts preflightOptions, runner preflightCommandRunner) preflightReceipt {
	receipt := preflightReceipt{
		SchemaVersion: 1, Action: "preflight", Outcome: "PASS", Issue: opts.Issue,
		ProjectID: opts.ProjectID, Repository: opts.Repository, Branch: opts.Branch,
		Agent: opts.Agent, Model: opts.Model, Effort: opts.Effort, Gates: []preflightGate{},
	}
	add := func(gate, status, evidence, failureClass string) {
		receipt.Gates = append(receipt.Gates, preflightGate{Gate: gate, Status: status, Evidence: evidence, FailureClass: failureClass})
		if status == "BLOCKED" {
			receipt.Outcome = "BLOCKED"
		}
		if status == "RESUME" && receipt.Outcome != "BLOCKED" {
			receipt.Outcome = "RESUME"
		}
	}

	if _, err := c.Get(ctx, "/me", nil); err != nil {
		add("conductor_auth", "BLOCKED", "Conductor API identity check failed.", "auth")
	} else {
		add("conductor_auth", "PASS", "Conductor API identity check passed.", "")
	}

	projectPath := replacePathParam("/v0/projects/{projectId}", "projectId", opts.ProjectID)
	if _, err := c.Get(ctx, projectPath, nil); err != nil {
		add("project", "BLOCKED", "Conductor project is not readable.", "repository")
	} else {
		add("project", "PASS", "Conductor project is readable.", "")
	}

	if err := validateAgentModelEffort(opts.Agent, opts.Model, opts.Effort); err != nil {
		add("contract", "BLOCKED", "Pinned harness/model/effort contract rejected the launch shape.", "contract")
	} else {
		add("contract", "PASS", "Pinned harness/model/effort contract accepted the launch shape.", "")
	}

	githubOK := true
	for _, call := range [][]string{
		{"auth", "status"},
		{"repo", "view", opts.Repository, "--json", "nameWithOwner"},
		{"api", "repos/" + opts.Repository + "/branches/" + opts.Branch, "--silent"},
	} {
		if _, err := runner.Run(ctx, "gh", call...); err != nil {
			githubOK = false
			break
		}
	}
	if githubOK {
		add("github", "PASS", "GitHub auth, repository, and base branch are readable.", "")
	} else {
		add("github", "BLOCKED", "GitHub auth, repository, or base branch check failed.", "github")
	}

	missingEnv := []string{}
	for _, name := range opts.RequireEnv {
		if value, ok := os.LookupEnv(name); !ok || value == "" {
			missingEnv = append(missingEnv, name)
		}
	}
	if len(missingEnv) > 0 {
		add("environment", "BLOCKED", "Missing required environment names: "+strings.Join(missingEnv, ", ")+".", "auth")
	} else {
		add("environment", "PASS", "Required environment names are present; values were not read or printed.", "")
	}

	missingTools := []string{}
	for _, name := range opts.RequireTool {
		if err := runner.LookPath(name); err != nil {
			missingTools = append(missingTools, name)
		}
	}
	if len(missingTools) > 0 {
		add("tools", "BLOCKED", "Missing required tools: "+strings.Join(missingTools, ", ")+".", "bootstrap")
	} else {
		add("tools", "PASS", "Required executable names resolve.", "")
	}

	vaultsOK := true
	for _, name := range opts.RequireVault {
		if _, err := runner.Run(ctx, "op", "vault", "get", name, "--format", "json"); err != nil {
			vaultsOK = false
			break
		}
	}
	if vaultsOK {
		add("vaults", "PASS", "Required 1Password vault metadata is readable; no item or secret values were requested.", "")
	} else {
		add("vaults", "BLOCKED", "A required 1Password vault is not readable.", "auth")
	}

	missingFiles := []string{}
	for _, name := range opts.RequireFile {
		if _, err := os.Stat(filepath.Join(opts.RepoDir, filepath.Clean(name))); err != nil {
			missingFiles = append(missingFiles, name)
		}
	}
	if len(missingFiles) > 0 {
		add("bootstrap_files", "BLOCKED", "Missing required repository files: "+strings.Join(missingFiles, ", ")+".", "bootstrap")
	} else {
		add("bootstrap_files", "PASS", "Required repository files are present; none were executed.", "")
	}

	workspacesPath := replacePathParam("/v0/projects/{projectId}/workspaces", "projectId", opts.ProjectID)
	workspacesRaw, workspaceErr := c.Get(ctx, workspacesPath, map[string]string{"limit": "100"})
	prsRaw, prErr := runner.Run(ctx, "gh", "pr", "list", "--repo", opts.Repository, "--state", "all", "--search", opts.Issue, "--json", "number,title,headRefName,url")
	if workspaceErr != nil || prErr != nil {
		add("duplicate", "BLOCKED", "Could not prove the absence of canonical existing work.", "duplicate")
	} else if bytesContainFold(workspacesRaw, opts.Issue) || bytesContainFold(prsRaw, opts.Issue) {
		add("duplicate", "RESUME", "Canonical work matching the Linear issue already exists; reuse it.", "duplicate")
	} else {
		add("duplicate", "PASS", "No matching workspace or pull request was found.", "")
	}

	return receipt
}

func bytesContainFold(data []byte, needle string) bool {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return false
	}
	normalized, _ := json.Marshal(value)
	return strings.Contains(strings.ToLower(string(normalized)), strings.ToLower(needle))
}
