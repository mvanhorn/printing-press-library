// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/store"
	"github.com/spf13/cobra"
)

// digestEntry is one archived article in the digest output.
type digestEntry struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at"`
}

// pp:data-source local
// digest reads only from the local SQLite store (what is new since the last
// archive run); it makes no live API calls. Populate via author-archive first.
func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "A deduped, ranked 'what is new since last sync' across the authors, publications, and tags you have archived.",
		Example: strings.Trim(`
  medium-reader-pp-cli digest --since 7d --agent
  medium-reader-pp-cli digest --since 2w --limit 50
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagSince == "" {
				flagSince = "7d"
			}
			if limit <= 0 {
				limit = 30
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would digest archived articles published within %s\n", flagSince)
				return nil
			}

			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			cutoff := time.Now().Add(-window)

			if dbPath == "" {
				dbPath = defaultDBPath("medium-reader-pp-cli")
			}

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local mirror found; run 'medium-reader-pp-cli author-archive <username>' first")
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), make([]digestEntry, 0), flags)
				}
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.List("articles", 0)
			if err != nil {
				return fmt.Errorf("reading local store: %w", err)
			}

			seen := make(map[string]bool)
			type sortable struct {
				entry digestEntry
				when  time.Time
			}
			var collected []sortable
			for _, raw := range rows {
				var obj map[string]any
				if json.Unmarshal(raw, &obj) != nil {
					continue
				}
				id := asString(obj["id"])
				if id == "" || seen[id] {
					continue
				}
				published := asString(obj["published_at"])
				when := parsePublishedAt(published)
				if when.IsZero() || when.Before(cutoff) {
					continue
				}
				seen[id] = true
				collected = append(collected, sortable{
					entry: digestEntry{
						ID:          id,
						Title:       asString(obj["title"]),
						Author:      asString(obj["author"]),
						URL:         asString(obj["url"]),
						PublishedAt: published,
					},
					when: when,
				})
			}

			sort.SliceStable(collected, func(i, j int) bool {
				return collected[i].when.After(collected[j].when)
			})

			entries := make([]digestEntry, 0, len(collected))
			for _, s := range collected {
				if len(entries) >= limit {
					break
				}
				entries = append(entries, s.entry)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(entries) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "nothing archived in the last %s\n", flagSince)
					return nil
				}
				headers := []string{"PUBLISHED", "TITLE", "AUTHOR", "ID"}
				tableRows := make([][]string, 0, len(entries))
				for _, e := range entries {
					tableRows = append(tableRows, []string{e.PublishedAt, e.Title, e.Author, e.ID})
				}
				return flags.printTable(cmd, headers, tableRows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Only include articles published within this window (e.g. 7d, 2w, 36h)")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum number of entries to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (default: standard data dir)")
	return cmd
}

// parsePublishedAt parses Medium's published_at timestamps. The API serves
// "2006-01-02 15:04:05"; we also accept RFC3339 and the SQLite-stored layouts
// via cliutil.ParseStoredTime as a fallback.
func parsePublishedAt(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return cliutil.ParseStoredTime(s)
}
