// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelRpcCallCmd(flags *rootFlags) *cobra.Command {
	var argsJSON string
	cmd := &cobra.Command{
		Use:   "call <module> <function> [json-args]",
		Short: "Call any GL RPC module/function directly (the universal escape hatch).",
		Long:  "Invoke any GL JSON-RPC module.function. Pass arguments as a JSON object, either as the third positional argument or via --args. VPN modules are hyphenated (wg-client, ovpn-client).",
		Example: "  gl-inet-pp-cli rpc call netmode get_mode --agent\n" +
			"  gl-inet-pp-cli rpc call wg-client get_all_config_list\n" +
			"  gl-inet-pp-cli rpc call repeater connect '{\"ssid\":\"Cafe\",\"key\":\"pw\"}'",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			module, function := args[0], args[1]

			rawArgs := strings.TrimSpace(argsJSON)
			if rawArgs == "" && len(args) >= 3 {
				rawArgs = strings.TrimSpace(args[2])
			}
			var parsed any
			if rawArgs != "" {
				if err := json.Unmarshal([]byte(rawArgs), &parsed); err != nil {
					return usageErr(fmt.Errorf("parsing json-args: %w", err))
				}
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			// Client.Call already short-circuits under verify and honors --dry-run.
			data, err := c.Call(ctx, module, function, parsed)
			if err != nil {
				return classifyGLError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&argsJSON, "args", "", "JSON object of RPC arguments (alternative to the positional json-args)")
	return cmd
}
