// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command report channel-mix: channel-source distribution.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/internal/store"
	"github.com/spf13/cobra"
)

type channelBucket struct {
	Channel string `json:"channel"`
	Count   int    `json:"count"`
}

type channelMixView struct {
	Channels         []channelBucket `json:"channels"`
	TotalChannelRows int             `json:"total_channel_rows"`
}

// pp:data-source local

func newNovelReportChannelMixCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "channel-mix",
		Short:       "See which messaging channels (WhatsApp, Instagram, email...) your contacts actually use.",
		Long:        "Aggregates channel sources from synced channel/space rows in the local mirror (space, space-channel resource types).",
		Example:     "  respondio-pp-cli report channel-mix --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "report channel-mix")
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("respondio-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: respondio-pp-cli sync --resources space,space-channel --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), emptyChannelMix(), flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No synced channels yet.")
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type IN ('space','space-channel')`)
			if err != nil {
				return fmt.Errorf("querying channels: %w", err)
			}
			var datas [][]byte
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan channel: %w", err)
				}
				datas = append(datas, data)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate channels: %w", err)
			}
			_ = rows.Close()

			view := emptyChannelMix()
			buckets := map[string]int{}
			for _, raw := range datas {
				var c map[string]any
				if err := json.Unmarshal(raw, &c); err != nil {
					continue
				}
				view.TotalChannelRows++
				src := str(c["source"])
				if src == "" {
					src = "unknown"
				}
				buckets[src]++
			}
			for ch, n := range buckets {
				view.Channels = append(view.Channels, channelBucket{Channel: ch, Count: n})
			}
			sort.Slice(view.Channels, func(i, j int) bool { return view.Channels[i].Count > view.Channels[j].Count })

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, b := range view.Channels {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %d\n", b.Channel, b.Count)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func emptyChannelMix() channelMixView {
	return channelMixView{Channels: make([]channelBucket, 0)}
}
