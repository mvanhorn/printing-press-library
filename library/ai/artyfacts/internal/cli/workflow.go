// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Multi-step workflows for common agent patterns",
		Long: `High-level commands that combine multiple API calls into atomic workflows.

Designed for CI pipelines, agent automation, and multi-section artifact creation.`,
	}

	cmd.AddCommand(newWorkflowCreateCmd(flags))

	return cmd
}
