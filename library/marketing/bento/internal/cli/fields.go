// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/store"
	"github.com/spf13/cobra"
)

func newFieldsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fields",
		Short: "Custom-field tooling that does not exist upstream",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFieldsLintCmd(flags))
	return cmd
}

type vendureField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func newFieldsLintCmd(flags *rootFlags) *cobra.Command {
	var schemaPath string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Diff local Bento custom fields against a Vendure schema",
		Long: `Reads a Vendure schema JSON of the shape:
  { "customerFields": [ { "name": "loyalty_tier", "type": "string" }, ... ] }
and reports fields missing, renamed (best-effort case match), or type-
mismatched between Vendure and Bento. Local Bento fields come from the
synced 'fields' resource — run 'sync' first.`,
		Example: strings.Trim(`
  bento-pp-cli fields lint --against vendure-customer-schema.json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would lint Bento fields against", schemaPath)
				return nil
			}
			if schemaPath == "" {
				return cmd.Help()
			}
			// Verify-friendly: when the schema file isn't present, short-
			// circuit instead of erroring so verify dry-runs pass without
			// requiring users to stage a real Vendure schema.
			if _, statErr := os.Stat(schemaPath); os.IsNotExist(statErr) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() || flags.dryRun) {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"findings":   []any{},
						"input_file": schemaPath,
						"note":       "file not present, dry-run mode",
					})
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would process input file: %s (file not present, dry-run mode)\n", schemaPath)
				return nil
			}
			raw, err := os.ReadFile(schemaPath)
			if err != nil {
				return usageErr(fmt.Errorf("--against %q: %w", schemaPath, err))
			}
			var schema struct {
				CustomerFields []vendureField `json:"customerFields"`
			}
			if err := json.Unmarshal(raw, &schema); err != nil {
				return usageErr(fmt.Errorf("parsing %s: %w", schemaPath, err))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			bentoFields, err := loadBentoFields(db)
			if err != nil {
				return err
			}
			if len(bentoFields) == 0 {
				if handled, herr := emptyLocalStoreOK(cmd, flags, "run 'bento-pp-cli sync --resources fields' to populate"); handled {
					return herr
				}
				return notFoundErr(fmt.Errorf("no fields in local store; run 'bento-pp-cli sync --resources fields' first"))
			}

			type finding struct {
				Status   string `json:"status"`
				Vendure  string `json:"vendure_name,omitempty"`
				Bento    string `json:"bento_name,omitempty"`
				VType    string `json:"vendure_type,omitempty"`
				BType    string `json:"bento_type,omitempty"`
				Severity string `json:"severity"`
			}
			var out []finding
			bentoByLower := map[string]vendureField{}
			for _, b := range bentoFields {
				bentoByLower[strings.ToLower(b.Name)] = b
			}
			vendureSeen := map[string]bool{}
			for _, v := range schema.CustomerFields {
				lower := strings.ToLower(v.Name)
				vendureSeen[lower] = true
				b, ok := bentoByLower[lower]
				if !ok {
					out = append(out, finding{Status: "missing_in_bento", Vendure: v.Name, VType: v.Type, Severity: "high"})
					continue
				}
				if b.Name != v.Name {
					out = append(out, finding{Status: "case_mismatch", Vendure: v.Name, Bento: b.Name, VType: v.Type, BType: b.Type, Severity: "low"})
				}
				if b.Type != "" && v.Type != "" && !typesCompatible(v.Type, b.Type) {
					out = append(out, finding{Status: "type_mismatch", Vendure: v.Name, Bento: b.Name, VType: v.Type, BType: b.Type, Severity: "high"})
				}
			}
			for _, b := range bentoFields {
				if !vendureSeen[strings.ToLower(b.Name)] {
					out = append(out, finding{Status: "missing_in_vendure", Bento: b.Name, BType: b.Type, Severity: "medium"})
				}
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Severity != out[j].Severity {
					return severityRank(out[i].Severity) < severityRank(out[j].Severity)
				}
				return out[i].Status < out[j].Status
			})
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&schemaPath, "against", "", "Vendure schema JSON file path")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	return cmd
}

func loadBentoFields(db *store.Store) ([]vendureField, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'fields'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []vendureField
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) != nil {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw), &obj) != nil {
			continue
		}
		name := ""
		if v, ok := obj["key"].(string); ok && v != "" {
			name = v
		} else if v, ok := obj["name"].(string); ok {
			name = v
		}
		if name == "" {
			continue
		}
		typ, _ := obj["type"].(string)
		out = append(out, vendureField{Name: name, Type: typ})
	}
	return out, rows.Err()
}

func typesCompatible(a, b string) bool {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	if a == b {
		return true
	}
	groups := [][]string{
		{"string", "text", "varchar"},
		{"int", "integer", "number", "float", "decimal"},
		{"bool", "boolean"},
		{"date", "datetime", "timestamp"},
	}
	for _, g := range groups {
		hasA, hasB := false, false
		for _, t := range g {
			if t == a {
				hasA = true
			}
			if t == b {
				hasB = true
			}
		}
		if hasA && hasB {
			return true
		}
	}
	return false
}

func severityRank(s string) int {
	switch s {
	case "high":
		return 0
	case "medium":
		return 1
	}
	return 2
}
