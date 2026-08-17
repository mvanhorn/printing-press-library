// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/store"
	"github.com/spf13/cobra"
)

// docaiSchemaEntry is one saved extraction schema.
type docaiSchemaEntry struct {
	Name      string          `json:"name"`
	Schema    json.RawMessage `json:"schema"`
	UpdatedAt string          `json:"updated_at,omitempty"`
}

const docaiSchemaResourceType = "docai_schema"

func openDocaiSchemaDB(cmd *cobra.Command) (*store.Store, error) {
	dbPath := defaultDBPath("sarvam-pp-cli")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: sarvam-pp-cli sync --db %s\n", dbPath, dbPath)
		return nil, nil
	}
	return store.OpenWithContext(cmd.Context(), dbPath)
}

func newNovelDocaiSchemaCmd(flags *rootFlags) *cobra.Command {
	var flagFile string

	cmd := &cobra.Command{
		Use:         "schema",
		Short:       "Save, list, and diff doc-ai extraction schemas locally",
		Example:     "  sarvam-pp-cli docai schema list\n  sarvam-pp-cli docai schema save invoice-v1 --file schema.json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	saveCmd := &cobra.Command{
		Use:         "save",
		Short:       "Save an extraction schema locally",
		Example:     "  sarvam-pp-cli docai schema save invoice-v1 --file schema.json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docai schema save")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional argument: schema name"))
			}
			name := args[0]
			if flagFile == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required to save a schema"))
			}
			// #nosec G304 -- flagFile is a user-supplied path to their own schema file.
			schemaBytes, err := os.ReadFile(flagFile)
			if err != nil {
				return usageErr(fmt.Errorf("reading schema file: %w", err))
			}
			if !json.Valid(schemaBytes) {
				return usageErr(fmt.Errorf("schema file %s is not valid JSON", flagFile))
			}
			db, err := openDocaiSchemaDB(cmd)
			if err != nil {
				return err
			}
			if db == nil {
				return apiErr(fmt.Errorf("no local database. Run 'sarvam-pp-cli sync' first"))
			}
			defer db.Close()
			entry := docaiSchemaEntry{Name: name, Schema: json.RawMessage(schemaBytes)}
			if err := db.Upsert(docaiSchemaResourceType, name, mustJSON(entry)); err != nil {
				return fmt.Errorf("saving schema: %w", err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entry, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved schema %q (%d bytes)\n", name, len(schemaBytes))
			return nil
		},
	}
	saveCmd.Flags().StringVar(&flagFile, "file", "", "Path to the extraction schema JSON file")
	cmd.AddCommand(saveCmd)

	listCmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved extraction schemas",
		Example:     "  sarvam-pp-cli docai schema list",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docai schema list")
			}
			db, err := openDocaiSchemaDB(cmd)
			if err != nil {
				return err
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), make([]docaiSchemaEntry, 0), flags)
			}
			defer db.Close()
			items, err := db.List(docaiSchemaResourceType, 0)
			if err != nil {
				return fmt.Errorf("listing schemas: %w", err)
			}
			entries := make([]docaiSchemaEntry, 0, len(items))
			for _, item := range items {
				var e docaiSchemaEntry
				if err := json.Unmarshal(item, &e); err == nil {
					entries = append(entries, e)
				}
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %d bytes\n", e.Name, len(e.Schema))
			}
			return nil
		},
	}
	cmd.AddCommand(listCmd)

	getCmd := &cobra.Command{
		Use:         "get",
		Short:       "Show a saved extraction schema",
		Example:     "  sarvam-pp-cli docai schema get invoice-v1",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docai schema get")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional argument: schema name"))
			}
			db, err := openDocaiSchemaDB(cmd)
			if err != nil {
				return err
			}
			if db == nil {
				return apiErr(fmt.Errorf("no local database. Run 'sarvam-pp-cli sync' first"))
			}
			defer db.Close()
			raw, err := db.Get(docaiSchemaResourceType, args[0])
			if err != nil {
				return notFoundErr(fmt.Errorf("schema %q not found", args[0]))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(raw), flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(raw))
			return nil
		},
	}
	cmd.AddCommand(getCmd)

	diffCmd := &cobra.Command{
		Use:         "diff",
		Short:       "Compare two saved extraction schemas",
		Example:     "  sarvam-pp-cli docai schema diff invoice-v1 invoice-v2",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docai schema diff")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional arguments: <name-a> <name-b>"))
			}
			db, err := openDocaiSchemaDB(cmd)
			if err != nil {
				return err
			}
			if db == nil {
				return apiErr(fmt.Errorf("no local database. Run 'sarvam-pp-cli sync' first"))
			}
			defer db.Close()
			rawA, err := db.Get(docaiSchemaResourceType, args[0])
			if err != nil {
				return notFoundErr(fmt.Errorf("schema %q not found", args[0]))
			}
			rawB, err := db.Get(docaiSchemaResourceType, args[1])
			if err != nil {
				return notFoundErr(fmt.Errorf("schema %q not found", args[1]))
			}
			var a, b map[string]any
			_ = json.Unmarshal(rawA, &a)
			_ = json.Unmarshal(rawB, &b)

			type diffEntry struct {
				Field   string `json:"field"`
				InA     any    `json:"in_a,omitempty"`
				InB     any    `json:"in_b,omitempty"`
				Changed bool   `json:"changed"`
			}
			diff := make([]diffEntry, 0)
			allKeys := map[string]bool{}
			for k := range a {
				allKeys[k] = true
			}
			for k := range b {
				allKeys[k] = true
			}
			keys := make([]string, 0, len(allKeys))
			for k := range allKeys {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				va, hasA := a[k]
				vb, hasB := b[k]
				if hasA && !hasB {
					diff = append(diff, diffEntry{Field: k, InA: va, Changed: true})
				} else if !hasA && hasB {
					diff = append(diff, diffEntry{Field: k, InB: vb, Changed: true})
				} else {
					ja, _ := json.Marshal(va)
					jb, _ := json.Marshal(vb)
					if string(ja) != string(jb) {
						diff = append(diff, diffEntry{Field: k, InA: va, InB: vb, Changed: true})
					}
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), diff, flags)
			}
			if len(diff) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "schemas are identical")
				return nil
			}
			for _, d := range diff {
				fmt.Fprintf(cmd.OutOrStdout(), "changed: %s\n", d.Field)
			}
			return nil
		},
	}
	cmd.AddCommand(diffCmd)

	deleteCmd := &cobra.Command{
		Use:         "delete",
		Short:       "Delete a saved extraction schema",
		Example:     "  sarvam-pp-cli docai schema delete invoice-v1",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docai schema delete")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional argument: schema name"))
			}
			db, err := openDocaiSchemaDB(cmd)
			if err != nil {
				return err
			}
			if db == nil {
				return apiErr(fmt.Errorf("no local database. Run 'sarvam-pp-cli sync' first"))
			}
			defer db.Close()
			_, err = db.DB().Exec("DELETE FROM resources WHERE resource_type = ? AND id = ?", docaiSchemaResourceType, args[0])
			if err != nil {
				return fmt.Errorf("deleting schema: %w", err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"deleted": args[0]}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted schema %q\n", args[0])
			return nil
		},
	}
	cmd.AddCommand(deleteCmd)

	return cmd
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
