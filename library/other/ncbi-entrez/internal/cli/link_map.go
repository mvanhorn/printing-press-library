package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// linkMapNode represents a node in the materialized cross-database link graph.
type linkMapNode struct {
	ID        string `json:"id"`
	DB        string `json:"db"`
	DataJSON  string `json:"data_json,omitempty"`
	FetchedAt string `json:"fetched_at"`
}

// linkMapEdge represents a directed edge between two nodes in different databases.
type linkMapEdge struct {
	SourceID  string `json:"source_id"`
	SourceDB  string `json:"source_db"`
	TargetID  string `json:"target_id"`
	TargetDB  string `json:"target_db"`
	LinkName  string `json:"link_name"`
	CreatedAt string `json:"created_at"`
}

// linkMapStatus summarises the materialized graph.
type linkMapStatus struct {
	TotalNodes int             `json:"total_nodes"`
	TotalEdges int             `json:"total_edges"`
	PerDB      map[string]int  `json:"per_db"`
	EdgePairs  []edgePairCount `json:"edge_pairs"`
}

type edgePairCount struct {
	SourceDB string `json:"source_db"`
	TargetDB string `json:"target_db"`
	Count    int    `json:"count"`
}

func ensureLinkMapTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS link_map_nodes (
			id TEXT NOT NULL,
			db TEXT NOT NULL,
			data_json TEXT,
			fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, db)
		)`,
		`CREATE TABLE IF NOT EXISTS link_map_edges (
			source_id TEXT NOT NULL,
			source_db TEXT NOT NULL,
			target_id TEXT NOT NULL,
			target_db TEXT NOT NULL,
			link_name TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (source_id, source_db, target_id, target_db)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_link_map_nodes_db ON link_map_nodes(db)`,
		`CREATE INDEX IF NOT EXISTS idx_link_map_edges_source ON link_map_edges(source_db, target_db)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating link_map tables: %w", err)
		}
	}
	return nil
}

func newLinkMapCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagThrough string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "link-map <query>",
		Short: "Materialize cross-database links from a search query",
		Long: strings.TrimSpace(`
Cross-Database Link Materializer -- searches a source database and
traverses ELink through a chain of target databases, storing the full
node/edge graph in SQLite.

Use 'link-map query' to query the materialized graph and 'link-map status'
to inspect node/edge counts.`),
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli link-map "BRCA1" --from pubmed --through gene,protein
  ncbi-entrez-pp-cli link-map "TP53" --from pubmed --through gene --limit 10
  ncbi-entrez-pp-cli link-map status
  ncbi-entrez-pp-cli link-map query --from pubmed --to protein`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
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

			if err := ensureLinkMapTables(db); err != nil {
				return err
			}

			throughDBs := strings.Split(flagThrough, ",")
			for i := range throughDBs {
				throughDBs[i] = strings.TrimSpace(throughDBs[i])
			}

			// Step 1: ESearch the --from database
			params := map[string]string{
				"db":      flagFrom,
				"term":    query,
				"retmax":  fmt.Sprintf("%d", flagLimit),
				"retmode": "json",
			}
			data, err := c.Get("/esearch.fcgi", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			sourceIDs := extractPMIDsFromEsearch(data)
			if len(sourceIDs) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"status":  "no_results",
					"query":   query,
					"from_db": flagFrom,
				}, flags)
			}

			fmt.Fprintf(os.Stderr, "found %d IDs in %s\n", len(sourceIDs), flagFrom)

			// Insert source nodes
			tx, err := db.DB().Begin()
			if err != nil {
				return err
			}
			for _, id := range sourceIDs {
				_, _ = tx.Exec(
					`INSERT OR IGNORE INTO link_map_nodes (id, db, fetched_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
					id, flagFrom,
				)
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("committing source nodes: %w", err)
			}

			// Step 2: For each --through database, ELink from previous layer
			currentIDs := sourceIDs
			currentDB := flagFrom

			for _, targetDB := range throughDBs {
				if targetDB == "" {
					continue
				}
				fmt.Fprintf(os.Stderr, "linking %s -> %s (%d IDs)...\n", currentDB, targetDB, len(currentIDs))

				batchSize := 50
				var nextIDs []string

				for i := 0; i < len(currentIDs); i += batchSize {
					end := i + batchSize
					if end > len(currentIDs) {
						end = len(currentIDs)
					}
					batch := currentIDs[i:end]

					linkParams := map[string]string{
						"dbfrom":  currentDB,
						"db":      targetDB,
						"id":      strings.Join(batch, ","),
						"retmode": "json",
						"cmd":     "neighbor",
					}

					linkData, err := c.Get("/elink.fcgi", linkParams)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: ELink batch failed: %v\n", err)
						continue
					}

					// Parse ELink response
					linked := parseLinkMapElink(linkData, currentDB, targetDB)

					tx, err := db.DB().Begin()
					if err != nil {
						return err
					}

					for sourceID, targets := range linked {
						for _, target := range targets {
							_, _ = tx.Exec(
								`INSERT OR IGNORE INTO link_map_nodes (id, db, fetched_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
								target.id, targetDB,
							)
							_, _ = tx.Exec(
								`INSERT OR IGNORE INTO link_map_edges (source_id, source_db, target_id, target_db, link_name, created_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
								sourceID, currentDB, target.id, targetDB, target.linkName,
							)
							nextIDs = append(nextIDs, target.id)
						}
					}

					if err := tx.Commit(); err != nil {
						return fmt.Errorf("committing link batch: %w", err)
					}
				}

				currentIDs = nextIDs
				currentDB = targetDB
			}

			// Build output
			var nodeCount, edgeCount int
			db.DB().QueryRow(`SELECT COUNT(*) FROM link_map_nodes`).Scan(&nodeCount)
			db.DB().QueryRow(`SELECT COUNT(*) FROM link_map_edges`).Scan(&edgeCount)

			result := map[string]any{
				"status":      "complete",
				"query":       query,
				"from_db":     flagFrom,
				"through":     throughDBs,
				"source_ids":  len(sourceIDs),
				"total_nodes": nodeCount,
				"total_edges": edgeCount,
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "pubmed", "Source database to search")
	cmd.Flags().StringVar(&flagThrough, "through", "gene", "Comma-separated databases to traverse via ELink")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum number of initial search results")

	cmd.AddCommand(newLinkMapQueryCmd(flags))
	cmd.AddCommand(newLinkMapStatusCmd(flags))

	return cmd
}

type linkTarget struct {
	id       string
	linkName string
}

// parseLinkMapElink extracts source->target mappings from an ELink JSON response.
func parseLinkMapElink(data json.RawMessage, fromDB, toDB string) map[string][]linkTarget {
	result := make(map[string][]linkTarget)

	// Try the standard ELink JSON format
	var resp struct {
		LinkSets []json.RawMessage `json:"linksets"`
	}
	if json.Unmarshal(data, &resp) != nil {
		return result
	}

	for _, lsRaw := range resp.LinkSets {
		var ls struct {
			IDs        []json.RawMessage `json:"ids"`
			LinkSetDBs []struct {
				DBTo     string `json:"dbto"`
				LinkName string `json:"linkname"`
				Links    []struct {
					ID json.RawMessage `json:"id"`
				} `json:"links"`
			} `json:"linksetdbs"`
		}
		if json.Unmarshal(lsRaw, &ls) != nil {
			continue
		}

		sourceID := ""
		if len(ls.IDs) > 0 {
			var sid string
			if json.Unmarshal(ls.IDs[0], &sid) == nil {
				sourceID = sid
			} else {
				var num float64
				if json.Unmarshal(ls.IDs[0], &num) == nil {
					sourceID = fmt.Sprintf("%d", int64(num))
				}
			}
		}
		if sourceID == "" {
			continue
		}

		for _, ldb := range ls.LinkSetDBs {
			linkName := ldb.LinkName
			for _, link := range ldb.Links {
				var tid string
				if json.Unmarshal(link.ID, &tid) == nil {
					result[sourceID] = append(result[sourceID], linkTarget{id: tid, linkName: linkName})
				} else {
					var num float64
					if json.Unmarshal(link.ID, &num) == nil {
						result[sourceID] = append(result[sourceID], linkTarget{id: fmt.Sprintf("%d", int64(num)), linkName: linkName})
					}
				}
			}
		}
	}

	return result
}

func newLinkMapQueryCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:         "query",
		Short:       "Query the materialized link graph",
		Annotations: map[string]string{"mcp:read-only": "true"},
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

			if err := ensureLinkMapTables(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(
				`SELECT e.source_id, e.source_db, e.target_id, e.target_db, e.link_name, e.created_at
				 FROM link_map_edges e
				 WHERE e.source_db = ? AND e.target_db = ?
				 ORDER BY e.created_at DESC
				 LIMIT 100`,
				flagFrom, flagTo,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			var edges []linkMapEdge
			for rows.Next() {
				var e linkMapEdge
				if err := rows.Scan(&e.SourceID, &e.SourceDB, &e.TargetID, &e.TargetDB, &e.LinkName, &e.CreatedAt); err != nil {
					return err
				}
				edges = append(edges, e)
			}
			if edges == nil {
				edges = []linkMapEdge{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"from":  flagFrom,
				"to":    flagTo,
				"edges": edges,
				"count": len(edges),
			}, flags)
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "pubmed", "Source database")
	cmd.Flags().StringVar(&flagTo, "to", "gene", "Target database")

	return cmd
}

func newLinkMapStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show node and edge counts in the materialized cross-database link graph, grouped by database pair",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Show current graph size
  ncbi-entrez-pp-cli link-map status --json

  # Agent-friendly summary
  ncbi-entrez-pp-cli link-map status --agent`,
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

			if err := ensureLinkMapTables(db); err != nil {
				return err
			}

			status := linkMapStatus{
				PerDB:     make(map[string]int),
				EdgePairs: []edgePairCount{},
			}

			db.DB().QueryRow(`SELECT COUNT(*) FROM link_map_nodes`).Scan(&status.TotalNodes)
			db.DB().QueryRow(`SELECT COUNT(*) FROM link_map_edges`).Scan(&status.TotalEdges)

			// Per-database node counts
			rows, err := db.DB().Query(`SELECT db, COUNT(*) FROM link_map_nodes GROUP BY db`)
			if err == nil {
				for rows.Next() {
					var dbName string
					var count int
					if rows.Scan(&dbName, &count) == nil {
						status.PerDB[dbName] = count
					}
				}
				rows.Close()
			}

			// Edge pair counts
			rows, err = db.DB().Query(
				`SELECT source_db, target_db, COUNT(*) FROM link_map_edges GROUP BY source_db, target_db`,
			)
			if err == nil {
				for rows.Next() {
					var ep edgePairCount
					if rows.Scan(&ep.SourceDB, &ep.TargetDB, &ep.Count) == nil {
						status.EdgePairs = append(status.EdgePairs, ep)
					}
				}
				rows.Close()
			}

			return printJSONFiltered(cmd.OutOrStdout(), status, flags)
		},
	}
}

// Compile-time guards.
var _ = time.Now
var _ = sql.ErrNoRows
