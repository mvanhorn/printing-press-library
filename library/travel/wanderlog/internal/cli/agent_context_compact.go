// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

type agentContextSummary struct {
	agentContext
	View   string `json:"view"`
	Detail string `json:"detail"`
}

func compactAgentContext(root *cobra.Command) agentContextSummary {
	base := buildAgentContext(root)
	base.SchemaVersion = "4"
	var slim func([]agentContextCommand)
	slim = func(commands []agentContextCommand) {
		for i := range commands {
			commands[i].Flags = nil
			slim(commands[i].Subcommands)
		}
	}
	slim(base.Commands)
	return agentContextSummary{base, "summary", "agent-context --command 'COMMAND PATH' for local flag schemas; --full for every schema; which 'TASK' for focused discovery"}
}

func scopedAgentContext(root *cobra.Command, path string) (any, error) {
	current := root
	parts := strings.Fields(path)
	if len(parts) == 0 {
		return nil, fmt.Errorf("--command requires an exact command path")
	}
	for _, part := range parts {
		var next *cobra.Command
		for _, child := range current.Commands() {
			if child.Name() == part {
				next = child
				break
			}
		}
		if next == nil {
			return nil, fmt.Errorf("unknown command path %q; use which or agent-context", path)
		}
		current = next
	}
	if current.Name() == "agent-context" {
		return nil, fmt.Errorf("use agent-context --help for discovery flags")
	}
	var schema agentContextCommand
	for _, candidate := range collectAgentCommands(current.Parent()) {
		if candidate.Name == current.Name() {
			schema = candidate
			break
		}
	}
	return map[string]any{"schema_version": "4", "view": "command", "path": strings.Join(parts, " "), "command": schema,
		"shared_flags":        map[string]string{"--agent": "JSON, compact, non-interactive output; does not preview writes", "--dry-run": "Preview without remote writes", "--select": "Explicit field projection"},
		"global_flag_schemas": "Use COMMAND PATH --help-all for inherited flags"}, nil
}

// Task views select workflows, not guessed permissions; schemas and safety
// annotations come from the same live Cobra tree as --command.
func taskAgentContext(root *cobra.Command, task string) (any, error) {
	type step struct {
		Path    string `json:"path"`
		Purpose string `json:"purpose"`
		Command any    `json:"command"`
	}
	var paths, purposes []string
	switch task {
	case "review":
		paths = []string{"trips home", "plan overview", "plan days", "plan day"}
		purposes = []string{"Find the target trip key", "Optional orientation for focused questions; skip before a complete all-days read", "Complete review: read all required days once, including full notes and shared constraints; reuse returned travel and checks", "Read one complete day, or a delta only when the consumer has the matching persisted baseline"}
	case "create":
		paths = []string{"geos autocomplete", "trips create", "places autocomplete", "plan block add-batch", "plan days"}
		purposes = []string{"Resolve destination geo IDs", "Preview then create a blank trip with explicit title and dates", "Resolve candidate place IDs", "Preview then add places, notes and checklists through one semantic batch", "Verify selected days, complete notes, booking constraints and saved travel estimates"}
	case "edit":
		paths = []string{"plan overview", "plan days", "plan edit", "plan day"}
		purposes = []string{"Optional: locate relevant days when not already known", "Read complete selected days and stable IDs before changing them", "Preview named changes-file edits, then apply the authorized edits", "Verify the resulting day constraints and schedule"}
	default:
		return nil, fmt.Errorf("unknown task %q; choose review, create, or edit", task)
	}
	steps := make([]step, 0, len(paths))
	for i, path := range paths {
		payload, err := scopedAgentContext(root, path)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step{path, purposes[i], payload.(map[string]any)["command"]})
	}
	base := buildAgentContext(root)
	return map[string]any{"schema_version": "4", "view": "task", "task": task, "cli": base.CLI, "auth": base.Auth, "steps": steps,
		"safety":       "Read the target and current data first. --agent is output formatting, not write protection. Preview mutators with --dry-run; apply only authorized changes. Saved travel data has unknown freshness; unavailable estimates are not zero travel time.",
		"shared_flags": map[string]string{"--agent": "Compact JSON, non-interactive output", "--dry-run": "Preview mutators without remote writes"},
		"more":         "agent-context for complete safety inventory; --command 'PATH' for another schema; COMMAND --help-all for inherited flags"}, nil
}
