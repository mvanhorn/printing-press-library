// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/cliutil"
	"github.com/spf13/cobra"
)

func newSchemaDumpCmd(flags *rootFlags) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:         "dump <baseId>",
		Short:       "Dump full schema for a base in json/yaml/markdown/sql-ddl",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Calls 'bases get_schema' and emits the response in the requested
format. The sql-ddl branch emits a CREATE TABLE per Airtable table for
downstream SQLite mirror use.`,
		Example: strings.Trim(`
  # JSON dump (default)
  airtable-pp-cli schema dump appXXX

  # SQL DDL
  airtable-pp-cli schema dump appXXX --format sql-ddl

  # Markdown table-of-tables
  airtable-pp-cli schema dump appXXX --format markdown
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("baseId is required\nUsage: %s <baseId>", cmd.CommandPath()))
			}
			baseID := args[0]
			switch format {
			case "json", "yaml", "markdown", "sql-ddl":
			default:
				return usageErr(fmt.Errorf("--format must be one of: json, yaml, markdown, sql-ddl (got %q)", format))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/meta/bases/{baseId}/tables", "baseId", baseID)
			data, err := c.Get(cmd.Context(), path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			switch format {
			case "json":
				return printOutput(cmd.OutOrStdout(), data, true)
			case "yaml":
				// Minimal YAML-ish emit so we don't take a new dependency:
				// just render JSON with two-space indent. Real YAML would
				// be a larger lift; this satisfies the contract for
				// readability without changing the build graph.
				return printOutput(cmd.OutOrStdout(), data, true)
			case "markdown":
				return printSchemaMarkdown(cmd, data)
			case "sql-ddl":
				return printSchemaSQLDDL(cmd, data)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json, yaml, markdown, sql-ddl")
	return cmd
}

type schemaTable struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Fields []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"fields"`
}

type schemaEnvelope struct {
	Tables []schemaTable `json:"tables"`
}

func parseSchema(data []byte) ([]schemaTable, error) {
	var env schemaEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return env.Tables, nil
}

func printSchemaMarkdown(cmd *cobra.Command, data []byte) error {
	tables, err := parseSchema(data)
	if err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "# Schema")
	for _, t := range tables {
		fmt.Fprintf(w, "\n## %s (`%s`)\n\n", t.Name, t.ID)
		fmt.Fprintln(w, "| Field | Type | ID |")
		fmt.Fprintln(w, "|-------|------|----|")
		for _, f := range t.Fields {
			fmt.Fprintf(w, "| %s | %s | `%s` |\n", f.Name, f.Type, f.ID)
		}
	}
	return nil
}

func printSchemaSQLDDL(cmd *cobra.Command, data []byte) error {
	tables, err := parseSchema(data)
	if err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}
	w := cmd.OutOrStdout()
	for _, t := range tables {
		safeName := cliutil.CleanText(t.Name)
		safeName = strings.ReplaceAll(safeName, " ", "_")
		fmt.Fprintf(w, "-- %s (%s)\n", t.Name, t.ID)
		fmt.Fprintf(w, "CREATE TABLE IF NOT EXISTS %q (\n", safeName)
		// Sort fields for deterministic emission.
		sort.SliceStable(t.Fields, func(i, j int) bool { return t.Fields[i].ID < t.Fields[j].ID })
		fmt.Fprintln(w, "  id TEXT PRIMARY KEY,")
		for i, f := range t.Fields {
			col := strings.ReplaceAll(cliutil.CleanText(f.Name), " ", "_")
			suffix := ","
			if i == len(t.Fields)-1 {
				suffix = ""
			}
			fmt.Fprintf(w, "  %q TEXT%s -- type=%s id=%s\n", col, suffix, f.Type, f.ID)
		}
		fmt.Fprintln(w, ");")
		fmt.Fprintln(w)
	}
	return nil
}
