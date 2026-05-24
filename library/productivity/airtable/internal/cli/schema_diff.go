// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newSchemaDiffCmd(flags *rootFlags) *cobra.Command {
	var cachePath string

	cmd := &cobra.Command{
		Use:         "diff <baseId>",
		Short:       "Compare cached schema against a fresh fetch; exit 1 on drift",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Compares ~/.cache/airtable-pp-cli/<baseId>/schema.json against a fresh
'bases get_schema' call. Reports added/removed/renamed tables and fields.
Exit code 1 on drift (CI-friendly).`,
		Example: strings.Trim(`
  # Diff cached vs live
  airtable-pp-cli schema diff appXXX

  # CI usage: exit-code 1 means schema drifted
  airtable-pp-cli schema diff appXXX --json || echo "drift detected"
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

			if cachePath == "" {
				home, _ := os.UserHomeDir()
				cachePath = filepath.Join(home, ".cache", "airtable-pp-cli", baseID, "schema.json")
			}

			cached, cachedErr := os.ReadFile(cachePath)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/meta/bases/{baseId}/tables", "baseId", baseID)
			live, err := c.Get(cmd.Context(), path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// First-time use (no cache): write live and report "initialized".
			if cachedErr != nil {
				_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
				_ = os.WriteFile(cachePath, live, 0o644)
				return flags.printJSON(cmd, map[string]any{
					"status":      "initialized",
					"cache_path":  cachePath,
					"description": "no cached schema found; wrote current schema as baseline",
				})
			}

			diff := diffSchemas(cached, live)
			if err := flags.printJSON(cmd, diff); err != nil {
				return err
			}
			if diff.HasDrift {
				return &cliError{code: 1, err: fmt.Errorf("schema drift detected")}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cachePath, "cache", "", "Path to cached schema (default: ~/.cache/airtable-pp-cli/<baseId>/schema.json)")
	return cmd
}

type schemaDiff struct {
	HasDrift      bool     `json:"has_drift"`
	AddedTables   []string `json:"added_tables,omitempty"`
	RemovedTables []string `json:"removed_tables,omitempty"`
	AddedFields   []string `json:"added_fields,omitempty"`
	RemovedFields []string `json:"removed_fields,omitempty"`
}

func diffSchemas(a, b []byte) schemaDiff {
	old, _ := parseSchema(a)
	new_, _ := parseSchema(b)

	oldTables := map[string]map[string]bool{}
	for _, t := range old {
		fields := map[string]bool{}
		for _, f := range t.Fields {
			fields[f.ID] = true
		}
		oldTables[t.ID] = fields
	}
	newTables := map[string]map[string]bool{}
	for _, t := range new_ {
		fields := map[string]bool{}
		for _, f := range t.Fields {
			fields[f.ID] = true
		}
		newTables[t.ID] = fields
	}

	var d schemaDiff
	for id := range newTables {
		if _, ok := oldTables[id]; !ok {
			d.AddedTables = append(d.AddedTables, id)
		}
	}
	for id, fields := range oldTables {
		nf, ok := newTables[id]
		if !ok {
			d.RemovedTables = append(d.RemovedTables, id)
			continue
		}
		for f := range nf {
			if !fields[f] {
				d.AddedFields = append(d.AddedFields, id+"."+f)
			}
		}
		for f := range fields {
			if !nf[f] {
				d.RemovedFields = append(d.RemovedFields, id+"."+f)
			}
		}
	}
	sort.Strings(d.AddedTables)
	sort.Strings(d.RemovedTables)
	sort.Strings(d.AddedFields)
	sort.Strings(d.RemovedFields)
	d.HasDrift = len(d.AddedTables)+len(d.RemovedTables)+len(d.AddedFields)+len(d.RemovedFields) > 0
	return d
}

// silence unused-import warning if json removed in future edits
var _ = json.Valid
