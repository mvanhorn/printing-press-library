// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newRulesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "rules",
		Short:  "Automation rules: route, tag, auto-reply, escalate on incoming tickets",
		Hidden: true,
	}

	cmd.AddCommand(newRulesCreateCmd(flags))
	cmd.AddCommand(newRulesDeleteCmd(flags))
	cmd.AddCommand(newRulesGetCmd(flags))
	cmd.AddCommand(newRulesListCmd(flags))
	cmd.AddCommand(newRulesSetPrioritiesCmd(flags))
	cmd.AddCommand(newRulesUpdateCmd(flags))
	return cmd
}
