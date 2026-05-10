package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// prismaResult represents a tagged search result for PRISMA deduplication.
type prismaResult struct {
	PMID     int    `json:"pmid"`
	Strategy string `json:"strategy"`
	Query    string `json:"query"`
	TaggedAt string `json:"tagged_at"`
}

func ensurePrismaTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS prisma_results (
			pmid INTEGER NOT NULL,
			strategy TEXT NOT NULL,
			query TEXT NOT NULL,
			tagged_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (pmid, strategy)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_prisma_strategy ON prisma_results(strategy)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating prisma tables: %w", err)
		}
	}
	return nil
}

func newPrismaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prisma",
		Short: "PRISMA-ready search deduplication across strategies",
		Long: strings.TrimSpace(`
PRISMA-Ready Deduplication -- tag search results with strategy names,
then deduplicate across strategies to produce PRISMA-compliant flow data.

Use 'prisma tag' to run and tag searches, 'prisma dedup' to find overlaps,
and 'prisma export' to produce a CSV for PRISMA flow diagrams.`),
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli prisma tag "breast cancer screening" --strategy medline-main --db pubmed
  ncbi-entrez-pp-cli prisma tag "mammography effectiveness" --strategy embase-main --db pubmed
  ncbi-entrez-pp-cli prisma dedup --strategies medline-main,embase-main
  ncbi-entrez-pp-cli prisma export --strategies medline-main,embase-main --output prisma-flow.csv`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newPrismaTagCmd(flags))
	cmd.AddCommand(newPrismaDedupCmd(flags))
	cmd.AddCommand(newPrismaExportCmd(flags))

	return cmd
}

func newPrismaTagCmd(flags *rootFlags) *cobra.Command {
	var flagStrategy string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "tag <query>",
		Short: "Run a search and tag results with a strategy name",
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli prisma tag "breast cancer screening" --strategy medline-main
  ncbi-entrez-pp-cli prisma tag "mammography" --strategy embase-main --db pubmed`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("search query required as argument")
			}
			if dryRunOK(flags) {
				return nil
			}

			if flagStrategy == "" {
				return fmt.Errorf("--strategy is required")
			}

			query := strings.Join(args, " ")

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

			if err := ensurePrismaTables(db); err != nil {
				return err
			}

			// Run ESearch
			params := map[string]string{
				"db":      flagDB,
				"term":    query,
				"retmax":  "10000",
				"retmode": "json",
			}
			data, err := c.Get("/esearch.fcgi", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			pmids := extractPMIDsFromEsearch(data)

			// Tag all results
			tx, err := db.DB().Begin()
			if err != nil {
				return err
			}

			stmt, err := tx.Prepare(
				`INSERT OR IGNORE INTO prisma_results (pmid, strategy, query, tagged_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			)
			if err != nil {
				return err
			}
			defer stmt.Close()

			tagged := 0
			for _, pmid := range pmids {
				if _, err := stmt.Exec(pmid, flagStrategy, query); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to tag PMID %s: %v\n", pmid, err)
					continue
				}
				tagged++
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			result := map[string]any{
				"status":   "tagged",
				"strategy": flagStrategy,
				"query":    query,
				"db":       flagDB,
				"tagged":   tagged,
				"total":    len(pmids),
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagStrategy, "strategy", "", "Strategy name for tagging results")
	cmd.Flags().StringVar(&flagDB, "db", "pubmed", "Target database")

	return cmd
}

func newPrismaDedupCmd(flags *rootFlags) *cobra.Command {
	var flagStrategies string

	cmd := &cobra.Command{
		Use:         "dedup",
		Short:       "Find duplicates across strategies",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if flagStrategies == "" {
				return fmt.Errorf("--strategies is required (comma-separated)")
			}

			strategies := strings.Split(flagStrategies, ",")
			for i := range strategies {
				strategies[i] = strings.TrimSpace(strategies[i])
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePrismaTables(db); err != nil {
				return err
			}

			// Per-strategy totals
			perStrategy := make(map[string]int)
			for _, s := range strategies {
				var count int
				db.DB().QueryRow(`SELECT COUNT(*) FROM prisma_results WHERE strategy = ?`, s).Scan(&count)
				perStrategy[s] = count
			}

			// Unique per strategy (only found in that strategy, not in any other)
			uniquePerStrategy := make(map[string]int)
			for _, s := range strategies {
				var others []string
				for _, o := range strategies {
					if o != s {
						others = append(others, "'"+o+"'")
					}
				}
				if len(others) == 0 {
					uniquePerStrategy[s] = perStrategy[s]
					continue
				}
				var count int
				q := fmt.Sprintf(
					`SELECT COUNT(DISTINCT pmid) FROM prisma_results WHERE strategy = ? AND pmid NOT IN (SELECT pmid FROM prisma_results WHERE strategy IN (%s))`,
					strings.Join(others, ","),
				)
				db.DB().QueryRow(q, s).Scan(&count)
				uniquePerStrategy[s] = count
			}

			// Pairwise overlaps
			var overlaps []map[string]any
			for i := 0; i < len(strategies); i++ {
				for j := i + 1; j < len(strategies); j++ {
					var count int
					db.DB().QueryRow(
						`SELECT COUNT(*) FROM prisma_results a
						 INNER JOIN prisma_results b ON a.pmid = b.pmid
						 WHERE a.strategy = ? AND b.strategy = ?`,
						strategies[i], strategies[j],
					).Scan(&count)
					overlaps = append(overlaps, map[string]any{
						"strategy_a": strategies[i],
						"strategy_b": strategies[j],
						"overlap":    count,
					})
				}
			}

			// Total unique across all strategies
			placeholders := make([]string, len(strategies))
			stratArgs := make([]any, len(strategies))
			for i, s := range strategies {
				placeholders[i] = "?"
				stratArgs[i] = s
			}
			var totalUnique int
			db.DB().QueryRow(
				fmt.Sprintf(`SELECT COUNT(DISTINCT pmid) FROM prisma_results WHERE strategy IN (%s)`, strings.Join(placeholders, ",")),
				stratArgs...,
			).Scan(&totalUnique)

			result := map[string]any{
				"strategies":          strategies,
				"per_strategy":        perStrategy,
				"unique_per_strategy": uniquePerStrategy,
				"overlaps":            overlaps,
				"total_unique":        totalUnique,
			}
			if overlaps == nil {
				result["overlaps"] = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagStrategies, "strategies", "", "Comma-separated strategy names to compare")

	return cmd
}

func newPrismaExportCmd(flags *rootFlags) *cobra.Command {
	var flagStrategies string
	var flagOutput string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export PRISMA flow data as CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if flagStrategies == "" {
				return fmt.Errorf("--strategies is required")
			}

			strategies := strings.Split(flagStrategies, ",")
			for i := range strategies {
				strategies[i] = strings.TrimSpace(strategies[i])
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePrismaTables(db); err != nil {
				return err
			}

			// Gather all unique PMIDs across selected strategies
			placeholders := make([]string, len(strategies))
			stratArgs := make([]any, len(strategies))
			for i, s := range strategies {
				placeholders[i] = "?"
				stratArgs[i] = s
			}

			rows, err := db.DB().Query(
				fmt.Sprintf(`SELECT pmid, GROUP_CONCAT(strategy, ',') as strategies
				 FROM prisma_results
				 WHERE strategy IN (%s)
				 GROUP BY pmid
				 ORDER BY pmid`, strings.Join(placeholders, ",")),
				stratArgs...,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			type exportRow struct {
				PMID        string
				Strategies  string
				IsDuplicate bool
			}
			var exportRows []exportRow
			for rows.Next() {
				var pmid int
				var strats string
				if err := rows.Scan(&pmid, &strats); err != nil {
					return err
				}
				isDup := strings.Contains(strats, ",")
				exportRows = append(exportRows, exportRow{
					PMID:        fmt.Sprintf("%d", pmid),
					Strategies:  strats,
					IsDuplicate: isDup,
				})
			}

			// Write CSV
			var w *csv.Writer
			if flagOutput != "" {
				f, err := os.Create(flagOutput)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer f.Close()
				w = csv.NewWriter(f)
			} else {
				w = csv.NewWriter(cmd.OutOrStdout())
			}

			w.Write([]string{"pmid", "strategies", "is_duplicate"})
			for _, r := range exportRows {
				dup := "false"
				if r.IsDuplicate {
					dup = "true"
				}
				w.Write([]string{r.PMID, r.Strategies, dup})
			}
			w.Flush()

			if flagOutput != "" {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"status": "exported",
					"output": flagOutput,
					"rows":   len(exportRows),
				}, flags)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&flagStrategies, "strategies", "", "Comma-separated strategy names to export")
	cmd.Flags().StringVar(&flagOutput, "output", "", "Output CSV file path")

	return cmd
}

// Compile-time guards.
var _ = sort.Strings
var _ = time.Now
var _ json.RawMessage
