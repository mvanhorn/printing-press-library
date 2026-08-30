// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: export — dump cached marketplace data to JSON/CSV.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

// exportAPILimit is passed to SearchApis for the default (unfiltered) api
// export so it isn't capped at SearchApis' own interactive-style default of
// 50 — the marketplace has 79k+ APIs, and export's whole purpose is
// dumping everything cached, matching the uncapped generic resource-type
// export path below.
const exportAPILimit = 1_000_000

func newExportCmd(flags *rootFlags) *cobra.Command {
	var format string
	var out string
	var resource string

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export cached marketplace data (apis, categories, collections) to JSON or CSV",
		Long:        "Export locally cached marketplace records to a file (JSON or CSV). Reads from the local store, so it works offline after a prior search.",
		Example:     "  rapidapi-pp-cli export --resource api --format json --out apis.json\n  rapidapi-pp-cli export --resource api --format csv --out apis.csv",
		Annotations: map[string]string{"pp:export": "true", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if resource == "" {
				resource = "api"
			}
			if format == "" {
				format = "json"
			}
			if out == "" {
				out = resource + "-export." + format
			}
			if rp, err := resourceReadPath(resource); err == nil && rp != "" {
				_ = rp
			}
			s, err := store.OpenWithContext(cmd.Context(), learnDBPath(""))
			if err != nil {
				return err
			}
			defer s.Close()

			// Prefer the typed domain search when exporting APIs: it returns
			// structured records from the local cache (offline-capable).
			if resource == "api" {
				// SearchApis treats limit<=0 as its own default page size
				// (50), not "unlimited" — that default exists for
				// interactive-style callers, but this command's whole
				// purpose is exporting everything cached (the generic
				// resource-type branch below has no cap at all). Pass an
				// explicit large limit so an api export isn't silently
				// truncated the way an empty query used to return zero
				// rows before this fix.
				rows, err := s.SearchApis("", exportAPILimit)
				if err != nil {
					return err
				}
				items := make([]map[string]any, 0, len(rows))
				for _, raw := range rows {
					var m map[string]any
					if json.Unmarshal(raw, &m) == nil {
						items = append(items, m)
					}
				}
				return writeExportFile(cmd, out, format, items)
			}

			rows, err := s.Query("SELECT id, data FROM resources WHERE resource_type = ? ORDER BY id", resource)
			if err != nil {
				return err
			}
			defer rows.Close()

			items := []map[string]any{}
			for rows.Next() {
				var id string
				var raw []byte
				if err := rows.Scan(&id, &raw); err != nil {
					return err
				}
				var m map[string]any
				if json.Unmarshal(raw, &m) == nil {
					m["id"] = id
					items = append(items, m)
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil && filepath.Dir(out) != "." {
				return err
			}
			// #nosec G304 -- `out` is a user-supplied export destination flag,
			// not attacker-controlled input; the CLI writes where the user asks.
			f, err := os.Create(out)
			if err != nil {
				return err
			}
			defer f.Close()

			switch format {
			case "json":
				enc := json.NewEncoder(f)
				enc.SetIndent("", "  ")
				if err := enc.Encode(items); err != nil {
					return err
				}
			case "csv":
				if len(items) > 0 {
					// Header from first item keys
					keys := make([]string, 0, len(items[0]))
					for k := range items[0] {
						keys = append(keys, k)
					}
					for _, k := range keys {
						if k != "id" {
							fmt.Fprintf(f, "%s,", k)
						}
					}
					fmt.Fprintln(f, "id")
					for _, it := range items {
						for _, k := range keys {
							if k != "id" {
								fmt.Fprintf(f, "%v,", it[k])
							}
						}
						fmt.Fprintf(f, "%v\n", it["id"])
					}
				}
			default:
				return fmt.Errorf("unsupported format %q (use json or csv)", format)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported %d %s records to %s\n", len(items), resource, out)
			return nil
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "api", "Resource type to export (api, category, collection)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format (json or csv)")
	cmd.Flags().StringVar(&out, "out", "", "Output file path (default <resource>-export.<format>)")

	return cmd
}

// writeExportFile writes export items to a file in the given format.
func writeExportFile(cmd *cobra.Command, out, format string, items []map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil && filepath.Dir(out) != "." {
		return err
	}
	// #nosec G304 -- `out` is a user-supplied export destination flag,
	// not attacker-controlled input; the CLI writes where the user asks.
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	switch format {
	case "json":
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(items); err != nil {
			return err
		}
	case "csv":
		if len(items) > 0 {
			keys := make([]string, 0, len(items[0]))
			for k := range items[0] {
				keys = append(keys, k)
			}
			for _, k := range keys {
				if k != "id" {
					fmt.Fprintf(f, "%s,", k)
				}
			}
			fmt.Fprintln(f, "id")
			for _, it := range items {
				for _, k := range keys {
					if k != "id" {
						fmt.Fprintf(f, "%v,", it[k])
					}
				}
				fmt.Fprintf(f, "%v\n", it["id"])
			}
		}
	default:
		return fmt.Errorf("unsupported format %q (use json or csv)", format)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Exported %d %s records to %s\n", len(items), "cached", out)
	return nil
}
