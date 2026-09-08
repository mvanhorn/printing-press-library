// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(registerPlannerLearningPreview)
}

// registerPlannerLearningPreview repairs the dry-run output boundary for the
// allowlisted learning write commands whose own RunE can stay silent when
// learning is disabled. The wrapper is installed after the complete Cobra tree
// is built, so non-dry-run execution and command validation remain owned by the
// original RunE.
func registerPlannerLearningPreview(root *cobra.Command, flags *rootFlags) {
	wrapPlannerLearningDryRun(findSubcommand(root, "teach"), flags, "teach")
	wrapPlannerLearningDryRun(findSubcommand(root, "teach-playbook"), flags, "teach-playbook")
	wrapPlannerLearningDryRun(findSubcommand(root, "teach-pattern"), flags, "teach-pattern")
	wrapPlannerLearningDryRun(findSubcommand(root, "teach-lookup"), flags, "teach-lookup")
	playbook := findSubcommand(root, "playbook")
	if playbook != nil {
		wrapPlannerLearningDryRun(findSubcommand(playbook, "amend"), flags, "playbook amend")
	}
	learnings := findSubcommand(root, "learnings")
	if learnings != nil {
		wrapPlannerLearningDryRun(findSubcommand(learnings, "forget"), flags, "learnings forget")
	}
}

func wrapPlannerLearningDryRun(command *cobra.Command, flags *rootFlags, action string) {
	if command == nil || command.RunE == nil {
		return
	}
	originalRunE := command.RunE
	command.RunE = func(cmd *cobra.Command, args []string) error {
		if flags != nil && flags.dryRun {
			return writeDryRun(cmd.OutOrStdout(), flags, action)
		}
		return originalRunE(cmd, args)
	}
}
