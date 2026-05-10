package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensureEnrichedEntityTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS enriched_entities (
			article_id TEXT NOT NULL,
			entity_accession TEXT NOT NULL,
			database TEXT NOT NULL,
			entity_type TEXT,
			obtained_by TEXT,
			section TEXT,
			found_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (article_id, entity_accession, database)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_enriched_db ON enriched_entities(database)`,
		`CREATE INDEX IF NOT EXISTS idx_enriched_article ON enriched_entities(article_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.DB().Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func newEnrichCmd(flags *rootFlags) *cobra.Command {
	var flagSource string
	var flagID string
	var flagDatabases string

	cmd := &cobra.Command{
		Use:   "enrich",
		Short: "Cross-database entity enrichment from database links and annotations",
		Long: `Pull database links and text-mined annotations for an article, then merge
into a unified entity table. Supports filtering by specific databases
(UNIPROT, PDB, CHEBI, CHEMBL, EMBL, OMIM, etc.).`,
		Example: `  europe-pmc-pp-cli enrich --source MED --id 33024307 --databases UNIPROT,PDB
  europe-pmc-pp-cli enrich --source MED --id 33024307`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagSource == "" || flagID == "" {
				return fmt.Errorf("both --source and --id are required")
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

			if err := ensureEnrichedEntityTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			articleID := fmt.Sprintf("%s:%s", flagSource, flagID)
			entitiesFound := 0

			// Parse database filter
			dbFilter := map[string]bool{}
			if flagDatabases != "" {
				for _, d := range strings.Split(flagDatabases, ",") {
					dbFilter[strings.TrimSpace(strings.ToUpper(d))] = true
				}
			}

			// Fetch database links
			dbLinkPath := fmt.Sprintf("/%s/%s/databaseLinks", flagSource, flagID)
			dbLinkParams := map[string]string{"format": "json", "pageSize": "100"}
			dbLinkData, dbLinkErr := c.Get(dbLinkPath, dbLinkParams)
			if dbLinkErr == nil {
				var dbEnvelope struct {
					DbCrossReferenceList struct {
						DbCrossReference []struct {
							DbName               string `json:"dbName"`
							DbCrossReferenceInfo []struct {
								Info1 string `json:"info1"`
								Info2 string `json:"info2"`
								Info3 string `json:"info3"`
							} `json:"dbCrossReferenceInfo"`
						} `json:"dbCrossReference"`
					} `json:"dbCrossReferenceList"`
				}
				if json.Unmarshal(dbLinkData, &dbEnvelope) == nil {
					for _, dbRef := range dbEnvelope.DbCrossReferenceList.DbCrossReference {
						dbName := strings.ToUpper(dbRef.DbName)
						if len(dbFilter) > 0 && !dbFilter[dbName] {
							continue
						}
						for _, info := range dbRef.DbCrossReferenceInfo {
							accession := info.Info1
							if accession == "" {
								continue
							}
							_, err := db.DB().Exec(
								`INSERT INTO enriched_entities (article_id, entity_accession, database, entity_type, obtained_by, found_at)
								 VALUES (?, ?, ?, ?, 'database_link', ?)
								 ON CONFLICT(article_id, entity_accession, database) DO NOTHING`,
								articleID, accession, dbName, info.Info2, time.Now(),
							)
							if err == nil {
								entitiesFound++
							}
						}
					}
				}
			}

			// Fetch annotations
			annotPath := fmt.Sprintf("/%s/%s/annotations", flagSource, flagID)
			annotParams := map[string]string{"format": "json"}
			annotData, annotErr := c.Get(annotPath, annotParams)
			if annotErr == nil {
				var annotations []struct {
					Provider string `json:"provider"`
					Anns     []struct {
						Exact string `json:"exact"`
						Type  string `json:"type"`
						Tags  []struct {
							Name string `json:"name"`
							URI  string `json:"uri"`
						} `json:"tags"`
						Section string `json:"section"`
					} `json:"anns"`
				}
				if json.Unmarshal(annotData, &annotations) == nil {
					for _, prov := range annotations {
						for _, ann := range prov.Anns {
							for _, tag := range ann.Tags {
								accession := tag.Name
								if accession == "" {
									accession = ann.Exact
								}
								if accession == "" {
									continue
								}
								entityDB := prov.Provider
								if ann.Type != "" {
									entityDB = ann.Type
								}
								_, err := db.DB().Exec(
									`INSERT INTO enriched_entities (article_id, entity_accession, database, entity_type, obtained_by, section, found_at)
									 VALUES (?, ?, ?, ?, 'text_mining', ?, ?)
									 ON CONFLICT(article_id, entity_accession, database) DO NOTHING`,
									articleID, accession, entityDB, ann.Type, ann.Section, time.Now(),
								)
								if err == nil {
									entitiesFound++
								}
							}
						}
					}
				}
			}

			result := map[string]any{
				"article_id":     articleID,
				"source":         flagSource,
				"id":             flagID,
				"entities_found": entitiesFound,
			}
			if flagDatabases != "" {
				result["database_filter"] = flagDatabases
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagSource, "source", "MED", "Source database (MED, PMC)")
	cmd.Flags().StringVar(&flagID, "id", "", "Article identifier")
	cmd.Flags().StringVar(&flagDatabases, "databases", "", "Comma-separated database filter (e.g. UNIPROT,PDB)")

	return cmd
}
