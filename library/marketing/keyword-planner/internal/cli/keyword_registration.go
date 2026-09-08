// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// registerKeywordPlannerCommands is the preserved reachability boundary for
// this printed CLI. Generic generated Google resource commands are deliberately
// removed here; only the curated Planner reads and local portfolio views are
// allowed to enter the user-facing Cobra and mirrored MCP trees.
func init() {
	registerNovelCommand(registerKeywordPlannerCommands)
}

func registerKeywordPlannerCommands(rootCmd *cobra.Command, flags *rootFlags) {
	if rootCmd == nil {
		return
	}
	// Match the catalogue binary name in help and version output.
	rootCmd.Use = "keyword-planner-pp-cli"
	rootCmd.SetVersionTemplate("keyword-planner-pp-cli {{ .Version }}\n")
	if homeFlag := rootCmd.PersistentFlags().Lookup("home"); homeFlag != nil {
		homeFlag.Usage = "Override config, portfolio, state, and cache roots; Planner per-customer request pacing remains shared"
	}
	if rateFlag := rootCmd.PersistentFlags().Lookup("rate-limit"); rateFlag != nil {
		rateFlag.Usage = "Planner requests per second: default auto is 1; positive values up to 1 are allowed; disabling is rejected"
	}
	removeKeywordRootCommand(rootCmd, "customers")
	removeKeywordRootCommand(rootCmd, "auth")
	addNovelCommandIfAbsent(rootCmd, newPlannerIdeasCmd(flags))
	addNovelCommandIfAbsent(rootCmd, newPlannerHistoricalCmd(flags))

	portfolioCommand := findKeywordRootCommand(rootCmd, "portfolio")
	if portfolioCommand == nil {
		portfolioCommand = &cobra.Command{
			Use:         "portfolio",
			Short:       "Work with the local Planner evidence portfolio",
			Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:parent-group": "true"},
			RunE:        parentNoSubcommandRunE(flags),
		}
		rootCmd.AddCommand(portfolioCommand)
	}
	if portfolioCommand.Annotations == nil {
		portfolioCommand.Annotations = map[string]string{}
	}
	portfolioCommand.Annotations["pp:parent-group"] = "true"
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioListCmd(flags))
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioShowCmd(flags))
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioSearchCmd(flags))
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioExportCmd(flags))
	registerKeywordPlannerViews(portfolioCommand, flags)
}

func findKeywordRootCommand(root *cobra.Command, name string) *cobra.Command {
	if root == nil {
		return nil
	}
	for _, child := range root.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}

func removeKeywordRootCommand(root *cobra.Command, name string) {
	if command := findKeywordRootCommand(root, name); command != nil {
		root.RemoveCommand(command)
	}
}
