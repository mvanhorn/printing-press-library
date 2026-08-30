// Copyright 2026 SomSamantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: schema snapshot / schema snapshot list.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSchemaSnapshotCmd(flags *rootFlags) *cobra.Command {
	var flagLabel string
	var flagCollection string

	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Save a point-in-time copy of every collection's config to the local store, then browse history over time.",
		Example:     "  weaviate-collections-pp-cli schema snapshot --label pre-migration",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would snapshot collection config(s) to local store")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var classes []map[string]any
			if flagCollection != "" {
				cls, err := fetchOneClass(ctx, flags, flagCollection)
				if err != nil {
					return err
				}
				classes = []map[string]any{cls}
			} else {
				all, err := fetchAllClasses(ctx, flags)
				if err != nil {
					return err
				}
				classes = all
			}

			label := flagLabel
			now := time.Now().UTC()
			takenAt := now.Format(time.RFC3339)
			if label == "" {
				// Nanosecond resolution: two unlabeled snapshots within the
				// same second would otherwise share a label, and the store
				// Upsert would silently overwrite the first one.
				label = "snapshot-" + now.Format(time.RFC3339Nano)
			}

			rec := schemaSnapshotRecord{
				Label:    label,
				TakenAt:  takenAt,
				Classes:  classes,
				NumClass: len(classes),
			}
			payload, err := json.Marshal(rec)
			if err != nil {
				return fmt.Errorf("encoding snapshot: %w", err)
			}

			st, err := openNovelStore(ctx, flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()
			if err := st.Upsert(schemaSnapshotResourceType, label, payload); err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "saved snapshot %q: %d collection(s)\n", label, len(classes))
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"label":       label,
				"taken_at":    takenAt,
				"num_classes": len(classes),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagLabel, "label", "", "Label for this snapshot (default: snapshot-<timestamp>)")
	cmd.Flags().StringVar(&flagCollection, "collection", "", "Snapshot only this collection (default: all collections)")
	cmd.AddCommand(newNovelSchemaSnapshotListCmd(flags))
	return cmd
}

// pp:data-source local
func newNovelSchemaSnapshotListCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Browse saved schema snapshots.",
		Example:     "  weaviate-collections-pp-cli schema snapshot list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list saved snapshots")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			st, err := openNovelStore(ctx, flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()

			raw, err := st.List(schemaSnapshotResourceType, flagLimit)
			if err != nil {
				return fmt.Errorf("listing snapshots: %w", err)
			}
			summaries := make([]map[string]any, 0, len(raw))
			for _, r := range raw {
				var rec schemaSnapshotRecord
				if err := json.Unmarshal(r, &rec); err != nil {
					continue
				}
				summaries = append(summaries, map[string]any{
					"label":       rec.Label,
					"taken_at":    rec.TakenAt,
					"num_classes": rec.NumClass,
				})
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(summaries) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no snapshots saved yet. Run 'schema snapshot' to create one.")
					return nil
				}
				return printAutoTable(cmd.OutOrStdout(), summaries)
			}
			return printJSONFiltered(cmd.OutOrStdout(), summaries, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum snapshots to list")
	return cmd
}
