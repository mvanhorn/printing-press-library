// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// export: deterministic, VCS-friendly dump of a mirrored container.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath, container, format string
	var flat bool

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Deterministic, version-control-friendly dump of a mirrored container",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Emit a mirrored container as deterministic JSON suitable for checking into
version control: entities grouped by kind, sorted by name, with volatile
metadata (fingerprint, path, ids) stripped so diffs stay clean. Use
--format gtm to keep raw entities grouped like a GTM container export.
Run 'pull' first.`,
		Example: `  # Flattened, git-friendly export
  google-tag-manager-pp-cli export --flat > container.json

  # Raw GTM-shaped export
  google-tag-manager-pp-cli export --format gtm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export the mirrored container")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, snaps, ok, err := gtmReadSnapshots(ctx, cmd, flags, gtmDBPath(dbPath))
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer s.Close()

			snap, err := resolveSnapshot(snaps, container)
			if err != nil {
				return usageErr(err)
			}
			entities, err := snapshotEntities(ctx, s.DB(), snap.ID, "")
			if err != nil {
				return err
			}

			if flat {
				format = "flat"
			}
			raw := format == "gtm"
			grouped := map[string][]json.RawMessage{}
			for _, e := range entities {
				data := e.Data
				if !raw {
					data = stripVolatile(e.Data)
				}
				grouped[e.Kind] = append(grouped[e.Kind], data)
			}
			// Sorting is by name; entities already arrive ordered by (kind,name).
			out := map[string]any{
				"container": displayName(snap),
				"publicId":  snap.PublicID,
				"source":    snap.Source,
				"versionId": snap.VersionID,
			}
			kinds := make([]string, 0, len(grouped))
			for k := range grouped {
				kinds = append(kinds, k)
			}
			sort.Strings(kinds)
			byKind := map[string][]json.RawMessage{}
			for _, k := range kinds {
				byKind[k] = grouped[k]
			}
			if raw {
				out["containerVersion"] = byKind
			} else {
				out["entities"] = byKind
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&container, "snapshot", "", "Snapshot ref: live, workspace:<id>, container:<id>, version:<id>, or a snapshot id (default: most recent)")
	cmd.Flags().StringVar(&format, "format", "flat", "Export format: flat (volatile fields stripped) | gtm (raw)")
	cmd.Flags().BoolVar(&flat, "flat", false, "Alias for --format flat (the default)")
	return cmd
}

func stripVolatile(data json.RawMessage) json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return data
	}
	for k := range m {
		if volatileFields[k] {
			delete(m, k)
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return data
	}
	return out
}
