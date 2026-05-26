// Cross-Actor FTS over the local pp_dataset_items table.
// Registered as `search items` so the existing `search` command (generic
// resources FTS) keeps working unchanged.
//
//	apify-pp-cli search items "model context protocol" --since 7d \
//	    --actors trudax/reddit-scraper,apidojo/twitter-scraper \
//	    --json --select url,title,source_actor,published_at
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/store"
)

// newSearchItemsCmd is wired up by the search-command parent capture in root.go.
func newSearchItemsCmd(flags *rootFlags) *cobra.Command {
	var (
		sinceStr string
		actorCSV string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "items <query>",
		Short: "Full-text search across every cached dataset item, normalized across Actors",
		Long: strings.Trim(`
Search across normalized dataset items pulled into the local store by
'apify-pp-cli run' or 'apify-pp-cli sync'. Filters: --since (time window),
--actors (CSV of source actors), --limit. JSON output includes the unified
shape (url, title, body, author, published_at, source_actor) plus the raw
item for callers that need original fields.

Examples:
  apify-pp-cli search items "agent"            --since 30d --json
  apify-pp-cli search items "vision pro"       --actors apidojo/twitter-scraper --limit 50
  apify-pp-cli search items "new claude model" --since 24h --agent --select url,title,source_actor
`, "\n"),
		Example: strings.Trim(`
  apify-pp-cli search items "agent" --since 30d --json
  apify-pp-cli search items "open source" --actors trudax/reddit-scraper --limit 25 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return usageErr(fmt.Errorf("query string cannot be empty"))
			}

			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
			if err != nil {
				return configErr(fmt.Errorf("opening local store: %w", err))
			}
			defer db.Close()
			if err := db.EnsureExtensions(ctx); err != nil {
				return configErr(fmt.Errorf("ensuring extensions: %w", err))
			}

			actors := splitCSV(actorCSV)
			sinceDur := parseSinceWindow(sinceStr)
			items, err := queryDatasetItemsFTS(ctx, db, query, actors, sinceDur, limit)
			if err != nil {
				return apiErr(fmt.Errorf("querying FTS: %w", err))
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"query":  query,
				"count":  len(items),
				"items":  items,
			}, flags)
		},
	}

	cmd.Flags().StringVar(&sinceStr, "since", "", "Limit to items fetched within this window (e.g. 24h, 7d, 30d). Empty disables.")
	cmd.Flags().StringVar(&actorCSV, "actors", "", "Comma-separated source actors to restrict the search to")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum items returned")
	return cmd
}

// FoundItem is the JSON envelope returned for each search hit.
// Keeps the Item shape from internal/normalize without importing it
// (FoundItem is the projection visible to agents).
type FoundItem struct {
	Hash        string          `json:"hash"`
	URL         string          `json:"url,omitempty"`
	Title       string          `json:"title,omitempty"`
	Body        string          `json:"body,omitempty"`
	Author      string          `json:"author,omitempty"`
	PublishedAt string          `json:"published_at,omitempty"`
	SourceActor string          `json:"source_actor"`
	RunID       string          `json:"run_id,omitempty"`
	FetchedAt   string          `json:"fetched_at,omitempty"`
	Engagement  int64           `json:"engagement_score,omitempty"`
	Raw         json.RawMessage `json:"raw,omitempty"`
}

func queryDatasetItemsFTS(ctx context.Context, db *store.Store, query string,
	actors []string, since time.Duration, limit int) ([]FoundItem, error) {
	if limit <= 0 {
		limit = 100
	}
	// FTS5 escape: wrap query in double quotes to treat as phrase, escape inner quotes
	ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`

	q := `SELECT i.hash, i.url, i.title, i.body, i.author, i.published_at,
	             i.source_actor, i.run_id, i.fetched_at, i.engagement_score, i.raw_json
	      FROM pp_dataset_items i
	      JOIN pp_dataset_items_fts f ON f.rowid = i.rowid
	      WHERE pp_dataset_items_fts MATCH ?`
	args := []any{ftsQuery}

	if len(actors) > 0 {
		placeholders := strings.Repeat("?,", len(actors))
		placeholders = strings.TrimRight(placeholders, ",")
		q += ` AND i.source_actor IN (` + placeholders + `)`
		for _, a := range actors {
			args = append(args, a)
		}
	}
	if since > 0 {
		cutoff := time.Now().Add(-since).UTC().Format(time.RFC3339)
		q += ` AND i.fetched_at >= ?`
		args = append(args, cutoff)
	}
	q += fmt.Sprintf(` ORDER BY i.fetched_at DESC LIMIT %d`, limit)

	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FoundItem
	for rows.Next() {
		var it FoundItem
		var rawJSON string
		if err := rows.Scan(&it.Hash, &it.URL, &it.Title, &it.Body, &it.Author,
			&it.PublishedAt, &it.SourceActor, &it.RunID, &it.FetchedAt,
			&it.Engagement, &rawJSON); err != nil {
			return nil, err
		}
		it.Raw = json.RawMessage(rawJSON)
		out = append(out, it)
	}
	return out, rows.Err()
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
