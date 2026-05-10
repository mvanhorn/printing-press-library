package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// triangleDrug represents a tracked drug in the triangle.
type triangleDrug struct {
	Drug       string `json:"drug"`
	SearchedAt string `json:"searched_at"`
}

// triangleGene represents a gene linked to a drug.
type triangleGene struct {
	Drug       string `json:"drug"`
	GeneID     int    `json:"gene_id"`
	GeneSymbol string `json:"gene_symbol"`
}

// trianglePaper represents a paper linked through the drug-gene-literature triangle.
type trianglePaper struct {
	Drug   string `json:"drug"`
	GeneID int    `json:"gene_id"`
	PMID   int    `json:"pmid"`
	SeenAt string `json:"seen_at"`
}

func ensureTriangleTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS triangle_drugs (
			drug TEXT PRIMARY KEY,
			searched_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS triangle_genes (
			drug TEXT NOT NULL,
			gene_id INTEGER NOT NULL,
			gene_symbol TEXT,
			PRIMARY KEY (drug, gene_id)
		)`,
		`CREATE TABLE IF NOT EXISTS triangle_papers (
			drug TEXT NOT NULL,
			gene_id INTEGER NOT NULL,
			pmid INTEGER NOT NULL,
			seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (drug, gene_id, pmid)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_triangle_genes_drug ON triangle_genes(drug)`,
		`CREATE INDEX IF NOT EXISTS idx_triangle_papers_drug ON triangle_papers(drug)`,
		`CREATE INDEX IF NOT EXISTS idx_triangle_papers_gene ON triangle_papers(gene_id)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating triangle tables: %w", err)
		}
	}
	return nil
}

func newTriangleCmd(flags *rootFlags) *cobra.Command {
	var flagDrug string
	var flagUnseenOnly bool
	var flagGeneSort string

	cmd := &cobra.Command{
		Use:   "triangle",
		Short: "Build drug-gene-literature triangles",
		Long: strings.TrimSpace(`
Drug-Gene-Literature Triangle -- takes a drug name, finds related genes
via ELink, then finds papers about each gene. Stores the full triangle
in SQLite for incremental discovery.

Use --unseen-only to show only papers not previously recorded.`),
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli triangle --drug metformin
  ncbi-entrez-pp-cli triangle --drug ibuprofen --unseen-only
  ncbi-entrez-pp-cli triangle --drug aspirin --gene-sort pub-count
  ncbi-entrez-pp-cli triangle list`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagDrug == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

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

			if err := ensureTriangleTables(db); err != nil {
				return err
			}

			// Track which PMIDs we already knew about (for --unseen-only)
			existingPMIDs := make(map[string]bool)
			if flagUnseenOnly {
				rows, err := db.DB().Query(
					`SELECT pmid FROM triangle_papers WHERE drug = ?`, flagDrug,
				)
				if err == nil {
					for rows.Next() {
						var pmid int
						if rows.Scan(&pmid) == nil {
							existingPMIDs[fmt.Sprintf("%d", pmid)] = true
						}
					}
					rows.Close()
				}
			}

			// Step 1: Search PubMed for the drug to find related gene IDs
			fmt.Fprintf(os.Stderr, "searching pubmed for %s...\n", flagDrug)

			searchParams := map[string]string{
				"db":      "pubmed",
				"term":    flagDrug,
				"retmax":  "50",
				"retmode": "json",
			}
			searchData, err := c.Get("/esearch.fcgi", searchParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			pmids := extractPMIDsFromEsearch(searchData)
			if len(pmids) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"status": "no_results",
					"drug":   flagDrug,
				}, flags)
			}

			// ELink pubmed -> gene
			fmt.Fprintf(os.Stderr, "linking %d PMIDs to genes...\n", len(pmids))

			linkParams := map[string]string{
				"dbfrom":  "pubmed",
				"db":      "gene",
				"id":      strings.Join(pmids, ","),
				"retmode": "json",
				"cmd":     "neighbor",
			}
			linkData, err := c.Get("/elink.fcgi", linkParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Extract gene IDs from ELink response
			geneIDs := extractLinkedIDs(linkData)

			// Record drug
			db.DB().Exec(
				`INSERT OR REPLACE INTO triangle_drugs (drug, searched_at) VALUES (?, CURRENT_TIMESTAMP)`,
				flagDrug,
			)

			// Insert genes and then find papers for each gene
			var allNewPapers []map[string]any

			for _, geneID := range geneIDs {
				// Insert gene
				_, _ = db.DB().Exec(
					`INSERT OR IGNORE INTO triangle_genes (drug, gene_id, gene_symbol) VALUES (?, ?, ?)`,
					flagDrug, geneID, "",
				)

				// Step 2: ELink gene -> pubmed for each gene
				geneLinkParams := map[string]string{
					"dbfrom":  "gene",
					"db":      "pubmed",
					"id":      geneID,
					"retmode": "json",
					"cmd":     "neighbor",
					"retmax":  "20",
				}
				geneLinkData, err := c.Get("/elink.fcgi", geneLinkParams)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: gene %s link failed: %v\n", geneID, err)
					continue
				}

				genePMIDs := extractLinkedIDs(geneLinkData)

				tx, err := db.DB().Begin()
				if err != nil {
					return err
				}

				for _, pmid := range genePMIDs {
					isNew := !existingPMIDs[pmid]
					_, _ = tx.Exec(
						`INSERT OR IGNORE INTO triangle_papers (drug, gene_id, pmid, seen_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
						flagDrug, geneID, pmid,
					)
					if flagUnseenOnly && isNew {
						allNewPapers = append(allNewPapers, map[string]any{
							"gene_id": geneID,
							"pmid":    pmid,
						})
					}
				}

				if err := tx.Commit(); err != nil {
					return fmt.Errorf("committing gene papers: %w", err)
				}
			}

			// Build output
			var geneCount, paperCount int
			db.DB().QueryRow(`SELECT COUNT(*) FROM triangle_genes WHERE drug = ?`, flagDrug).Scan(&geneCount)
			db.DB().QueryRow(`SELECT COUNT(*) FROM triangle_papers WHERE drug = ?`, flagDrug).Scan(&paperCount)

			result := map[string]any{
				"status":      "complete",
				"drug":        flagDrug,
				"gene_count":  geneCount,
				"paper_count": paperCount,
			}

			if flagUnseenOnly {
				if allNewPapers == nil {
					allNewPapers = []map[string]any{}
				}
				result["unseen_papers"] = allNewPapers
				result["unseen_count"] = len(allNewPapers)
			}

			if flagGeneSort == "pub-count" {
				rows, err := db.DB().Query(
					`SELECT gene_id, COUNT(pmid) as pcount FROM triangle_papers
					 WHERE drug = ? GROUP BY gene_id ORDER BY pcount DESC`,
					flagDrug,
				)
				if err == nil {
					var sorted []map[string]any
					for rows.Next() {
						var gid, pc int
						if rows.Scan(&gid, &pc) == nil {
							sorted = append(sorted, map[string]any{
								"gene_id":   gid,
								"pub_count": pc,
							})
						}
					}
					rows.Close()
					if sorted == nil {
						sorted = []map[string]any{}
					}
					result["genes_by_pub_count"] = sorted
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagDrug, "drug", "", "Drug name to search")
	cmd.Flags().BoolVar(&flagUnseenOnly, "unseen-only", false, "Show only papers not previously seen")
	cmd.Flags().StringVar(&flagGeneSort, "gene-sort", "", "Sort genes by: pub-count")

	cmd.AddCommand(newTriangleListCmd(flags))

	return cmd
}

// extractLinkedIDs pulls all linked IDs from an ELink response, regardless of source.
func extractLinkedIDs(data json.RawMessage) []string {
	seen := make(map[string]bool)
	var ids []string

	var resp struct {
		LinkSets []json.RawMessage `json:"linksets"`
	}
	if json.Unmarshal(data, &resp) != nil {
		return ids
	}

	for _, lsRaw := range resp.LinkSets {
		var ls struct {
			LinkSetDBs []struct {
				Links []struct {
					ID json.RawMessage `json:"id"`
				} `json:"links"`
			} `json:"linksetdbs"`
		}
		if json.Unmarshal(lsRaw, &ls) != nil {
			continue
		}

		for _, ldb := range ls.LinkSetDBs {
			for _, link := range ldb.Links {
				var sid string
				if json.Unmarshal(link.ID, &sid) == nil {
					if !seen[sid] {
						seen[sid] = true
						ids = append(ids, sid)
					}
				} else {
					var num float64
					if json.Unmarshal(link.ID, &num) == nil {
						s := fmt.Sprintf("%d", int64(num))
						if !seen[s] {
							seen[s] = true
							ids = append(ids, s)
						}
					}
				}
			}
		}
	}

	return ids
}

func newTriangleListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List all tracked drugs with gene/paper counts",
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

			if err := ensureTriangleTables(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(
				`SELECT d.drug, d.searched_at,
				        (SELECT COUNT(*) FROM triangle_genes g WHERE g.drug = d.drug) as gene_count,
				        (SELECT COUNT(*) FROM triangle_papers p WHERE p.drug = d.drug) as paper_count
				 FROM triangle_drugs d
				 ORDER BY d.searched_at DESC`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			var drugs []map[string]any
			for rows.Next() {
				var drug, searchedAt string
				var gc, pc int
				if err := rows.Scan(&drug, &searchedAt, &gc, &pc); err != nil {
					return err
				}
				drugs = append(drugs, map[string]any{
					"drug":        drug,
					"searched_at": searchedAt,
					"gene_count":  gc,
					"paper_count": pc,
				})
			}
			if drugs == nil {
				drugs = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), drugs, flags)
		},
	}
}
