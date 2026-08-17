// Copyright 2026 SomSamantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: schema diff.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source computed
func newNovelSchemaDiffCmd(flags *rootFlags) *cobra.Command {
	var flagAgainst string

	cmd := &cobra.Command{
		Use:         "diff <className>",
		Short:       "Diff a collection's live config against a saved snapshot or another collection.",
		Example:     "  weaviate-collections-pp-cli schema diff Article --against pre-migration",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff collection config against a snapshot or another collection")
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("className is required"))
			}
			className := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			liveCls, err := fetchOneClass(ctx, flags, className)
			if err != nil {
				return err
			}

			var againstCls map[string]any
			var againstLabel string

			if flagAgainst != "" {
				// Try a saved snapshot label first; fall back to treating the
				// value as another live collection name.
				st, err := openNovelStore(ctx, flags)
				if err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				defer st.Close()
				raw, snapErr := st.Get(schemaSnapshotResourceType, flagAgainst)
				if snapErr == nil && raw != nil {
					var rec schemaSnapshotRecord
					if err := json.Unmarshal(raw, &rec); err != nil {
						return fmt.Errorf("parsing snapshot %q: %w", flagAgainst, err)
					}
					for _, c := range rec.Classes {
						if classNameOf(c) == className {
							againstCls = c
							againstLabel = fmt.Sprintf("snapshot %q", flagAgainst)
							break
						}
					}
					if againstCls == nil {
						return fmt.Errorf("snapshot %q exists but has no config for collection %q", flagAgainst, className)
					}
				} else {
					liveOther, err := fetchOneClass(ctx, flags, flagAgainst)
					if err != nil {
						return fmt.Errorf("--against %q is neither a saved snapshot label nor an existing collection: %w", flagAgainst, err)
					}
					againstCls = liveOther
					againstLabel = fmt.Sprintf("collection %q", flagAgainst)
				}
			} else {
				// Default: diff against the most recent snapshot that contains this collection.
				st, err := openNovelStore(ctx, flags)
				if err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				defer st.Close()
				raw, listErr := st.List(schemaSnapshotResourceType, 200)
				if listErr != nil {
					return fmt.Errorf("listing snapshots: %w", listErr)
				}
				var latest schemaSnapshotRecord
				for _, r := range raw {
					var rec schemaSnapshotRecord
					if err := json.Unmarshal(r, &rec); err != nil {
						continue
					}
					for _, c := range rec.Classes {
						if classNameOf(c) == className && rec.TakenAt > latest.TakenAt {
							latest = rec
						}
					}
				}
				if latest.TakenAt == "" {
					return fmt.Errorf("no saved snapshot contains collection %q; pass --against <label|collection> or run 'schema snapshot' first", className)
				}
				for _, c := range latest.Classes {
					if classNameOf(c) == className {
						againstCls = c
					}
				}
				againstLabel = fmt.Sprintf("snapshot %q (most recent)", latest.Label)
			}

			diffs := diffJSON(againstCls, liveCls, "")

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(diffs) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: no drift vs %s\n", className, againstLabel)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s vs %s: %d difference(s)\n", className, againstLabel, len(diffs))
				for _, d := range diffs {
					switch d.Kind {
					case "added":
						fmt.Fprintf(cmd.OutOrStdout(), "  + %s = %v\n", d.Path, d.After)
					case "removed":
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s (was %v)\n", d.Path, d.Before)
					default:
						fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s: %v -> %v\n", d.Path, d.Before, d.After)
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"collection": className,
				"against":    againstLabel,
				"diffs":      diffs,
				"drifted":    len(diffs) > 0,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagAgainst, "against", "", "Snapshot label or another collection name to diff against (default: most recent snapshot containing this collection)")
	return cmd
}
