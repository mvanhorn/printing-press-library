// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

type briefResult struct {
	Title      string                   `json:"title"`
	Source     string                   `json:"source"`
	Window     string                   `json:"window,omitempty"`
	ItemCount  int                      `json:"item_count"`
	Highlights []briefHighlight         `json:"highlights"`
	Items      []collectionItemSnapshot `json:"items"`
}

type briefHighlight struct {
	TweetID string `json:"tweet_id"`
	Reason  string `json:"reason"`
	URL     string `json:"url"`
}

func newNovelBriefCmd(flags *rootFlags) *cobra.Command {
	var dbPath, monitor, collection, ids, format, since string
	var limit int
	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Package monitor, collection, or post results into a source-backed brief",
		Example: `  x-twitter-pp-cli brief --monitor ai-labs --since 24h --agent
  x-twitter-pp-cli brief --collection launch-feedback --format markdown
  x-twitter-pp-cli brief --ids 123,456 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			items, source, err := briefItems(cmd, flags, dbPath, monitor, collection, ids, since, limit)
			if err != nil {
				return err
			}
			result := buildBrief(source, since, items)
			if strings.EqualFold(format, "markdown") || strings.EqualFold(format, "md") {
				return writeBriefMarkdown(cmd.OutOrStdout(), result)
			}
			if format != "" && !strings.EqualFold(format, "json") {
				return usageErr(fmt.Errorf("invalid --format %q: expected json or markdown", format))
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta:    workflowMeta("brief", result.Source),
				Results: result,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	cmd.Flags().StringVar(&monitor, "monitor", "", "Build a brief from saved monitor results")
	cmd.Flags().StringVar(&collection, "collection", "", "Build a brief from a local collection")
	cmd.Flags().StringVar(&ids, "ids", "", "Comma-separated post IDs or URLs to resolve into the brief")
	cmd.Flags().StringVar(&since, "since", "", "Limit local monitor results to a window such as 24h or 7d")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum items to include")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or markdown")
	return cmd
}

func briefItems(cmd *cobra.Command, flags *rootFlags, dbPath, monitor, collection, ids, since string, limit int) ([]collectionItemSnapshot, string, error) {
	set := 0
	for _, value := range []string{monitor, collection, ids} {
		if strings.TrimSpace(value) != "" {
			set++
		}
	}
	if set != 1 {
		return nil, "", usageErr(fmt.Errorf("set exactly one of --monitor, --collection, or --ids"))
	}
	if limit <= 0 {
		limit = 25
	}
	switch {
	case monitor != "":
		db, err := openWorkflowDB(cmd, dbPath)
		if err != nil {
			return nil, "", err
		}
		defer db.Close()
		items, err := listMonitorResultItems(cmd, db, monitor, since, limit)
		return items, "local", err
	case collection != "":
		db, err := openWorkflowDB(cmd, dbPath)
		if err != nil {
			return nil, "", err
		}
		defer db.Close()
		items, err := listCollectionItems(cmd, db, collection, limit, false)
		return items, "local", err
	default:
		include := parseIncludeSet("author,links,refs,metrics")
		var out []collectionItemSnapshot
		for _, input := range strings.Split(ids, ",") {
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}
			rec, err := resolvePost(cmd, flags, input, dbPath, flags.dataSource, include)
			if err != nil {
				return nil, "", err
			}
			out = append(out, collectionItemFromPost(rec, ""))
			if len(out) >= limit {
				break
			}
		}
		return out, "mixed", nil
	}
}

func listMonitorResultItems(cmd *cobra.Command, db *store.Store, monitor, since string, limit int) ([]collectionItemSnapshot, error) {
	q := `SELECT tweet_id, tweet_json, source_url, seen_at
		FROM workflow_monitor_results
		WHERE monitor_name = ?`
	args := []any{monitor}
	if start, ok, err := sinceStartTime(since); err != nil {
		return nil, err
	} else if ok {
		q += ` AND seen_at >= ?`
		args = append(args, start.Format(time.RFC3339))
	}
	q += ` ORDER BY seen_at DESC, tweet_id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.DB().QueryContext(cmd.Context(), q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []collectionItemSnapshot
	for rows.Next() {
		var tweetID, raw, seenAt string
		var urlValue sql.NullString
		if err := rows.Scan(&tweetID, &raw, &urlValue, &seenAt); err != nil {
			return nil, err
		}
		var rec resolvedPostRecord
		_ = json.Unmarshal([]byte(raw), &rec)
		item := collectionItemFromPost(&rec, seenAt)
		item.TweetID = tweetID
		if item.URL == "" && urlValue.Valid {
			item.URL = urlValue.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		exists, err := monitorExists(cmd, db, monitor)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, notFoundErr(fmt.Errorf("monitor %q not found", monitor))
		}
	}
	return items, nil
}

func monitorExists(cmd *cobra.Command, db *store.Store, name string) (bool, error) {
	var one int
	err := db.DB().QueryRowContext(cmd.Context(), `SELECT 1 FROM workflow_monitors WHERE name = ?`, name).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func buildBrief(source, since string, items []collectionItemSnapshot) briefResult {
	result := briefResult{
		Title:     "X activity brief",
		Source:    source,
		Window:    since,
		ItemCount: len(items),
		Items:     items,
	}
	for _, item := range items {
		reason := "Recent source item"
		if item.Author != nil && authorDisplay(item.Author) != "" {
			reason = "Recent source item from " + authorDisplay(item.Author)
		}
		result.Highlights = append(result.Highlights, briefHighlight{TweetID: item.TweetID, URL: item.URL, Reason: reason})
		if len(result.Highlights) >= 5 {
			break
		}
	}
	return result
}

func writeBriefMarkdown(w workflowWriter, result briefResult) error {
	if err := workflowFprintf(w, "# %s\n\n", result.Title); err != nil {
		return err
	}
	if result.Window != "" {
		if err := workflowFprintf(w, "- Window: %s\n", result.Window); err != nil {
			return err
		}
	}
	if err := workflowFprintf(w, "- Items: %d\n\n", result.ItemCount); err != nil {
		return err
	}
	if len(result.Highlights) > 0 {
		if err := workflowFprintln(w, "## Highlights"); err != nil {
			return err
		}
		for _, h := range result.Highlights {
			if err := workflowFprintf(w, "- [%s](%s): %s\n", h.TweetID, h.URL, h.Reason); err != nil {
				return err
			}
		}
		if err := workflowFprintln(w); err != nil {
			return err
		}
	}
	if err := workflowFprintln(w, "## Sources"); err != nil {
		return err
	}
	for _, item := range result.Items {
		if err := workflowFprintf(w, "### %s\n\n", item.URL); err != nil {
			return err
		}
		if item.Author != nil {
			if err := workflowFprintf(w, "- Author: %s\n", authorDisplay(item.Author)); err != nil {
				return err
			}
		}
		if item.SavedAt != "" {
			if err := workflowFprintf(w, "- Seen: %s\n", item.SavedAt); err != nil {
				return err
			}
		}
		if err := workflowFprintf(w, "\n%s\n\n", item.Text); err != nil {
			return err
		}
	}
	return nil
}
