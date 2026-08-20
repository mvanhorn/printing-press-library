// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #2 — snapshot tag / diff / list. Hand-authored.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Tag the current local-store state of a resource and diff any two tags.",
		Long:        "snapshot lets you bookmark the current local-store contents (with a label like 'monday-baseline'), then later compare any two tags to see exactly what changed between them.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSnapshotTagCmd(flags))
	cmd.AddCommand(newSnapshotDiffCmd(flags))
	cmd.AddCommand(newSnapshotListCmd(flags))
	return cmd
}

func newSnapshotTagCmd(flags *rootFlags) *cobra.Command {
	var resource string

	cmd := &cobra.Command{
		Use:         "tag [label]",
		Short:       "Tag the current local-store state of one or all resource types with a label.",
		Example:     "  semrush-pp-cli snapshot tag monday-baseline\n  semrush-pp-cli snapshot tag pre-launch --resource keyword",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			label := args[0]
			now := time.Now().Unix()

			var resourceTypes []string
			if resource != "" {
				resourceTypes = []string{resource}
			} else {
				rows, err := db.DB().QueryContext(ctx,
					`SELECT DISTINCT resource_type FROM resources`)
				if err != nil {
					return fmt.Errorf("listing resource types: %w", err)
				}
				defer rows.Close()
				for rows.Next() {
					var rt string
					if err := rows.Scan(&rt); err != nil {
						return fmt.Errorf("scan resource_type: %w", err)
					}
					resourceTypes = append(resourceTypes, rt)
				}
				if err := rows.Err(); err != nil {
					return fmt.Errorf("iterate resource types: %w", err)
				}
			}

			if len(resourceTypes) == 0 {
				return fmt.Errorf("no synced resources to tag — run 'semrush-pp-cli sync' first")
			}

			tagged := 0
			for _, rt := range resourceTypes {
				if _, err := db.DB().ExecContext(ctx,
					`INSERT INTO snapshot_labels (label, resource_type, taken_at) VALUES (?, ?, ?)
					 ON CONFLICT(label, resource_type) DO UPDATE SET taken_at = excluded.taken_at`,
					label, rt, now); err != nil {
					return fmt.Errorf("tagging %s: %w", rt, err)
				}
				tagged++
			}

			out := map[string]any{
				"label":          label,
				"taken_at":       now,
				"resource_types": resourceTypes,
				"tagged":         tagged,
			}
			raw, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "", "Tag only one resource type (e.g. keyword, backlink); default: tag all currently-synced types")
	return cmd
}

func newSnapshotDiffCmd(flags *rootFlags) *cobra.Command {
	var resource string

	cmd := &cobra.Command{
		Use:         "diff [label-a] [label-b]",
		Short:       "Diff two snapshot labels — added / removed / changed resource IDs per type.",
		Example:     "  semrush-pp-cli snapshot diff monday-baseline today --resource keyword",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			a, b := args[0], args[1]

			labelTimes := func(label string) (map[string]int64, error) {
				out := map[string]int64{}
				rows, err := db.DB().QueryContext(ctx,
					`SELECT resource_type, taken_at FROM snapshot_labels WHERE label = ?`, label)
				if err != nil {
					return nil, err
				}
				defer rows.Close()
				for rows.Next() {
					var rt string
					var ts int64
					if err := rows.Scan(&rt, &ts); err != nil {
						return nil, err
					}
					out[rt] = ts
				}
				return out, rows.Err()
			}

			aTimes, err := labelTimes(a)
			if err != nil {
				return fmt.Errorf("loading label %q: %w", a, err)
			}
			bTimes, err := labelTimes(b)
			if err != nil {
				return fmt.Errorf("loading label %q: %w", b, err)
			}
			if len(aTimes) == 0 {
				return notFoundErr(fmt.Errorf("label %q has no resource_types — run 'snapshot tag %s' first", a, a))
			}
			if len(bTimes) == 0 {
				return notFoundErr(fmt.Errorf("label %q has no resource_types — run 'snapshot tag %s' first", b, b))
			}

			// Resource types to diff: intersection of both labels (or single
			// override). Sort the intersection so the JSON output's diff
			// section order is stable across runs — without this sort,
			// ranging over the aTimes Go map produces a different order
			// each invocation, breaking scripted diffs of two `snapshot
			// diff` outputs.
			var resourceTypes []string
			if resource != "" {
				resourceTypes = []string{resource}
			} else {
				for rt := range aTimes {
					if _, ok := bTimes[rt]; ok {
						resourceTypes = append(resourceTypes, rt)
					}
				}
				sort.Strings(resourceTypes)
			}

			idsAt := func(rt string, takenAt int64) (map[string]string, error) {
				out := map[string]string{}
				rows, err := db.DB().QueryContext(ctx,
					`SELECT id, data FROM resources
					 WHERE resource_type = ? AND CAST(strftime('%s', synced_at) AS INTEGER) <= ?`,
					rt, takenAt)
				if err != nil {
					return nil, err
				}
				defer rows.Close()
				for rows.Next() {
					var id, data string
					if err := rows.Scan(&id, &data); err != nil {
						return nil, err
					}
					out[id] = data
				}
				return out, rows.Err()
			}

			type diffSection struct {
				ResourceType string   `json:"resource_type"`
				Added        []string `json:"added"`
				Removed      []string `json:"removed"`
				Changed      []string `json:"changed"`
				AddedCount   int      `json:"added_count"`
				RemovedCount int      `json:"removed_count"`
				ChangedCount int      `json:"changed_count"`
			}

			result := struct {
				LabelA   string        `json:"label_a"`
				LabelB   string        `json:"label_b"`
				Sections []diffSection `json:"sections"`
			}{LabelA: a, LabelB: b}

			for _, rt := range resourceTypes {
				aIDs, err := idsAt(rt, aTimes[rt])
				if err != nil {
					return fmt.Errorf("loading snapshot a for %s: %w", rt, err)
				}
				bIDs, err := idsAt(rt, bTimes[rt])
				if err != nil {
					return fmt.Errorf("loading snapshot b for %s: %w", rt, err)
				}
				sec := diffSection{ResourceType: rt}
				for id, bData := range bIDs {
					aData, ok := aIDs[id]
					if !ok {
						sec.Added = append(sec.Added, id)
					} else if aData != bData {
						sec.Changed = append(sec.Changed, id)
					}
				}
				for id := range aIDs {
					if _, ok := bIDs[id]; !ok {
						sec.Removed = append(sec.Removed, id)
					}
				}
				// Sort each id array for deterministic JSON output — all
				// three are built by ranging over Go maps (aIDs/bIDs), so
				// without this sort the order within each diff section
				// varies between runs and scripted diff-of-diffs breaks.
				sort.Strings(sec.Added)
				sort.Strings(sec.Removed)
				sort.Strings(sec.Changed)
				sec.AddedCount = len(sec.Added)
				sec.RemovedCount = len(sec.Removed)
				sec.ChangedCount = len(sec.Changed)
				result.Sections = append(result.Sections, sec)
			}

			raw, err := json.Marshal(result)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "", "Diff only one resource type; default: diff every type present in both labels")
	return cmd
}

func newSnapshotListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all snapshot tags with their per-resource taken_at timestamps.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx,
				`SELECT label, resource_type, taken_at, created_at
				 FROM snapshot_labels
				 ORDER BY label, resource_type`)
			if err != nil {
				return fmt.Errorf("listing snapshots: %w", err)
			}
			defer rows.Close()

			type item struct {
				Label        string `json:"label"`
				ResourceType string `json:"resource_type"`
				TakenAt      int64  `json:"taken_at"`
				TakenAtISO   string `json:"taken_at_iso"`
				CreatedAt    int64  `json:"created_at"`
			}
			var items []item
			for rows.Next() {
				var it item
				if err := rows.Scan(&it.Label, &it.ResourceType, &it.TakenAt, &it.CreatedAt); err != nil {
					return fmt.Errorf("scan snapshot row: %w", err)
				}
				it.TakenAtISO = time.Unix(it.TakenAt, 0).UTC().Format(time.RFC3339)
				items = append(items, it)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate snapshots: %w", err)
			}

			raw, _ := json.Marshal(items)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
