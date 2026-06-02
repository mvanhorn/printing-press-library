// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built ClickUp Docs (v3 API) support for clickup-2-pp-cli. The Docs API
// lives at /v3/workspaces/{workspace_id}/docs..., a separate version from the
// v2 surface the generator covers. It uses the same personal-token auth and
// the same base URL (https://api.clickup.com/api), so the generated client's
// request methods reach it directly.

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newDocsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Work with ClickUp Docs and their pages (v3 API)",
		Long: `Read and write ClickUp Docs via the v3 API.

Every command needs a workspace (team) id: pass --workspace or set
CLICKUP_WORKSPACE. Write commands print the request they would send by
default and only execute with --confirm (or honor --dry-run for a no-op
preview).`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newDocsListCmd(flags))
	cmd.AddCommand(newDocsGetCmd(flags))
	cmd.AddCommand(newDocsPagesCmd(flags))
	cmd.AddCommand(newDocsPageContentCmd(flags))
	cmd.AddCommand(newDocsPageGetCmd(flags))
	cmd.AddCommand(newDocsCreateCmd(flags))
	cmd.AddCommand(newDocsPageCreateCmd(flags))
	cmd.AddCommand(newDocsPageEditCmd(flags))
	return cmd
}

// docsDryRun prints the write a command would perform without sending it.
// It respects --json/--agent so machine consumers (and the verify matrix) get
// valid JSON instead of a human sentence.
func docsDryRun(cmd *cobra.Command, flags *rootFlags, method, path, action string, confirm bool) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		payload := map[string]any{
			"dry_run":       true,
			"method":        method,
			"path":          path,
			"confirmed":     confirm,
			"would_execute": confirm,
		}
		return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "would %s %s\n", method, path)
	if !confirm {
		fmt.Fprintf(cmd.OutOrStdout(), "(re-run with --confirm to %s)\n", action)
	}
	return nil
}

// workspaceID resolves the workspace from the flag or CLICKUP_WORKSPACE.
func workspaceID(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if v := os.Getenv("CLICKUP_WORKSPACE"); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("workspace id required: pass --workspace <id> or set CLICKUP_WORKSPACE (find it with 'clickup-2-pp-cli workspace list')")
}

func docsReadFlag(cmd *cobra.Command, ws *string) {
	cmd.Flags().StringVar(ws, "workspace", "", "Workspace (team) id; or set CLICKUP_WORKSPACE")
}

func newDocsListCmd(flags *rootFlags) *cobra.Command {
	var ws string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Search and list Docs in a workspace",
		Example:     "  clickup-2-pp-cli docs list --workspace 90010 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if ws == "" && os.Getenv("CLICKUP_WORKSPACE") == "" {
				return cmd.Help()
			}
			wid, err := workspaceID(ws)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), fmt.Sprintf("/v3/workspaces/%s/docs", wid), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	docsReadFlag(cmd, &ws)
	return cmd
}

func newDocsGetCmd(flags *rootFlags) *cobra.Command {
	var ws string
	cmd := &cobra.Command{
		Use:         "get <doc_id>",
		Short:       "Get a Doc's metadata",
		Example:     "  clickup-2-pp-cli docs get 8cb1a2 --workspace 90010",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			wid, err := workspaceID(ws)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), fmt.Sprintf("/v3/workspaces/%s/docs/%s", wid, args[0]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	docsReadFlag(cmd, &ws)
	return cmd
}

func newDocsPagesCmd(flags *rootFlags) *cobra.Command {
	var ws string
	cmd := &cobra.Command{
		Use:         "pages <doc_id>",
		Short:       "List the page tree of a Doc",
		Example:     "  clickup-2-pp-cli docs pages 8cb1a2 --workspace 90010 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			wid, err := workspaceID(ws)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), fmt.Sprintf("/v3/workspaces/%s/docs/%s/pageListing", wid, args[0]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	docsReadFlag(cmd, &ws)
	return cmd
}

func newDocsPageContentCmd(flags *rootFlags) *cobra.Command {
	var ws string
	cmd := &cobra.Command{
		Use:         "page-content <doc_id>",
		Short:       "Get the content of all pages in a Doc",
		Example:     "  clickup-2-pp-cli docs page-content 8cb1a2 --workspace 90010",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			wid, err := workspaceID(ws)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), fmt.Sprintf("/v3/workspaces/%s/docs/%s/pages", wid, args[0]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	docsReadFlag(cmd, &ws)
	return cmd
}

func newDocsPageGetCmd(flags *rootFlags) *cobra.Command {
	var ws string
	cmd := &cobra.Command{
		Use:         "page-get <doc_id> <page_id>",
		Short:       "Get a single page in a Doc",
		Example:     "  clickup-2-pp-cli docs page-get 8cb1a2 p7f3 --workspace 90010",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			wid, err := workspaceID(ws)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), fmt.Sprintf("/v3/workspaces/%s/docs/%s/pages/%s", wid, args[0], args[1]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	docsReadFlag(cmd, &ws)
	return cmd
}

// --- write commands (print-by-default, --confirm to execute) ---

func newDocsCreateCmd(flags *rootFlags) *cobra.Command {
	var ws, name, parentID string
	var confirm bool
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new Doc in a workspace",
		Example: "  clickup-2-pp-cli docs create --workspace 90010 --name \"Runbook\" --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (ws == "" && os.Getenv("CLICKUP_WORKSPACE") == "") || name == "" {
				return cmd.Help()
			}
			wid, err := workspaceID(ws)
			if err != nil {
				return err
			}
			body := map[string]any{"name": name}
			if parentID != "" {
				body["parent"] = map[string]any{"id": parentID}
			}
			path := fmt.Sprintf("/v3/workspaces/%s/docs", wid)
			if dryRunOK(flags) || !confirm {
				return docsDryRun(cmd, flags, "POST", path, "create the Doc", confirm)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Post(cmd.Context(), path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	docsReadFlag(cmd, &ws)
	cmd.Flags().StringVar(&name, "name", "", "Doc name (required)")
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent doc/page id (optional)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually create the Doc")
	return cmd
}

func newDocsPageCreateCmd(flags *rootFlags) *cobra.Command {
	var ws, name, content string
	var confirm bool
	cmd := &cobra.Command{
		Use:     "page-create <doc_id>",
		Short:   "Create a page in a Doc",
		Example: "  clickup-2-pp-cli docs page-create 8cb1a2 --workspace 90010 --name Intro --content \"# Intro\" --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			wid, err := workspaceID(ws)
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			body := map[string]any{"name": name, "content": content, "content_format": "text/md"}
			path := fmt.Sprintf("/v3/workspaces/%s/docs/%s/pages", wid, args[0])
			if dryRunOK(flags) || !confirm {
				return docsDryRun(cmd, flags, "POST", path, "create the page", confirm)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Post(cmd.Context(), path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	docsReadFlag(cmd, &ws)
	cmd.Flags().StringVar(&name, "name", "", "Page name (required)")
	cmd.Flags().StringVar(&content, "content", "", "Markdown content")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually create the page")
	return cmd
}

func newDocsPageEditCmd(flags *rootFlags) *cobra.Command {
	var ws, name, content string
	var confirm bool
	cmd := &cobra.Command{
		Use:     "page-edit <doc_id> <page_id>",
		Short:   "Edit a page in a Doc",
		Example: "  clickup-2-pp-cli docs page-edit 8cb1a2 p7f3 --workspace 90010 --content \"# Updated\" --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			wid, err := workspaceID(ws)
			if err != nil {
				return err
			}
			body := map[string]any{}
			if name != "" {
				body["name"] = name
			}
			if content != "" {
				body["content"] = content
				body["content_format"] = "text/md"
				body["content_edit_mode"] = "replace"
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update: pass --name and/or --content")
			}
			path := fmt.Sprintf("/v3/workspaces/%s/docs/%s/pages/%s", wid, args[0], args[1])
			if dryRunOK(flags) || !confirm {
				return docsDryRun(cmd, flags, "PUT", path, "edit the page", confirm)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Put(cmd.Context(), path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	docsReadFlag(cmd, &ws)
	cmd.Flags().StringVar(&name, "name", "", "New page name")
	cmd.Flags().StringVar(&content, "content", "", "New markdown content (replaces existing)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually edit the page")
	return cmd
}
