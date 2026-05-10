package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensureTrackedPreprintTables(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS tracked_preprints (
		ppr_id TEXT PRIMARY KEY,
		doi TEXT,
		title TEXT,
		first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		published_version_pmid TEXT,
		published_at DATETIME,
		status TEXT DEFAULT 'preprint'
	)`)
	return err
}

func newTrackPreprintCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagCheckUpdates bool
	var flagPageSize int

	cmd := &cobra.Command{
		Use:   "track-preprint",
		Short: "Track preprints through their lifecycle to peer-reviewed publication",
		Long: `Search Europe PMC preprint sources (SRC:PPR), store preprints locally,
and check whether published versions have appeared.

Use --check-updates to re-check all tracked preprints for publication status.`,
		Example: `  europe-pmc-pp-cli track-preprint --query "SRC:PPR AND CRISPR"
  europe-pmc-pp-cli track-preprint --query "SRC:PPR AND COVID-19" --check-updates
  europe-pmc-pp-cli track-preprint list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagQuery == "" {
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

			if err := ensureTrackedPreprintTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			if flagCheckUpdates && flagQuery == "" {
				return checkAllStoredPreprints(cmd, db, c, flags)
			}

			if flagQuery == "" {
				return cmd.Help()
			}

			// Search for preprints
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
					Result []json.RawMessage `json:"result"`
				} `json:"resultList"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			type resultRow struct {
				PPRID  string `json:"ppr_id"`
				DOI    string `json:"doi"`
				Title  string `json:"title"`
				Status string `json:"status"`
			}

			var tracked []resultRow
			now := time.Now()
			for _, raw := range envelope.ResultList.Result {
				var article struct {
					ID     string `json:"id"`
					DOI    string `json:"doi"`
					Title  string `json:"title"`
					Source string `json:"source"`
					PMID   string `json:"pmid"`
				}
				if err := json.Unmarshal(raw, &article); err != nil {
					continue
				}
				pprID := article.ID
				if pprID == "" {
					continue
				}

				status := "preprint"
				var pubPMID sql.NullString
				var pubAt sql.NullTime

				if flagCheckUpdates && article.DOI != "" {
					// Check if a published version exists by searching for the DOI
					checkParams := map[string]string{
						"query":  fmt.Sprintf("DOI:%s NOT SRC:PPR", article.DOI),
						"format": "json",
					}
					checkData, checkErr := c.Get("/search", checkParams)
					if checkErr == nil {
						var checkEnv struct {
							ResultList struct {
								Result []struct {
									PMID string `json:"pmid"`
								} `json:"result"`
							} `json:"resultList"`
						}
						if json.Unmarshal(checkData, &checkEnv) == nil && len(checkEnv.ResultList.Result) > 0 {
							status = "published"
							pubPMID = sql.NullString{String: checkEnv.ResultList.Result[0].PMID, Valid: true}
							pubAt = sql.NullTime{Time: now, Valid: true}
						}
					}
				}

				_, err := db.DB().Exec(
					`INSERT INTO tracked_preprints (ppr_id, doi, title, first_seen, published_version_pmid, published_at, status)
					 VALUES (?, ?, ?, ?, ?, ?, ?)
					 ON CONFLICT(ppr_id) DO UPDATE SET
					   doi = COALESCE(excluded.doi, tracked_preprints.doi),
					   published_version_pmid = COALESCE(excluded.published_version_pmid, tracked_preprints.published_version_pmid),
					   published_at = COALESCE(excluded.published_at, tracked_preprints.published_at),
					   status = CASE WHEN excluded.status = 'published' THEN 'published' ELSE tracked_preprints.status END`,
					pprID, article.DOI, article.Title, now, pubPMID, pubAt, status,
				)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to store %s: %v\n", pprID, err)
					continue
				}
				tracked = append(tracked, resultRow{
					PPRID:  pprID,
					DOI:    article.DOI,
					Title:  truncate(article.Title, 80),
					Status: status,
				})
			}

			result := map[string]any{
				"tracked_count": len(tracked),
				"query":         flagQuery,
				"preprints":     tracked,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagQuery, "query", "", "Europe PMC search query (e.g. 'SRC:PPR AND CRISPR')")
	cmd.Flags().BoolVar(&flagCheckUpdates, "check-updates", false, "Check tracked preprints for published versions")
	cmd.Flags().IntVar(&flagPageSize, "page-size", 25, "Results per page")

	cmd.AddCommand(newTrackPreprintListCmd(flags))
	return cmd
}

func checkAllStoredPreprints(cmd *cobra.Command, db *store.Store, c *client.Client, flags *rootFlags) error {
	rows, err := db.DB().Query(
		`SELECT ppr_id, doi FROM tracked_preprints WHERE status = 'preprint'`,
	)
	if err != nil {
		return fmt.Errorf("querying stored preprints: %w", err)
	}
	defer rows.Close()

	type entry struct{ pprID, doi string }
	var pending []entry
	for rows.Next() {
		var e entry
		rows.Scan(&e.pprID, &e.doi)
		pending = append(pending, e)
	}

	updated := 0
	for _, e := range pending {
		if e.doi == "" {
			continue
		}
		checkData, err := c.Get("/search", map[string]string{
			"query":      fmt.Sprintf(`DOI:"%s" AND SRC:MED`, e.doi),
			"format":     "json",
			"resultType": "lite",
			"pageSize":   "1",
		})
		if err != nil {
			continue
		}
		var env struct {
			HitCount int `json:"hitCount"`
			ResultList struct {
				Result []struct{ PMID string `json:"pmid"` } `json:"result"`
			} `json:"resultList"`
		}
		if json.Unmarshal(checkData, &env) == nil && len(env.ResultList.Result) > 0 {
			db.DB().Exec(
				`UPDATE tracked_preprints SET status = 'published', published_version_pmid = ?, published_at = ? WHERE ppr_id = ?`,
				env.ResultList.Result[0].PMID, time.Now(), e.pprID,
			)
			updated++
		}
	}

	result := map[string]any{
		"checked": len(pending),
		"updated": updated,
	}
	return printJSONFiltered(cmd.OutOrStdout(), result, flags)
}

func newTrackPreprintListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List all tracked preprints with DOI, publication status, and time-to-publish",
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

			if err := ensureTrackedPreprintTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			rows, err := db.DB().Query(
				`SELECT ppr_id, doi, title, first_seen, published_version_pmid, published_at, status
				 FROM tracked_preprints ORDER BY first_seen DESC`,
			)
			if err != nil {
				return fmt.Errorf("querying preprints: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var pprID, title, status string
				var doi, pubPMID sql.NullString
				var firstSeen time.Time
				var pubAt sql.NullTime
				if err := rows.Scan(&pprID, &doi, &title, &firstSeen, &pubPMID, &pubAt, &status); err != nil {
					continue
				}
				row := map[string]any{
					"ppr_id":     pprID,
					"doi":        doi.String,
					"title":      title,
					"first_seen": firstSeen.Format(time.RFC3339),
					"status":     status,
				}
				if pubPMID.Valid {
					row["published_version_pmid"] = pubPMID.String
				}
				if pubAt.Valid {
					row["published_at"] = pubAt.Time.Format(time.RFC3339)
				}
				results = append(results, row)
			}
			if len(results) == 0 {
				results = []map[string]any{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
}
