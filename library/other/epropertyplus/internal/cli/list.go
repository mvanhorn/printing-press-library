// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// Hand-built novel command: `list` — top-level convenience over the index.
// --kind all prints the lightweight index rows; --kind structure|lot hydrates
// detail (store-first, else live) and classifies.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/landbank"

	"github.com/spf13/cobra"
)

func newListCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List an instance's inventory, optionally filtered to structures or vacant lots",
		Long: strings.Trim(`
List inventory for the active land-bank instance.

  --kind all        (default) print the lightweight index rows (id, lat, lng)
  --kind structure  hydrate detail and keep only buildings
  --kind lot        hydrate detail and keep only vacant lots

Classification rule:
  structure = structureType is non-empty OR propertyClass does NOT contain
              "Vacant" (case-insensitive); lot = otherwise.

Structure/lot filtering requires full detail. The synced local store is used
when present (run 'sync' first); otherwise detail is hydrated live up to
--limit to avoid hammering the full catalogue.
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  epropertyplus-pp-cli list --kind all --limit 20
  epropertyplus-pp-cli list --kind structure --limit 20 --json
  epropertyplus-pp-cli --instance kclb list --kind lot --json --select id,zonedAs
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch strings.ToLower(kind) {
			case "", "all", "structure", "lot":
			default:
				return usageErr(fmt.Errorf("invalid --kind %q: must be all, structure, or lot", kind))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// --kind all: just the index rows, no hydration.
			if kind == "" || strings.EqualFold(kind, "all") {
				idx, err := fetchIndex(c)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				rows := idx.Rows
				if limit > 0 && len(rows) > limit {
					rows = rows[:limit]
				}
				out := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					m := map[string]any{"id": r.ID.String()}
					if r.Lat != nil {
						m["lat"] = *r.Lat
					}
					if r.Lng != nil {
						m["lng"] = *r.Lng
					}
					out = append(out, m)
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// structure | lot: hydrate + classify.
			loaded, source, err := loadProperties(cmd.Context(), c, flags, limit, kind)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "%d %s(s) (%s)\n", len(loaded), landbank.Kind(strings.ToLower(kind)), source)
			}
			return emitProperties(cmd, flags, loaded)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "all", "Filter by classification: all, structure, or lot")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max properties to hydrate when classifying live (and max rows for --kind all)")
	return cmd
}
