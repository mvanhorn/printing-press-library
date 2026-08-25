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

// snapshotReloadCmd best-effort reloads the services a config change can touch.
const snapshotReloadCmd = "/etc/init.d/network reload 2>/dev/null; wifi reload 2>/dev/null; /etc/init.d/firewall reload 2>/dev/null; /etc/init.d/dnsmasq reload 2>/dev/null"

// pp:data-source local
func newNovelSnapshotApplyCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Restore a saved profile, with a safety check that it matches this device.",
		Long:  "Restore a saved snapshot by issuing the minimal set of uci set/delete commands that bring the live config to match it, then commit and reload affected services. Refuses to apply a snapshot from a different model unless --force.",
		Example: "  gl-inet-pp-cli snapshot apply home --dry-run\n" +
			"  gl-inet-pp-cli snapshot apply home --yes",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := strings.TrimSpace(args[0])
			out := cmd.OutOrStdout()
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			st, err := openSnapshotStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			snap, err := st.GetSnapshot(name)
			if err != nil {
				return notFoundErr(fmt.Errorf("snapshot %q not found: %w", name, err))
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would apply snapshot %q (device-match gated; --force to override)\n", name)
				return nil
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}

			// Provenance gate: refuse a cross-model restore unless forced.
			info, _, _ := fetchSystemInfo(ctx, c)
			if snapshotModelMismatch(snap.Model, info.Model, force) {
				return apiErr(fmt.Errorf("snapshot model %q does not match this device %q; refusing to apply (use --force to override)", snap.Model, info.Model))
			}
			var warnings []string
			if snap.Firmware != "" && info.FirmwareVersion != "" && snap.Firmware != info.FirmwareVersion {
				w := fmt.Sprintf("snapshot firmware %s differs from device %s", snap.Firmware, info.FirmwareVersion)
				warnings = append(warnings, w)
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s; applying anyway\n", w)
			}

			cfg, err := glSSHConfig(c)
			if err != nil {
				return classifyGLError(err, flags)
			}
			show, err := glssh.UCIShow(ctx, cfg, "")
			if err != nil {
				return classifyGLError(err, flags)
			}
			curRaw := parseUCIShow(show)
			snapRaw := parseUCIShow(snap.UCIShow)
			cmds, packages := uciApplyCommands(curRaw, snapRaw)

			if len(cmds) == 0 {
				raw, _ := json.Marshal(map[string]any{"status": "in_sync", "name": name, "changes": 0})
				return printOutputWithFlags(out, raw, flags)
			}

			// Full script: mutations (tolerant ;), per-package commit, reload.
			var parts []string
			parts = append(parts, cmds...)
			for _, p := range packages {
				parts = append(parts, "uci commit "+p)
			}
			script := strings.Join(parts, " ; ")

			plan := map[string]any{
				"name":               name,
				"changes":            len(cmds),
				"packages":           packages,
				"commands":           cmds,
				"reload":             snapshotReloadCmd,
				"provenance_warns":   warnings,
				"changed_packages":   packages,
				"apply_script_lines": len(parts),
			}

			if dryRunOK(flags) {
				plan["status"] = "dry_run"
				raw, _ := json.Marshal(plan)
				return printOutputWithFlags(out, raw, flags)
			}

			// Human safety gate: on an interactive terminal without --yes/--agent,
			// print the plan and stop. Non-TTY / --yes / --agent proceed.
			if isTerminal(out) && !flags.yes && !flags.agent && !flags.noInput {
				plan["status"] = "needs_confirmation"
				plan["hint"] = "re-run with --yes to apply these changes"
				raw, _ := json.Marshal(plan)
				if perr := printOutputWithFlags(out, raw, flags); perr != nil {
					return perr
				}
				return nil
			}

			if _, err := glssh.Run(ctx, cfg, script); err != nil {
				return classifyGLError(fmt.Errorf("applying uci changes: %w", err), flags)
			}
			// Reload is best-effort; a non-zero exit here does not undo the commit.
			_, _ = glssh.Run(ctx, cfg, snapshotReloadCmd)

			plan["status"] = "applied"
			raw, _ := json.Marshal(plan)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Apply even when the snapshot's model does not match this device")
	return cmd
}
