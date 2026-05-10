package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensureReviewResultTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS review_results (
			article_id TEXT NOT NULL,
			source TEXT,
			doi TEXT,
			pmid TEXT,
			pmcid TEXT,
			strategy TEXT NOT NULL,
			tagged_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (article_id, strategy)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_review_strategy ON review_results(strategy)`,
		`CREATE INDEX IF NOT EXISTS idx_review_doi ON review_results(doi)`,
	}
	for _, stmt := range stmts {
		if _, err := db.DB().Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func newSystematicReviewCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagTag string
	var flagPageSize int

	cmd := &cobra.Command{
		Use:   "systematic-review",
		Short: "Systematic review workbench with PRISMA workflow support",
		Long: `Run search strategies, tag results by strategy, and perform cross-source
deduplication using DOI/PMID matching. Generate PRISMA flow counts.`,
		Example: `  europe-pmc-pp-cli systematic-review --query "CRISPR gene therapy" --tag strategy-a
  europe-pmc-pp-cli systematic-review dedup --strategies a,b
  europe-pmc-pp-cli systematic-review prisma --strategies a,b`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagQuery == "" || flagTag == "" {
				return fmt.Errorf("both --query and --tag are required")
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

			if err := ensureReviewResultTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			params := map[string]string{
				"query":      flagQuery,
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
						DOI    string `json:"doi"`
						PMID   string `json:"pmid"`
						PMCID  string `json:"pmcid"`
						Title  string `json:"title"`
					} `json:"result"`
				} `json:"resultList"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			stored := 0
			for _, r := range envelope.ResultList.Result {
				articleID := r.ID
				if articleID == "" {
					continue
				}
				_, err := db.DB().Exec(
					`INSERT INTO review_results (article_id, source, doi, pmid, pmcid, strategy, tagged_at)
					 VALUES (?, ?, ?, ?, ?, ?, ?)
					 ON CONFLICT(article_id, strategy) DO UPDATE SET
					   doi = COALESCE(excluded.doi, review_results.doi),
					   pmid = COALESCE(excluded.pmid, review_results.pmid),
					   pmcid = COALESCE(excluded.pmcid, review_results.pmcid)`,
					articleID, r.Source, r.DOI, r.PMID, r.PMCID, flagTag, time.Now(),
				)
				if err == nil {
					stored++
				}
			}

			result := map[string]any{
				"strategy":      flagTag,
				"query":         flagQuery,
				"results_found": len(envelope.ResultList.Result),
				"stored":        stored,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagQuery, "query", "", "Search query")
	cmd.Flags().StringVar(&flagTag, "tag", "", "Strategy tag name")
	cmd.Flags().IntVar(&flagPageSize, "page-size", 25, "Results per page")

	cmd.AddCommand(newReviewDedupCmd(flags))
	cmd.AddCommand(newReviewPrismaCmd(flags))
	return cmd
}

func newReviewDedupCmd(flags *rootFlags) *cobra.Command {
	var flagStrategies string

	cmd := &cobra.Command{
		Use:   "dedup",
		Short: "Cross-source deduplication using DOI/PMID matching",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagStrategies == "" {
				return fmt.Errorf("--strategies is required (comma-separated)")
			}

			db, err := store.Open(defaultDBPath("europe-pmc-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureReviewResultTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			strategies := strings.Split(flagStrategies, ",")
			placeholders := make([]string, len(strategies))
			stratArgs := make([]any, len(strategies))
			for i, s := range strategies {
				placeholders[i] = "?"
				stratArgs[i] = strings.TrimSpace(s)
			}
			inClause := strings.Join(placeholders, ",")

			// Count total per strategy
			strategyCounts := map[string]int{}
			for _, s := range strategies {
				s = strings.TrimSpace(s)
				var count int
				db.DB().QueryRow(
					`SELECT COUNT(*) FROM review_results WHERE strategy = ?`, s,
				).Scan(&count)
				strategyCounts[s] = count
			}

			// Find duplicates by DOI
			var doiDups int
			query := fmt.Sprintf(
				`SELECT COUNT(*) FROM (
					SELECT doi FROM review_results
					WHERE strategy IN (%s) AND doi != '' AND doi IS NOT NULL
					GROUP BY doi HAVING COUNT(DISTINCT strategy) > 1
				)`, inClause,
			)
			db.DB().QueryRow(query, stratArgs...).Scan(&doiDups)

			// Find duplicates by PMID
			var pmidDups int
			query = fmt.Sprintf(
				`SELECT COUNT(*) FROM (
					SELECT pmid FROM review_results
					WHERE strategy IN (%s) AND pmid != '' AND pmid IS NOT NULL
					GROUP BY pmid HAVING COUNT(DISTINCT strategy) > 1
				)`, inClause,
			)
			db.DB().QueryRow(query, stratArgs...).Scan(&pmidDups)

			// Unique articles across all strategies
			var totalUnique int
			query = fmt.Sprintf(
				`SELECT COUNT(DISTINCT article_id) FROM review_results WHERE strategy IN (%s)`, inClause,
			)
			db.DB().QueryRow(query, stratArgs...).Scan(&totalUnique)

			result := map[string]any{
				"strategies":      strategyCounts,
				"doi_duplicates":  doiDups,
				"pmid_duplicates": pmidDups,
				"unique_articles": totalUnique,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagStrategies, "strategies", "", "Comma-separated strategy tags (e.g. a,b)")
	return cmd
}

func newReviewPrismaCmd(flags *rootFlags) *cobra.Command {
	var flagStrategies string

	cmd := &cobra.Command{
		Use:   "prisma",
		Short: "Generate PRISMA flow counts for the review",
		Example: `  europe-pmc-pp-cli systematic-review prisma --strategies a,b
  europe-pmc-pp-cli systematic-review prisma --strategies medline,embase --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagStrategies == "" {
				return fmt.Errorf("--strategies is required (comma-separated)")
			}

			db, err := store.Open(defaultDBPath("europe-pmc-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureReviewResultTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			strategies := strings.Split(flagStrategies, ",")
			placeholders := make([]string, len(strategies))
			stratArgs := make([]any, len(strategies))
			for i, s := range strategies {
				placeholders[i] = "?"
				stratArgs[i] = strings.TrimSpace(s)
			}
			inClause := strings.Join(placeholders, ",")

			// Total records identified
			var totalIdentified int
			query := fmt.Sprintf(
				`SELECT COUNT(*) FROM review_results WHERE strategy IN (%s)`, inClause,
			)
			db.DB().QueryRow(query, stratArgs...).Scan(&totalIdentified)

			var uniqueAfterDedup int
			query = fmt.Sprintf(
				`SELECT COUNT(*) FROM (
					SELECT MIN(rowid) FROM review_results WHERE strategy IN (%s)
					GROUP BY COALESCE(NULLIF(doi,''), ''), COALESCE(NULLIF(pmid,''), '')
					HAVING COALESCE(NULLIF(doi,''), '') != '' OR COALESCE(NULLIF(pmid,''), '') != ''
					UNION
					SELECT MIN(rowid) FROM review_results WHERE strategy IN (%s)
					AND COALESCE(NULLIF(doi,''), '') = '' AND COALESCE(NULLIF(pmid,''), '') = ''
					GROUP BY article_id
				)`, inClause, inClause,
			)
			allArgs := append(stratArgs, stratArgs...)
			db.DB().QueryRow(query, allArgs...).Scan(&uniqueAfterDedup)

			duplicatesRemoved := totalIdentified - uniqueAfterDedup

			// Per-strategy counts
			perStrategy := map[string]int{}
			for _, s := range strategies {
				s = strings.TrimSpace(s)
				var count int
				db.DB().QueryRow(`SELECT COUNT(*) FROM review_results WHERE strategy = ?`, s).Scan(&count)
				perStrategy[s] = count
			}

			result := map[string]any{
				"prisma_flow": map[string]any{
					"identification": map[string]any{
						"total_records_identified": totalIdentified,
						"per_strategy":             perStrategy,
					},
					"screening": map[string]any{
						"duplicates_removed": duplicatesRemoved,
						"unique_records":     uniqueAfterDedup,
					},
				},
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagStrategies, "strategies", "", "Comma-separated strategy tags")
	return cmd
}
