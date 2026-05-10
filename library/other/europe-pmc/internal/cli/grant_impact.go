package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensureGrantPublicationTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS grant_publications (
			grant_id TEXT NOT NULL,
			agency TEXT NOT NULL,
			pmid TEXT NOT NULL,
			title TEXT,
			cited_by_count INTEGER DEFAULT 0,
			is_open_access INTEGER DEFAULT 0,
			found_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (grant_id, pmid)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_grant_pub_agency ON grant_publications(agency)`,
	}
	for _, stmt := range stmts {
		if _, err := db.DB().Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func newGrantImpactCmd(flags *rootFlags) *cobra.Command {
	var flagAgency string
	var flagGrantID string
	var flagPageSize int

	cmd := &cobra.Command{
		Use:   "grant-impact",
		Short: "Aggregate grant-linked publications with citation and OA metrics",
		Long: `Search Europe PMC for publications linked to a grant by GRANT_ID and
GRANT_AGENCY fields. Aggregates citation counts and open access status.`,
		Example: `  europe-pmc-pp-cli grant-impact --agency "Wellcome Trust" --grant-id WT098051
  europe-pmc-pp-cli grant-impact --agency NIH --grant-id R01CA12345
  europe-pmc-pp-cli grant-impact list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagGrantID == "" {
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

			if err := ensureGrantPublicationTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			// Build query with grant fields
			query := fmt.Sprintf(`GRANT_ID:"%s"`, flagGrantID)
			if flagAgency != "" {
				query = fmt.Sprintf(`GRANT_ID:"%s" AND GRANT_AGENCY:"%s"`, flagGrantID, flagAgency)
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
				HitCount   int `json:"hitCount"`
				ResultList struct {
					Result []struct {
						ID           string `json:"id"`
						PMID         string `json:"pmid"`
						Title        string `json:"title"`
						CitedByCount int    `json:"citedByCount"`
						IsOpenAccess string `json:"isOpenAccess"`
					} `json:"result"`
				} `json:"resultList"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			stored := 0
			totalCitations := 0
			openAccessCount := 0

			for _, r := range envelope.ResultList.Result {
				pmid := r.PMID
				if pmid == "" {
					pmid = r.ID
				}
				isOA := 0
				if r.IsOpenAccess == "Y" {
					isOA = 1
					openAccessCount++
				}
				totalCitations += r.CitedByCount

				_, err := db.DB().Exec(
					`INSERT INTO grant_publications (grant_id, agency, pmid, title, cited_by_count, is_open_access, found_at)
					 VALUES (?, ?, ?, ?, ?, ?, ?)
					 ON CONFLICT(grant_id, pmid) DO UPDATE SET
					   cited_by_count = excluded.cited_by_count,
					   is_open_access = excluded.is_open_access`,
					flagGrantID, flagAgency, pmid, r.Title, r.CitedByCount, isOA, time.Now(),
				)
				if err == nil {
					stored++
				}
			}

			oaRate := 0.0
			if len(envelope.ResultList.Result) > 0 {
				oaRate = float64(openAccessCount) / float64(len(envelope.ResultList.Result)) * 100
			}

			result := map[string]any{
				"grant_id":        flagGrantID,
				"agency":          flagAgency,
				"publications":    len(envelope.ResultList.Result),
				"total_citations": totalCitations,
				"open_access":     openAccessCount,
				"oa_rate_percent": fmt.Sprintf("%.1f", oaRate),
				"stored":          stored,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagAgency, "agency", "", "Grant agency (e.g. 'Wellcome Trust', NIH)")
	cmd.Flags().StringVar(&flagGrantID, "grant-id", "", "Grant identifier")
	cmd.Flags().IntVar(&flagPageSize, "page-size", 25, "Results per page")

	cmd.AddCommand(newGrantImpactListCmd(flags))
	return cmd
}

func newGrantImpactListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List tracked grants with publication counts, citation totals, and OA rates",
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

			if err := ensureGrantPublicationTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			rows, err := db.DB().Query(
				`SELECT grant_id, agency, COUNT(*) as pub_count,
				        SUM(cited_by_count) as total_citations,
				        SUM(is_open_access) as oa_count
				 FROM grant_publications
				 GROUP BY grant_id, agency
				 ORDER BY total_citations DESC`,
			)
			if err != nil {
				return fmt.Errorf("querying grants: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var grantID, agency string
				var pubCount, totalCitations, oaCount int
				if err := rows.Scan(&grantID, &agency, &pubCount, &totalCitations, &oaCount); err != nil {
					continue
				}
				results = append(results, map[string]any{
					"grant_id":        grantID,
					"agency":          agency,
					"publications":    pubCount,
					"total_citations": totalCitations,
					"open_access":     oaCount,
				})
			}
			if len(results) == 0 {
				results = []map[string]any{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
}
