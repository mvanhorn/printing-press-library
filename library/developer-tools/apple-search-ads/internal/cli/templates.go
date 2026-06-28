// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelTemplatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "templates",
		Short:       "Version-control campaign structures and apply them across orgs",
		Long:        `Save campaign structures as JSON templates and apply them across multiple org IDs with a diff preview. Useful for maintaining consistent campaign structure across multiple apps.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelTemplatesListCmd(flags))
	cmd.AddCommand(newNovelTemplatesSaveCmd(flags))
	cmd.AddCommand(newNovelTemplatesApplyCmd(flags))
	return cmd
}
