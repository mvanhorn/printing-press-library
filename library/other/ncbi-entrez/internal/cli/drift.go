package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// driftResult captures the diff between a saved snapshot and the current results.
type driftResult struct {
	Name      string   `json:"name"`
	Since     string   `json:"since,omitempty"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Unchanged int      `json:"unchanged"`
	AddedN    int      `json:"added_count"`
	RemovedN  int      `json:"removed_count"`
}

func ensureDriftTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS drift_snapshots (
			name TEXT NOT NULL,
			query TEXT NOT NULL,
			db_name TEXT NOT NULL DEFAULT 'pubmed',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (name)
		)`,
		`CREATE TABLE IF NOT EXISTS drift_pmids (
			snapshot_name TEXT NOT NULL,
			pmid TEXT NOT NULL,
			PRIMARY KEY (snapshot_name, pmid),
			FOREIGN KEY (snapshot_name) REFERENCES drift_snapshots(name)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating drift tables: %w", err)
		}
	}
	return nil
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "drift <name>",
		Short: "Detect drift in saved search results over time",
		Long: `Saved Search Drift Detector — compare snapshots of a PubMed search
to find which PMIDs appeared, disappeared, or were retracted since a
given date.

Use 'drift snapshot' to save a new snapshot and 'drift <name> --since <date>'
to compare current results against the stored snapshot.`,
		Example: `  ncbi-entrez-pp-cli drift snapshot my-search --query "GLP-1 agonist safety" --db pubmed
  ncbi-entrez-pp-cli drift my-search --since 2025-06-01
  ncbi-entrez-pp-cli drift list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

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

			if err := ensureDriftTables(db); err != nil {
				return err
			}

			// Load stored snapshot
			var query, dbName string
			err = db.DB().QueryRow(
				`SELECT query, db_name FROM drift_snapshots WHERE name = ?`, name,
			).Scan(&query, &dbName)
			if err == sql.ErrNoRows {
				return fmt.Errorf("snapshot %q not found. Run 'drift snapshot %s --query <term>' first", name, name)
			}
			if err != nil {
				return fmt.Errorf("reading snapshot: %w", err)
			}

			// Load stored PMIDs
			rows, err := db.DB().Query(
				`SELECT pmid FROM drift_pmids WHERE snapshot_name = ?`, name,
			)
			if err != nil {
				return fmt.Errorf("reading snapshot PMIDs: %w", err)
			}
			storedSet := make(map[string]bool)
			for rows.Next() {
				var pmid string
				if err := rows.Scan(&pmid); err != nil {
					rows.Close()
					return err
				}
				storedSet[pmid] = true
			}
			rows.Close()

			// Run current search with optional date filter
			params := map[string]string{
				"db":      dbName,
				"term":    query,
				"retmax":  "10000",
				"retmode": "json",
			}
			if flagSince != "" {
				params["datetype"] = "edat"
				params["mindate"] = flagSince
			}

			data, err := c.Get("/esearch.fcgi", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			currentPMIDs := extractPMIDsFromEsearch(data)
			currentSet := make(map[string]bool, len(currentPMIDs))
			for _, pmid := range currentPMIDs {
				currentSet[pmid] = true
			}

			// Compute diff
			var added, removed []string
			unchanged := 0

			for pmid := range currentSet {
				if !storedSet[pmid] {
					added = append(added, pmid)
				} else {
					unchanged++
				}
			}
			for pmid := range storedSet {
				if !currentSet[pmid] {
					removed = append(removed, pmid)
				}
			}

			result := driftResult{
				Name:      name,
				Since:     flagSince,
				Added:     added,
				Removed:   removed,
				Unchanged: unchanged,
				AddedN:    len(added),
				RemovedN:  len(removed),
			}
			if result.Added == nil {
				result.Added = []string{}
			}
			if result.Removed == nil {
				result.Removed = []string{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "", "Compare results since this date (YYYY/MM/DD or YYYY-MM-DD)")

	cmd.AddCommand(newDriftSnapshotCmd(flags))
	cmd.AddCommand(newDriftListCmd(flags))

	return cmd
}

func newDriftSnapshotCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "snapshot <name>",
		Short: "Save a snapshot of current search results",
		Example: `  ncbi-entrez-pp-cli drift snapshot my-search --query "GLP-1 safety"
  ncbi-entrez-pp-cli drift snapshot cardio-review --query "cardiac safety" --db pubmed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("snapshot name required")
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			if flagQuery == "" {
				return fmt.Errorf("--query is required")
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

			if err := ensureDriftTables(db); err != nil {
				return err
			}

			// Run the search
			params := map[string]string{
				"db":      flagDB,
				"term":    flagQuery,
				"retmax":  "10000",
				"retmode": "json",
			}
			data, err := c.Get("/esearch.fcgi", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			pmids := extractPMIDsFromEsearch(data)

			// Store snapshot in a transaction
			tx, err := db.DB().Begin()
			if err != nil {
				return err
			}
			defer tx.Rollback()

			// Upsert snapshot metadata
			_, err = tx.Exec(
				`INSERT INTO drift_snapshots (name, query, db_name, created_at)
				 VALUES (?, ?, ?, CURRENT_TIMESTAMP)
				 ON CONFLICT(name) DO UPDATE SET query = excluded.query, db_name = excluded.db_name, created_at = CURRENT_TIMESTAMP`,
				name, flagQuery, flagDB,
			)
			if err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}

			// Clear old PMIDs for this snapshot
			if _, err := tx.Exec(`DELETE FROM drift_pmids WHERE snapshot_name = ?`, name); err != nil {
				return fmt.Errorf("clearing old PMIDs: %w", err)
			}

			// Insert new PMIDs
			stmt, err := tx.Prepare(`INSERT INTO drift_pmids (snapshot_name, pmid) VALUES (?, ?)`)
			if err != nil {
				return err
			}
			defer stmt.Close()

			for _, pmid := range pmids {
				if _, err := stmt.Exec(name, pmid); err != nil {
					return fmt.Errorf("inserting PMID %s: %w", pmid, err)
				}
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			result := map[string]any{
				"status":     "snapshot_saved",
				"name":       name,
				"query":      flagQuery,
				"db":         flagDB,
				"pmid_count": len(pmids),
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagQuery, "query", "", "Entrez search query to snapshot")
	cmd.Flags().StringVar(&flagDB, "db", "pubmed", "Target database")

	return cmd
}

func newDriftListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List saved drift snapshots with their queries, databases, and PMID counts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # List all saved snapshots
  ncbi-entrez-pp-cli drift list --json

  # Agent-friendly list
  ncbi-entrez-pp-cli drift list --agent`,
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

			if err := ensureDriftTables(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(
				`SELECT s.name, s.query, s.db_name, s.created_at, COUNT(p.pmid) as pmid_count
				 FROM drift_snapshots s
				 LEFT JOIN drift_pmids p ON s.name = p.snapshot_name
				 GROUP BY s.name
				 ORDER BY s.created_at DESC`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			var snapshots []map[string]any
			for rows.Next() {
				var name, query, dbName, createdAt string
				var count int
				if err := rows.Scan(&name, &query, &dbName, &createdAt, &count); err != nil {
					return err
				}
				snapshots = append(snapshots, map[string]any{
					"name":       name,
					"query":      query,
					"db":         dbName,
					"created_at": createdAt,
					"pmid_count": count,
				})
			}
			if snapshots == nil {
				snapshots = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), snapshots, flags)
		},
	}
}

// extractPMIDsFromEsearch parses an ESearch JSON response and returns the list of IDs.
func extractPMIDsFromEsearch(data json.RawMessage) []string {
	// Try the esearchresult envelope
	var resp struct {
		ESearchResult struct {
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	if json.Unmarshal(data, &resp) == nil && len(resp.ESearchResult.IDList) > 0 {
		return resp.ESearchResult.IDList
	}

	// Try direct idlist
	var direct struct {
		IDList []string `json:"idlist"`
	}
	if json.Unmarshal(data, &direct) == nil && len(direct.IDList) > 0 {
		return direct.IDList
	}

	// Try flat array of strings
	var ids []string
	if json.Unmarshal(data, &ids) == nil {
		return ids
	}

	// Try flat array of numbers
	var nums []json.Number
	if json.Unmarshal(data, &nums) == nil {
		for _, n := range nums {
			ids = append(ids, n.String())
		}
		return ids
	}

	// Try to extract from a generic object
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) == nil {
		for _, key := range []string{"idlist", "ids", "IdList", "ID", "id"} {
			lowerKey := strings.ToLower(key)
			for k, v := range obj {
				if strings.ToLower(k) == lowerKey {
					var list []string
					if json.Unmarshal(v, &list) == nil {
						return list
					}
				}
			}
		}
	}

	return nil
}
