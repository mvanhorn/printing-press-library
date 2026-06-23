// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/spf13/cobra"
)

func newRpcUciCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "uci",
		Short:       "Read and write raw UCI config options over SSH",
		Long:        "uci subcommands: get <key>, show [pkg], set <key>=<value>.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRpcUciGetCmd(flags))
	cmd.AddCommand(newRpcUciShowCmd(flags))
	cmd.AddCommand(newRpcUciSetCmd(flags))
	return cmd
}

func newRpcUciGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "get <key>",
		Short:       "Get a single UCI option value (e.g. wireless.mt798111.country)",
		Example:     "  gl-inet-pp-cli rpc uci get wireless.mt798111.country",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			key := strings.TrimSpace(args[0])
			if !uciKeyRE.MatchString(key) {
				return usageErr(fmt.Errorf("invalid uci key %q", key))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would read uci option %q\n", key)
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: uci -q get %s\n", key)
				return nil
			}
			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cfg, err := glSSHConfig(c)
			if err != nil {
				return classifyGLError(err, flags)
			}
			val, err := glssh.Run(ctx, cfg, "uci -q get "+shellQuoteSingle(key))
			if err != nil {
				return classifyGLError(err, flags)
			}
			val = strings.TrimRight(val, "\n")
			raw, _ := json.Marshal(map[string]string{"key": key, "value": val})
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
}

func newRpcUciShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show [pkg]",
		Short:       "Show all UCI options, or just one package (e.g. wireless)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := ""
			if len(args) > 0 {
				pkg = strings.TrimSpace(args[0])
			}
			if pkg != "" && !uciKeyRE.MatchString(pkg) && !isPlainName(pkg) {
				return usageErr(fmt.Errorf("invalid uci package %q", pkg))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would run uci show")
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: uci show %s\n", pkg)
				return nil
			}
			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cfg, err := glSSHConfig(c)
			if err != nil {
				return classifyGLError(err, flags)
			}
			out, err := glssh.UCIShow(ctx, cfg, pkg)
			if err != nil {
				return classifyGLError(err, flags)
			}
			if flags.asJSON {
				m := parseUCIShow(out)
				disp := make(map[string]string, len(m))
				for k, v := range m {
					disp[k] = uciDisplayValue(v)
				}
				raw, _ := json.Marshal(disp)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func newRpcUciSetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "set <key>=<value>",
		Short:       "Set a UCI option and commit its package (mutating; print-by-default)",
		Long:        "Set a UCI option then commit its package. Prints the planned command by default on an interactive terminal; pass --yes (or --agent) to apply.",
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			key, value, ok := strings.Cut(strings.TrimSpace(args[0]), "=")
			if !ok {
				return usageErr(fmt.Errorf("expected <key>=<value>, got %q", args[0]))
			}
			key = strings.TrimSpace(key)
			if !uciKeyRE.MatchString(key) {
				return usageErr(fmt.Errorf("invalid uci key %q", key))
			}
			pkg := uciPackage(key)
			command := fmt.Sprintf("uci set %s=%s ; uci commit %s", shellQuoteSingle(key), shellQuoteSingle(value), shellQuoteSingle(pkg))
			out := cmd.OutOrStdout()

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would set %s=%s and commit %s\n", key, value, pkg)
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintf(out, "dry-run: %s\n", command)
				return nil
			}
			// Print-by-default mutation gate.
			if isTerminal(out) && !flags.yes && !flags.agent && !flags.noInput {
				raw, _ := json.Marshal(map[string]any{
					"status":  "needs_confirmation",
					"command": command,
					"hint":    "re-run with --yes to apply",
				})
				return printOutputWithFlags(out, raw, flags)
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cfg, err := glSSHConfig(c)
			if err != nil {
				return classifyGLError(err, flags)
			}
			if _, err := glssh.Run(ctx, cfg, command); err != nil {
				return classifyGLError(err, flags)
			}
			raw, _ := json.Marshal(map[string]any{"status": "set", "key": key, "value": value, "committed": pkg})
			return printOutputWithFlags(out, raw, flags)
		},
	}
}

// isPlainName reports whether s is a bare uci package name (no dots/brackets).
func isPlainName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
