// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/store"
	"github.com/spf13/cobra"
)

func newNovelSnapshotCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Capture, compare, and restore the router's whole configuration",
		Long:        "snapshot subcommands: save, list, show, diff, apply.\n\nCapture the router's entire UCI configuration as a named profile, see exactly what a venue changed versus your standard, and restore a profile with a device-match safety gate.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelSnapshotSaveCmd(flags))
	cmd.AddCommand(newNovelSnapshotListCmd(flags))
	cmd.AddCommand(newNovelSnapshotShowCmd(flags))
	cmd.AddCommand(newNovelSnapshotDiffCmd(flags))
	cmd.AddCommand(newNovelSnapshotApplyCmd(flags))
	cmd.AddCommand(newNovelSnapshotDeleteCmd(flags))
	return cmd
}

// snapshotListRow is the metadata projection shown by `snapshot list` and the
// promoted `snapshots` command. Heavy uci blobs are reduced to a byte size.
type snapshotListRow struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	Model     string `json:"model"`
	Firmware  string `json:"firmware"`
	OpenWrt   string `json:"openwrt"`
	Country   string `json:"country_codes"`
	Notes     string `json:"notes,omitempty"`
	SizeBytes int    `json:"size_bytes"`
}

func snapshotListRows(snaps []store.ConfigSnapshot) []snapshotListRow {
	rows := make([]snapshotListRow, 0, len(snaps))
	for _, s := range snaps {
		rows = append(rows, snapshotListRow{
			Name:      s.Name,
			CreatedAt: s.CreatedAt,
			Model:     s.Model,
			Firmware:  s.Firmware,
			OpenWrt:   s.OpenWrt,
			Country:   s.CountryCodes,
			Notes:     s.Notes,
			SizeBytes: len(s.UCIExport),
		})
	}
	return rows
}

// pp:data-source local
func newNovelSnapshotListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved config snapshots (name, created, model, firmware, size)",
		Example:     "  gl-inet-pp-cli snapshot list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openSnapshotStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			snaps, err := st.ListSnapshots()
			if err != nil {
				return apiErr(err)
			}
			raw, _ := json.Marshal(snapshotListRows(snaps))
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}

func newNovelSnapshotShowCmd(flags *rootFlags) *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:         "show <name>",
		Short:       "Show a snapshot's metadata and provenance (--full prints the stored config)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openSnapshotStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			snap, err := st.GetSnapshot(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			meta := map[string]any{
				"name":          snap.Name,
				"created_at":    snap.CreatedAt,
				"model":         snap.Model,
				"firmware":      snap.Firmware,
				"openwrt":       snap.OpenWrt,
				"luci":          snap.Luci,
				"country_codes": snap.CountryCodes,
				"notes":         snap.Notes,
				"size_bytes":    len(snap.UCIExport),
			}
			if full {
				meta["uci_export"] = snap.UCIExport
			}
			raw, _ := json.Marshal(meta)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Include the stored uci export in the output")
	return cmd
}
