// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type fieldCoverage struct {
	Slug        string  `json:"slug"`
	DisplayName string  `json:"displayName,omitempty"`
	Type        string  `json:"type,omitempty"`
	Required    bool    `json:"required"`
	Filled      int     `json:"filled"`
	Empty       int     `json:"empty"`
	FillRate    float64 `json:"fillRate"`
	Issue       string  `json:"issue,omitempty"`
}

type completenessView struct {
	CollectionID   string          `json:"collectionId,omitempty"`
	CollectionName string          `json:"collectionName,omitempty"`
	ItemsScanned   int             `json:"itemsScanned"`
	FieldsScanned  int             `json:"fieldsScanned"`
	Source         string          `json:"source,omitempty"`
	Fields         []fieldCoverage `json:"fields"`
	RequiredGaps   int             `json:"requiredFieldGaps"`
	DeadFields     int             `json:"deadFields"`
	Note           string          `json:"note,omitempty"`
}

func newNovelCollectionsCompletenessCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var includeDrafts bool
	var maxScanPages int

	cmd := &cobra.Command{
		Use:   "completeness [collection-id]",
		Short: "Per-field fill rate across every item in a CMS collection",
		Long: strings.Trim(`
Use this command to measure per-field fill rates and required-but-empty fields
inside one CMS collection.

Do NOT use this command for page SEO metadata gaps; use 'seo audit' instead.

A field counts as filled when the item's fieldData carries a non-empty value
for it. Fields that are required but empty on any item are flagged as gaps;
fields empty on every item are flagged as dead schema.

Reads the local mirror when it holds this collection, otherwise fetches the
collection schema and items from the API.
`, "\n"),
		Example: strings.Trim(`
  webflow-pp-cli collections completeness 580e63fc8c9a982ac9b8b745 --agent
  webflow-pp-cli collections completeness 580e63fc8c9a982ac9b8b745 --json --select fields.slug,fields.fillRate
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// A wrong id and an unreachable data source are indistinguishable
			// here: both yield zero rows. Returning non-zero for the second
			// would make an un-synced, un-credentialed run look like a crash.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would compute per-field fill rates for a CMS collection")
			}

			lq, cleanup, ok, err := openLocalMirror(cmd, flags, dbPath, "sites,items")
			if err != nil {
				return err
			}
			defer cleanup()
			if !ok {
				return nil
			}

			collectionID := resolveCollectionID(args)
			if collectionID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a collection id is required (positionally or via WEBFLOW_COLLECTION_ID); without one, fields from one collection would be scored against another collection's items"))
			}

			var itemRows []rawRow
			source := "local-mirror"
			if lq.hasTable("items") {
				if itemRows, err = lq.selectRaw(
					`SELECT "id", "collections_id", "data" FROM "items" WHERE "collections_id" = ?`,
					collectionID); err != nil {
					return err
				}
			}
			var c liveFetcher
			if len(itemRows) == 0 && flags.dataSource != "local" {
				lc, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				c = lc
				if itemRows, err = lq.fetchPaged(c, "/collections/"+escapeSeg(collectionID)+"/items",
					"items", collectionID, maxScanPages); err != nil {
					if note, degrade := liveFallbackErr(err, "this collection's items"); degrade {
						return emitEmptyLocal(cmd, flags, dbPath, "items", completenessView{
							CollectionID: collectionID, Fields: make([]fieldCoverage, 0),
							Source: "unavailable", Note: note,
						})
					}
					return fmt.Errorf("fetching collection items: %w", err)
				}
				source = "api"
			}

			// Schema lookup runs only after the item rows are fully drained.
			// The table list is a closed literal set, so the %q identifier
			// interpolation below is not attacker-reachable.
			var schemaRows []rawRow
			for _, table := range []string{"sites_collections", "collections"} {
				if !lq.hasTable(table) {
					continue
				}
				found, qerr := lq.selectRaw(
					fmt.Sprintf(`SELECT "id", "id", "data" FROM %q WHERE "id" = ?`, table), collectionID)
				if qerr != nil {
					return qerr
				}
				if len(found) > 0 {
					schemaRows = found
					break
				}
			}
			// Generated sync cannot reach collections in this Press version, so
			// the field schema is normally absent. One GET gives us required
			// flags and field types, which is what makes the gap check possible.
			if len(schemaRows) == 0 && flags.dataSource != "local" {
				if c == nil {
					lc, cerr := flags.newClient()
					if cerr != nil {
						return cerr
					}
					c = lc
				}
				if schemaRows, err = lq.fetchOne(c, "/collections/"+escapeSeg(collectionID), collectionID); err != nil {
					if _, degrade := liveFallbackErr(err, "this collection's schema"); !degrade {
						return fmt.Errorf("fetching collection schema: %w", err)
					}
					// Schema is optional: fields are inferred from item data.
					schemaRows = nil
				}
			}

			view := buildCompleteness(collectionID, schemaRows, itemRows, includeDrafts)
			view.Source = source

			return emitLocalResult(cmd, flags, view, func() {
				if len(view.Fields) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
					return
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s — %d items, %d fields (%d required gaps, %d dead fields)\n\n",
					view.CollectionName, view.ItemsScanned, view.FieldsScanned, view.RequiredGaps, view.DeadFields)
				fmt.Fprintf(cmd.OutOrStdout(), "%-28s %8s %8s %8s  %s\n", "FIELD", "FILLED", "EMPTY", "RATE", "ISSUE")
				for _, f := range view.Fields {
					fmt.Fprintf(cmd.OutOrStdout(), "%-28s %8d %8d %7.0f%%  %s\n",
						truncate(f.Slug, 28), f.Filled, f.Empty, f.FillRate*100, f.Issue)
				}
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (defaults to the standard data directory)")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 10, "When fetching from the API rather than the mirror, maximum 100-item pages to scan")
	cmd.Flags().BoolVar(&includeDrafts, "include-drafts", false, "Count draft and archived items too (skipped by default)")
	return cmd
}

// buildCompleteness joins the collection's field schema against every item's
// values. Split out so it is testable without a store.
func buildCompleteness(collectionID string, schemaRows, itemRows []rawRow, includeDrafts bool) completenessView {
	view := completenessView{
		CollectionID: collectionID,
		Fields:       make([]fieldCoverage, 0, 16),
	}

	// Field schema, when collections were synced. Without it, fall back to the
	// union of keys actually present on the items so the command still answers.
	schema := map[string]wfField{}
	if len(schemaRows) > 0 {
		colls := decodeRows[wfCollection](schemaRows)
		if len(colls) > 0 {
			view.CollectionName = colls[0].DisplayName
			for _, f := range colls[0].Fields {
				if f.Slug != "" {
					schema[f.Slug] = f
				}
			}
		}
	}

	items := make([]wfItem, 0, len(itemRows))
	for _, r := range itemRows {
		decoded := decodeRows[wfItem]([]rawRow{r})
		if len(decoded) == 0 {
			continue
		}
		it := decoded[0]
		if !includeDrafts && (it.draft() || it.archived()) {
			continue
		}
		items = append(items, it)
	}
	view.ItemsScanned = len(items)

	if len(schema) == 0 {
		for _, it := range items {
			for k := range it.FieldData {
				if _, seen := schema[k]; !seen {
					schema[k] = wfField{Slug: k}
				}
			}
		}
		if len(schemaRows) == 0 {
			view.Note = "the collection schema was unavailable, so fields are inferred from item data and required-field gaps cannot be detected"
		}
	}
	view.FieldsScanned = len(schema)

	if view.ItemsScanned == 0 {
		if view.Note == "" {
			view.Note = "the collection has no items to measure"
		}
		return view
	}

	slugs := make([]string, 0, len(schema))
	for s := range schema {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)

	for _, slug := range slugs {
		f := schema[slug]
		cov := fieldCoverage{
			Slug:        slug,
			DisplayName: f.DisplayName,
			Type:        f.Type,
			Required:    f.required(),
		}
		for _, it := range items {
			if isFilledValue(it.FieldData[slug]) {
				cov.Filled++
			} else {
				cov.Empty++
			}
		}
		if view.ItemsScanned > 0 {
			cov.FillRate = float64(cov.Filled) / float64(view.ItemsScanned)
		}
		switch {
		case cov.Required && cov.Empty > 0:
			cov.Issue = fmt.Sprintf("required but empty on %d item(s)", cov.Empty)
			view.RequiredGaps++
		case cov.Filled == 0:
			cov.Issue = "empty on every item (dead schema field)"
			view.DeadFields++
		}
		view.Fields = append(view.Fields, cov)
	}

	sort.SliceStable(view.Fields, func(i, j int) bool {
		if (view.Fields[i].Issue != "") != (view.Fields[j].Issue != "") {
			return view.Fields[i].Issue != ""
		}
		if view.Fields[i].FillRate != view.Fields[j].FillRate {
			return view.Fields[i].FillRate < view.Fields[j].FillRate
		}
		return view.Fields[i].Slug < view.Fields[j].Slug
	})
	return view
}

// isFilledValue decides whether a fieldData value counts as populated. Webflow
// omits unset fields entirely on some field types and sends an explicit null,
// empty string, or empty collection on others.
func isFilledValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(t) != ""
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	case bool:
		// A boolean field is answered either way; false is a real value.
		return true
	default:
		return true
	}
}
