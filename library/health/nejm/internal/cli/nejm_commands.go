// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-coded Phase 3: current, recent, search, articles, sql commands.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/nejm/internal/store"
	"github.com/spf13/cobra"
)

// newCurrentCmd lists articles from the most recent publication week in the
// local corpus — the closest analog to "the current issue" now that the corpus
// is sourced from OpenAlex, which carries no issue partition.
func newCurrentCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var freeOnly bool

	cmd := &cobra.Command{
		Use:         "current",
		Short:       "List the current NEJM issue's articles from the local corpus",
		Long:        "List articles from the most recent NEJM publication week in the local corpus (approximates the current issue). Run 'nejm-pp-cli sync' first to populate the corpus from OpenAlex.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli current\n  nejm-pp-cli current --free --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "article", flags.maxAge)

			where := nejmCurrentIssueWhere
			var qargs []any
			if freeOnly {
				where += ` AND is_free = 1`
			}
			items, err := nejmQueryArticles(db, where, qargs, limit)
			if err != nil {
				return fmt.Errorf("querying articles: %w", err)
			}
			return nejmPrintArticles(cmd, items, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of articles to return (0 = all)")
	cmd.Flags().BoolVar(&freeOnly, "free", false, "Show only open-access articles")
	return cmd
}

// newRecentCmd lists recently published articles from the local corpus.
func newRecentCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var freeOnly bool

	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "List recently published NEJM articles from the local corpus",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli recent\n  nejm-pp-cli recent --limit 20 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "article", flags.maxAge)

			// Newest-first over the whole corpus; the retired RSS transport's
			// 'axatoc' (recently-published) feed partition no longer exists.
			where := ``
			var qargs []any
			if freeOnly {
				where = `is_free = 1`
			}
			items, err := nejmQueryArticles(db, where, qargs, limit)
			if err != nil {
				return fmt.Errorf("querying articles: %w", err)
			}
			return nejmPrintArticles(cmd, items, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of articles to return")
	cmd.Flags().BoolVar(&freeOnly, "free", false, "Show only open-access articles")
	return cmd
}

// newSearchCmd searches the local corpus using SQLite FTS.
func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var resourceTypes []string

	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search over the local NEJM corpus",
		Long:        "Searches article titles, authors, abstracts, and specialties in the locally synced SQLite corpus. Run 'nejm-pp-cli sync' first.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli search sepsis\n  nejm-pp-cli search \"cardiac arrest\" --limit 10 --json",
		Args:        cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "article", flags.maxAge)

			if len(resourceTypes) == 0 {
				resourceTypes = []string{"article"}
			}
			results, err := db.Search(query, limit, resourceTypes...)
			if err != nil {
				return fmt.Errorf("search error: %w", err)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: no results for %q. Try 'nejm-pp-cli sync' to index more articles.\n", query)
				if flags.asJSON {
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
				}
				return nil
			}
			data, err := json.Marshal(results)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of results")
	cmd.Flags().StringSliceVar(&resourceTypes, "type", nil, "Resource type to search (default: article)")
	return cmd
}

// newArticlesCmd lists articles with optional filters.
func newArticlesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var specialty string
	var articleType string
	var author string
	var freeOnly bool
	var since string

	cmd := &cobra.Command{
		Use:         "articles",
		Short:       "List NEJM articles from the local corpus with optional filters",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli articles --specialty cardiology\n  nejm-pp-cli articles --type \"Original Article\" --free --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "article", flags.maxAge)

			var clauses []string
			var qargs []any

			if specialty != "" {
				clauses = append(clauses, `specialties LIKE ?`)
				qargs = append(qargs, "%"+specialty+"%")
			}
			if articleType != "" {
				clauses = append(clauses, `article_type LIKE ?`)
				qargs = append(qargs, "%"+articleType+"%")
			}
			if author != "" {
				clauses = append(clauses, `authors LIKE ?`)
				qargs = append(qargs, "%"+author+"%")
			}
			if freeOnly {
				clauses = append(clauses, `is_free = 1`)
			}
			if since != "" {
				cutoff, err := nejmParseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
				clauses = append(clauses, `date >= ?`)
				qargs = append(qargs, cutoff.Format("2006-01-02"))
			}

			where := strings.Join(clauses, " AND ")
			items, err := nejmQueryArticles(db, where, qargs, limit)
			if err != nil {
				return fmt.Errorf("querying articles: %w", err)
			}
			return nejmPrintArticles(cmd, items, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of articles")
	cmd.Flags().StringVar(&specialty, "specialty", "", "Filter by specialty (partial match, e.g. cardiology)")
	cmd.Flags().StringVar(&articleType, "type", "", "Filter by article type (e.g. 'Original Article')")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author name (partial match)")
	cmd.Flags().BoolVar(&freeOnly, "free", false, "Show only open-access articles")
	cmd.Flags().StringVar(&since, "since", "", "Only articles published since this window (e.g. 7d, 48h, 2w)")
	return cmd
}

// newSQLCmd runs arbitrary SQL SELECT statements against the local store.
func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a SQL SELECT against the local NEJM corpus",
		Long:  "Runs a raw SQL SELECT query against the local SQLite database. Tables: article, specialty. Use 'SELECT *' or name specific columns.",
		Example: `  nejm-pp-cli sql "SELECT doi, title FROM article LIMIT 10"
  nejm-pp-cli sql "SELECT article_type, COUNT(*) as n FROM article GROUP BY article_type ORDER BY n DESC"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			if !strings.Contains(strings.ToUpper(query), "SELECT") {
				return usageErr(fmt.Errorf("only SELECT statements are allowed"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("nejm-pp-cli")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'nejm-pp-cli sync' first", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("query error: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}

			var results []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = new(sql.RawBytes)
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				row := make(map[string]any, len(cols))
				for i, col := range cols {
					rb := ptrs[i].(*sql.RawBytes)
					row[col] = string(*rb)
				}
				results = append(results, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			data, err := json.Marshal(results)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite database (default: ~/.local/share/nejm-pp-cli/data.db)")
	return cmd
}
