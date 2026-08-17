// pp:data-source local
// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/store"

	"github.com/spf13/cobra"
)

type queueRow struct {
	ID       int64
	Title    string
	Status   string
	Modified string
	Date     string
	Link     string
	AuthorID sql.NullInt64
}

type queueItem struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DaysInState    int    `json:"days_in_state"`
	Link           string `json:"link"`
	Author         string `json:"author"`
	MissedSchedule bool   `json:"missed_schedule,omitempty"`
	stateAt        time.Time
}

type queueBucket struct {
	Status string      `json:"status"`
	Count  int         `json:"count"`
	Items  []queueItem `json:"items"`
}

type queueOutput struct {
	Buckets []queueBucket `json:"buckets"`
	Scanned int           `json:"scanned"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		commands := []*cobra.Command{
			newQueueCmd(flags),
			newAuditCmd(flags),
			newOrphansCmd(flags),
			newFleetCmd(flags),
		}
		for _, command := range commands {
			for _, existing := range root.Commands() {
				if existing.Name() == command.Name() {
					root.RemoveCommand(existing)
				}
			}
			root.AddCommand(command)
		}
	})
}

// newNovelQueueCmd keeps compatibility with older generated root wiring. The
// additive hook above replaces this instance with the durable local command.
func newNovelQueueCmd(flags *rootFlags) *cobra.Command {
	return newQueueCmd(flags)
}

func newQueueCmd(flags *rootFlags) *cobra.Command {
	var statusesRaw string
	var olderThanRaw string
	var limit int

	cmd := &cobra.Command{
		Use:   "queue [--status draft,pending,future] [--older-than 14d] [--limit 50] [--json]",
		Short: "Show the editorial pipeline with age in state",
		Long:  "Use this command for pipeline state and how long content has been sitting in it. Do NOT use this command to find missing featured images, categories, or excerpts on content; use 'audit' instead.",
		Example: "  wordpress-pp-cli queue --older-than 14d --json\n" +
			"  wordpress-pp-cli queue --status draft,pending --limit 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No bare-invocation help branch: `queue` takes no required input,
			// so running it bare must do the work, matching the framework's
			// own zero-arg list commands (e.g. `profile list`).
			if len(args) != 0 {
				return usageErr(fmt.Errorf("queue accepts flags only"))
			}
			if strings.EqualFold(strings.TrimSpace(flags.dataSource), "live") {
				return usageErr(fmt.Errorf("queue has no live equivalent; sync the site and use the local mirror"))
			}
			statuses, err := parseQueueStatuses(statusesRaw)
			if err != nil {
				return usageErr(err)
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			var olderThan time.Duration
			if strings.TrimSpace(olderThanRaw) != "" {
				olderThan, err = cliutil.ParseDurationLoose(olderThanRaw)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --older-than %q: %w", olderThanRaw, err))
				}
				if olderThan <= 0 {
					return usageErr(fmt.Errorf("--older-than must be greater than zero"))
				}
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

			rawRows, err := loadQueueRows(ctx, db, statuses)
			if err != nil {
				return err
			}
			authors, err := loadQueueAuthors(ctx, db)
			if err != nil {
				return err
			}
			if !hintIfUnsynced(cmd, db, "posts") {
				hintIfStale(cmd, db, "posts", flags.maxAge)
			}

			now := time.Now()
			wanted := make(map[string]struct{}, len(statuses))
			for _, status := range statuses {
				wanted[status] = struct{}{}
			}
			bucketItems := make(map[string][]queueItem, len(statuses))
			scanned := 0
			for _, row := range rawRows {
				if _, ok := wanted[row.Status]; !ok {
					continue
				}
				scanned++
				stamp := row.Modified
				if row.Status == "future" {
					stamp = row.Date
				}
				stateAt, ok := parseWordPressTime(stamp, now.Location())
				if olderThan > 0 && (!ok || now.Sub(stateAt) < olderThan) {
					continue
				}
				item := queueItem{
					ID:      row.ID,
					Title:   cliutil.CleanText(row.Title),
					Status:  row.Status,
					Link:    row.Link,
					stateAt: stateAt,
				}
				if ok {
					item.DaysInState = daysInState(now, stateAt)
					item.MissedSchedule = row.Status == "future" && stateAt.Before(now)
				}
				if row.AuthorID.Valid {
					item.Author = authors[row.AuthorID.Int64]
					if item.Author == "" {
						item.Author = strconv.FormatInt(row.AuthorID.Int64, 10)
					}
				}
				bucketItems[row.Status] = append(bucketItems[row.Status], item)
			}

			buckets := make([]queueBucket, 0, len(statuses))
			for _, status := range statuses {
				items := bucketItems[status]
				sort.SliceStable(items, func(i, j int) bool {
					if items[i].stateAt.Equal(items[j].stateAt) {
						return items[i].ID < items[j].ID
					}
					return items[i].stateAt.Before(items[j].stateAt)
				})
				count := len(items)
				if len(items) > limit {
					items = items[:limit]
				}
				if items == nil {
					items = make([]queueItem, 0)
				}
				buckets = append(buckets, queueBucket{Status: status, Count: count, Items: items})
			}

			output := queueOutput{Buckets: buckets, Scanned: scanned}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), output, flags)
			}
			return printQueueTable(cmd, buckets)
		},
	}
	cmd.Flags().StringVar(&statusesRaw, "status", "draft,pending,future", "Comma-separated queue statuses: draft, pending, future")
	cmd.Flags().StringVar(&olderThanRaw, "older-than", "", "Only include items older than a loose duration such as 14d, 3w, or 24h")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum items shown in each status bucket")
	return cmd
}

func parseQueueStatuses(raw string) ([]string, error) {
	allowed := map[string]struct{}{"draft": {}, "pending": {}, "future": {}}
	seen := make(map[string]struct{})
	statuses := make([]string, 0, 3)
	for _, part := range strings.Split(raw, ",") {
		status := strings.ToLower(strings.TrimSpace(part))
		if status == "" {
			continue
		}
		if _, ok := allowed[status]; !ok {
			return nil, fmt.Errorf("unsupported --status %q; use draft, pending, or future", status)
		}
		if _, duplicate := seen[status]; duplicate {
			continue
		}
		seen[status] = struct{}{}
		statuses = append(statuses, status)
	}
	if len(statuses) == 0 {
		return nil, fmt.Errorf("--status must include at least one of draft, pending, or future")
	}
	return statuses, nil
}

func loadQueueRows(ctx context.Context, db *store.Store, statuses []string) ([]queueRow, error) {
	if len(statuses) == 0 {
		return make([]queueRow, 0), nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(statuses)), ",")
	query := `
		SELECT
			COALESCE(CAST(json_extract(data, '$.id') AS INTEGER), CAST(id AS INTEGER)),
			COALESCE(json_extract(data, '$.title.rendered'), ''),
			COALESCE(json_extract(data, '$.status'), ''),
			COALESCE(json_extract(data, '$.modified'), ''),
			COALESCE(json_extract(data, '$.date'), ''),
			COALESCE(json_extract(data, '$.link'), ''),
			CAST(json_extract(data, '$.author') AS INTEGER)
		FROM resources
		WHERE resource_type IN ('posts', 'pages')
		  AND COALESCE(json_extract(data, '$.status'), '') IN (` + placeholders + `)`
	args := make([]any, 0, len(statuses))
	for _, status := range statuses {
		args = append(args, status)
	}
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apiErr(fmt.Errorf("query editorial queue: %w", err))
	}
	rawRows := make([]queueRow, 0)
	for rows.Next() {
		var id sql.NullInt64
		var row queueRow
		if err := rows.Scan(&id, &row.Title, &row.Status, &row.Modified, &row.Date, &row.Link, &row.AuthorID); err != nil {
			_ = rows.Close()
			return nil, apiErr(fmt.Errorf("scan editorial queue row: %w", err))
		}
		if !id.Valid {
			continue
		}
		row.ID = id.Int64
		rawRows = append(rawRows, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apiErr(fmt.Errorf("read editorial queue rows: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, apiErr(fmt.Errorf("close editorial queue rows: %w", err))
	}
	return rawRows, nil
}

func loadQueueAuthors(ctx context.Context, db *store.Store) (map[int64]string, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT
			COALESCE(CAST(json_extract(data, '$.id') AS INTEGER), CAST(id AS INTEGER)),
			COALESCE(json_extract(data, '$.name'), '')
		FROM resources
		WHERE resource_type IN ('users', 'posts_users', 'pages_users')`)
	if err != nil {
		return nil, apiErr(fmt.Errorf("query local authors: %w", err))
	}
	authors := make(map[int64]string)
	for rows.Next() {
		var id sql.NullInt64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			_ = rows.Close()
			return nil, apiErr(fmt.Errorf("scan local author: %w", err))
		}
		if id.Valid {
			authors[id.Int64] = cliutil.CleanText(name)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apiErr(fmt.Errorf("read local authors: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, apiErr(fmt.Errorf("close local authors: %w", err))
	}
	return authors, nil
}

func parseWordPressTime(value string, location *time.Location) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05Z07:00"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func daysInState(now, stateAt time.Time) int {
	return int(math.Floor(now.Sub(stateAt).Hours() / 24))
}

func printQueueTable(cmd *cobra.Command, buckets []queueBucket) error {
	summary := make([]map[string]any, 0, len(buckets))
	items := make([]map[string]any, 0)
	for _, bucket := range buckets {
		summary = append(summary, map[string]any{"status": bucket.Status, "count": bucket.Count})
		for _, item := range bucket.Items {
			items = append(items, map[string]any{
				"id":              item.ID,
				"title":           item.Title,
				"status":          item.Status,
				"days_in_state":   item.DaysInState,
				"link":            item.Link,
				"author":          item.Author,
				"missed_schedule": item.MissedSchedule,
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
