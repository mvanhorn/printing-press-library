// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newWorkspacesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspaces",
		Short: "List Granola workspaces",
	}
	cmd.AddCommand(newWorkspacesListCmd(flags))
	return cmd
}

func newWorkspacesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspaces from cache, falling back to live",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := openGranolaCache()
			if err == nil && len(c.Workspaces) > 0 {
				out := make([]map[string]any, 0, len(c.Workspaces))
				for _, w := range c.Workspaces {
					var inner map[string]any
					_ = json.Unmarshal(w.Workspace, &inner)
					out = append(out, map[string]any{
						"plan_type": w.PlanType,
						"role":      w.Role,
						"workspace": inner,
					})
				}
				return emitJSON(cmd, flags, out)
			}
			if flags.dataSource == "local" {
				return notFoundErr(err)
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			ws, err := ic.GetWorkspaces()
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, ws)
		},
	}
	return cmd
}
