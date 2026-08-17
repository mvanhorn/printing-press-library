// pp:data-source local
// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"net/url"
	"os"
	pathpkg "path"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/store"

	"github.com/spf13/cobra"
)

var (
	contentURLPattern = regexp.MustCompile(`(?i)(?:https?:)?//[^\s"'<>]+|/[^\s"'<>]+`)
	resizeSuffix      = regexp.MustCompile(`-\d+[xX]\d+$`)
)

type orphanMediaRow struct {
	ID        int64
	SourceURL string
	Filesize  sql.NullInt64
	Date      string
	Title     string
	MIMEType  string
}

type orphanContentRow struct {
	ID            int64
	Content       string
	FeaturedMedia sql.NullInt64
}

type orphanItem struct {
	ID        int64  `json:"id"`
	SourceURL string `json:"source_url"`
	Filesize  *int64 `json:"filesize,omitempty"`
	Date      string `json:"date"`
	Title     string `json:"title"`
	MIMEType  string `json:"mime_type"`
}

type orphansOutput struct {
	Orphans               []orphanItem `json:"orphans"`
	OrphanCount           int          `json:"orphan_count"`
	TotalReclaimableBytes int64        `json:"total_reclaimable_bytes"`
	ScannedMedia          int          `json:"scanned_media"`
	ScannedContent        int          `json:"scanned_content"`
}

// newNovelOrphansCmd keeps compatibility with older generated root wiring. The
// additive hook in queue.go replaces this instance with the durable command.
func newNovelOrphansCmd(flags *rootFlags) *cobra.Command {
	return newOrphansCmd(flags)
}

func newOrphansCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "orphans [--limit 100] [--json]",
		Short: "Find media files no post or page references",
		Long:  "Use this command to find media files no content references. Do NOT use this command for defects on posts themselves; use 'audit' instead.",
		Example: "  wordpress-pp-cli orphans --limit 100\n" +
			"  wordpress-pp-cli orphans --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No bare-invocation help branch: `orphans` takes no required input,
			// so running it bare must do the work, matching the framework's
			// own zero-arg list commands (e.g. `profile list`).
			if len(args) != 0 {
				return usageErr(fmt.Errorf("orphans accepts flags only"))
			}
			if strings.EqualFold(strings.TrimSpace(flags.dataSource), "live") {
				return usageErr(fmt.Errorf("orphans has no live equivalent; sync the site and use the local mirror"))
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

			mediaRows, err := loadOrphanMedia(ctx, db)
			if err != nil {
				return err
			}
			contentRows, err := loadOrphanContent(ctx, db)
			if err != nil {
				return err
			}
			if !hintIfUnsynced(cmd, db, "posts") {
				hintIfStale(cmd, db, "posts", flags.maxAge)
			}

			featured := make(map[int64]struct{})
			bodies := make([]string, 0, len(contentRows))
			for _, content := range contentRows {
				bodies = append(bodies, content.Content)
				if content.FeaturedMedia.Valid && content.FeaturedMedia.Int64 > 0 {
					featured[content.FeaturedMedia.Int64] = struct{}{}
				}
			}
			referencedPaths := collectReferencedMediaPaths(bodies)

			allOrphans := make([]orphanItem, 0)
			var reclaimable int64
			for _, media := range mediaRows {
				if _, usedAsFeatured := featured[media.ID]; usedAsFeatured {
					continue
				}
				if _, referenced := referencedPaths[normalizeMediaPath(media.SourceURL)]; referenced {
					continue
				}
				item := orphanItem{
					ID:        media.ID,
					SourceURL: media.SourceURL,
					Date:      media.Date,
					Title:     cliutil.CleanText(media.Title),
					MIMEType:  media.MIMEType,
				}
				if media.Filesize.Valid {
					size := media.Filesize.Int64
					item.Filesize = &size
					reclaimable += size
				}
				allOrphans = append(allOrphans, item)
			}
			sort.SliceStable(allOrphans, func(i, j int) bool {
				if allOrphans[i].Date == allOrphans[j].Date {
					return allOrphans[i].ID < allOrphans[j].ID
				}
				return allOrphans[i].Date < allOrphans[j].Date
			})
			orphanCount := len(allOrphans)
			if len(allOrphans) > limit {
				allOrphans = allOrphans[:limit]
			}
			if allOrphans == nil {
				allOrphans = make([]orphanItem, 0)
			}
			output := orphansOutput{
				Orphans:               allOrphans,
				OrphanCount:           orphanCount,
				TotalReclaimableBytes: reclaimable,
				ScannedMedia:          len(mediaRows),
				ScannedContent:        len(contentRows),
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), output, flags)
			}
			return printOrphansTable(cmd, output)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum orphaned media rows shown")
	return cmd
}

func loadOrphanMedia(ctx context.Context, db *store.Store) ([]orphanMediaRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT
			COALESCE(CAST(json_extract(data, '$.id') AS INTEGER), CAST(id AS INTEGER)),
			COALESCE(json_extract(data, '$.source_url'), ''),
			CAST(json_extract(data, '$.media_details.filesize') AS INTEGER),
			COALESCE(json_extract(data, '$.date'), ''),
			COALESCE(json_extract(data, '$.title.rendered'), ''),
			COALESCE(json_extract(data, '$.mime_type'), '')
		FROM resources
		WHERE resource_type IN ('media', 'posts_media', 'pages_media')`)
	if err != nil {
		return nil, apiErr(fmt.Errorf("query local media: %w", err))
	}
	mediaRows := make([]orphanMediaRow, 0)
	for rows.Next() {
		var id sql.NullInt64
		var media orphanMediaRow
		if err := rows.Scan(&id, &media.SourceURL, &media.Filesize, &media.Date, &media.Title, &media.MIMEType); err != nil {
			_ = rows.Close()
			return nil, apiErr(fmt.Errorf("scan local media row: %w", err))
		}
		if !id.Valid {
			continue
		}
		media.ID = id.Int64
		mediaRows = append(mediaRows, media)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apiErr(fmt.Errorf("read local media rows: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, apiErr(fmt.Errorf("close local media rows: %w", err))
	}
	return mediaRows, nil
}

func loadOrphanContent(ctx context.Context, db *store.Store) ([]orphanContentRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT
			COALESCE(CAST(json_extract(data, '$.id') AS INTEGER), CAST(id AS INTEGER)),
			COALESCE(json_extract(data, '$.content.rendered'), ''),
			CAST(json_extract(data, '$.featured_media') AS INTEGER)
		FROM resources
		WHERE resource_type IN ('posts', 'pages')`)
	if err != nil {
		return nil, apiErr(fmt.Errorf("query local content references: %w", err))
	}
	contentRows := make([]orphanContentRow, 0)
	for rows.Next() {
		var id sql.NullInt64
		var content orphanContentRow
		if err := rows.Scan(&id, &content.Content, &content.FeaturedMedia); err != nil {
			_ = rows.Close()
			return nil, apiErr(fmt.Errorf("scan local content reference: %w", err))
		}
		if !id.Valid {
			continue
		}
		content.ID = id.Int64
		contentRows = append(contentRows, content)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apiErr(fmt.Errorf("read local content references: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, apiErr(fmt.Errorf("close local content references: %w", err))
	}
	return contentRows, nil
}

func mediaReferencedByContent(mediaURL string, contentBodies []string) bool {
	mediaPath := normalizeMediaPath(mediaURL)
	if mediaPath == "" {
		return false
	}
	_, found := collectReferencedMediaPaths(contentBodies)[mediaPath]
	return found
}

func collectReferencedMediaPaths(contentBodies []string) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, body := range contentBodies {
		for _, candidate := range contentURLPattern.FindAllString(html.UnescapeString(body), -1) {
			normalized := normalizeMediaPath(candidate)
			if normalized != "" {
				paths[normalized] = struct{}{}
			}
		}
	}
	return paths
}

func normalizeMediaPath(raw string) string {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	raw = strings.Trim(raw, `"'(),`)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	mediaPath := parsed.Path
	if mediaPath == "" && strings.HasPrefix(raw, "/") {
		mediaPath = strings.SplitN(raw, "?", 2)[0]
	}
	if decoded, err := url.PathUnescape(mediaPath); err == nil {
		mediaPath = decoded
	}
	if mediaPath == "" {
		return ""
	}
	mediaPath = pathpkg.Clean(mediaPath)
	lowerPath := strings.ToLower(mediaPath)
	if uploadIndex := strings.Index(lowerPath, "/wp-content/uploads/"); uploadIndex >= 0 {
		mediaPath = mediaPath[uploadIndex:]
	}
	extension := pathpkg.Ext(mediaPath)
	base := strings.TrimSuffix(mediaPath, extension)
	base = resizeSuffix.ReplaceAllString(base, "")
	return base + extension
}

func printOrphansTable(cmd *cobra.Command, output orphansOutput) error {
	summary := []map[string]any{{
		"orphan_count":            output.OrphanCount,
		"total_reclaimable_bytes": output.TotalReclaimableBytes,
		"scanned_media":           output.ScannedMedia,
		"scanned_content":         output.ScannedContent,
	}}
	if err := printAutoTable(cmd.OutOrStdout(), summary); err != nil {
		return err
	}
	if len(output.Orphans) == 0 {
		return nil
	}
	items := make([]map[string]any, 0, len(output.Orphans))
	for _, item := range output.Orphans {
		var filesize any = ""
		if item.Filesize != nil {
			filesize = *item.Filesize
		}
		items = append(items, map[string]any{
			"id":         item.ID,
			"title":      item.Title,
			"mime_type":  item.MIMEType,
			"filesize":   filesize,
			"date":       item.Date,
			"source_url": item.SourceURL,
		})
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return printAutoTable(cmd.OutOrStdout(), items)
}
