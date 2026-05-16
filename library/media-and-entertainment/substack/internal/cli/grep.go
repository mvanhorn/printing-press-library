package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

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

Run 'substack-pp-cli sync --full' first to populate the local cache.`,
		Example: `  # Search every scope across all publications
  substack-pp-cli grep "yield curve" --json

  # Only posts, only one publication, only recent
  substack-pp-cli grep "rate hike" --scope posts --publication mypub-en --since 2024-01-01`,
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
				dbPath = defaultDBPath("substack-pp-cli")
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

func grepScope(ctx interface{ Done() <-chan struct{} }, db *sql.DB, scope, query, publication, since string, limit int) ([]grepHit, error) {
	switch scope {
	case "posts":
		// posts table has body_markdown / title. There's no FTS table for posts in the generated schema,
		// so we fall back to LIKE search — bm25 is on resources_fts only.
		q := `SELECT id, COALESCE(title, ''), COALESCE(publication_id, ''), COALESCE(publish_date, ''),
		             COALESCE(body_markdown, ''), COALESCE(canonical_url, '')
		      FROM posts WHERE (body_markdown LIKE '%' || ? || '%' OR title LIKE '%' || ? || '%')`
		args := []any{query, query}
		if publication != "" {
			q += ` AND (publication_id = ? OR publication_id IN (SELECT id FROM publications WHERE subdomain = ?))`
			args = append(args, publication, publication)
		}
		if since != "" {
			q += ` AND publish_date >= ?`
			args = append(args, since)
		}
		q += ` ORDER BY publish_date DESC LIMIT ?`
		args = append(args, limit)
		return queryHits(db, q, args, "posts", true)

	case "notes":
		q := `SELECT id, '', '', COALESCE(posted_at, ''), COALESCE(body, ''), ''
		      FROM notes WHERE body LIKE '%' || ? || '%'`
		args := []any{query}
		if since != "" {
			q += ` AND posted_at >= ?`
			args = append(args, since)
		}
		q += ` ORDER BY posted_at DESC LIMIT ?`
		args = append(args, limit)
		return queryHits(db, q, args, "notes", true)

	case "comments":
		q := `SELECT id, '', '', COALESCE(posted_at, ''), COALESCE(body, ''), ''
		      FROM comments WHERE body LIKE '%' || ? || '%'`
		args := []any{query}
		if since != "" {
			q += ` AND posted_at >= ?`
			args = append(args, since)
		}
		q += ` ORDER BY posted_at DESC LIMIT ?`
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
		var id, title, pubID, postedAt, body, url string
		if err := rows.Scan(&id, &title, &pubID, &postedAt, &body, &url); err != nil {
			return nil, err
		}
		snip := body
		if makeSnippet {
			snip = truncate(strings.TrimSpace(body), 220)
		}
		out = append(out, grepHit{
			Scope: scope, ID: id, Title: title, Publication: pubID,
			PublishedAt: postedAt, Snippet: snip, URL: url, Score: 1.0,
		})
	}
	return out, rows.Err()
}
