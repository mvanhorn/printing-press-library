// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Lists items added to a live webset since your last sync or
// within a window, so lead-gen sweeps only see what changed.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/exa/internal/store"
)

// websetNewItem is one item row surfaced by `webset new`.
type websetNewItem struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	URL       string `json:"url,omitempty"`
	SyncedAt  string `json:"syncedAt,omitempty"`
	Entity    string `json:"entity,omitempty"`
	EntityID  string `json:"entityId,omitempty"`
	EntityURL string `json:"entityUrl,omitempty"`
}

// decodeWebsetItem pulls the human-facing fields out of a stored webset item
// (WebsetItem shape: id + typed entity properties per entity kind).
func decodeWebsetItem(data json.RawMessage) websetNewItem {
	var item struct {
		ID     string `json:"id"`
		URL    string `json:"url"`
		Title  string `json:"title"`
		Entity struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"entity"`
	}
	_ = json.Unmarshal(data, &item)
	out := websetNewItem{ID: item.ID, URL: item.URL, Title: item.Title}
	if out.URL == "" && item.Entity.URL != "" {
		out.URL = item.Entity.URL
	}
	if out.Title == "" && item.Entity.Name != "" {
		out.Title = item.Entity.Name
	}
	out.Entity = item.Entity.Type
	out.EntityID = item.Entity.ID
	out.EntityURL = item.Entity.URL
	return out
}

func newNovelWebsetNewCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "new [webset-id]",
		Short: "List items added to a live webset since your last sync, so you only see what changed.",
		Long: `Use this command for what is new inside one live webset since you last looked.
Do NOT use it to compare scheduled monitor runs; use 'monitor diff'.
Do NOT use it for a named entity's timeline; use 'entity report'.`,
		Example:     "  exa-pp-cli webset new '<webset-id>' --since 7d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3", "pp:happy-args": "<webset-id>=ws-1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webset new")
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("webset-id is required"))
			}
			websetID := strings.TrimSpace(args[0])
			if websetID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("webset-id is required"))
			}

			window := 7 * 24 * time.Hour
			if flagSince != "" {
				d, err := parseHumanDuration(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since must be a duration like 7d or 24h, got %q", flagSince))
				}
				window = d
			}

			dbPath := defaultDBPath("exa-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no results for webset %s \u2014 no local mirror yet\nrun: exa-pp-cli sync --resources websets --db %s\n", websetID, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"websetId": websetID, "error": "no-local-mirror",
						"syncHint": "exa-pp-cli sync --resources websets",
					}, flags)
				}
				return fmt.Errorf("no results for webset %s: run 'exa-pp-cli sync --resources websets' first", websetID)
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "items") {
				hintIfStale(cmd, db, "items", flags.maxAge)
			}

			cutoff := time.Now().Add(-window)
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, data, synced_at FROM items WHERE websets_id = ? ORDER BY synced_at DESC`, websetID)
			if err != nil {
				return fmt.Errorf("querying webset items: %w", err)
			}
			defer rows.Close()
			var items []websetNewItem
			for rows.Next() {
				var id, data, syncedAt string
				if err := rows.Scan(&id, &data, &syncedAt); err != nil {
					return fmt.Errorf("scanning webset item: %w", err)
				}
				ts := parseSyncedAt(syncedAt)
				if ts.IsZero() || ts.Before(cutoff) {
					continue
				}
				item := decodeWebsetItem(json.RawMessage(data))
				if item.ID == "" {
					item.ID = id
				}
				item.SyncedAt = ts.UTC().Format(time.RFC3339)
				items = append(items, item)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating webset items: %w", err)
			}
			sort.Slice(items, func(i, j int) bool { return items[i].SyncedAt > items[j].SyncedAt })

			view := struct {
				WebsetID string          `json:"websetId"`
				Since    string          `json:"since"`
				Count    int             `json:"addedCount"`
				Items    []websetNewItem `json:"items"`
				Source   string          `json:"source"`
			}{
				WebsetID: websetID,
				Since:    flagSinceIfSet(flagSince, window),
				Count:    len(items),
				Items:    items,
				Source:   "local",
			}

			if len(items) == 0 {
				if flags.asJSON || flags.agent {
					_ = printJSONFiltered(cmd.OutOrStdout(), view, flags)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No results for webset %s within the last %s.\n", websetID, view.Since)
				}
				return notFoundErr(fmt.Errorf("no results for webset %s within the last %s", websetID, view.Since))
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Webset %s — %d new item(s) in the last %s:\n", websetID, len(items), view.Since)
			for _, it := range items {
				title := it.Title
				if title == "" {
					title = it.URL
				}
				if title == "" {
					title = it.ID
				}
				line := fmt.Sprintf("  + %s (%s)", title, it.SyncedAt)
				if it.Entity != "" {
					line += fmt.Sprintf(" [%s]", it.Entity)
				}
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Only list items synced within this window (e.g. 7d, 24h)")
	return cmd
}

func flagSinceIfSet(raw string, window time.Duration) string {
	if raw != "" {
		return raw
	}
	return window.String()
}
