package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// snowballNode represents a paper in the citation graph.
type snowballNode struct {
	PMID      string `json:"pmid"`
	Depth     int    `json:"depth"`
	Fetched   bool   `json:"fetched"`
	CreatedAt string `json:"created_at"`
}

// snowballEdge represents a citation link between two papers.
type snowballEdge struct {
	SourcePMID string `json:"source_pmid"`
	TargetPMID string `json:"target_pmid"`
	Depth      int    `json:"depth"`
}

// snowballStatus summarises the current state of the citation graph.
type snowballStatus struct {
	TotalNodes    int `json:"total_nodes"`
	TotalEdges    int `json:"total_edges"`
	FetchedNodes  int `json:"fetched_nodes"`
	FrontierNodes int `json:"frontier_nodes"`
	MaxDepth      int `json:"max_depth"`
}

func ensureSnowballTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS snowball_nodes (
			pmid TEXT PRIMARY KEY,
			depth INTEGER NOT NULL,
			fetched INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS snowball_edges (
			source_pmid TEXT NOT NULL,
			target_pmid TEXT NOT NULL,
			depth INTEGER NOT NULL,
			PRIMARY KEY (source_pmid, target_pmid)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snowball_nodes_depth ON snowball_nodes(depth)`,
		`CREATE INDEX IF NOT EXISTS idx_snowball_nodes_fetched ON snowball_nodes(fetched)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating snowball tables: %w", err)
		}
	}
	return nil
}

func newSnowballCmd(flags *rootFlags) *cobra.Command {
	var flagSeed string
	var flagDepth int
	var flagFrontierOnly bool

	cmd := &cobra.Command{
		Use:   "snowball",
		Short: "Recursively build a citation graph from seed PMIDs",
		Long: `Citation Snowball Tracker — takes seed PMIDs and recursively expands
the citation graph using ELink (pubmed_pubmed_citedin). On subsequent runs,
only fetches new frontier papers not already seen.

Use 'snowball status' to inspect the current graph.`,
		Example: `  ncbi-entrez-pp-cli snowball --seed 12345,67890 --depth 2
  ncbi-entrez-pp-cli snowball --seed 12345 --frontier-only
  ncbi-entrez-pp-cli snowball status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagSeed == "" && !flags.dryRun {
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

			if err := ensureSnowballTables(db); err != nil {
				return err
			}

			seeds := strings.Split(flagSeed, ",")
			for i := range seeds {
				seeds[i] = strings.TrimSpace(seeds[i])
			}

			// Insert seed nodes at depth 0
			for _, pmid := range seeds {
				if pmid == "" {
					continue
				}
				_, err := db.DB().Exec(
					`INSERT OR IGNORE INTO snowball_nodes (pmid, depth, fetched) VALUES (?, 0, 0)`,
					pmid,
				)
				if err != nil {
					return fmt.Errorf("inserting seed %s: %w", pmid, err)
				}
			}

			// Iterative BFS expansion up to flagDepth
			for d := 0; d < flagDepth; d++ {
				// Find unfetched nodes at current depth
				rows, err := db.DB().Query(
					`SELECT pmid FROM snowball_nodes WHERE depth = ? AND fetched = 0`,
					d,
				)
				if err != nil {
					return fmt.Errorf("querying frontier at depth %d: %w", d, err)
				}

				var frontier []string
				for rows.Next() {
					var pmid string
					if err := rows.Scan(&pmid); err != nil {
						rows.Close()
						return err
					}
					frontier = append(frontier, pmid)
				}
				rows.Close()

				if len(frontier) == 0 {
					fmt.Fprintf(os.Stderr, "depth %d: no unfetched nodes, stopping\n", d)
					break
				}

				fmt.Fprintf(os.Stderr, "depth %d: expanding %d nodes...\n", d, len(frontier))

				// Process in batches of up to 50 IDs per ELink request
				batchSize := 50
				for i := 0; i < len(frontier); i += batchSize {
					end := i + batchSize
					if end > len(frontier) {
						end = len(frontier)
					}
					batch := frontier[i:end]
					idList := strings.Join(batch, ",")

					params := map[string]string{
						"dbfrom":   "pubmed",
						"db":       "pubmed",
						"id":       idList,
						"linkname": "pubmed_pubmed_citedin",
						"retmode":  "json",
						"cmd":      "neighbor",
					}

					data, err := c.Get("/elink.fcgi", params)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: ELink batch failed: %v\n", err)
						continue
					}

					// Parse ELink JSON response to extract linked PMIDs
					citingPMIDs := parseElinkCitations(data)

					// Insert new nodes and edges
					tx, txErr := db.DB().Begin()
					if txErr != nil {
						return txErr
					}

					for _, source := range batch {
						if linked, ok := citingPMIDs[source]; ok {
							for _, target := range linked {
								// Insert node at next depth if not already present
								_, _ = tx.Exec(
									`INSERT OR IGNORE INTO snowball_nodes (pmid, depth, fetched) VALUES (?, ?, 0)`,
									target, d+1,
								)
								// Insert edge
								_, _ = tx.Exec(
									`INSERT OR IGNORE INTO snowball_edges (source_pmid, target_pmid, depth) VALUES (?, ?, ?)`,
									source, target, d+1,
								)
							}
						}
						// Mark source as fetched
						_, _ = tx.Exec(
							`UPDATE snowball_nodes SET fetched = 1 WHERE pmid = ?`,
							source,
						)
					}

					if err := tx.Commit(); err != nil {
						return fmt.Errorf("committing batch: %w", err)
					}
				}
			}

			// Build output
			var result any
			if flagFrontierOnly {
				rows, err := db.DB().Query(
					`SELECT pmid, depth, created_at FROM snowball_nodes WHERE fetched = 0 ORDER BY depth, pmid`,
				)
				if err != nil {
					return err
				}
				defer rows.Close()
				var nodes []snowballNode
				for rows.Next() {
					var n snowballNode
					var ca sql.NullString
					if err := rows.Scan(&n.PMID, &n.Depth, &ca); err != nil {
						return err
					}
					n.Fetched = false
					if ca.Valid {
						n.CreatedAt = ca.String
					}
					nodes = append(nodes, n)
				}
				if nodes == nil {
					nodes = []snowballNode{}
				}
				result = map[string]any{
					"frontier":       nodes,
					"frontier_count": len(nodes),
				}
			} else {
				status, err := getSnowballStatus(db)
				if err != nil {
					return err
				}
				result = map[string]any{
					"status":      "complete",
					"seeds":       seeds,
					"max_depth":   flagDepth,
					"graph_stats": status,
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagSeed, "seed", "", "Comma-separated seed PMIDs")
	cmd.Flags().IntVar(&flagDepth, "depth", 1, "Maximum citation depth to expand")
	cmd.Flags().BoolVar(&flagFrontierOnly, "frontier-only", false, "Show only unfetched frontier papers")

	cmd.AddCommand(newSnowballStatusCmd(flags))

	return cmd
}

func newSnowballStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show citation graph statistics including total nodes, edges, fetched fraction, and frontier size",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Check graph progress
  ncbi-entrez-pp-cli snowball status --json

  # Agent-friendly summary
  ncbi-entrez-pp-cli snowball status --agent`,
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

			if err := ensureSnowballTables(db); err != nil {
				return err
			}

			status, err := getSnowballStatus(db)
			if err != nil {
				return err
			}

			return printJSONFiltered(cmd.OutOrStdout(), status, flags)
		},
	}
}

func getSnowballStatus(db *store.Store) (*snowballStatus, error) {
	var s snowballStatus

	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM snowball_nodes`).Scan(&s.TotalNodes); err != nil {
		return nil, err
	}
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM snowball_edges`).Scan(&s.TotalEdges); err != nil {
		return nil, err
	}
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM snowball_nodes WHERE fetched = 1`).Scan(&s.FetchedNodes); err != nil {
		return nil, err
	}
	s.FrontierNodes = s.TotalNodes - s.FetchedNodes

	var maxDepth sql.NullInt64
	if err := db.DB().QueryRow(`SELECT MAX(depth) FROM snowball_nodes`).Scan(&maxDepth); err != nil {
		return nil, err
	}
	if maxDepth.Valid {
		s.MaxDepth = int(maxDepth.Int64)
	}

	return &s, nil
}

// parseElinkCitations extracts source->target PMID mappings from an ELink JSON response.
// NCBI elink JSON uses this structure:
//
//	{"linksets": [{"dbfrom":"pubmed","ids":[{"value":"35924517"}],
//	  "linksetdbs":[{"dbto":"pubmed","linkname":"pubmed_pubmed_citedin",
//	    "links":[{"id":"39201234"},{"id":"38501111"},...]}]}]}
//
// The parser handles both string and numeric ID formats for robustness.
func parseElinkCitations(data json.RawMessage) map[string][]string {
	result := make(map[string][]string)

	var resp struct {
		LinkSets []json.RawMessage `json:"linksets"`
	}
	if json.Unmarshal(data, &resp) != nil {
		return result
	}

	for _, lsRaw := range resp.LinkSets {
		var ls map[string]json.RawMessage
		if json.Unmarshal(lsRaw, &ls) != nil {
			continue
		}

		// Extract source ID from "ids" array. NCBI wraps each in {"value":"..."}
		// but some responses use plain strings or numbers.
		sourceID := ""
		if idsRaw, ok := ls["ids"]; ok {
			var idsObjArr []map[string]json.RawMessage
			if json.Unmarshal(idsRaw, &idsObjArr) == nil && len(idsObjArr) > 0 {
				// {"value":"12345"} shape
				if valRaw, vok := idsObjArr[0]["value"]; vok {
					var v string
					if json.Unmarshal(valRaw, &v) == nil {
						sourceID = v
					}
				}
				// fallback: {"id":"12345"} shape
				if sourceID == "" {
					if idRaw, iok := idsObjArr[0]["id"]; iok {
						var v string
						if json.Unmarshal(idRaw, &v) == nil {
							sourceID = v
						}
					}
				}
			}
			// Fallback: plain string array ["12345"]
			if sourceID == "" {
				var plainIDs []json.RawMessage
				if json.Unmarshal(idsRaw, &plainIDs) == nil && len(plainIDs) > 0 {
					var s string
					if json.Unmarshal(plainIDs[0], &s) == nil {
						sourceID = s
					} else {
						var n float64
						if json.Unmarshal(plainIDs[0], &n) == nil {
							sourceID = strconv.FormatInt(int64(n), 10)
						}
					}
				}
			}
		}

		// Also try "idlist" (some retmode variants)
		if sourceID == "" {
			if idlistRaw, ok := ls["idlist"]; ok {
				var ids []string
				if json.Unmarshal(idlistRaw, &ids) == nil && len(ids) > 0 {
					sourceID = ids[0]
				}
			}
		}

		if sourceID == "" {
			continue
		}

		// Extract linked IDs from linksetdbs[].links[]
		ldbsRaw, ok := ls["linksetdbs"]
		if !ok {
			continue
		}
		var ldbs []map[string]json.RawMessage
		if json.Unmarshal(ldbsRaw, &ldbs) != nil {
			continue
		}
		for _, ldb := range ldbs {
			linksRaw, lok := ldb["links"]
			if !lok {
				continue
			}

			var links []json.RawMessage
			if json.Unmarshal(linksRaw, &links) != nil {
				continue
			}
			for _, linkRaw := range links {
				// Try {"id":"12345"} object
				var linkObj map[string]json.RawMessage
				if json.Unmarshal(linkRaw, &linkObj) == nil {
					if idRaw, iok := linkObj["id"]; iok {
						var id string
						if json.Unmarshal(idRaw, &id) == nil && id != "" {
							result[sourceID] = append(result[sourceID], id)
							continue
						}
						var n float64
						if json.Unmarshal(idRaw, &n) == nil {
							result[sourceID] = append(result[sourceID], strconv.FormatInt(int64(n), 10))
							continue
						}
					}
				}
				// Try plain string
				var linkStr string
				if json.Unmarshal(linkRaw, &linkStr) == nil && linkStr != "" {
					result[sourceID] = append(result[sourceID], linkStr)
					continue
				}
				// Try plain number
				var linkNum float64
				if json.Unmarshal(linkRaw, &linkNum) == nil {
					result[sourceID] = append(result[sourceID], strconv.FormatInt(int64(linkNum), 10))
				}
			}
		}
	}

	return result
}

// Compile-time guard: ensure time is used.
var _ = time.Now
