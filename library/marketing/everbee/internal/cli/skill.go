// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	everbee "github.com/mvanhorn/printing-press-library/library/marketing/everbee"

	"github.com/spf13/cobra"
)

const defaultSkillName = "pp-everbee"

type skillInstallResult struct {
	Target string `json:"target"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

func newSkillCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the EverBee agent skill",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSkillInstallCmd(flags))
	return cmd
}

func newSkillInstallCmd(flags *rootFlags) *cobra.Command {
	var targets []string
	var name string
	var source string
	var force bool
	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install the EverBee skill for Claude and Codex",
		Example: "  everbee-pp-cli skill install\n  everbee-pp-cli skill install --target claude --target codex\n  everbee-pp-cli skill install --source ./SKILL.md --force=false",
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := skillContent(source)
			if err != nil {
				return configErr(err)
			}
			dests, err := skillDestinations(targets, name)
			if err != nil {
				return usageErr(err)
			}
			results, err := installSkillFiles(content, dests, force, flags.dryRun)
			if err != nil {
				return configErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"installed": results,
					"restart":   "Restart Claude/Codex or start a new session to load newly installed skills.",
				}, flags)
			}
			w := cmd.OutOrStdout()
			for _, result := range results {
				fmt.Fprintf(w, "%s: %s %s\n", result.Target, result.Status, result.Path)
			}
			fmt.Fprintln(w, "Restart Claude/Codex or start a new session to load newly installed skills.")
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&targets, "target", []string{"all"}, "Skill target: all, claude, codex (repeatable)")
	cmd.Flags().StringVar(&name, "name", defaultSkillName, "Skill directory name")
	cmd.Flags().StringVar(&source, "source", "", "Optional SKILL.md source path; defaults to bundled skill")
	cmd.Flags().BoolVar(&force, "force", true, "Overwrite existing installed SKILL.md")
	return cmd
}

func skillContent(source string) (string, error) {
	if source == "" {
		return everbee.SkillMD, nil
	}
	cleanSource := filepath.Clean(source)
	if filepath.Base(cleanSource) != "SKILL.md" {
		return "", fmt.Errorf("skill source must point to a SKILL.md file")
	}
	data, err := os.ReadFile(cleanSource) // #nosec G304 -- --source is an explicit local SKILL.md path provided by the operator.
	if err != nil {
		return "", fmt.Errorf("reading skill source: %w", err)
	}
	return string(data), nil
}

func skillDestinations(targets []string, name string) (map[string]string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("invalid skill name %q", name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home dir: %w", err)
	}
	expanded := map[string]bool{}
	for _, target := range targets {
		target = strings.ToLower(strings.TrimSpace(target))
		if target == "" {
			continue
		}
		if target == "all" {
			expanded["claude"] = true
			expanded["codex"] = true
			continue
		}
		switch target {
		case "claude", "codex":
			expanded[target] = true
		default:
			return nil, fmt.Errorf("unknown skill target %q; valid: all, claude, codex", target)
		}
	}
	if len(expanded) == 0 {
		return nil, fmt.Errorf("no skill targets selected")
	}
	dests := map[string]string{}
	if expanded["claude"] {
		dests["claude"] = filepath.Join(home, ".claude", "skills", name, "SKILL.md")
	}
	if expanded["codex"] {
		dests["codex"] = filepath.Join(home, ".codex", "skills", name, "SKILL.md")
	}
	return dests, nil
}

func installSkillFiles(content string, dests map[string]string, force, dryRun bool) ([]skillInstallResult, error) {
	targets := make([]string, 0, len(dests))
	for target := range dests {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	results := make([]skillInstallResult, 0, len(targets))
	for _, target := range targets {
		path := dests[target]
		if !force {
			if _, err := os.Stat(path); err == nil {
				return nil, fmt.Errorf("%s already exists; pass --force to overwrite", path)
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("checking existing skill: %w", err)
			}
		}
		status := "installed"
		if dryRun {
			status = "would-install"
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return nil, fmt.Errorf("creating skill directory: %w", err)
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				return nil, fmt.Errorf("writing skill: %w", err)
			}
		}
		results = append(results, skillInstallResult{Target: target, Path: path, Status: status})
	}
	return results, nil
}
