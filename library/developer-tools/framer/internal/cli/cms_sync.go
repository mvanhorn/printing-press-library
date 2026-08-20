// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
)

type cmsSyncDiff struct {
	Added   []cmsSyncItem `json:"added"`
	Updated []cmsSyncItem `json:"updated"`
	Deleted []cmsSyncItem `json:"deleted"`
}

type cmsSyncItem struct {
	Slug          string            `json:"slug"`
	Fields        map[string]any    `json:"fields,omitempty"`
	ChangedFields map[string][2]any `json:"changed_fields,omitempty"` // [old, new]
}

func newCmsSyncCmd(flags *rootFlags) *cobra.Command {
	var collection string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "cms-sync <source_file>",
		Short: "Import CMS content from CSV/JSON and diff against local store",
		Long: strings.Trim(`
Import CMS content from a CSV or JSON file, compare it against the synced CMS
data in the local SQLite store, and show a diff of adds, updates, and deletes.

File format is detected by extension:
  .csv  — first row is headers; must include a "slug" column
  .json — array of objects; each must include a "slug" field

Use --dry-run to preview changes without modifying anything.
Without --dry-run, the diff is printed with a note that live push
requires FRAMER_API_KEY (not yet implemented).`, "\n"),
		Example: strings.Trim(`
  # Dry-run diff from a CSV file
  framer-pp-cli cms-sync content.csv --collection blog-posts --dry-run

  # Diff from a JSON file with JSON output
  framer-pp-cli cms-sync items.json --collection products --dry-run --json

  # Show what would change (live push not yet implemented)
  framer-pp-cli cms-sync data.csv --collection articles`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// NOTE: no early dryRunOK() guard here — cms-sync implements its own
			// dry-run by computing and printing the diff below and gating only the
			// live push on `!flags.dryRun`. An early return would make --dry-run
			// (the documented preview mode) emit nothing.
			if collection == "" {
				return usageErr(fmt.Errorf("--collection is required"))
			}

			sourceFile := args[0]
			incoming, err := parseSourceFile(sourceFile)
			if err != nil {
				return fmt.Errorf("parsing source file: %w", err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("framer-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'framer-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			existing, err := loadCMSItems(db, collection)
			if err != nil {
				return fmt.Errorf("loading existing CMS items: %w", err)
			}

			diff := computeCMSDiff(existing, incoming)

			if flags.asJSON {
				return flags.printJSON(cmd, diff)
			}

			printCMSDiffTable(cmd, diff)

			if !flags.dryRun {
				// Resolve collection name → ID from the local store
				collectionID, err := resolveCollectionID(db, collection)
				if err != nil {
					return fmt.Errorf("resolving collection ID: %w", err)
				}

				bc, err := client.NewBridgeClient()
				if err != nil {
					return err
				}

				// Push adds and updates via items-upsert
				upsertItems := make([]map[string]any, 0, len(diff.Added)+len(diff.Updated))
				for _, item := range diff.Added {
					fieldData := make(map[string]any)
					for k, v := range item.Fields {
						if k == "slug" {
							continue
						}
						fieldData[k] = map[string]any{"value": v}
					}
					upsertItems = append(upsertItems, map[string]any{
						"slug":      item.Slug,
						"fieldData": fieldData,
					})
				}
				for _, item := range diff.Updated {
					fieldData := make(map[string]any)
					for k, vals := range item.ChangedFields {
						fieldData[k] = map[string]any{"value": vals[1]}
					}
					upsertItems = append(upsertItems, map[string]any{
						"slug":      item.Slug,
						"fieldData": fieldData,
					})
				}

				if len(upsertItems) > 0 {
					payload := map[string]any{
						"collectionId": collectionID,
						"items":        upsertItems,
					}
					payloadBytes, _ := json.Marshal(payload)
					_, err := bc.Call("items-upsert", string(payloadBytes))
					if err != nil {
						return fmt.Errorf("upserting items: %w", err)
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "Upserted %d items.\n", len(upsertItems))
				}

				// Push deletes (with confirmation)
				if len(diff.Deleted) > 0 {
					if !flags.yes {
						fmt.Fprintf(cmd.ErrOrStderr(), "Delete %d items? [y/N] ", len(diff.Deleted))
						var confirm string
						fmt.Fscanln(cmd.InOrStdin(), &confirm)
						if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
							fmt.Fprintf(cmd.ErrOrStderr(), "Skipping deletes.\n")
							return nil
						}
					}

					// Resolve item slugs to IDs from the existing store data
					deleteIDs := make([]string, 0, len(diff.Deleted))
					for _, item := range diff.Deleted {
						if existingItem, ok := existing[item.Slug]; ok {
							if id, ok := existingItem["id"].(string); ok && id != "" {
								deleteIDs = append(deleteIDs, id)
							}
						}
					}

					if len(deleteIDs) > 0 {
						payload := map[string]any{
							"collectionId": collectionID,
							"ids":          deleteIDs,
						}
						payloadBytes, _ := json.Marshal(payload)
						_, err := bc.Call("items-remove", string(payloadBytes))
						if err != nil {
							return fmt.Errorf("removing items: %w", err)
						}
						fmt.Fprintf(cmd.ErrOrStderr(), "Removed %d items.\n", len(deleteIDs))
					}
				}

				// Refresh local store after push
				fmt.Fprintf(cmd.ErrOrStderr(), "Refreshing local store...\n")
				raw, err := bc.Call("sync-all")
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: post-push sync failed: %v\n", err)
				} else {
					_ = refreshLocalStore(db, raw)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&collection, "collection", "", "CMS collection name (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")

	return cmd
}

// parseSourceFile reads a CSV or JSON file and returns a map of slug -> fields.
func parseSourceFile(path string) (map[string]map[string]any, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return parseCSVFile(path)
	case ".json":
		return parseJSONFile(path)
	default:
		return nil, fmt.Errorf("unsupported file extension %q; use .csv or .json", ext)
	}
}

func parseCSVFile(path string) (map[string]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have a header row and at least one data row")
	}

	headers := records[0]
	slugIdx := -1
	for i, h := range headers {
		if strings.ToLower(strings.TrimSpace(h)) == "slug" {
			slugIdx = i
			break
		}
	}
	if slugIdx < 0 {
		return nil, fmt.Errorf("CSV must have a 'slug' column")
	}

	items := make(map[string]map[string]any, len(records)-1)
	for _, row := range records[1:] {
		if len(row) <= slugIdx {
			continue
		}
		slug := strings.TrimSpace(row[slugIdx])
		if slug == "" {
			continue
		}
		fields := make(map[string]any, len(headers))
		for i, h := range headers {
			if i < len(row) {
				fields[h] = row[i]
			}
		}
		items[slug] = fields
	}
	return items, nil
}

func parseJSONFile(path string) (map[string]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var records []map[string]any
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w (expected an array of objects)", err)
	}

	items := make(map[string]map[string]any, len(records))
	for _, rec := range records {
		slug, _ := rec["slug"].(string)
		if slug == "" {
			continue
		}
		items[slug] = rec
	}
	return items, nil
}

// loadCMSItems reads CMS items from the local store for a given collection.
func loadCMSItems(db *store.Store, collection string) (map[string]map[string]any, error) {
	rows, err := db.List("cms-items", 0)
	if err != nil {
		return nil, err
	}

	items := make(map[string]map[string]any)
	for _, raw := range rows {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}

		// Filter by collection if the item has a collectionId or collection field
		itemCollection, _ := obj["collectionId"].(string)
		if itemCollection == "" {
			itemCollection, _ = obj["collection"].(string)
		}
		// Match by collection name or ID
		if itemCollection != collection {
			collName, _ := obj["collectionName"].(string)
			if collName != collection {
				continue
			}
		}

		slug, _ := obj["slug"].(string)
		if slug == "" {
			continue
		}

		items[slug] = flattenStoredCMSItem(obj, slug)
	}
	return items, nil
}

// flattenStoredCMSItem converts a stored CMS item — whose field values are
// nested under fieldData as {field: {"value": v}} — into the flat {field: v}
// shape used by incoming file items, so computeCMSDiff compares like-for-like.
// Without this, every incoming field name resolves to nil on the stored item
// and every item is always reported as changed. Top-level id and slug are
// preserved because the delete path resolves item IDs from this map.
func flattenStoredCMSItem(obj map[string]any, slug string) map[string]any {
	flat := map[string]any{"slug": slug}
	if id, ok := obj["id"].(string); ok {
		flat["id"] = id
	}
	if fieldData, ok := obj["fieldData"].(map[string]any); ok {
		for k, wrapped := range fieldData {
			if m, ok := wrapped.(map[string]any); ok {
				flat[k] = m["value"]
			} else {
				flat[k] = wrapped
			}
		}
	}
	return flat
}

// computeCMSDiff compares existing (store) items against incoming (file) items.
func computeCMSDiff(existing, incoming map[string]map[string]any) cmsSyncDiff {
	var diff cmsSyncDiff

	// Find new and updated items
	slugs := make([]string, 0, len(incoming))
	for slug := range incoming {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	for _, slug := range slugs {
		incomingFields := incoming[slug]
		existingFields, exists := existing[slug]

		if !exists {
			diff.Added = append(diff.Added, cmsSyncItem{
				Slug:   slug,
				Fields: incomingFields,
			})
			continue
		}

		// Check for changed fields
		changed := make(map[string][2]any)
		for k, newVal := range incomingFields {
			if k == "slug" {
				continue
			}
			oldVal := existingFields[k]
			newStr := fmt.Sprintf("%v", newVal)
			oldStr := fmt.Sprintf("%v", oldVal)
			if newStr != oldStr {
				changed[k] = [2]any{oldVal, newVal}
			}
		}
		if len(changed) > 0 {
			diff.Updated = append(diff.Updated, cmsSyncItem{
				Slug:          slug,
				ChangedFields: changed,
			})
		}
	}

	// Find deleted items (in store but not in file)
	existingSlugs := make([]string, 0, len(existing))
	for slug := range existing {
		existingSlugs = append(existingSlugs, slug)
	}
	sort.Strings(existingSlugs)

	for _, slug := range existingSlugs {
		if _, exists := incoming[slug]; !exists {
			diff.Deleted = append(diff.Deleted, cmsSyncItem{
				Slug: slug,
			})
		}
	}

	return diff
}

// resolveCollectionID looks up the collection ID from the local store given a collection name.
// It checks both the "name" and "id" fields of cms-collections resources.
func resolveCollectionID(db *store.Store, collection string) (string, error) {
	rows, err := db.List("cms-collections", 0)
	if err != nil {
		return "", fmt.Errorf("listing collections: %w", err)
	}
	for _, raw := range rows {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		name, _ := obj["name"].(string)
		id, _ := obj["id"].(string)
		if name == collection || id == collection {
			if id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("collection %q not found in local store; run 'framer-pp-cli sync' first", collection)
}

// refreshLocalStore re-populates CMS data in the local database from a sync-all bridge response.
func refreshLocalStore(db *store.Store, raw json.RawMessage) error {
	var data struct {
		Collections []json.RawMessage `json:"collections"`
		Items       []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}

	upsert := func(resourceType string, items []json.RawMessage) (map[string]bool, error) {
		seen := make(map[string]bool)
		for _, item := range items {
			var obj map[string]any
			if json.Unmarshal(item, &obj) != nil {
				continue
			}
			id, _ := obj["id"].(string)
			if id == "" {
				id, _ = obj["name"].(string)
			}
			if id == "" {
				continue
			}
			seen[id] = true
			dataBytes, _ := json.Marshal(obj)
			if err := db.Upsert(resourceType, id, json.RawMessage(dataBytes)); err != nil {
				return nil, err
			}
		}
		return seen, nil
	}

	collectionIDs, err := upsert("cms-collections", data.Collections)
	if err != nil {
		return err
	}
	itemIDs, err := upsert("cms-items", data.Items)
	if err != nil {
		return err
	}
	return deleteMissingCMSRows(db, map[string]map[string]bool{
		"cms-collections": collectionIDs,
		"cms-items":       itemIDs,
	})
}

func deleteMissingCMSRows(db *store.Store, seenByType map[string]map[string]bool) error {
	for resourceType, seen := range seenByType {
		rows, err := db.DB().Query(`SELECT id FROM resources WHERE resource_type = ?`, resourceType)
		if err != nil {
			return err
		}
		var stale []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			if !seen[id] {
				stale = append(stale, id)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, id := range stale {
			if err := db.Delete(resourceType, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func printCMSDiffTable(cmd *cobra.Command, diff cmsSyncDiff) {
	w := cmd.OutOrStdout()

	total := len(diff.Added) + len(diff.Updated) + len(diff.Deleted)
	if total == 0 {
		fmt.Fprintln(w, "No changes detected.")
		return
	}

	fmt.Fprintf(w, "CMS Sync Diff: +%d added, ~%d updated, -%d deleted\n\n",
		len(diff.Added), len(diff.Updated), len(diff.Deleted))

	if len(diff.Added) > 0 {
		fmt.Fprintln(w, "ADDED:")
		for _, item := range diff.Added {
			fmt.Fprintf(w, "  + %s\n", item.Slug)
		}
		fmt.Fprintln(w)
	}

	if len(diff.Updated) > 0 {
		fmt.Fprintln(w, "UPDATED:")
		for _, item := range diff.Updated {
			fmt.Fprintf(w, "  ~ %s\n", item.Slug)
			for field, vals := range item.ChangedFields {
				fmt.Fprintf(w, "      %s: %v -> %v\n", field, vals[0], vals[1])
			}
		}
		fmt.Fprintln(w)
	}

	if len(diff.Deleted) > 0 {
		fmt.Fprintln(w, "DELETED:")
		for _, item := range diff.Deleted {
			fmt.Fprintf(w, "  - %s\n", item.Slug)
		}
		fmt.Fprintln(w)
	}
}
