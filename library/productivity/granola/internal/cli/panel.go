// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newPanelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "panel",
		Short: "Read AI panels for a meeting",
	}
	cmd.AddCommand(newPanelGetCmd(flags))
	return cmd
}

func newPanelGetCmd(flags *rootFlags) *cobra.Command {
	var template string
	var asMarkdown, asPlain bool
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Fetch AI panels (summary, action items, etc.) for a meeting",
		Long: `Calls Granola's /v1/get-document-panels and returns the panel content
keyed by template slug. --template selects one panel; without it, the
whole map is returned.`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			panels, err := ic.GetDocumentPanels(id)
			if err != nil {
				return apiErr(err)
			}
			if template != "" {
				val, ok := panels[template]
				if !ok {
					return notFoundErr(fmt.Errorf("panel template %q not present for meeting %s", template, id))
				}
				if asPlain || asMarkdown {
					fmt.Fprintln(cmd.OutOrStdout(), val)
					return nil
				}
				return emitJSON(cmd, flags, map[string]string{template: val})
			}
			if asMarkdown {
				for slug, content := range panels {
					fmt.Fprintf(cmd.OutOrStdout(), "## %s\n\n%s\n\n", slug, content)
				}
				return nil
			}
			return emitJSON(cmd, flags, panels)
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "Select one panel by template slug")
	cmd.Flags().BoolVar(&asMarkdown, "markdown", false, "Render as markdown sections")
	cmd.Flags().BoolVar(&asPlain, "plain", false, "Print the selected panel as plain text")
	return cmd
}
