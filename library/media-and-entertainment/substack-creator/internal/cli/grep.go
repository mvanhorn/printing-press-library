package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-creator/internal/store"

	"github.com/spf13/cobra"
)

type grepHit struct {
	Scope       string  `json:"scope"`
	ID          string  `json:"id"`
	Title       string  `json:"title,omitempty"`
	Publication string  `json:"publication,omitempty"`
	PublishedAt string  `json:"published_at,omitempty"`
	Snippet     string  `json:"snippet"`
	URL         string  `json:"url,omitempty"`
	Score       float64 `json:"score"`
}

func newGrepCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		scope       string
		publication string
		since       string
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "grep <query>",
		Short: "FTS5 over post bodies + Notes + comments.",
		Long: `Full-text search across locally synced posts, notes, and comments using SQLite FTS5
with bm25 ranking. Returns titles, snippets, and source URLs.

Run 'substack-creator-pp-cli sync --full' first to populate the local cache.`,
		Example: `  # Search every scope across all publications
  substack-creator-pp-cli grep "yield curve" --json

  # Only posts, only one publication, only recent
  substack-creator-pp-cli grep "rate hike" --scope posts --publication mypub-en --since 2024-01-01`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "grep"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return usageErr(fmt.Errorf("query is required"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-creator-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()

			scopes := []string{scope}
			if scope == "all" || scope == "" {
				scopes = []string{"posts", "notes", "comments"}
			}

			var hits []grepHit
			for _, sc := range scopes {
				more, err := grepScope(cmd.Context(), db, sc, query, publication, since, limit)
				if err != nil {
					// FTS table for the scope might not exist (no sync yet); surface a soft warn and continue
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: grep on %s skipped: %v\n", sc, err)
					continue
				}
				hits = append(hits, more...)
			}

			// Cap final result set
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(hits)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			w := cmd.OutOrStdout()
			if len(hits) == 0 {
				fmt.Fprintf(w, "No matches for %q.\n", query)
				return nil
			}
			fmt.Fprintf(w, "Found %d match(es) for %q\n", len(hits), query)
			fmt.Fprintln(w, strings.Repeat("─", 78))
			for _, h := range hits {
				fmt.Fprintf(w, "[%s] %s — %s\n", h.Scope, h.Publication, h.PublishedAt)
				if h.Title != "" {
					fmt.Fprintf(w, "  %s\n", h.Title)
				}
				fmt.Fprintf(w, "  %s\n", truncate(h.Snippet, 200))
				if h.URL != "" {
					fmt.Fprintf(w, "  %s\n", h.URL)
				}
				fmt.Fprintln(w)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&scope, "scope", "all", "Scope: posts | notes | comments | all")
	cmd.Flags().StringVar(&publication, "publication", "", "Filter by publication subdomain or id")
	cmd.Flags().StringVar(&since, "since", "", "Only results since YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

// ftsPhraseQuery wraps the user's input as an FTS5 phrase. Phrase form
// neutralizes every FTS5 operator (* + - ( ) ^ : NEAR AND OR NOT) so a
// query that happens to contain SQL- or FTS-significant characters does
// not become a syntax error. Embedded double quotes are doubled per FTS5
// quoting rules.
func ftsPhraseQuery(q string) string {
	return `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
}

func grepScope(ctx interface{ Done() <-chan struct{} }, db *sql.DB, scope, query, publication, since string, limit int) ([]grepHit, error) {
	// All three scopes go through resources_fts. upsertGenericResourceTx
	// indexes every synced post/note/comment under its resource_type, and
	// bm25(resources_fts) gives a real relevance score (negated so larger
	// = better in the JSON output). The typed table is joined for metadata
	// that isn't in the JSON-flattened FTS content.
	matchExpr := ftsPhraseQuery(query)
	switch scope {
	case "posts":
		q := `SELECT p.id, COALESCE(p.title, ''), COALESCE(p.publication_id, ''),
		             COALESCE(p.publish_date, ''), COALESCE(p.body_markdown, ''),
		             COALESCE(p.canonical_url, ''), -bm25(resources_fts) AS score
		      FROM resources_fts
		      JOIN posts p ON p.id = resources_fts.id
		      WHERE resources_fts MATCH ?
		        AND resources_fts.resource_type = 'posts'`
		args := []any{matchExpr}
		if publication != "" {
			q += ` AND (p.publication_id = ? OR p.publication_id IN (SELECT id FROM publications WHERE subdomain = ?))`
			args = append(args, publication, publication)
		}
		if since != "" {
			q += ` AND p.publish_date >= ?`
			args = append(args, since)
		}
		q += ` ORDER BY bm25(resources_fts) ASC LIMIT ?`
		args = append(args, limit)
		return queryHits(db, q, args, "posts", true)

	case "notes":
		q := `SELECT n.id, '', '', COALESCE(n.posted_at, ''),
		             COALESCE(n.body, ''), '', -bm25(resources_fts) AS score
		      FROM resources_fts
		      JOIN notes n ON n.id = resources_fts.id
		      WHERE resources_fts MATCH ?
		        AND resources_fts.resource_type = 'notes'`
		args := []any{matchExpr}
		if since != "" {
			q += ` AND n.posted_at >= ?`
			args = append(args, since)
		}
		q += ` ORDER BY bm25(resources_fts) ASC LIMIT ?`
		args = append(args, limit)
		return queryHits(db, q, args, "notes", true)

	case "comments":
		q := `SELECT c.id, '', '', COALESCE(c.posted_at, ''),
		             COALESCE(c.body, ''), '', -bm25(resources_fts) AS score
		      FROM resources_fts
		      JOIN comments c ON c.id = resources_fts.id
		      WHERE resources_fts MATCH ?
		        AND resources_fts.resource_type = 'comments'`
		args := []any{matchExpr}
		if since != "" {
			q += ` AND c.posted_at >= ?`
			args = append(args, since)
		}
		q += ` ORDER BY bm25(resources_fts) ASC LIMIT ?`
		args = append(args, limit)
		return queryHits(db, q, args, "comments", true)
	}
	return nil, fmt.Errorf("unknown scope %q", scope)
}

func queryHits(db *sql.DB, q string, args []any, scope string, makeSnippet bool) ([]grepHit, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []grepHit
	for rows.Next() {
		var (
			id, title, pubID, postedAt, body, url string
			score                                 float64
		)
		if err := rows.Scan(&id, &title, &pubID, &postedAt, &body, &url, &score); err != nil {
			return nil, err
		}
		snip := body
		if makeSnippet {
			snip = truncate(strings.TrimSpace(body), 220)
		}
		out = append(out, grepHit{
			Scope: scope, ID: id, Title: title, Publication: pubID,
			PublishedAt: postedAt, Snippet: snip, URL: url, Score: score,
		})
	}
	return out, rows.Err()
}
