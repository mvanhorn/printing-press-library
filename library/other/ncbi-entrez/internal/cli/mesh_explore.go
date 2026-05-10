package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// meshTree represents a stored MeSH term with hierarchy information.
type meshTree struct {
	Term        string `json:"term"`
	MeshID      string `json:"mesh_id"`
	TreeNumbers string `json:"tree_numbers"`
	ParentTerms string `json:"parent_terms"`
	SnapshotAt  string `json:"snapshot_at"`
}

func ensureMeshTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS mesh_trees (
			term TEXT NOT NULL,
			mesh_id TEXT NOT NULL,
			tree_numbers TEXT,
			parent_terms TEXT,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (term, mesh_id, snapshot_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mesh_term ON mesh_trees(term)`,
		`CREATE INDEX IF NOT EXISTS idx_mesh_snapshot ON mesh_trees(snapshot_at)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating mesh tables: %w", err)
		}
	}
	return nil
}

func newMeshExploreCmd(flags *rootFlags) *cobra.Command {
	var flagExplode bool
	var flagStore bool

	cmd := &cobra.Command{
		Use:   "mesh <term>",
		Short: "Explore MeSH hierarchy for a term",
		Long: strings.TrimSpace(`
MeSH Hierarchy Explorer -- searches the MeSH database for a term,
fetches tree numbers, and optionally stores the hierarchy for diff
analysis across snapshots.

Use --explode to show the full tree hierarchy and --store to save a snapshot.`),
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli mesh "Cardiovascular Diseases"
  ncbi-entrez-pp-cli mesh "Neoplasms" --explode
  ncbi-entrez-pp-cli mesh "Diabetes Mellitus" --store
  ncbi-entrez-pp-cli mesh diff --since 2025-01
  ncbi-entrez-pp-cli mesh list`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			term := strings.Join(args, " ")

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureMeshTables(db); err != nil {
				return err
			}

			// Step 1: ESearch MeSH database
			searchParams := map[string]string{
				"db":      "mesh",
				"term":    term,
				"retmax":  "20",
				"retmode": "json",
			}
			searchData, err := c.Get("/esearch.fcgi", searchParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			meshIDs := extractPMIDsFromEsearch(searchData)
			if len(meshIDs) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"status": "no_results",
					"term":   term,
				}, flags)
			}

			// Step 2: EFetch to get MeSH records
			fetchParams := map[string]string{
				"db":      "mesh",
				"id":      strings.Join(meshIDs, ","),
				"retmode": "json",
			}
			fetchData, err := c.Get("/efetch.fcgi", fetchParams)
			if err != nil {
				// EFetch may not support JSON for mesh; try XML
				fetchParams["retmode"] = "xml"
				fetchData, err = c.Get("/efetch.fcgi", fetchParams)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}

			// Parse MeSH records - extract tree numbers and descriptors
			meshRecords := parseMeshRecords(fetchData, meshIDs, term)

			// Store if requested
			if flagStore && len(meshRecords) > 0 {
				tx, err := db.DB().Begin()
				if err != nil {
					return err
				}
				for _, rec := range meshRecords {
					_, _ = tx.Exec(
						`INSERT INTO mesh_trees (term, mesh_id, tree_numbers, parent_terms, snapshot_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
						rec.Term, rec.MeshID, rec.TreeNumbers, rec.ParentTerms,
					)
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("storing mesh snapshot: %w", err)
				}
			}

			// Build result
			result := map[string]any{
				"term":     term,
				"mesh_ids": meshIDs,
				"records":  meshRecords,
				"stored":   flagStore,
			}

			if flagExplode && len(meshRecords) > 0 {
				// Show hierarchical tree information
				var treeInfo []map[string]any
				for _, rec := range meshRecords {
					if rec.TreeNumbers != "" {
						trees := strings.Split(rec.TreeNumbers, ";")
						for _, t := range trees {
							t = strings.TrimSpace(t)
							if t == "" {
								continue
							}
							parts := strings.Split(t, ".")
							depth := len(parts)
							treeInfo = append(treeInfo, map[string]any{
								"mesh_id":     rec.MeshID,
								"term":        rec.Term,
								"tree_number": t,
								"depth":       depth,
								"parent_tree": parentTreeNumber(t),
							})
						}
					}
				}
				if treeInfo == nil {
					treeInfo = []map[string]any{}
				}
				result["tree_hierarchy"] = treeInfo
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().BoolVar(&flagExplode, "explode", false, "Show full MeSH tree hierarchy")
	cmd.Flags().BoolVar(&flagStore, "store", false, "Store MeSH tree snapshot locally")

	cmd.AddCommand(newMeshDiffCmd(flags))
	cmd.AddCommand(newMeshListCmd(flags))

	return cmd
}

// treeNumberRe extracts <TreeNumber>C14.280.647</TreeNumber> values from MeSH XML.
var treeNumberRe = regexp.MustCompile(`<TreeNumber>([^<]+)</TreeNumber>`)

// descriptorNameRe extracts <String>Cardiovascular Diseases</String> inside <DescriptorName>.
var descriptorNameRe = regexp.MustCompile(`<DescriptorName>[^<]*<String>([^<]+)</String>`)

// descriptorUIRe extracts <DescriptorUI>D002318</DescriptorUI>.
var descriptorUIRe = regexp.MustCompile(`<DescriptorUI>([^<]+)</DescriptorUI>`)

// descriptorRecordRe splits the response into individual DescriptorRecord blocks.
var descriptorRecordRe = regexp.MustCompile(`(?s)<DescriptorRecord[^>]*>(.*?)</DescriptorRecord>`)

// parseMeshRecords extracts structured MeSH info from EFetch response.
// NCBI MeSH EFetch typically returns XML (not JSON), so this parser handles
// both XML <DescriptorRecord> blocks and JSON fallback shapes.
func parseMeshRecords(data json.RawMessage, meshIDs []string, searchTerm string) []meshTree {
	var records []meshTree

	raw := string(data)

	// Try XML parsing first — NCBI MeSH EFetch returns XML
	if strings.Contains(raw, "<DescriptorRecord") {
		// Unescape JSON-encoded XML if needed (the data arrives as a JSON string)
		unescaped := raw
		var s string
		if json.Unmarshal(data, &s) == nil {
			unescaped = s
		}

		blocks := descriptorRecordRe.FindAllStringSubmatch(unescaped, -1)
		for _, block := range blocks {
			body := block[1]
			rec := meshTree{Term: searchTerm}

			if m := descriptorUIRe.FindStringSubmatch(body); len(m) > 1 {
				rec.MeshID = m[1]
			}
			if m := descriptorNameRe.FindStringSubmatch(body); len(m) > 1 {
				rec.Term = m[1]
			}

			treeMatches := treeNumberRe.FindAllStringSubmatch(body, -1)
			var trees []string
			for _, tm := range treeMatches {
				if len(tm) > 1 {
					trees = append(trees, tm[1])
				}
			}
			rec.TreeNumbers = strings.Join(trees, ";")

			// Derive parent terms from tree numbers by computing parent tree codes
			var parents []string
			for _, t := range trees {
				parent := parentTreeNumber(t)
				if parent != "" {
					parents = append(parents, parent)
				}
			}
			rec.ParentTerms = strings.Join(parents, ";")

			records = append(records, rec)
		}

		if len(records) > 0 {
			return records
		}
	}

	// Try JSON parsing
	var resp map[string]json.RawMessage
	if json.Unmarshal(data, &resp) == nil {
		for _, key := range []string{"result", "records", "data"} {
			if raw, ok := resp[key]; ok {
				var items []map[string]json.RawMessage
				if json.Unmarshal(raw, &items) == nil {
					for _, item := range items {
						rec := meshTree{Term: searchTerm}
						if uid, ok := item["uid"]; ok {
							var id string
							json.Unmarshal(uid, &id)
							rec.MeshID = id
						}
						if tn, ok := item["ds_meshtreenumberlist"]; ok {
							var trees []string
							json.Unmarshal(tn, &trees)
							rec.TreeNumbers = strings.Join(trees, ";")
						}
						if tn, ok := item["ds_meshtermslist"]; ok {
							var terms []string
							json.Unmarshal(tn, &terms)
							rec.ParentTerms = strings.Join(terms, ";")
						}
						records = append(records, rec)
					}
					return records
				}
			}
		}
	}

	// Fallback: create basic records from the IDs
	for _, id := range meshIDs {
		records = append(records, meshTree{
			Term:   searchTerm,
			MeshID: id,
		})
	}

	return records
}

// parentTreeNumber returns the parent tree number (e.g., "C14.280" -> "C14").
func parentTreeNumber(treeNum string) string {
	lastDot := strings.LastIndex(treeNum, ".")
	if lastDot < 0 {
		return ""
	}
	return treeNum[:lastDot]
}

func newMeshDiffCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Compare MeSH tree snapshots over time",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  ncbi-entrez-pp-cli mesh diff --since 2025-01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if flagSince == "" {
				return fmt.Errorf("--since is required (e.g., 2025-01)")
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureMeshTables(db); err != nil {
				return err
			}

			// Get terms from "before" snapshot
			oldRows, err := db.DB().Query(
				`SELECT term, mesh_id, tree_numbers FROM mesh_trees WHERE snapshot_at < ? GROUP BY term, mesh_id`,
				flagSince+"-31",
			)
			if err != nil {
				return err
			}

			oldTerms := make(map[string]string) // mesh_id -> tree_numbers
			for oldRows.Next() {
				var term, meshID, treeNumbers string
				if oldRows.Scan(&term, &meshID, &treeNumbers) == nil {
					oldTerms[meshID] = treeNumbers
				}
			}
			oldRows.Close()

			// Get terms from "after" snapshot
			newRows, err := db.DB().Query(
				`SELECT term, mesh_id, tree_numbers FROM mesh_trees WHERE snapshot_at >= ? GROUP BY term, mesh_id`,
				flagSince+"-01",
			)
			if err != nil {
				return err
			}

			newTerms := make(map[string]string)
			for newRows.Next() {
				var term, meshID, treeNumbers string
				if newRows.Scan(&term, &meshID, &treeNumbers) == nil {
					newTerms[meshID] = treeNumbers
				}
			}
			newRows.Close()

			// Compute diff
			var added, removed, changed []map[string]any
			for id, newTree := range newTerms {
				if oldTree, ok := oldTerms[id]; !ok {
					added = append(added, map[string]any{"mesh_id": id, "tree_numbers": newTree})
				} else if oldTree != newTree {
					changed = append(changed, map[string]any{
						"mesh_id":  id,
						"old_tree": oldTree,
						"new_tree": newTree,
					})
				}
			}
			for id, oldTree := range oldTerms {
				if _, ok := newTerms[id]; !ok {
					removed = append(removed, map[string]any{"mesh_id": id, "tree_numbers": oldTree})
				}
			}

			if added == nil {
				added = []map[string]any{}
			}
			if removed == nil {
				removed = []map[string]any{}
			}
			if changed == nil {
				changed = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"since":         flagSince,
				"added":         added,
				"removed":       removed,
				"changed":       changed,
				"added_count":   len(added),
				"removed_count": len(removed),
				"changed_count": len(changed),
			}, flags)
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "", "Compare against snapshots since this date (YYYY-MM)")

	return cmd
}

func newMeshListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List all stored MeSH terms",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureMeshTables(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(
				`SELECT term, mesh_id, tree_numbers, parent_terms, MAX(snapshot_at) as latest
				 FROM mesh_trees
				 GROUP BY term, mesh_id
				 ORDER BY latest DESC`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			var terms []meshTree
			for rows.Next() {
				var t meshTree
				if err := rows.Scan(&t.Term, &t.MeshID, &t.TreeNumbers, &t.ParentTerms, &t.SnapshotAt); err != nil {
					return err
				}
				terms = append(terms, t)
			}
			if terms == nil {
				terms = []meshTree{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), terms, flags)
		},
	}
}
