// Hand-written novel feature registration. Not generated.
// Wires the transcendence subcommands into the root command tree with
// explicit AddCommand calls so dogfood's static wiring check can see them.
package cli

import "github.com/spf13/cobra"

// registerNovelFeatures adds the 13 transcendence commands plus `sql` to the
// root command tree. Top-level commands attach to rootCmd directly; child
// commands attach to their existing spec-derived parent commands by name.
func registerNovelFeatures(rootCmd *cobra.Command, flags *rootFlags) {
	// Top-level novel commands.
	rootCmd.AddCommand(newTriageCmd(flags))
	rootCmd.AddCommand(newStandupCmd(flags))
	rootCmd.AddCommand(newTimeCmd(flags))
	rootCmd.AddCommand(newContractsCmd(flags))
	rootCmd.AddCommand(newRulesCmd(flags))
	rootCmd.AddCommand(newSQLCmd(flags))

	// Children attached to spec-derived parent resource commands.
	if ticketsCmd := findChildCmd(rootCmd, "tickets"); ticketsCmd != nil {
		ticketsCmd.Hidden = false
		ticketsCmd.AddCommand(newTicketsAgeOutCmd(flags))
		ticketsCmd.AddCommand(newTicketsChangedSinceCmd(flags))
	}
	if slaCmd := findChildCmd(rootCmd, "sla"); slaCmd != nil {
		slaCmd.Hidden = false
		slaCmd.AddCommand(newSLABreachingCmd(flags))
	}
	if agentCmd := findChildCmd(rootCmd, "agent"); agentCmd != nil {
		agentCmd.Hidden = false
		agentCmd.AddCommand(newAgentWorkloadCmd(flags))
	}
	if clientCmd := findChildCmd(rootCmd, "client"); clientCmd != nil {
		clientCmd.Hidden = false
		clientCmd.AddCommand(newClientCardCmd(flags))
		clientCmd.AddCommand(newClientOverlayCmd(flags))
	}
	if assetCmd := findChildCmd(rootCmd, "asset"); assetCmd != nil {
		assetCmd.Hidden = false
		assetCmd.AddCommand(newAssetHistoryCmd(flags))
	}
	if kbCmd := findChildCmd(rootCmd, "kbarticle"); kbCmd != nil {
		kbCmd.Hidden = false
		kbCmd.AddCommand(newKBArticleSuggestCmd(flags))
	}
}

func findChildCmd(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
