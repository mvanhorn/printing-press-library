package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensureCiteGraphTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS cite_graph_nodes (
			id TEXT NOT NULL,
			source TEXT NOT NULL,
			title TEXT,
			year TEXT,
			cited_by_count INTEGER DEFAULT 0,
			PRIMARY KEY (id, source)
		)`,
		`CREATE TABLE IF NOT EXISTS cite_graph_edges (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			edge_type TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (source_id, target_id, edge_type)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.DB().Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func newCiteGraphCmd(flags *rootFlags) *cobra.Command {
	var flagSource string
	var flagID string
	var flagDepth int
	var flagDirection string

	cmd := &cobra.Command{
		Use:   "cite-graph",
		Short: "Build a citation network graph by walking citations and references",
		Long: `Recursively walk citations and references for an article to build a
navigable citation graph stored in local SQLite.

Directions: citations (who cites this), references (what this cites), both.`,
		Example: `  europe-pmc-pp-cli cite-graph --source MED --id 33024307 --depth 2 --direction both
  europe-pmc-pp-cli cite-graph --source MED --id 33024307 --direction citations
  europe-pmc-pp-cli cite-graph status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagSource == "" || flagID == "" {
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

			if err := ensureCiteGraphTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			type queueItem struct {
				source string
				id     string
				depth  int
			}

			visited := map[string]bool{}
			queue := []queueItem{{source: flagSource, id: flagID, depth: 0}}
			nodesAdded := 0
			edgesAdded := 0

			for len(queue) > 0 {
				item := queue[0]
				queue = queue[1:]
				key := item.source + ":" + item.id
				if visited[key] {
					continue
				}
				visited[key] = true

				// Upsert the node
				_, err := db.DB().Exec(
					`INSERT INTO cite_graph_nodes (id, source, title, year, cited_by_count)
					 VALUES (?, ?, '', '', 0)
					 ON CONFLICT(id, source) DO NOTHING`,
					item.id, item.source,
				)
				if err == nil {
					nodesAdded++
				}

				if item.depth >= flagDepth {
					continue
				}

				// Fetch citations (paginated)
				if flagDirection == "citations" || flagDirection == "both" {
					citePath := fmt.Sprintf("/%s/%s/citations", item.source, item.id)
					for page := 1; ; page++ {
						citeParams := map[string]string{"format": "json", "pageSize": "100", "page": fmt.Sprintf("%d", page)}
						citeData, citeErr := c.Get(citePath, citeParams)
						if citeErr != nil {
							break
						}
						refs := parseCitationResults(citeData)
						if len(refs) == 0 {
							break
						}
						for _, ref := range refs {
							_, eErr := db.DB().Exec(
								`INSERT INTO cite_graph_edges (source_id, target_id, edge_type, created_at)
								 VALUES (?, ?, 'cited_by', ?)
								 ON CONFLICT(source_id, target_id, edge_type) DO NOTHING`,
								item.id, ref.id, time.Now(),
							)
							if eErr == nil {
								edgesAdded++
							}
							if !visited[ref.source+":"+ref.id] {
								queue = append(queue, queueItem{source: ref.source, id: ref.id, depth: item.depth + 1})
							}
						}
						if len(refs) < 100 {
							break
						}
					}
				}

				// Fetch references (paginated)
				if flagDirection == "references" || flagDirection == "both" {
					refPath := fmt.Sprintf("/%s/%s/references", item.source, item.id)
					for refPage := 1; ; refPage++ {
						refParams := map[string]string{"format": "json", "pageSize": "100", "page": fmt.Sprintf("%d", refPage)}
						refData, refErr := c.Get(refPath, refParams)
						if refErr != nil {
							break
						}
						refs := parseCitationResults(refData)
						if len(refs) == 0 {
							break
						}
						for _, ref := range refs {
							_, eErr := db.DB().Exec(
								`INSERT INTO cite_graph_edges (source_id, target_id, edge_type, created_at)
								 VALUES (?, ?, 'references', ?)
								 ON CONFLICT(source_id, target_id, edge_type) DO NOTHING`,
								item.id, ref.id, time.Now(),
							)
							if eErr == nil {
								edgesAdded++
							}
							if !visited[ref.source+":"+ref.id] {
								queue = append(queue, queueItem{source: ref.source, id: ref.id, depth: item.depth + 1})
							}
						}
						if len(refs) < 100 {
							break
						}
					}
				}
			}

			result := map[string]any{
				"root_source":  flagSource,
				"root_id":      flagID,
				"depth":        flagDepth,
				"direction":    flagDirection,
				"nodes_added":  nodesAdded,
				"edges_added":  edgesAdded,
				"total_walked": len(visited),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagSource, "source", "MED", "Source database (MED, PMC, PPR, PAT)")
	cmd.Flags().StringVar(&flagID, "id", "", "Article identifier (e.g. PMID)")
	cmd.Flags().IntVar(&flagDepth, "depth", 1, "Recursion depth (1-3 recommended)")
	cmd.Flags().StringVar(&flagDirection, "direction", "both", "Direction: citations, references, both")

	cmd.AddCommand(newCiteGraphStatusCmd(flags))
	return cmd
}

type citationRef struct {
	id     string
	source string
}

func parseCitationResults(data json.RawMessage) []citationRef {
	// Try Europe PMC citation response envelope
	var envelope struct {
		CitationList struct {
			Citation []struct {
				ID     string `json:"id"`
				Source string `json:"source"`
			} `json:"citation"`
		} `json:"citationList"`
	}
	if json.Unmarshal(data, &envelope) == nil && len(envelope.CitationList.Citation) > 0 {
		refs := make([]citationRef, 0, len(envelope.CitationList.Citation))
		for _, c := range envelope.CitationList.Citation {
			if c.ID != "" {
				src := c.Source
				if src == "" {
					src = "MED"
				}
				refs = append(refs, citationRef{id: c.ID, source: src})
			}
		}
		return refs
	}

	// Try reference response envelope
	var refEnvelope struct {
		ReferenceList struct {
			Reference []struct {
				ID     string `json:"id"`
				Source string `json:"source"`
			} `json:"reference"`
		} `json:"referenceList"`
	}
	if json.Unmarshal(data, &refEnvelope) == nil && len(refEnvelope.ReferenceList.Reference) > 0 {
		refs := make([]citationRef, 0, len(refEnvelope.ReferenceList.Reference))
		for _, r := range refEnvelope.ReferenceList.Reference {
			if r.ID != "" {
				src := r.Source
				if src == "" {
					src = "MED"
				}
				refs = append(refs, citationRef{id: r.ID, source: src})
			}
		}
		return refs
	}

	// Fallback: try plain array
	var items []struct {
		ID     string `json:"id"`
		Source string `json:"source"`
	}
	if json.Unmarshal(data, &items) == nil {
		refs := make([]citationRef, 0, len(items))
		for _, item := range items {
			if item.ID != "" {
				src := item.Source
				if src == "" {
					src = "MED"
				}
				refs = append(refs, citationRef{id: item.ID, source: src})
			}
		}
		return refs
	}

	return nil
}

func newCiteGraphStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show citation graph node and edge counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.Open(defaultDBPath("europe-pmc-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureCiteGraphTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			var nodeCount, edgeCount int
			db.DB().QueryRow(`SELECT COUNT(*) FROM cite_graph_nodes`).Scan(&nodeCount)
			db.DB().QueryRow(`SELECT COUNT(*) FROM cite_graph_edges`).Scan(&edgeCount)

			var citedByCount, refsCount int
			db.DB().QueryRow(`SELECT COUNT(*) FROM cite_graph_edges WHERE edge_type = 'cited_by'`).Scan(&citedByCount)
			db.DB().QueryRow(`SELECT COUNT(*) FROM cite_graph_edges WHERE edge_type = 'references'`).Scan(&refsCount)

			result := map[string]any{
				"nodes":            nodeCount,
				"edges":            edgeCount,
				"cited_by_edges":   citedByCount,
				"references_edges": refsCount,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
}
