// pp:data-source local
// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/store"

	"github.com/spf13/cobra"
)

type auditContentRow struct {
	ID            int64
	Title         string
	Link          string
	FeaturedMedia sql.NullInt64
	Excerpt       string
	CategoriesRaw string
	TagsRaw       string
}

type auditItem struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Link  string `json:"link"`
}

type auditCheck struct {
	Name     string      `json:"name"`
	Severity string      `json:"severity"`
	Count    int         `json:"count"`
	Items    []auditItem `json:"items"`
}

type auditOutput struct {
	Checks  []auditCheck `json:"checks"`
	Scanned int          `json:"scanned"`
}

// newNovelAuditCmd keeps compatibility with older generated root wiring. The
// additive hook in queue.go replaces this instance with the durable command.
func newNovelAuditCmd(flags *rootFlags) *cobra.Command {
	return newAuditCmd(flags)
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	var typesRaw string
	var limit int

	cmd := &cobra.Command{
		Use:   "audit [--type posts,pages] [--limit 100] [--json]",
		Short: "Find completeness and hygiene defects on posts and pages",
		Long:  "Use this command for completeness and hygiene defects on posts and pages. Do NOT use this command to find unused media files; use 'orphans' instead. Do NOT use it for workflow-state questions; use 'queue' instead.",
		Example: "  wordpress-pp-cli audit --type posts,pages --limit 100\n" +
			"  wordpress-pp-cli audit --type posts --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No bare-invocation help branch: `audit` takes no required input,
			// so running it bare must do the work, matching the framework's
			// own zero-arg list commands (e.g. `profile list`).
			if len(args) != 0 {
				return usageErr(fmt.Errorf("audit accepts flags only"))
			}
			if strings.EqualFold(strings.TrimSpace(flags.dataSource), "live") {
				return usageErr(fmt.Errorf("audit has no live equivalent; sync the site and use the local mirror"))
			}
			types, err := parseAuditTypes(typesRaw)
			if err != nil {
				return usageErr(err)
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store")
				return nil
			}

			dbPath := wordpressDBPath(flags)
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: wordpress-pp-cli sync --resources posts,pages,media --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			} else if statErr != nil {
				return configErr(fmt.Errorf("inspect local mirror %s: %w", dbPath, statErr))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return configErr(fmt.Errorf("open local mirror %s: %w", dbPath, err))
			}
			defer db.Close()

			contents, err := loadAuditContent(ctx, db, types)
			if err != nil {
				return err
			}
			mediaIDs, err := loadAuditMediaIDs(ctx, db)
			if err != nil {
				return err
			}
			uncategorizedIDs, err := loadUncategorizedIDs(ctx, db)
			if err != nil {
				return err
			}
			if !hintIfUnsynced(cmd, db, "posts") {
				hintIfStale(cmd, db, "posts", flags.maxAge)
			}

			checks := []auditCheck{
				{Name: "no_featured_image", Severity: "defect", Items: make([]auditItem, 0)},
				{Name: "no_real_category", Severity: "defect", Items: make([]auditItem, 0)},
				{Name: "empty_excerpt", Severity: "defect", Items: make([]auditItem, 0)},
				{Name: "no_tags", Severity: "informational", Items: make([]auditItem, 0)},
			}
			scanned := 0
			for _, content := range contents {
				scanned++
				item := auditItem{ID: content.ID, Title: cliutil.CleanText(content.Title), Link: content.Link}

				if !content.FeaturedMedia.Valid || content.FeaturedMedia.Int64 <= 0 {
					addAuditFinding(&checks[0], item, limit)
				} else if _, ok := mediaIDs[content.FeaturedMedia.Int64]; !ok {
					addAuditFinding(&checks[0], item, limit)
				}

				categories, err := parseJSONInt64Array(content.CategoriesRaw)
				if err != nil {
					return apiErr(fmt.Errorf("parse categories for content %d: %w", content.ID, err))
				}
				if hasNoRealCategory(categories, uncategorizedIDs) {
					addAuditFinding(&checks[1], item, limit)
				}

				if strings.TrimSpace(cliutil.CleanText(content.Excerpt)) == "" {
					addAuditFinding(&checks[2], item, limit)
				}

				tags, err := parseJSONInt64Array(content.TagsRaw)
				if err != nil {
					return apiErr(fmt.Errorf("parse tags for content %d: %w", content.ID, err))
				}
				if len(tags) == 0 {
					addAuditFinding(&checks[3], item, limit)
				}
			}

			output := auditOutput{Checks: checks, Scanned: scanned}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), output, flags)
			}
			return printAuditTable(cmd, output)
		},
	}
	cmd.Flags().StringVar(&typesRaw, "type", "posts,pages", "Comma-separated content types: posts, pages")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum offending items shown for each check")
	return cmd
}

func parseAuditTypes(raw string) ([]string, error) {
	allowed := map[string]struct{}{"posts": {}, "pages": {}}
	seen := make(map[string]struct{})
	types := make([]string, 0, 2)
	for _, part := range strings.Split(raw, ",") {
		resourceType := strings.ToLower(strings.TrimSpace(part))
		if resourceType == "" {
			continue
		}
		if _, ok := allowed[resourceType]; !ok {
			return nil, fmt.Errorf("unsupported --type %q; use posts or pages", resourceType)
		}
		if _, duplicate := seen[resourceType]; duplicate {
			continue
		}
		seen[resourceType] = struct{}{}
		types = append(types, resourceType)
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("--type must include posts, pages, or both")
	}
	return types, nil
}

func loadAuditContent(ctx context.Context, db *store.Store, resourceTypes []string) ([]auditContentRow, error) {
	if len(resourceTypes) == 0 {
		return make([]auditContentRow, 0), nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(resourceTypes)), ",")
	query := `
		SELECT
			COALESCE(CAST(json_extract(data, '$.id') AS INTEGER), CAST(id AS INTEGER)),
			COALESCE(json_extract(data, '$.title.rendered'), ''),
			COALESCE(json_extract(data, '$.link'), ''),
			CAST(json_extract(data, '$.featured_media') AS INTEGER),
			COALESCE(json_extract(data, '$.excerpt.rendered'), ''),
			COALESCE(json_extract(data, '$.categories'), '[]'),
			COALESCE(json_extract(data, '$.tags'), '[]')
		FROM resources
		WHERE resource_type IN (` + placeholders + `)`
	args := make([]any, 0, len(resourceTypes))
	for _, resourceType := range resourceTypes {
		args = append(args, resourceType)
	}
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apiErr(fmt.Errorf("query content audit rows: %w", err))
	}
	contents := make([]auditContentRow, 0)
	for rows.Next() {
		var id sql.NullInt64
		var content auditContentRow
		if err := rows.Scan(&id, &content.Title, &content.Link, &content.FeaturedMedia, &content.Excerpt, &content.CategoriesRaw, &content.TagsRaw); err != nil {
			_ = rows.Close()
			return nil, apiErr(fmt.Errorf("scan content audit row: %w", err))
		}
		if !id.Valid {
			continue
		}
		content.ID = id.Int64
		contents = append(contents, content)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apiErr(fmt.Errorf("read content audit rows: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, apiErr(fmt.Errorf("close content audit rows: %w", err))
	}
	return contents, nil
}

func loadAuditMediaIDs(ctx context.Context, db *store.Store) (map[int64]struct{}, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT COALESCE(CAST(json_extract(data, '$.id') AS INTEGER), CAST(id AS INTEGER))
		FROM resources
		WHERE resource_type IN ('media', 'posts_media', 'pages_media')`)
	if err != nil {
		return nil, apiErr(fmt.Errorf("query local media ids: %w", err))
	}
	ids := make(map[int64]struct{})
	for rows.Next() {
		var id sql.NullInt64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, apiErr(fmt.Errorf("scan local media id: %w", err))
		}
		if id.Valid {
			ids[id.Int64] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apiErr(fmt.Errorf("read local media ids: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, apiErr(fmt.Errorf("close local media ids: %w", err))
	}
	return ids, nil
}

func loadUncategorizedIDs(ctx context.Context, db *store.Store) (map[int64]struct{}, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT
			COALESCE(CAST(json_extract(data, '$.id') AS INTEGER), CAST(id AS INTEGER)),
			COALESCE(json_extract(data, '$.slug'), '')
		FROM resources
		WHERE resource_type IN ('categories', 'posts_categories', 'pages_categories')`)
	if err != nil {
		return nil, apiErr(fmt.Errorf("query local categories: %w", err))
	}
	ids := make(map[int64]struct{})
	for rows.Next() {
		var id sql.NullInt64
		var slug string
		if err := rows.Scan(&id, &slug); err != nil {
			_ = rows.Close()
			return nil, apiErr(fmt.Errorf("scan local category: %w", err))
		}
		if id.Valid && strings.EqualFold(strings.TrimSpace(slug), "uncategorized") {
			ids[id.Int64] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apiErr(fmt.Errorf("read local categories: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, apiErr(fmt.Errorf("close local categories: %w", err))
	}
	return ids, nil
}

func parseJSONInt64Array(raw string) ([]int64, error) {
	values := make([]int64, 0)
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return values, nil
	}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func hasNoRealCategory(categories []int64, uncategorized map[int64]struct{}) bool {
	if len(categories) == 0 {
		return true
	}
	if len(categories) != 1 {
		return false
	}
	_, onlyUncategorized := uncategorized[categories[0]]
	return onlyUncategorized
}

func addAuditFinding(check *auditCheck, item auditItem, limit int) {
	check.Count++
	if len(check.Items) < limit {
		check.Items = append(check.Items, item)
	}
}

func printAuditTable(cmd *cobra.Command, output auditOutput) error {
	summary := make([]map[string]any, 0, len(output.Checks))
	items := make([]map[string]any, 0)
	for _, check := range output.Checks {
		summary = append(summary, map[string]any{
			"check":    check.Name,
			"severity": check.Severity,
			"count":    check.Count,
			"scanned":  output.Scanned,
		})
		for _, item := range check.Items {
			items = append(items, map[string]any{
				"check": check.Name,
				"id":    item.ID,
				"title": item.Title,
				"link":  item.Link,
			})
		}
	}
	if err := printAutoTable(cmd.OutOrStdout(), summary); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return printAutoTable(cmd.OutOrStdout(), items)
}
