// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: build the local sitemap index that the offline read
// commands (top-revenue, hunt, grep, stats, whats-new) query.

package cli

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/starterstory/internal/store"
	"github.com/spf13/cobra"
)

const sitemapURL = "https://www.starterstory.com/sitemap"

const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// ssLocRE captures the path portion of a StarterStory <loc> entry.
var ssLocRE = regexp.MustCompile(`<loc>https://www\.starterstory\.com/([^<]*)</loc>`)

// ssSections is the set of first-path-segments that classify into a section.
var ssSections = map[string]bool{
	"stories":    true,
	"ideas":      true,
	"businesses": true,
	"breakdowns": true,
	"tools":      true,
	"data":       true,
}

func newIndexCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Fetch the StarterStory sitemap and rebuild the local offline index.",
		Long: `Fetch the StarterStory sitemap (gzipped XML), classify every URL into a
section (stories, ideas, businesses, breakdowns, tools, data), parse an
approximate monthly revenue from story slugs, and upsert the results into the
local SQLite index. The offline commands (top-revenue, hunt, grep, stats,
whats-new) read from this index. Re-run to refresh; whats-new diffs against the
previous run.`,
		Example:     "  starterstory-pp-cli index\n  starterstory-pp-cli index --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch sitemap and rebuild local index")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("starterstory-pp-cli")
			}

			xml, err := fetchSitemap(ctx)
			if err != nil {
				return fmt.Errorf("fetching sitemap: %w", err)
			}

			rows := parseSitemapRows(xml)
			if len(rows) == 0 {
				return fmt.Errorf("sitemap parsed but no classifiable URLs found (site structure may have changed)")
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureStarterStoryIndex(ctx); err != nil {
				return err
			}

			now := time.Now().UTC().Format(time.RFC3339)
			if err := db.RebuildStarterStoryIndex(ctx, rows, now); err != nil {
				return err
			}

			counts := map[string]int{}
			for _, r := range rows {
				counts[r.Section]++
			}

			if flags.asJSON || flags.agent {
				payload := map[string]any{
					"total":      len(rows),
					"by_section": counts,
					"indexed_at": now,
					"db":         dbPath,
				}
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Indexed %d URLs into %s\n", len(rows), dbPath)
			sections := make([]string, 0, len(counts))
			for s := range counts {
				sections = append(sections, s)
			}
			sort.Strings(sections)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SECTION\tCOUNT")
			for _, s := range sections {
				fmt.Fprintf(tw, "%s\t%d\n", s, counts[s])
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// fetchSitemap GETs the sitemap, following the 302 to the CloudFront gzip, and
// returns the decompressed XML bytes. It handles both a gzip-magic body and a
// plain-text body (defensive against upstream Content-Encoding changes).
func fetchSitemap(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUserAgent)
	// Avoid transport-level gzip so we control decompression from the magic
	// bytes; the CloudFront object is served as application/gzip regardless.
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Gunzip when the body starts with the gzip magic (0x1f 0x8b).
	if len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer zr.Close()
		out, err := io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("gunzip: %w", err)
		}
		return out, nil
	}
	return body, nil
}

// parseSitemapRows extracts and classifies every StarterStory URL from the
// sitemap XML into IndexRow records. URLs whose first path segment is not a
// known section (and the bare root) are skipped.
func parseSitemapRows(xml []byte) []store.IndexRow {
	matches := ssLocRE.FindAllSubmatch(xml, -1)
	rows := make([]store.IndexRow, 0, len(matches))
	seen := map[string]bool{}
	for _, m := range matches {
		path := strings.TrimSpace(string(m[1]))
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			continue
		}
		segments := strings.Split(path, "/")
		section := segments[0]
		if !ssSections[section] {
			continue
		}
		url := "https://www.starterstory.com/" + path
		if seen[url] {
			continue
		}
		seen[url] = true
		slug := segments[len(segments)-1]
		rows = append(rows, store.IndexRow{
			URL:     url,
			Section: section,
			Slug:    slug,
			Title:   humanizeSlug(slug),
			Revenue: parseRevenueFromSlug(slug),
		})
	}
	return rows
}

// humanizeSlug turns "i-turned-my-hobby-into-120k-month-apps" into
// "I Turned My Hobby Into 120k Month Apps".
func humanizeSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}
