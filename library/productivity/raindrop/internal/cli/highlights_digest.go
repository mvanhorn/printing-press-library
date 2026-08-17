// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type digestHighlight struct {
	ID         string    `json:"id"`
	BookmarkID string    `json:"bookmark_id"`
	Title      string    `json:"title,omitempty"`
	Text       string    `json:"text"`
	Note       string    `json:"note,omitempty"`
	Created    time.Time `json:"created,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
}

func newNovelHighlightsDigestCmd(flags *rootFlags) *cobra.Command {
	var since, groupBy, dbPath string
	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "Create a deduplicated study digest from synced highlights",
		Example:     "  raindrop-pp-cli highlights digest --since 30d --group-by tag",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			age, err := parseAge(since, 30*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			if groupBy != "tag" && groupBy != "bookmark" {
				return usageErr(fmt.Errorf("--group-by must be tag or bookmark"))
			}
			db, path, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.List("highlights", 0)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-age)
			seen := map[string]bool{}
			groups := map[string][]digestHighlight{}
			for _, raw := range rows {
				var obj map[string]any
				if json.Unmarshal(raw, &obj) != nil {
					continue
				}
				h := digestHighlight{ID: valueID(obj["_id"]), BookmarkID: valueID(obj["raindropRef"]), Text: strings.TrimSpace(valueString(obj["text"])), Note: strings.TrimSpace(valueString(obj["note"])), Created: valueTime(obj["created"])}
				if h.ID == "" {
					h.ID = valueID(obj["id"])
				}
				if h.BookmarkID == "" {
					h.BookmarkID = valueID(obj["raindrop"])
				}
				if h.Text == "" || (!h.Created.IsZero() && h.Created.Before(cutoff)) {
					continue
				}
				key := h.BookmarkID + "\x00" + strings.ToLower(h.Text) + "\x00" + strings.ToLower(h.Note)
				if seen[key] {
					continue
				}
				seen[key] = true
				group := h.BookmarkID
				if group == "" {
					group = "unknown"
				}
				if groupBy == "tag" {
					group = "untagged"
					if tags, ok := obj["tags"].([]any); ok && len(tags) > 0 {
						group = valueString(tags[0])
						for _, tag := range tags {
							h.Tags = append(h.Tags, valueString(tag))
						}
					}
				}
				groups[group] = append(groups[group], h)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"database": path, "since": cutoff, "group_by": groupBy, "count": len(seen), "groups": groups}, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Include highlights created within this window")
	cmd.Flags().StringVar(&groupBy, "group-by", "bookmark", "Group by bookmark or tag")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newHighlightsExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath, format string
	cmd := &cobra.Command{
		Use: "export", Short: "Export synced highlights as Markdown or JSONL",
		Example:     "  raindrop-pp-cli highlights export --format md",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "md" && format != "jsonl" {
				return usageErr(fmt.Errorf("--format must be md or jsonl"))
			}
			db, _, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.List("highlights", 0)
			if err != nil {
				return err
			}
			sort.Slice(rows, func(i, j int) bool { return string(rows[i]) < string(rows[j]) })
			if flags.asJSON {
				items := make([]any, 0, len(rows))
				for _, raw := range rows {
					var item any
					if json.Unmarshal(raw, &item) == nil {
						items = append(items, item)
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"format": "json", "count": len(items), "items": items}, flags)
			}
			for _, raw := range rows {
				if format == "jsonl" {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(raw)); err != nil {
						return err
					}
					continue
				}
				var obj map[string]any
				if json.Unmarshal(raw, &obj) != nil {
					continue
				}
				text := strings.TrimSpace(valueString(obj["text"]))
				if text == "" {
					continue
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", text); err != nil {
					return err
				}
				if note := strings.TrimSpace(valueString(obj["note"])); note != "" {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  - Note: %s\n", note); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "Output format: md or jsonl")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
