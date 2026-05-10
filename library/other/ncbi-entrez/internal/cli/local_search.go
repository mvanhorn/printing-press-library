package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// nearSlashRe matches the common NEAR/N proximity syntax (e.g., "chimeric NEAR/3 receptor")
// and rewrites it to the FTS5-compatible NEAR(term1 term2, N) form.
var nearSlashRe = regexp.MustCompile(`(?i)(\S+)\s+NEAR/(\d+)\s+(\S+)`)

// translateNearSyntax rewrites user-friendly NEAR/N syntax to FTS5 NEAR(a b, N).
func translateNearSyntax(query string) string {
	return nearSlashRe.ReplaceAllString(query, "NEAR($1 $3, $2)")
}

// localSearchResult represents a single FTS5 search hit.
type localSearchResult struct {
	ID           string  `json:"id"`
	ResourceType string  `json:"resource_type"`
	Snippet      string  `json:"snippet,omitempty"`
	Rank         float64 `json:"rank,omitempty"`
}

func newLocalSearchCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagRank string
	var flagResourceType string

	cmd := &cobra.Command{
		Use:         "local-search <query>",
		Short:       "Full-text search over cached abstracts with FTS5 proximity operators, regex, and BM25 ranking",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `FTS5 Abstract Corpus Search -- searches the local FTS5 index over
cached abstracts and records synced via 'sync'. Supports the full FTS5
query syntax including NEAR, AND, OR, NOT, and prefix* operators.

Proximity: use "term1 NEAR/N term2" (automatically translated to FTS5
NEAR(term1 term2, N) syntax) or native FTS5 "NEAR(term1 term2, N)".

Data must be synced first via 'ncbi-entrez-pp-cli sync'.`,
		Example: `  ncbi-entrez-pp-cli local-search "chimeric NEAR/3 receptor"
  ncbi-entrez-pp-cli local-search "NEAR(adverse cardiac, 5)"
  ncbi-entrez-pp-cli local-search "GLP-1 AND safety NOT diabetes" --rank bm25
  ncbi-entrez-pp-cli local-search "immuno*" --limit 50
  ncbi-entrez-pp-cli local-search "CRISPR OR cas9" --type esearch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			query := strings.Join(args, " ")

			// Translate user-friendly NEAR/N to FTS5-compatible NEAR(a b, N)
			query = translateNearSyntax(query)

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			var results []localSearchResult

			if flagRank == "bm25" {
				results, err = ftsSearchBM25(db, query, flagResourceType, flagLimit)
			} else {
				results, err = ftsSearchDefault(db, query, flagResourceType, flagLimit)
			}
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if results == nil {
				results = []localSearchResult{}
			}

			output := map[string]any{
				"query":   query,
				"count":   len(results),
				"results": results,
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum results to return")
	cmd.Flags().StringVar(&flagRank, "rank", "bm25", "Ranking method (bm25 or default)")
	cmd.Flags().StringVar(&flagResourceType, "type", "", "Filter by resource type (e.g., esearch, efetch)")

	return cmd
}

// ftsSearchBM25 runs a FTS5 search with BM25 ranking.
func ftsSearchBM25(db *store.Store, query, resourceType string, limit int) ([]localSearchResult, error) {
	if limit <= 0 {
		limit = 50
	}

	var sqlQuery string
	var args []any

	if resourceType != "" {
		sqlQuery = `SELECT f.id, f.resource_type,
			snippet(resources_fts, 2, '>>>', '<<<', '...', 40) as snip,
			bm25(resources_fts) as rnk
			FROM resources_fts f
			WHERE resources_fts MATCH ?
			AND f.resource_type = ?
			ORDER BY rnk
			LIMIT ?`
		args = []any{query, resourceType, limit}
	} else {
		sqlQuery = `SELECT f.id, f.resource_type,
			snippet(resources_fts, 2, '>>>', '<<<', '...', 40) as snip,
			bm25(resources_fts) as rnk
			FROM resources_fts f
			WHERE resources_fts MATCH ?
			ORDER BY rnk
			LIMIT ?`
		args = []any{query, limit}
	}

	rows, err := db.DB().Query(sqlQuery, args...)
	if err != nil {
		// If FTS5 query syntax error, provide a helpful message
		if strings.Contains(err.Error(), "fts5") || strings.Contains(err.Error(), "syntax") {
			return nil, fmt.Errorf("FTS5 query error: %w\nHint: use AND, OR, NOT operators; prefix* for prefix search; \"exact phrase\" in quotes; NEAR(a b, N) for proximity", err)
		}
		return nil, err
	}
	defer rows.Close()

	return scanFTSResults(rows)
}

// ftsSearchDefault runs a FTS5 search with default ranking.
func ftsSearchDefault(db *store.Store, query, resourceType string, limit int) ([]localSearchResult, error) {
	if limit <= 0 {
		limit = 50
	}

	var sqlQuery string
	var args []any

	if resourceType != "" {
		sqlQuery = `SELECT f.id, f.resource_type,
			snippet(resources_fts, 2, '>>>', '<<<', '...', 40) as snip,
			rank as rnk
			FROM resources_fts f
			WHERE resources_fts MATCH ?
			AND f.resource_type = ?
			ORDER BY rank
			LIMIT ?`
		args = []any{query, resourceType, limit}
	} else {
		sqlQuery = `SELECT f.id, f.resource_type,
			snippet(resources_fts, 2, '>>>', '<<<', '...', 40) as snip,
			rank as rnk
			FROM resources_fts f
			WHERE resources_fts MATCH ?
			ORDER BY rank
			LIMIT ?`
		args = []any{query, limit}
	}

	rows, err := db.DB().Query(sqlQuery, args...)
	if err != nil {
		if strings.Contains(err.Error(), "fts5") || strings.Contains(err.Error(), "syntax") {
			return nil, fmt.Errorf("FTS5 query error: %w\nHint: use AND, OR, NOT operators; prefix* for prefix search; \"exact phrase\" in quotes; NEAR(a b, N) for proximity", err)
		}
		return nil, err
	}
	defer rows.Close()

	return scanFTSResults(rows)
}

func scanFTSResults(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]localSearchResult, error) {
	var results []localSearchResult
	for rows.Next() {
		var r localSearchResult
		var snippet string
		var rank float64
		if err := rows.Scan(&r.ID, &r.ResourceType, &snippet, &rank); err != nil {
			return nil, err
		}
		r.Snippet = snippet
		r.Rank = rank
		results = append(results, r)
	}
	return results, rows.Err()
}

// Compile guard for json import.
var _ = json.Marshal
