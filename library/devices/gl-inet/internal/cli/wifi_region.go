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

func newNovelWifiRegionCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "region",
		Short:       "Show, set, or diagnose the WiFi regulatory domain",
		Long:        "region subcommands: show, set, diagnose. The regulatory domain controls which 2.4GHz channels (e.g. 12/13 in Europe) the radios can see and use.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelWifiRegionShowCmd(flags))
	cmd.AddCommand(newNovelWifiRegionSetCmd(flags))
	cmd.AddCommand(newNovelWifiRegionDiagnoseCmd(flags))
	return cmd
}

func newNovelWifiRegionShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Short:       "Show the current regulatory domain per radio",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if cliutil.IsVerifyEnv() {
				raw, _ := json.Marshal(map[string]any{"status": "noop", "reason": "verify_short_circuit"})
				return printOutputWithFlags(out, raw, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			info, _, _ := fetchSystemInfo(ctx, c)
			cfg, err := glSSHConfig(c)
			if err != nil {
				return classifyGLError(err, flags)
			}
			wireless, err := glssh.UCIShow(ctx, cfg, "wireless")
			if err != nil {
				return classifyGLError(err, flags)
			}
			region := extractRadioCountries(parseUCIShow(wireless))
			raw, _ := json.Marshal(map[string]any{
				"country_code": info.CountryCode,
				"radios":       region,
			})
			return printOutputWithFlags(out, raw, flags)
		},
	}
}

func newNovelWifiRegionSetCmd(flags *rootFlags) *cobra.Command {
	var radio string
	cmd := &cobra.Command{
		Use:         "set <CC>",
		Short:       "Set the WiFi regulatory domain (2-letter country code)",
		Long:        "Set the regulatory country for all radios (default) or one radio, then commit and reload wifi. Mutating: prints the plan by default on a TTY; pass --yes/--agent to apply.",
		Example:     "  gl-inet-pp-cli wifi region set IT",
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			cc := strings.ToUpper(strings.TrimSpace(args[0]))
			if !validCountryCode(cc) {
				return usageErr(fmt.Errorf("invalid country code %q: expected 2 uppercase letters (e.g. IT, US)", cc))
			}
			out := cmd.OutOrStdout()
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would set WiFi region to %s\n", cc)
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

			radios := []string{}
			if radio != "" && radio != "all" {
				radios = []string{radio}
			} else if !dryRunOK(flags) {
				wireless, err := glssh.UCIShow(ctx, cfg, "wireless")
				if err != nil {
					return classifyGLError(err, flags)
				}
				radios = extractWifiDevices(parseUCIShow(wireless))
			}
			if dryRunOK(flags) {
				disp := radios
				if len(disp) == 0 {
					disp = []string{"<all radios>"}
				}
				fmt.Fprintf(out, "dry-run: %s\n", buildRegionSetScript(disp, cc))
				return nil
			}
			if len(radios) == 0 {
				return apiErr(fmt.Errorf("no wifi radios found in uci wireless config"))
			}
			script := buildRegionSetScript(radios, cc)

			if isTerminal(out) && !flags.yes && !flags.agent && !flags.noInput {
				raw, _ := json.Marshal(map[string]any{
					"status": "needs_confirmation", "country": cc, "radios": radios,
					"command": script, "hint": "re-run with --yes to apply",
				})
				return printOutputWithFlags(out, raw, flags)
			}
			if _, err := glssh.Run(ctx, cfg, script); err != nil {
				return classifyGLError(err, flags)
			}
			raw, _ := json.Marshal(map[string]any{"status": "set", "country": cc, "radios": radios})
			return printOutputWithFlags(out, raw, flags)
		},
	}
	cmd.Flags().StringVar(&radio, "radio", "all", "Which radio to set: all (default) or a radio name (e.g. mt798111)")
	return cmd
}
