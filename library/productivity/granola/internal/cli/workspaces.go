// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
)

func newWorkspacesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspaces",
		Short: "Inspect Granola workspaces, members, note counts, integrations, and MCP agents",
	}
	cmd.AddCommand(newWorkspacesListCmd(flags))
	cmd.AddCommand(newWorkspacesMembersCmd(flags))
	cmd.AddCommand(newWorkspacesNotesCountCmd(flags))
	cmd.AddCommand(newWorkspacesIntegrationsCmd(flags))
	cmd.AddCommand(newWorkspacesMCPAgentsCmd(flags))
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

func newWorkspacesMembersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members <workspace-id>",
		Short: "List members and pending invites for a workspace via the internal API",
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
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("workspaces members requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.GetWorkspaceMembers(args[0])
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
	return cmd
}

func newWorkspacesNotesCountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes-count",
		Short: "Fetch total and per-workspace note counts via the internal API",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("workspaces notes-count requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.GetWorkspacesNotesCount()
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
	return cmd
}

func newWorkspacesIntegrationsCmd(flags *rootFlags) *cobra.Command {
	var workspaceID string
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "List available workspace integrations via the internal API (chia shard)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("workspaces integrations requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.ListAvailableIntegrations(workspaceID)
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Workspace ID for X-Granola-Workspace-Id scoping")
	return cmd
}

func newWorkspacesMCPAgentsCmd(flags *rootFlags) *cobra.Command {
	var workspaceID string
	cmd := &cobra.Command{
		Use:   "mcp-agents",
		Short: "List configured MCP agents and connections via the internal API (cinnamon shard)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("workspaces mcp-agents requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.GetMCPAgents(workspaceID)
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Workspace ID for X-Granola-Workspace-Id scoping")
	return cmd
}
