// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// `skill install` — write the bundled Suno skill doc into the locations the
// major coding agents read, so Claude Code / Codex / Cursor learn to drive the
// CLI. Local only; no network, no auth. Writes files, so not read-only.

package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed skill_suno.md
var sunoSkillMarkdown string

// skillAgents maps an agent key to the default install path for its skill doc.
// Claude and Codex take the full SKILL.md verbatim; Cursor takes a .mdc rule
// (Cursor frontmatter + the SKILL body). Paths under the home dir are resolved
// at run time; the Cursor path is workspace-relative by design.
var skillAgents = []string{"claude", "codex", "cursor"}

// skillResult is one install outcome, surfaced in the --json envelope.
type skillResult struct {
	Agent   string `json:"agent"`
	Path    string `json:"path"`
	Written bool   `json:"written"`
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func newSkillInstallCmd(flags *rootFlags) *cobra.Command {
	var (
		agent     string
		printOnly bool
		pathOver  string
		force     bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Suno CLI as a skill for Claude Code, Codex, and Cursor",
		Long: "Write the bundled Suno skill doc into the locations the major coding " +
			"agents read so they learn to drive this CLI:\n" +
			"  claude  ~/.claude/skills/suno/SKILL.md\n" +
			"  codex   ~/.codex/skills/suno/SKILL.md\n" +
			"  cursor  ./.cursor/rules/suno.mdc (workspace-relative)\n\n" +
			"--agent selects one (claude|codex|cursor) or all (default). --print writes " +
			"to stdout instead of disk. --path overrides the target (single agent only). " +
			"--force overwrites an existing file.",
		Example: "  suno-pp-cli skill install\n" +
			"  suno-pp-cli skill install --agent claude\n" +
			"  suno-pp-cli skill install --agent cursor --print",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			agent = strings.ToLower(strings.TrimSpace(agent))
			agents, err := resolveSkillAgents(agent)
			if err != nil {
				return usageErr(err)
			}
			if pathOver != "" && len(agents) != 1 {
				return usageErr(fmt.Errorf("--path requires a single --agent (claude, codex, or cursor)"))
			}
			if dryRunOK(flags) {
				return nil
			}

			if printOnly {
				for i, a := range agents {
					if len(agents) > 1 {
						if i > 0 {
							fmt.Fprintln(cmd.OutOrStdout())
						}
						fmt.Fprintf(cmd.OutOrStdout(), "# === %s ===\n", a)
					}
					fmt.Fprint(cmd.OutOrStdout(), skillContentForAgent(a))
				}
				return nil
			}

			results := make([]skillResult, 0, len(agents))
			for _, a := range agents {
				dest := pathOver
				if dest == "" {
					dest, err = skillPathForAgent(a)
					if err != nil {
						return err
					}
				}
				res := skillResult{Agent: a, Path: dest}
				if _, statErr := os.Stat(dest); statErr == nil && !force {
					res.Skipped = true
					res.Reason = "exists (use --force to overwrite)"
					results = append(results, res)
					continue
				}
				if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
					return fmt.Errorf("creating %s: %w", filepath.Dir(dest), err)
				}
				if err := os.WriteFile(dest, []byte(skillContentForAgent(a)), 0o600); err != nil {
					return fmt.Errorf("writing %s: %w", dest, err)
				}
				res.Written = true
				results = append(results, res)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				if r.Written {
					fmt.Fprintf(cmd.OutOrStdout(), "wrote %s skill: %s\n", r.Agent, r.Path)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "skipped %s skill: %s — %s\n", r.Agent, r.Path, r.Reason)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "all", "Target agent: claude, codex, cursor, or all")
	cmd.Flags().BoolVar(&printOnly, "print", false, "Print the skill doc to stdout instead of writing files")
	cmd.Flags().StringVar(&pathOver, "path", "", "Override the install path (single --agent only)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing skill file")
	return cmd
}

// resolveSkillAgents expands the --agent value into the concrete agent list.
func resolveSkillAgents(agent string) ([]string, error) {
	if agent == "" || agent == "all" {
		out := append([]string{}, skillAgents...)
		sort.Strings(out)
		return out, nil
	}
	for _, a := range skillAgents {
		if a == agent {
			return []string{agent}, nil
		}
	}
	return nil, fmt.Errorf("invalid --agent %q: must be claude, codex, cursor, or all", agent)
}

// skillPathForAgent returns the default install path for an agent.
func skillPathForAgent(agent string) (string, error) {
	switch agent {
	case "cursor":
		// Workspace-relative Cursor rule.
		return filepath.Join(".cursor", "rules", "suno.mdc"), nil
	case "claude", "codex":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		dot := ".claude"
		if agent == "codex" {
			dot = ".codex"
		}
		return filepath.Join(home, dot, "skills", "suno", "SKILL.md"), nil
	}
	return "", fmt.Errorf("unknown agent %q", agent)
}

// skillContentForAgent returns the doc body to write for an agent. Claude and
// Codex take the SKILL.md verbatim; Cursor takes a .mdc rule (Cursor
// frontmatter wrapping the SKILL body).
func skillContentForAgent(agent string) string {
	if agent != "cursor" {
		return sunoSkillMarkdown
	}
	front, body := splitFrontmatter(sunoSkillMarkdown)
	desc := frontmatterValue(front, "description")
	if desc == "" {
		desc = "Suno CLI — generate, search, and manage AI music from the terminal."
	}
	return fmt.Sprintf("---\ndescription: %s\nalwaysApply: false\n---\n\n%s", desc, body)
}

// splitFrontmatter separates a leading `---`-delimited YAML frontmatter block
// from the markdown body. When no frontmatter is present the whole input is
// returned as the body.
func splitFrontmatter(md string) (front, body string) {
	if !strings.HasPrefix(md, "---\n") {
		return "", md
	}
	rest := md[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", md
	}
	front = rest[:end]
	body = strings.TrimLeft(rest[end+len("\n---\n"):], "\n")
	return front, body
}

// frontmatterValue extracts a top-level scalar value from a YAML frontmatter
// block by key. Returns "" when absent. Surrounding quotes are stripped.
func frontmatterValue(front, key string) string {
	for _, line := range strings.Split(front, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			v := strings.TrimSpace(trimmed[len(key)+1:])
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}
