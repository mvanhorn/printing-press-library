// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
)

type cmsValidationReport struct {
	BrokenRefs []brokenRef `json:"broken_references"`
	OrphanIDs  []orphanID  `json:"orphan_items"`
	Summary    valSummary  `json:"summary"`
}

type brokenRef struct {
	SourceID         string `json:"source_id"`
	SourceSlug       string `json:"source_slug,omitempty"`
	SourceCollection string `json:"source_collection,omitempty"`
	Field            string `json:"field"`
	TargetID         string `json:"target_id"`
}

type orphanID struct {
	ID         string `json:"id"`
	Slug       string `json:"slug,omitempty"`
	Collection string `json:"collection,omitempty"`
}

type valSummary struct {
	TotalItems       int `json:"total_items"`
	TotalCollections int `json:"total_collections"`
	BrokenRefCount   int `json:"broken_references"`
	OrphanCount      int `json:"orphan_items"`
}

type cmsItem struct {
	ID         string
	Slug       string
	Collection string
	Fields     map[string]any
}

func newCmsValidateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var includeOrphans bool

	cmd := &cobra.Command{
		Use:   "cms-validate",
		Short: "Validate CMS referential integrity across collections",
		Long: strings.Trim(`
Check CMS referential integrity by scanning all collection items for broken
references. Reports:

  - Broken references: collectionReference or multiCollectionReference fields
    that point to items that don't exist in the local store
  - Orphan items (opt-in, --orphans): items not referenced by any other item.
    Off by default because top-level content (blog posts, products) is expected
    to be unreferenced and would otherwise flood the report.

Requires synced data — run 'framer-pp-cli sync' first.`, "\n"),
		Example: strings.Trim(`
  # Validate CMS integrity
  framer-pp-cli cms-validate

  # JSON output for automation
  framer-pp-cli cms-validate --json

  # With specific fields selected
  framer-pp-cli cms-validate --json --select broken_references,summary`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("framer-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'framer-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			report, err := validateCMSRefs(db, includeOrphans)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, report)
			}

			printValidationReport(cmd, report)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")
	cmd.Flags().BoolVar(&includeOrphans, "orphans", false, "Also report orphan items not referenced by any other item (off by default — top-level content is expected to be unreferenced and would flood the report)")

	return cmd
}

func validateCMSRefs(db *store.Store, includeOrphans bool) (*cmsValidationReport, error) {
	// Load all CMS items
	itemRows, err := db.List("cms-items", 0)
	if err != nil {
		return nil, fmt.Errorf("listing CMS items: %w", err)
	}

	// Load all collections
	collRows, err := db.List("cms-collections", 0)
	if err != nil {
		return nil, fmt.Errorf("listing CMS collections: %w", err)
	}

	// Build lookup maps

	itemByID := make(map[string]*cmsItem)
	referencedIDs := make(map[string]bool)
	collectionSet := make(map[string]bool)

	for _, raw := range collRows {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		if id, ok := obj["id"].(string); ok {
			collectionSet[id] = true
		}
	}

	for _, raw := range itemRows {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		id, _ := obj["id"].(string)
		if id == "" {
			continue
		}
		slug, _ := obj["slug"].(string)
		collection, _ := obj["collectionId"].(string)
		if collection == "" {
			collection, _ = obj["collection"].(string)
		}

		itemByID[id] = &cmsItem{
			ID:         id,
			Slug:       slug,
			Collection: collection,
			Fields:     obj,
		}
	}

	report := &cmsValidationReport{}

	// Check references
	for _, item := range itemByID {
		checkRefField(item, "collectionReference", itemByID, referencedIDs, &report.BrokenRefs)
		checkRefField(item, "multiCollectionReference", itemByID, referencedIDs, &report.BrokenRefs)

		// Also check any field that looks like a reference (ends with "Ref" or "Reference")
		for fieldName, fieldVal := range item.Fields {
			if fieldName == "collectionReference" || fieldName == "multiCollectionReference" {
				continue
			}
			if strings.HasSuffix(fieldName, "Ref") || strings.HasSuffix(fieldName, "Reference") {
				checkSingleRef(item, fieldName, fieldVal, itemByID, referencedIDs, &report.BrokenRefs)
			}
		}
	}

	// Find orphan items (not referenced by any other item). Off by default: in a
	// typical CMS, top-level content (blog posts, products, announcements) is
	// never referenced by other items and IS the authoritative content, so an
	// unconditional orphan scan flags nearly everything and drowns out the real
	// broken-reference signal. Opt in with --orphans when you specifically want
	// to audit for unreferenced items.
	if includeOrphans {
		for id, item := range itemByID {
			if !referencedIDs[id] {
				report.OrphanIDs = append(report.OrphanIDs, orphanID{
					ID:         id,
					Slug:       item.Slug,
					Collection: item.Collection,
				})
			}
		}
		sort.Slice(report.OrphanIDs, func(i, j int) bool {
			return report.OrphanIDs[i].ID < report.OrphanIDs[j].ID
		})
	}

	report.Summary = valSummary{
		TotalItems:       len(itemByID),
		TotalCollections: len(collectionSet),
		BrokenRefCount:   len(report.BrokenRefs),
		OrphanCount:      len(report.OrphanIDs),
	}

	return report, nil
}

func checkRefField(item *cmsItem, fieldName string, itemByID map[string]*cmsItem, referencedIDs map[string]bool, broken *[]brokenRef) {
	val, ok := item.Fields[fieldName]
	if !ok || val == nil {
		return
	}
	checkSingleRef(item, fieldName, val, itemByID, referencedIDs, broken)
}

func checkSingleRef(item *cmsItem, fieldName string, val any, itemByID map[string]*cmsItem, referencedIDs map[string]bool, broken *[]brokenRef) {
	switch v := val.(type) {
	case string:
		if v == "" {
			return
		}
		referencedIDs[v] = true
		if _, exists := itemByID[v]; !exists {
			*broken = append(*broken, brokenRef{
				SourceID:         item.ID,
				SourceSlug:       item.Slug,
				SourceCollection: item.Collection,
				Field:            fieldName,
				TargetID:         v,
			})
		}
	case []any:
		for _, ref := range v {
			if refStr, ok := ref.(string); ok && refStr != "" {
				referencedIDs[refStr] = true
				if _, exists := itemByID[refStr]; !exists {
					*broken = append(*broken, brokenRef{
						SourceID:         item.ID,
						SourceSlug:       item.Slug,
						SourceCollection: item.Collection,
						Field:            fieldName,
						TargetID:         refStr,
					})
				}
			}
		}
	}
}

func printValidationReport(cmd *cobra.Command, report *cmsValidationReport) {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "CMS Validation Report\n")
	fmt.Fprintf(w, "=====================\n\n")
	fmt.Fprintf(w, "Collections: %d\n", report.Summary.TotalCollections)
	fmt.Fprintf(w, "Items:       %d\n", report.Summary.TotalItems)
	fmt.Fprintln(w)

	if len(report.BrokenRefs) == 0 {
		fmt.Fprintln(w, "Broken References: none")
	} else {
		fmt.Fprintf(w, "Broken References: %d\n", len(report.BrokenRefs))
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "SOURCE\tSLUG\tFIELD\tMISSING TARGET")
		for _, br := range report.BrokenRefs {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				truncate(br.SourceID, 20),
				truncate(br.SourceSlug, 20),
				br.Field,
				truncate(br.TargetID, 20),
			)
		}
		_ = tw.Flush()
	}

	fmt.Fprintln(w)
	if len(report.OrphanIDs) == 0 {
		fmt.Fprintln(w, "Orphan Items: none")
	} else {
		fmt.Fprintf(w, "Orphan Items: %d (not referenced by any other item)\n", len(report.OrphanIDs))
		if len(report.OrphanIDs) <= 50 {
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "ID\tSLUG\tCOLLECTION")
			for _, o := range report.OrphanIDs {
				fmt.Fprintf(tw, "%s\t%s\t%s\n",
					truncate(o.ID, 20),
					truncate(o.Slug, 20),
					truncate(o.Collection, 20),
				)
			}
			_ = tw.Flush()
		} else {
			fmt.Fprintf(w, "  (showing first 10 of %d)\n", len(report.OrphanIDs))
			for _, o := range report.OrphanIDs[:10] {
				fmt.Fprintf(w, "  %s (%s)\n", o.ID, o.Slug)
			}
		}
	}
}
