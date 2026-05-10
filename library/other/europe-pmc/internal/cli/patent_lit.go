package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensurePatentLiteratureTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS patent_literature (
			patent_id TEXT NOT NULL,
			cited_pmid TEXT NOT NULL,
			cited_source TEXT,
			link_type TEXT DEFAULT 'reference',
			found_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (patent_id, cited_pmid)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_patent_lit_patent ON patent_literature(patent_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.DB().Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func newPatentLitCmd(flags *rootFlags) *cobra.Command {
	var flagSource string
	var flagID string
	var flagFindPriorArt bool

	cmd := &cobra.Command{
		Use:   "patent-lit",
		Short: "Bridge patents to literature by finding cited papers",
		Long: `Fetch patent references from Europe PMC to find cited scientific papers.
Uses the references and citations endpoints for patent source records.`,
		Example: `  europe-pmc-pp-cli patent-lit --source PAT --id EP123456 --find-prior-art
  europe-pmc-pp-cli patent-lit list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagID == "" {
				return cmd.Help()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			db, err := store.Open(defaultDBPath("europe-pmc-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePatentLiteratureTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			linksFound := 0

			if flagFindPriorArt {
				// Fetch references from the patent
				refPath := fmt.Sprintf("/%s/%s/references", flagSource, flagID)
				for page := 1; ; page++ {
					refParams := map[string]string{"format": "json", "pageSize": "100", "page": fmt.Sprintf("%d", page)}
					refData, refErr := c.Get(refPath, refParams)
					if refErr != nil {
						break
					}
					refs := parseCitationResults(refData)
					if len(refs) == 0 {
						break
					}
					for _, ref := range refs {
						_, err := db.DB().Exec(
							`INSERT INTO patent_literature (patent_id, cited_pmid, cited_source, link_type, found_at)
							 VALUES (?, ?, ?, 'reference', ?)
							 ON CONFLICT(patent_id, cited_pmid) DO NOTHING`,
							flagID, ref.id, ref.source, time.Now(),
						)
						if err == nil {
							linksFound++
						}
					}
					if len(refs) < 100 {
						break
					}
				}

				// Also fetch citations (papers that cite this patent)
				citePath := fmt.Sprintf("/%s/%s/citations", flagSource, flagID)
				for page := 1; ; page++ {
					citeParams := map[string]string{"format": "json", "pageSize": "100", "page": fmt.Sprintf("%d", page)}
					citeData, citeErr := c.Get(citePath, citeParams)
					if citeErr != nil {
						break
					}
					cites := parseCitationResults(citeData)
					if len(cites) == 0 {
						break
					}
					for _, cite := range cites {
						_, err := db.DB().Exec(
							`INSERT INTO patent_literature (patent_id, cited_pmid, cited_source, link_type, found_at)
							 VALUES (?, ?, ?, 'cited_by', ?)
							 ON CONFLICT(patent_id, cited_pmid) DO NOTHING`,
							flagID, cite.id, cite.source, time.Now(),
						)
						if err == nil {
							linksFound++
						}
					}
					if len(cites) < 100 {
						break
					}
				}
			}

			result := map[string]any{
				"patent_id":   flagID,
				"source":      flagSource,
				"links_found": linksFound,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagSource, "source", "PAT", "Source (PAT for patents)")
	cmd.Flags().StringVar(&flagID, "id", "", "Patent identifier (e.g. EP123456)")
	cmd.Flags().BoolVar(&flagFindPriorArt, "find-prior-art", false, "Search for cited papers and citing papers")

	cmd.AddCommand(newPatentLitListCmd(flags))
	return cmd
}

func newPatentLitListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List tracked patent-to-literature links with cited PMIDs and link types",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.Open(defaultDBPath("europe-pmc-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePatentLiteratureTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			rows, err := db.DB().Query(
				`SELECT patent_id, cited_pmid, cited_source, link_type, found_at
				 FROM patent_literature ORDER BY found_at DESC`,
			)
			if err != nil {
				return fmt.Errorf("querying patents: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var patentID, citedPMID, linkType string
				var citedSource sql.NullString
				var foundAt time.Time
				if err := rows.Scan(&patentID, &citedPMID, &citedSource, &linkType, &foundAt); err != nil {
					continue
				}
				results = append(results, map[string]any{
					"patent_id":    patentID,
					"cited_pmid":   citedPMID,
					"cited_source": citedSource.String,
					"link_type":    linkType,
					"found_at":     foundAt.Format(time.RFC3339),
				})
			}
			if len(results) == 0 {
				results = []map[string]any{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
}
