// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

type collectionSaveResult struct {
	Collection        string                   `json:"collection"`
	Added             int                      `json:"added"`
	SkippedDuplicates int                      `json:"skipped_duplicates"`
	Items             []collectionItemSnapshot `json:"items,omitempty"`
}

type collectionItemSnapshot struct {
	TweetID  string              `json:"tweet_id"`
	URL      string              `json:"url"`
	Author   *postAuthorSummary  `json:"author,omitempty"`
	Text     string              `json:"text,omitempty"`
	Note     string              `json:"note,omitempty"`
	Tags     []string            `json:"tags,omitempty"`
	SavedAt  string              `json:"saved_at,omitempty"`
	Source   string              `json:"source,omitempty"`
	Snapshot *resolvedPostRecord `json:"snapshot,omitempty"`
}

func newNovelCollectionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collection",
		Short: "Save, list, and export durable local collections of X posts",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelCollectionSaveCmd(flags))
	cmd.AddCommand(newNovelCollectionListCmd(flags))
	cmd.AddCommand(newNovelCollectionExportCmd(flags))
	return cmd
}

func newNovelCollectionSaveCmd(flags *rootFlags) *cobra.Command {
	var dbPath, collection, note, fromSearch string
	var tags []string
	var limit int
	var live bool

	cmd := &cobra.Command{
		Use:   "save <url-or-id>",
		Short: "Save one or more X posts into a named local collection",
		Long:  "Save resolves X post URLs or IDs into canonical snapshots and stores them locally. It never writes to X. Use --collection name with a single URL/ID, or pass collection name first: collection save research <url-or-id>.",
		Example: `  x-twitter-pp-cli collection save https://x.com/user/status/123 --collection ai-agents --note "Good framing" --agent
  x-twitter-pp-cli collection save ai-agents 123 --tag research --agent
  x-twitter-pp-cli collection save --collection ai-agents --from-search "agentic coding" --limit 25 --agent`,
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			name, inputs, err := collectionSaveInputs(collection, args, fromSearch)
			if err != nil {
				return err
			}
			mode := flags.dataSource
			if live {
				mode = "live"
			}
			db, err := openCollectionDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			var records []*resolvedPostRecord
			if fromSearch != "" {
				records, err = resolvePostsFromRecentSearch(cmd, flags, fromSearch, limit)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			} else {
				include := parseIncludeSet("author,media,links,refs,metrics")
				for _, input := range inputs {
					rec, err := resolvePost(cmd, flags, input, dbPath, mode, include)
					if err != nil {
						return err
					}
					records = append(records, rec)
				}
			}

			result := collectionSaveResult{Collection: name}
			if len(records) > 0 {
				added, duplicates, items, err := saveCollectionItems(cmd, db, name, records, note, tags)
				if err != nil {
					return err
				}
				result.Added = added
				result.SkippedDuplicates = duplicates
				result.Items = items
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printTable(cmd, []string{"COLLECTION", "ADDED", "DUPLICATES"}, [][]string{{
					result.Collection,
					fmt.Sprintf("%d", result.Added),
					fmt.Sprintf("%d", result.SkippedDuplicates),
				}})
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database (defaults to the standard cache location)")
	cmd.Flags().StringVar(&collection, "collection", "", "Collection name")
	cmd.Flags().StringVar(&note, "note", "", "Optional note stored with each saved post")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag to store with each saved post; repeatable")
	cmd.Flags().StringVar(&fromSearch, "from-search", "", "Save recent-search results for this query")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum posts to save from --from-search")
	cmd.Flags().BoolVar(&live, "live", false, "Bypass local lookup when saving explicit URLs/IDs")
	return cmd
}

func newNovelCollectionListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var includeSnapshot bool

	cmd := &cobra.Command{
		Use:   "list [collection]",
		Short: "List collections or the posts saved in one collection",
		Example: `  x-twitter-pp-cli collection list --agent
  x-twitter-pp-cli collection list ai-agents --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openCollectionDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if len(args) == 0 {
				collections, err := listCollections(cmd, db)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"collections": collections}, flags)
			}
			items, err := listCollectionItems(cmd, db, args[0], limit, includeSnapshot)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				rows := make([][]string, 0, len(items))
				for _, item := range items {
					rows = append(rows, []string{item.TweetID, item.URL, truncatePlain(item.Text, 80), item.Note})
				}
				return flags.printTable(cmd, []string{"ID", "URL", "TEXT", "NOTE"}, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"collection": args[0], "items": items}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database (defaults to the standard cache location)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum saved posts to list")
	cmd.Flags().BoolVar(&includeSnapshot, "include-snapshot", false, "Include the stored resolved post snapshot")
	return cmd
}

func newNovelCollectionExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath, format, output string

	cmd := &cobra.Command{
		Use:   "export <collection>",
		Short: "Export a saved post collection as markdown, JSON, JSONL, or CSV",
		Example: `  x-twitter-pp-cli collection export ai-agents --format markdown
  x-twitter-pp-cli collection export ai-agents --format jsonl > ai-agents.jsonl`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openCollectionDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			items, err := listCollectionItems(cmd, db, args[0], 0, false)
			if err != nil {
				return err
			}
			var w io.Writer = cmd.OutOrStdout()
			var f *os.File
			if output != "" {
				f, err = os.Create(output)
				if err != nil {
					return fmt.Errorf("creating export file: %w", err)
				}
				defer f.Close()
				w = f
			}
			if err := writeCollectionExport(w, args[0], items, format); err != nil {
				return err
			}
			if output != "" && (flags.asJSON || flags.agent) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"collection": args[0], "items": len(items), "export_path": output}, flags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database (defaults to the standard cache location)")
	cmd.Flags().StringVar(&format, "format", "markdown", "Export format: markdown, json, jsonl, csv")
	cmd.Flags().StringVar(&output, "output", "", "Write export to a file instead of stdout")
	return cmd
}

func collectionSaveInputs(collection string, args []string, fromSearch string) (string, []string, error) {
	name := strings.TrimSpace(collection)
	inputs := append([]string(nil), args...)
	if name == "" && len(inputs) >= 2 {
		name = inputs[0]
		inputs = inputs[1:]
	}
	if name == "" {
		return "", nil, usageErr(fmt.Errorf("collection name required; pass --collection <name> or use 'collection save <name> <url-or-id>'"))
	}
	if fromSearch == "" && len(inputs) == 0 {
		return "", nil, usageErr(fmt.Errorf("post URL/ID required unless --from-search is set"))
	}
	if fromSearch != "" && len(inputs) > 0 {
		return "", nil, usageErr(fmt.Errorf("--from-search cannot be combined with explicit post URLs/IDs"))
	}
	return name, inputs, nil
}

func openCollectionDB(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("x-twitter-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	return db, nil
}

func saveCollectionItems(cmd *cobra.Command, db *store.Store, collection string, records []*resolvedPostRecord, note string, tags []string) (int, int, []collectionItemSnapshot, error) {
	now := generatedAt()
	tagsRaw, err := json.Marshal(tags)
	if err != nil {
		return 0, 0, nil, err
	}
	tx, err := db.DB().BeginTx(cmd.Context(), nil)
	if err != nil {
		return 0, 0, nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(cmd.Context(),
		`INSERT INTO post_collections(name, created_at, updated_at)
		 VALUES(?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET updated_at = excluded.updated_at`,
		collection, now, now); err != nil {
		return 0, 0, nil, err
	}
	stmt, err := tx.PrepareContext(cmd.Context(),
		`INSERT INTO post_collection_items(collection_name, tweet_id, tweet_json, note, tags_json, source_url, saved_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(collection_name, tweet_id) DO NOTHING`)
	if err != nil {
		return 0, 0, nil, err
	}
	defer stmt.Close()

	added := 0
	duplicates := 0
	items := make([]collectionItemSnapshot, 0, len(records))
	for _, rec := range records {
		if rec == nil {
			continue
		}
		raw, err := json.Marshal(rec)
		if err != nil {
			return 0, 0, nil, err
		}
		result, err := stmt.ExecContext(cmd.Context(), collection, rec.TweetID, string(raw), note, string(tagsRaw), rec.URL, now)
		if err != nil {
			return 0, 0, nil, err
		}
		rows, _ := result.RowsAffected()
		if rows > 0 {
			added++
		} else {
			duplicates++
		}
		items = append(items, collectionItemSnapshot{
			TweetID:  rec.TweetID,
			URL:      rec.URL,
			Author:   rec.Author,
			Text:     rec.Text,
			Note:     note,
			Tags:     tags,
			SavedAt:  now,
			Source:   rec.Source,
			Snapshot: rec,
		})
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, nil, err
	}
	return added, duplicates, items, nil
}

func listCollections(cmd *cobra.Command, db *store.Store) ([]map[string]any, error) {
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT c.name, COUNT(i.tweet_id), c.updated_at
		 FROM post_collections c
		 LEFT JOIN post_collection_items i ON i.collection_name = c.name
		 GROUP BY c.name, c.updated_at
		 ORDER BY c.updated_at DESC, c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var name, updated string
		var count int
		if err := rows.Scan(&name, &count, &updated); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": name, "count": count, "updated_at": updated})
	}
	return out, rows.Err()
}

func listCollectionItems(cmd *cobra.Command, db *store.Store, collection string, limit int, includeSnapshot bool) ([]collectionItemSnapshot, error) {
	q := `SELECT tweet_id, tweet_json, note, tags_json, source_url, saved_at
		FROM post_collection_items
		WHERE collection_name = ?
		ORDER BY saved_at DESC, tweet_id DESC`
	args := []any{collection}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.DB().QueryContext(cmd.Context(), q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []collectionItemSnapshot
	for rows.Next() {
		var tweetID, raw, urlValue, savedAt string
		var note, tagsRaw sql.NullString
		if err := rows.Scan(&tweetID, &raw, &note, &tagsRaw, &urlValue, &savedAt); err != nil {
			return nil, err
		}
		var rec resolvedPostRecord
		_ = json.Unmarshal([]byte(raw), &rec)
		var tags []string
		if tagsRaw.Valid {
			_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
		}
		item := collectionItemSnapshot{
			TweetID: tweetID,
			URL:     urlValue,
			Author:  rec.Author,
			Text:    rec.Text,
			Tags:    tags,
			SavedAt: savedAt,
			Source:  rec.Source,
		}
		if note.Valid {
			item.Note = note.String
		}
		if includeSnapshot {
			item.Snapshot = &rec
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		exists, err := collectionExists(cmd, db, collection)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, notFoundErr(fmt.Errorf("collection %q not found", collection))
		}
	}
	return items, nil
}

func collectionExists(cmd *cobra.Command, db *store.Store, collection string) (bool, error) {
	var one int
	err := db.DB().QueryRowContext(cmd.Context(), `SELECT 1 FROM post_collections WHERE name = ?`, collection).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func resolvePostsFromRecentSearch(cmd *cobra.Command, flags *rootFlags, query string, limit int) ([]*resolvedPostRecord, error) {
	if limit <= 0 {
		limit = 25
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	maxResults := limit
	if maxResults < 10 {
		maxResults = 10
	}
	if maxResults > 100 {
		maxResults = 100
	}
	data, err := c.Get(cmd.Context(), "/2/tweets/search/recent", map[string]string{
		"query":        query,
		"max_results":  fmt.Sprintf("%d", maxResults),
		"tweet.fields": "author_id,conversation_id,created_at,entities,public_metrics,referenced_tweets",
		"expansions":   "author_id,attachments.media_keys",
		"user.fields":  "id,name,username,verified,public_metrics",
		"media.fields": "media_key,type,url,preview_image_url,width,height,alt_text",
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data     []json.RawMessage `json:"data"`
		Includes struct {
			Users []map[string]any `json:"users"`
		} `json:"includes"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decoding search results: %w", err)
	}
	userByID := map[string]*postAuthorSummary{}
	for _, u := range envelope.Includes.Users {
		user := userSummaryFromMap(u)
		if user.ID != "" {
			userByID[user.ID] = user
		}
	}
	include := parseIncludeSet("author,media,links,refs,metrics")
	records := make([]*resolvedPostRecord, 0, minInt(limit, len(envelope.Data)))
	for _, raw := range envelope.Data {
		if len(records) >= limit {
			break
		}
		rec, err := normalizeSearchTweetRecord(raw, userByID, include)
		if err == nil {
			records = append(records, rec)
		}
	}
	return records, nil
}

func normalizeSearchTweetRecord(raw json.RawMessage, userByID map[string]*postAuthorSummary, include map[string]bool) (*resolvedPostRecord, error) {
	return normalizeTweetRecordWithOwnID(raw, userByID, "live", "not_synced", include)
}

func writeCollectionExport(w io.Writer, collection string, items []collectionItemSnapshot, format string) error {
	switch strings.ToLower(format) {
	case "markdown", "md":
		if err := workflowFprintf(w, "# %s\n\n", collection); err != nil {
			return err
		}
		for _, item := range items {
			if err := workflowFprintf(w, "## %s\n\n", item.URL); err != nil {
				return err
			}
			if item.Author != nil {
				if err := workflowFprintf(w, "- Author: %s\n", authorDisplay(item.Author)); err != nil {
					return err
				}
			}
			if item.SavedAt != "" {
				if err := workflowFprintf(w, "- Saved: %s\n", item.SavedAt); err != nil {
					return err
				}
			}
			if item.Note != "" {
				if err := workflowFprintf(w, "- Note: %s\n", item.Note); err != nil {
					return err
				}
			}
			if len(item.Tags) > 0 {
				if err := workflowFprintf(w, "- Tags: %s\n", strings.Join(item.Tags, ", ")); err != nil {
					return err
				}
			}
			if err := workflowFprintf(w, "\n%s\n\n", item.Text); err != nil {
				return err
			}
		}
		return nil
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"collection": collection, "generated_at": generatedAt(), "items": items})
	case "jsonl":
		enc := json.NewEncoder(w)
		for _, item := range items {
			if err := enc.Encode(item); err != nil {
				return err
			}
		}
		return nil
	case "csv":
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"tweet_id", "url", "author", "text", "note", "tags", "saved_at"}); err != nil {
			return err
		}
		for _, item := range items {
			if err := cw.Write([]string{item.TweetID, item.URL, authorDisplay(item.Author), item.Text, item.Note, strings.Join(item.Tags, "|"), item.SavedAt}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	default:
		return usageErr(fmt.Errorf("invalid --format %q: expected markdown, json, jsonl, or csv", format))
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
