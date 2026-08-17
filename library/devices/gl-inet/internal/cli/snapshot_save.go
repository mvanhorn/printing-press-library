// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSnapshotSaveCmd(flags *rootFlags) *cobra.Command {
	var notes string
	cmd := &cobra.Command{
		Use:         "save <name>",
		Short:       "Capture your whole router config as a named, reusable profile.",
		Example:     "  gl-inet-pp-cli snapshot save home --notes 'baseline config'",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := strings.TrimSpace(args[0])
			if name == "" {
				return usageErr(fmt.Errorf("snapshot name must not be empty"))
			}
			out := cmd.OutOrStdout()

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would capture full uci config from the router and save snapshot %q\n", name)
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintf(out, "dry-run: would SSH 'uci export' + 'uci show', read /etc/glversion, /etc/openwrt_release and luci version, then save snapshot %q to the local store\n", name)
				return nil
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Provenance from the GL RPC layer (best effort).
			info, _, infoErr := fetchSystemInfo(ctx, c)

			cfg, err := glSSHConfig(c)
			if err != nil {
				return classifyGLError(err, flags)
			}
			export, err := glssh.UCIExport(ctx, cfg, "")
			if err != nil {
				return classifyGLError(fmt.Errorf("capturing uci export over SSH: %w", err), flags)
			}
			show, err := glssh.UCIShow(ctx, cfg, "")
			if err != nil {
				return classifyGLError(fmt.Errorf("capturing uci show over SSH: %w", err), flags)
			}

			openwrt := info.BoardInfo.OpenwrtVersion
			if rel, rerr := glssh.Run(ctx, cfg, "cat /etc/openwrt_release 2>/dev/null"); rerr == nil {
				if v := parseOpenwrtRelease(rel)["DISTRIB_RELEASE"]; v != "" {
					openwrt = v
				}
			}
			luci := ""
			if l, lerr := glssh.Run(ctx, cfg, "opkg list-installed 2>/dev/null | grep '^luci '"); lerr == nil {
				luci = parseLuciVersion(l)
			}
			firmware := info.FirmwareVersion
			if firmware == "" {
				if gv, gerr := glssh.Run(ctx, cfg, "cat /etc/glversion 2>/dev/null"); gerr == nil {
					firmware = strings.TrimSpace(gv)
				}
			}

			snap := store.ConfigSnapshot{
				Name:         name,
				CreatedAt:    time.Now().UTC().Format(time.RFC3339),
				Model:        info.Model,
				Firmware:     firmware,
				OpenWrt:      openwrt,
				Luci:         luci,
				CountryCodes: info.CountryCode,
				Notes:        notes,
				UCIExport:    export,
				UCIShow:      show,
			}

			st, err := openSnapshotStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			if err := st.SaveSnapshot(snap); err != nil {
				return apiErr(err)
			}

			result := map[string]any{
				"status":        "saved",
				"name":          name,
				"model":         snap.Model,
				"firmware":      snap.Firmware,
				"openwrt":       snap.OpenWrt,
				"luci":          snap.Luci,
				"country_codes": snap.CountryCodes,
				"size_bytes":    len(export),
			}
			if infoErr != nil {
				result["provenance_warning"] = "system.get_info unavailable: " + infoErr.Error()
			}
			raw, _ := json.Marshal(result)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Free-form note stored alongside the snapshot")
	return cmd
}
