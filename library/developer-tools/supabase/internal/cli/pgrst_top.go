// PATCH: novel pgrst-schema introspection wrapping /v1/projects/{ref}/api/rest for typed query planning.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newPgrstCmd is the top-level `pgrst` parent for PostgREST-related queries.
// Currently exposes 'schema' (the novel typed-schema fetch via Management API);
// future expansion may add row CRUD wrappers (currently a known gap).
func newPgrstCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pgrst",
		Short: "PostgREST helpers (schema introspection via Management API)",
		Long: `Per-project PostgREST helpers. Currently ships 'pgrst schema' which fetches the
project's PostgREST OpenAPI through the Management API — the documented
replacement for the anon-key /rest/v1/ OpenAPI fetch being removed April 2026.

Row CRUD (select/insert/upsert/delete) is a documented known gap; use supabase-js
or curl against /rest/v1/<table> for now.`,
	}
	cmd.AddCommand(newPgrstSchemaCmd(flags))
	return cmd
}

func newPgrstSchemaCmd(flags *rootFlags) *cobra.Command {
	var projectRef string
	var tableFilter string

	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Fetch per-project PostgREST schema (tables, columns, types) via Management API",
		Long: `Calls Management API GET /v1/projects/{ref}/api/rest, parses the returned
OpenAPI document, and lists tables with their columns and types. Use --table
to drill into a single table. Requires SUPABASE_ACCESS_TOKEN (Management PAT).

This is the documented replacement for the legacy anon-key /rest/v1/ OpenAPI
fetch being removed April 2026.`,
		Example: strings.Trim(`
  # List all tables and column counts for the current project
  supabase-pp-cli pgrst schema --json

  # Drill into a specific table
  supabase-pp-cli pgrst schema --table profiles --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Resolve project ref: --project-ref flag wins, else parse SUPABASE_URL.
			if projectRef == "" {
				if envURL := os.Getenv("SUPABASE_URL"); envURL != "" {
					projectRef = parseProjectRef(envURL)
				}
			}
			if projectRef == "" {
				return configErr(fmt.Errorf("project ref required; pass --project-ref <ref> or set SUPABASE_URL=https://<ref>.supabase.co"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// pp:client-call — real Management API GET (request timeout enforced
			// via the http.Client built from --timeout in flags.newClient).
			path := fmt.Sprintf("/v1/projects/%s/api/rest", projectRef)
			raw, err := c.Get(path, nil)
			if err != nil {
				return apiErr(fmt.Errorf("fetching PostgREST schema for project %s: %w", projectRef, err))
			}

			// Parse the OpenAPI doc.
			var doc struct {
				OpenAPI     string                     `json:"openapi"`
				Definitions map[string]json.RawMessage `json:"definitions"`
				Components  struct {
					Schemas map[string]json.RawMessage `json:"schemas"`
				} `json:"components"`
				Paths map[string]json.RawMessage `json:"paths"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				return fmt.Errorf("parsing schema response: %w", err)
			}

			// PostgREST OpenAPI uses "definitions" for tables in Swagger 2.0
			// shape; OpenAPI 3 puts them under components.schemas.
			schemas := doc.Definitions
			if len(schemas) == 0 {
				schemas = doc.Components.Schemas
			}

			type col struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				Format   string `json:"format,omitempty"`
				Nullable bool   `json:"nullable"`
			}
			type tbl struct {
				Name    string `json:"name"`
				Columns []col  `json:"columns"`
			}
			var tables []tbl
			for tname, raw := range schemas {
				if tableFilter != "" && tname != tableFilter {
					continue
				}
				var tdef struct {
					Type       string   `json:"type"`
					Required   []string `json:"required"`
					Properties map[string]struct {
						Type     any    `json:"type"`
						Format   string `json:"format"`
						Nullable bool   `json:"nullable"`
					} `json:"properties"`
				}
				if err := json.Unmarshal(raw, &tdef); err != nil {
					continue
				}
				if tdef.Properties == nil {
					continue
				}
				requiredSet := map[string]bool{}
				for _, r := range tdef.Required {
					requiredSet[r] = true
				}
				var cols []col
				for cname, cdef := range tdef.Properties {
					typeStr := ""
					switch t := cdef.Type.(type) {
					case string:
						typeStr = t
					case []any:
						parts := make([]string, 0, len(t))
						for _, p := range t {
							if s, ok := p.(string); ok && s != "null" {
								parts = append(parts, s)
							}
						}
						typeStr = strings.Join(parts, "|")
					}
					cols = append(cols, col{
						Name:     cname,
						Type:     typeStr,
						Format:   cdef.Format,
						Nullable: !requiredSet[cname] || cdef.Nullable,
					})
				}
				tables = append(tables, tbl{Name: tname, Columns: cols})
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"project_ref": projectRef,
					"table_count": len(tables),
					"tables":      tables,
				}, flags)
			}
			if len(tables) == 0 {
				if tableFilter != "" {
					fmt.Fprintf(out, "Table %q not found in project %s schema.\n", tableFilter, projectRef)
				} else {
					fmt.Fprintf(out, "No tables found in project %s schema.\n", projectRef)
				}
				return nil
			}
			if tableFilter != "" {
				t := tables[0]
				fmt.Fprintf(out, "Table: %s (%d columns)\n\n", t.Name, len(t.Columns))
				fmt.Fprintf(out, "%-30s %-15s %s\n", "COLUMN", "TYPE", "NULLABLE")
				fmt.Fprintf(out, "%-30s %-15s %s\n", "------", "----", "--------")
				for _, c := range t.Columns {
					fmt.Fprintf(out, "%-30s %-15s %t\n", truncate(c.Name, 28), truncate(c.Type, 13), c.Nullable)
				}
			} else {
				fmt.Fprintf(out, "Project %s schema: %d table(s)\n\n", projectRef, len(tables))
				fmt.Fprintf(out, "%-40s %s\n", "TABLE", "COLUMNS")
				fmt.Fprintf(out, "%-40s %s\n", "-----", "-------")
				for _, t := range tables {
					fmt.Fprintf(out, "%-40s %d\n", truncate(t.Name, 38), len(t.Columns))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&projectRef, "project-ref", "", "Project ref (default: parsed from SUPABASE_URL)")
	cmd.Flags().StringVar(&tableFilter, "table", "", "Drill into one table (show its columns)")
	return cmd
}
