package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensureAnnotationRelationTables(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS annotation_relations (
		first_entity TEXT NOT NULL,
		second_entity TEXT NOT NULL,
		article_count INTEGER DEFAULT 0,
		articles_json TEXT,
		last_checked DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (first_entity, second_entity)
	)`)
	return err
}

func newMineRelationsCmd(flags *rootFlags) *cobra.Command {
	var flagGene string
	var flagDisease string
	var flagChemical string
	var flagPageSize int

	cmd := &cobra.Command{
		Use:   "mine-relations",
		Short: "Mine text-mined annotation co-occurrences across articles",
		Long: `Query Europe PMC annotations to find co-occurrences of genes, diseases,
and chemicals across articles. Stores relation pairs with article counts.`,
		Example: `  europe-pmc-pp-cli mine-relations --gene BRCA1 --disease "breast cancer"
  europe-pmc-pp-cli mine-relations --gene TP53 --chemical cisplatin
  europe-pmc-pp-cli mine-relations list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			entities := []string{}
			if flagGene != "" {
				entities = append(entities, flagGene)
			}
			if flagDisease != "" {
				entities = append(entities, flagDisease)
			}
			if flagChemical != "" {
				entities = append(entities, flagChemical)
			}
			if len(entities) < 2 {
				return fmt.Errorf("at least two entity flags required (--gene, --disease, --chemical)")
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

			if err := ensureAnnotationRelationTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			// Build a search query combining entities
			query := ""
			if flagGene != "" && flagDisease != "" && flagChemical != "" {
				return fmt.Errorf("specify at most two entity flags at a time; got --gene, --disease, and --chemical")
			} else if flagGene != "" && flagDisease != "" {
				query = fmt.Sprintf(`"%s" AND "%s"`, flagGene, flagDisease)
			} else if flagGene != "" && flagChemical != "" {
				query = fmt.Sprintf(`"%s" AND "%s"`, flagGene, flagChemical)
			} else if flagDisease != "" && flagChemical != "" {
				query = fmt.Sprintf(`"%s" AND "%s"`, flagDisease, flagChemical)
			} else {
				query = fmt.Sprintf(`"%s" AND "%s"`, entities[0], entities[1])
			}

			params := map[string]string{
				"query":      query,
				"format":     "json",
				"resultType": "core",
				"pageSize":   fmt.Sprintf("%d", flagPageSize),
			}
			data, err := c.Get("/search", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var envelope struct {
				ResultList struct {
					Result []struct {
						ID     string `json:"id"`
						Source string `json:"source"`
						Title  string `json:"title"`
						PMID   string `json:"pmid"`
					} `json:"result"`
				} `json:"resultList"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			articleIDs := make([]string, 0, len(envelope.ResultList.Result))
			for _, r := range envelope.ResultList.Result {
				if r.ID != "" {
					articleIDs = append(articleIDs, r.ID)
				}
			}

			articlesJSON, _ := json.Marshal(articleIDs)
			firstEntity := entities[0]
			secondEntity := entities[1]

			_, err = db.DB().Exec(
				`INSERT INTO annotation_relations (first_entity, second_entity, article_count, articles_json, last_checked)
				 VALUES (?, ?, ?, ?, ?)
				 ON CONFLICT(first_entity, second_entity) DO UPDATE SET
				   article_count = excluded.article_count,
				   articles_json = excluded.articles_json,
				   last_checked = excluded.last_checked`,
				firstEntity, secondEntity, len(articleIDs), string(articlesJSON), time.Now(),
			)
			if err != nil {
				return fmt.Errorf("storing relation: %w", err)
			}

			result := map[string]any{
				"first_entity":  firstEntity,
				"second_entity": secondEntity,
				"article_count": len(articleIDs),
				"articles":      articleIDs,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagGene, "gene", "", "Gene name (e.g. BRCA1, TP53)")
	cmd.Flags().StringVar(&flagDisease, "disease", "", "Disease name (e.g. 'breast cancer')")
	cmd.Flags().StringVar(&flagChemical, "chemical", "", "Chemical/drug name (e.g. cisplatin)")
	cmd.Flags().IntVar(&flagPageSize, "page-size", 25, "Results per page")

	cmd.AddCommand(newMineRelationsListCmd(flags))
	return cmd
}

func newMineRelationsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List stored gene-disease-chemical relation pairs with co-occurrence article counts",
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

			if err := ensureAnnotationRelationTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			rows, err := db.DB().Query(
				`SELECT first_entity, second_entity, article_count, articles_json, last_checked
				 FROM annotation_relations ORDER BY article_count DESC`,
			)
			if err != nil {
				return fmt.Errorf("querying relations: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var first, second string
				var count int
				var articlesJSON sql.NullString
				var lastChecked time.Time
				if err := rows.Scan(&first, &second, &count, &articlesJSON, &lastChecked); err != nil {
					continue
				}
				results = append(results, map[string]any{
					"first_entity":  first,
					"second_entity": second,
					"article_count": count,
					"last_checked":  lastChecked.Format(time.RFC3339),
				})
			}
			if len(results) == 0 {
				results = []map[string]any{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
}
