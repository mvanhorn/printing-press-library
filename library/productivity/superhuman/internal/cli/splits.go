// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// splits.go declares the `splits` namespace and its `list` subcommand.
// Mirrors the Superhuman MCP's list_splits tool: surface the active
// account's configured split-inbox views.
//
// Implementation-time unknown (plan 2026-05-14-003 U7): the exact endpoint
// path. The hypothesis (chosen here) is POST /v3/userdata.read with path
// users/<gid>/splits. If the bundle reveals a different path (e.g., a
// dedicated /v3/userdata.getSplits route) swap splitsPathFor and the
// command's output shape stays correct.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newSplitsCmd registers the `splits` namespace.
func newSplitsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "splits",
		Short: "Split-inbox views (list)",
		Long: `Configured split-inbox views for the active account.

Splits are Superhuman's per-account filtered views (e.g., "VIPs",
"Newsletters", "Customers"). This command lists every configured split;
each row carries the split name, id, and filter expression.`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newSplitsListCmd(flags))
	return cmd
}

// newSplitsListCmd registers `splits list`.
func newSplitsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the active account's configured splits",
		Example: "  superhuman-pp-cli splits list\n  superhuman-pp-cli splits list --json",
		Annotations: map[string]string{
			"pp:endpoint":   "splits.list",
			"pp:method":     "POST",
			"pp:path":       "/v3/userdata.read",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSplitsList(cmd, flags)
		},
	}
	return cmd
}

// splitsPathFor returns the userdata.read path for the splits config of
// the active account.
func splitsPathFor(providerID string) string {
	return fmt.Sprintf("users/%s/splits", providerID)
}

func runSplitsList(cmd *cobra.Command, flags *rootFlags) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	providerID, perr := resolveProviderID(flags)
	if perr != nil {
		return authErr(fmt.Errorf("splits list: %w", perr))
	}

	body := map[string]any{
		"reads": []map[string]any{
			{"path": splitsPathFor(providerID)},
		},
		"pageToken": nil,
		"pageSize":  nil,
	}
	data, statusCode, err := c.Post("/v3/userdata.read", body)
	if err != nil {
		return classifyAPIError(err, flags)
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		var parsed any
		_ = json.Unmarshal(data, &parsed)
		envelope := map[string]any{
			"action":   "splits.list",
			"resource": "splits",
			"path":     "/v3/userdata.read",
			"status":   statusCode,
			"success":  statusCode >= 200 && statusCode < 300,
			"data":     parsed,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	// Human: best-effort table fall-through.
	var rows []map[string]any
	if jerr := json.Unmarshal(data, &rows); jerr == nil && len(rows) > 0 {
		if perr := printAutoTable(cmd.OutOrStdout(), rows); perr == nil {
			return nil
		}
	}
	var wrapped struct {
		Data []map[string]any `json:"data"`
	}
	if jerr := json.Unmarshal(data, &wrapped); jerr == nil && len(wrapped.Data) > 0 {
		if perr := printAutoTable(cmd.OutOrStdout(), wrapped.Data); perr == nil {
			return nil
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "No splits configured.")
	return nil
}
