package cli

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensureCanonicalIDTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS canonical_ids (
			doi TEXT,
			pmid TEXT,
			pmcid TEXT,
			ppr_id TEXT,
			source TEXT,
			title TEXT,
			resolved_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_canonical_doi ON canonical_ids(doi) WHERE doi IS NOT NULL AND doi != ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_canonical_pmid ON canonical_ids(pmid) WHERE pmid IS NOT NULL AND pmid != ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_canonical_pmcid ON canonical_ids(pmcid) WHERE pmcid IS NOT NULL AND pmcid != ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_canonical_ppr ON canonical_ids(ppr_id) WHERE ppr_id IS NOT NULL AND ppr_id != ''`,
	}
	for _, stmt := range stmts {
		if _, err := db.DB().Exec(stmt); err != nil {
			// Unique index might fail on existing data; ignore
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}
	return nil
}

func newDedupCmd(flags *rootFlags) *cobra.Command {
	var flagDOI string
	var flagInput string

	cmd := &cobra.Command{
		Use:   "dedup",
		Short: "Resolve any article identifier to all other identifiers",
		Long: `Given a DOI, PMID, PMCID, or PPR ID, resolve all other identifiers for
the same article. Supports bulk deduplication from a file of mixed-format IDs.`,
		Example: `  europe-pmc-pp-cli dedup --doi 10.1038/s41579-020-00459-7
  europe-pmc-pp-cli dedup --input ids.txt
  europe-pmc-pp-cli dedup list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagDOI == "" && flagInput == "" {
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

			if err := ensureCanonicalIDTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			var ids []string
			if flagDOI != "" {
				ids = append(ids, flagDOI)
			}
			if flagInput != "" {
				file, err := os.Open(flagInput)
				if err != nil {
					return fmt.Errorf("opening input file: %w", err)
				}
				defer file.Close()
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line != "" && !strings.HasPrefix(line, "#") {
						ids = append(ids, line)
					}
				}
			}

			var results []map[string]any
			for _, id := range ids {
				resolved, err := resolveIdentifier(c, db, id)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not resolve %s: %v\n", id, err)
					continue
				}
				results = append(results, resolved)
			}

			if len(results) == 0 {
				results = []map[string]any{}
			}

			output := map[string]any{
				"resolved_count": len(results),
				"results":        results,
			}
			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}

	cmd.Flags().StringVar(&flagDOI, "doi", "", "DOI to resolve (e.g. 10.1038/s41579-020-00459-7)")
	cmd.Flags().StringVar(&flagInput, "input", "", "File with one ID per line for bulk resolution")

	cmd.AddCommand(newDedupListCmd(flags))
	return cmd
}

type dedupClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func resolveIdentifier(c dedupClient, db *store.Store, id string) (map[string]any, error) {
	// Determine the query based on ID format
	var query string
	switch {
	case strings.HasPrefix(id, "10."):
		query = fmt.Sprintf(`DOI:"%s"`, id)
	case strings.HasPrefix(id, "PMC"):
		query = fmt.Sprintf("PMCID:%s", id)
	case strings.HasPrefix(id, "PPR"):
		query = fmt.Sprintf("SRC:PPR AND EXT_ID:%s", id)
	default:
		// Try as PMID
		query = fmt.Sprintf("EXT_ID:%s", id)
	}

	params := map[string]string{
		"query":      query,
		"format":     "json",
		"resultType": "core",
		"pageSize":   "5",
	}
	data, err := c.Get("/search", params)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if len(envelope.ResultList.Result) == 0 {
		return nil, fmt.Errorf("no results found for %s", id)
	}

	r := envelope.ResultList.Result[0]
	pprID := ""
	if r.Source == "PPR" {
		pprID = r.ID
	}

	// Store the canonical mapping
	_, err = db.DB().Exec(
		`INSERT OR REPLACE INTO canonical_ids (doi, pmid, pmcid, ppr_id, source, title, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.DOI, r.PMID, r.PMCID, pprID, r.Source, r.Title, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("storing canonical ID: %w", err)
	}

	result := map[string]any{
		"query_id": id,
		"doi":      r.DOI,
		"pmid":     r.PMID,
		"pmcid":    r.PMCID,
		"source":   r.Source,
		"title":    truncate(r.Title, 120),
	}
	if pprID != "" {
		result["ppr_id"] = pprID
	}
	return result, nil
}

func newDedupListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "Show stored canonical ID mappings",
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

			if err := ensureCanonicalIDTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			rows, err := db.DB().Query(
				`SELECT doi, pmid, pmcid, ppr_id, source, title, resolved_at
				 FROM canonical_ids ORDER BY resolved_at DESC`,
			)
			if err != nil {
				return fmt.Errorf("querying canonical IDs: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var doi, pmid, pmcid, pprID, source, title sql.NullString
				var resolvedAt time.Time
				if err := rows.Scan(&doi, &pmid, &pmcid, &pprID, &source, &title, &resolvedAt); err != nil {
					continue
				}
				row := map[string]any{
					"resolved_at": resolvedAt.Format(time.RFC3339),
				}
				if doi.Valid && doi.String != "" {
					row["doi"] = doi.String
				}
				if pmid.Valid && pmid.String != "" {
					row["pmid"] = pmid.String
				}
				if pmcid.Valid && pmcid.String != "" {
					row["pmcid"] = pmcid.String
				}
				if pprID.Valid && pprID.String != "" {
					row["ppr_id"] = pprID.String
				}
				if source.Valid {
					row["source"] = source.String
				}
				if title.Valid {
					row["title"] = title.String
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
