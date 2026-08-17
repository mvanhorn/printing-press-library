// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// schemaFile is the top-level structure of a CMS schema definition file.
type schemaFile struct {
	Collections map[string]schemaCollection `yaml:"collections" json:"collections"`
}

// schemaCollection describes a single CMS collection in the schema file.
type schemaCollection struct {
	Fields []schemaField `yaml:"fields" json:"fields"`
}

// schemaField describes a single field within a CMS collection.
type schemaField struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
}

// schemaDiffRow represents one row of the diff output.
type schemaDiffRow struct {
	Collection string `json:"collection"`
	Field      string `json:"field"`
	Status     string `json:"status"` // "added", "removed", "type_mismatch", "new_collection"
	Expected   string `json:"expected,omitempty"`
	Actual     string `json:"actual,omitempty"`
}

func newCmsSchemaDefCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "cms-schema-diff <schema_file>",
		Short: "Compare a local YAML/JSON CMS schema definition against live Framer collection fields",
		Long: strings.Trim(`
Compare a local CMS schema definition (YAML or JSON) against the collection
fields stored in the local database from the last sync. Reports added fields,
removed fields, type mismatches, and entirely new collections.

The schema file declares the desired state of your CMS collections. This
command diffs it against the actual state to surface drift.`, "\n"),
		Example: strings.Trim(`
  # Diff a YAML schema against synced collections
  framer-pp-cli cms-schema-diff schema.yaml

  # JSON output for piping
  framer-pp-cli cms-schema-diff schema.yaml --json

  # Use a custom database path
  framer-pp-cli cms-schema-diff schema.json --db /path/to/data.db`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			schemaPath := args[0]

			// Read and parse the schema file.
			data, err := os.ReadFile(schemaPath)
			if err != nil {
				return fmt.Errorf("reading schema file: %w", err)
			}

			schema, err := parseSchemaFile(schemaPath, data)
			if err != nil {
				return fmt.Errorf("parsing schema file: %w", err)
			}

			if len(schema.Collections) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No collections found in schema file.")
				return nil
			}

			// Load existing collection fields from the local store.
			liveCollections, err := loadLiveCollections(cmd, dbPath)
			if err != nil {
				return err
			}

			// Compute the diff.
			diffs := computeSchemaDiff(schema, liveCollections)

			if len(diffs) == 0 {
				if flags.asJSON {
					return flags.printJSON(cmd, []schemaDiffRow{})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Schema is in sync. No differences found.")
				return nil
			}

			// Output.
			if flags.asJSON {
				return flags.printJSON(cmd, diffs)
			}

			headers := []string{"COLLECTION", "FIELD", "STATUS", "EXPECTED", "ACTUAL"}
			rows := make([][]string, len(diffs))
			for i, d := range diffs {
				rows[i] = []string{d.Collection, d.Field, d.Status, d.Expected, d.Actual}
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")

	return cmd
}

// parseSchemaFile detects the format by extension and parses the schema.
func parseSchemaFile(path string, data []byte) (*schemaFile, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var schema schemaFile

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &schema); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	default:
		// Try YAML first, then JSON.
		if err := yaml.Unmarshal(data, &schema); err != nil {
			if err2 := json.Unmarshal(data, &schema); err2 != nil {
				return nil, fmt.Errorf("could not parse as YAML (%v) or JSON (%v)", err, err2)
			}
		}
	}
	return &schema, nil
}

// liveCollection holds the fields extracted from a synced CMS collection.
type liveCollection struct {
	Fields map[string]string // field name → field type
}

// loadLiveCollections reads CMS collection data from the local SQLite store
// and extracts field definitions. Returns a map of collection name → fields.
func loadLiveCollections(cmd *cobra.Command, dbPath string) (map[string]*liveCollection, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("framer-pp-cli")
	}
	collections := map[string]*liveCollection{}

	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store for the live baseline: %w\nRun 'framer-pp-cli sync' first — without it every field is reported as new.", err)
	}
	defer db.Close()

	rows, err := db.List("cms-collections", 0)
	if err != nil {
		return nil, fmt.Errorf("reading collections from local store: %w", err)
	}

	for _, raw := range rows {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}

		name, _ := obj["name"].(string)
		if name == "" {
			// Try "title" as an alternative.
			name, _ = obj["title"].(string)
		}
		if name == "" {
			continue
		}

		coll := &liveCollection{Fields: map[string]string{}}

		// Extract fields from the collection object.
		if fieldsRaw, ok := obj["fields"]; ok {
			if fieldsList, ok := fieldsRaw.([]any); ok {
				for _, f := range fieldsList {
					if fMap, ok := f.(map[string]any); ok {
						fName, _ := fMap["name"].(string)
						fType, _ := fMap["type"].(string)
						if fName != "" {
							coll.Fields[fName] = fType
						}
					}
				}
			}
		}

		collections[name] = coll
	}

	return collections, nil
}

// computeSchemaDiff compares the desired schema against live collections.
func computeSchemaDiff(schema *schemaFile, live map[string]*liveCollection) []schemaDiffRow {
	var diffs []schemaDiffRow

	// Sort collection names for deterministic output.
	collNames := make([]string, 0, len(schema.Collections))
	for name := range schema.Collections {
		collNames = append(collNames, name)
	}
	sort.Strings(collNames)

	for _, collName := range collNames {
		schemaColl := schema.Collections[collName]
		liveColl, exists := live[collName]

		if !exists {
			// Entire collection is new.
			for _, f := range schemaColl.Fields {
				diffs = append(diffs, schemaDiffRow{
					Collection: collName,
					Field:      f.Name,
					Status:     "new_collection",
					Expected:   f.Type,
					Actual:     "",
				})
			}
			continue
		}

		// Track which live fields we've seen for removal detection.
		seenLive := map[string]bool{}

		// Check each schema field against live.
		for _, f := range schemaColl.Fields {
			liveType, fieldExists := liveColl.Fields[f.Name]
			seenLive[f.Name] = true

			if !fieldExists {
				diffs = append(diffs, schemaDiffRow{
					Collection: collName,
					Field:      f.Name,
					Status:     "added",
					Expected:   f.Type,
					Actual:     "",
				})
			} else if liveType != f.Type {
				diffs = append(diffs, schemaDiffRow{
					Collection: collName,
					Field:      f.Name,
					Status:     "type_mismatch",
					Expected:   f.Type,
					Actual:     liveType,
				})
			}
		}

		// Check for fields in live that are not in schema (removed).
		liveFieldNames := make([]string, 0, len(liveColl.Fields))
		for name := range liveColl.Fields {
			liveFieldNames = append(liveFieldNames, name)
		}
		sort.Strings(liveFieldNames)

		for _, name := range liveFieldNames {
			if !seenLive[name] {
				diffs = append(diffs, schemaDiffRow{
					Collection: collName,
					Field:      name,
					Status:     "removed",
					Expected:   "",
					Actual:     liveColl.Fields[name],
				})
			}
		}
	}

	return diffs
}
