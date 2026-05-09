// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/store"

	"github.com/spf13/cobra"
)

// syncPagesCmd adds a "sync pages" subcommand that uses the search API to sync
// all pages and databases the integration can see into the local store.
func newSyncPagesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var full bool
	var objectType string

	cmd := &cobra.Command{
		Use:   "pages",
		Short: "Sync all pages and databases to local SQLite via search API",
		Long: `Uses POST /v1/search to enumerate every page and database the integration
can see, then stores metadata (id, title, parent, last_edited_time) locally.

Incremental: on subsequent runs, only fetches pages edited since the last sync.
Full: re-syncs everything from scratch.

Run this before using 'stale', 'changed', or SQL queries against page data.`,
		Example: `  notion-pp-cli sync pages
  notion-pp-cli sync pages --full
  notion-pp-cli sync pages --type database`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"event":"sync_complete","resource":"pages","total":0}`)
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			authHeader := cfg.AuthHeader()
			if authHeader == "" {
				return fmt.Errorf("not authenticated — run 'notion-pp-cli auth set-token' or set NOTION_TOKEN")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("notion-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			types := []string{"page", "database"}
			if objectType != "" {
				types = []string{objectType}
			}

			totalSynced := 0
			for _, t := range types {
				n, err := syncObjectType(cmd, cfg.BaseURL, authHeader, db, t, full)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"sync_error","resource":"%s","error":"%s"}`+"\n", t, err)
				} else {
					totalSynced += n
					fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_complete","resource":"%s","total":%d}`+"\n", t, n)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_summary","total":%d}`+"\n", totalSynced)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/notion-pp-cli/data.db)")
	cmd.Flags().BoolVar(&full, "full", false, "Full resync — ignore previous checkpoint")
	cmd.Flags().StringVar(&objectType, "type", "", "Sync only 'page' or 'database' (default: both)")

	return cmd
}

func syncObjectType(cmd *cobra.Command, baseURL, authHeader string, db *store.Store, objectType string, full bool) (int, error) {
	resourceKey := objectType + "s" // "pages" or "databases"

	// Get checkpoint for incremental sync
	var sinceTime time.Time
	if !full {
		_, lastSynced, _, _ := db.GetSyncState(resourceKey)
		sinceTime = lastSynced
	}

	fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_start","resource":"%s"}`+"\n", resourceKey)

	var cursor string
	total := 0
	done := false
	newestEditTime := time.Time{}

	for !done {
		body := map[string]any{
			"filter": map[string]any{
				"value":    objectType,
				"property": "object",
			},
			"sort": map[string]any{
				"direction": "descending",
				"timestamp": "last_edited_time",
			},
			"page_size": 100,
		}
		if cursor != "" {
			body["start_cursor"] = cursor
		}

		bodyBytes, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost,
			baseURL+"/v1/search", bytes.NewReader(bodyBytes))
		if err != nil {
			return total, fmt.Errorf("building request: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Notion-Version", "2022-06-28")
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return total, fmt.Errorf("search request failed: %w", err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return total, fmt.Errorf("reading response: %w", err)
		}
		if resp.StatusCode >= 400 {
			var apiErr map[string]any
			if json.Unmarshal(data, &apiErr) == nil {
				return total, fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr["message"])
			}
			return total, fmt.Errorf("API error %d", resp.StatusCode)
		}

		var page struct {
			Results    []json.RawMessage `json:"results"`
			HasMore    bool              `json:"has_more"`
			NextCursor string            `json:"next_cursor"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return total, fmt.Errorf("parsing response: %w", err)
		}

		for _, item := range page.Results {
			var meta struct {
				ID             string `json:"id"`
				LastEditedTime string `json:"last_edited_time"`
			}
			if err := json.Unmarshal(item, &meta); err != nil {
				continue
			}

			// Incremental: stop when we reach pages older than last sync
			if !sinceTime.IsZero() {
				editedAt, err := time.Parse(time.RFC3339, meta.LastEditedTime)
				if err == nil && editedAt.Before(sinceTime) {
					done = true
					break
				}
			}

			if err := db.Upsert(resourceKey, meta.ID, item); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"upsert_error","id":"%s","error":"%s"}`+"\n", meta.ID, err)
				continue
			}
			total++

			// Track the most recently edited time for checkpoint
			if editedAt, err := time.Parse(time.RFC3339, meta.LastEditedTime); err == nil {
				if newestEditTime.IsZero() || editedAt.After(newestEditTime) {
					newestEditTime = editedAt
				}
			}

			if total%100 == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_progress","resource":"%s","fetched":%d}`+"\n", resourceKey, total)
			}
		}

		if !page.HasMore || page.NextCursor == "" {
			done = true
		} else {
			cursor = page.NextCursor
		}
	}

	// Save checkpoint
	checkpoint := ""
	if !newestEditTime.IsZero() {
		checkpoint = newestEditTime.Format(time.RFC3339)
	}
	_ = db.SaveSyncState(resourceKey, checkpoint, total)

	return total, nil
}
