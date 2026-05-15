// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/store"

	"github.com/spf13/cobra"
)

// newTranscriptsCmd builds the `transcripts` parent: FTS5 search and bulk sync
// across cached transcripts. The data lives in the local SQLite store; without
// a prior `transcripts sync` run, search returns zero hits.
func newTranscriptsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "transcripts",
		Short:       "FTS5 search and sync across cached clip transcripts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        rejectUnknownSubcommand,
	}
	cmd.AddCommand(newTranscriptsSearchCmd(flags))
	cmd.AddCommand(newTranscriptsSyncCmd(flags))
	return cmd
}

func newTranscriptsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "FTS5 search across cached transcripts",
		Example:     `  tella-pp-cli transcripts search "checkout flow" --json --limit 10`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				return usageErr(fmt.Errorf("missing required positional argument"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("tella-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			query := strings.Join(args, " ")
			hits, err := db.SearchTranscripts(query, limit)
			if err != nil {
				return apiErr(err)
			}
			if hits == nil {
				hits = []store.TranscriptHit{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"query": query,
				"count": len(hits),
				"hits":  hits,
			}, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of hits to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}

func newTranscriptsSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxVideos int
	var maxClipsPerVideo int
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Fetch transcripts for every video and clip, store locally with FTS5 index",
		Example: "  tella-pp-cli transcripts sync --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			if dbPath == "" {
				dbPath = defaultDBPath("tella-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			videoIDs, err := listAllVideoIDs(c, maxVideos)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			stored := 0
			skipped := 0
			for _, vid := range videoIDs {
				clipIDs, err := listClipIDs(c, vid)
				if err != nil {
					skipped++
					continue
				}
				if maxClipsPerVideo > 0 && len(clipIDs) > maxClipsPerVideo {
					clipIDs = clipIDs[:maxClipsPerVideo]
				}
				for _, cid := range clipIDs {
					data, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/cut", vid, cid), nil)
					if err != nil {
						var apiE *client.APIError
						if errors.As(err, &apiE) && apiE.StatusCode == 404 {
							skipped++
							continue
						}
						skipped++
						continue
					}
					text, wordTimings := extractTranscriptText(data)
					if text == "" {
						skipped++
						continue
					}
					if err := db.UpsertTranscript(vid, cid, "cut", text, wordTimings); err != nil {
						skipped++
						continue
					}
					stored++
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"videos_seen":   len(videoIDs),
				"transcripts":   stored,
				"skipped_clips": skipped,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	cmd.Flags().IntVar(&maxVideos, "max-videos", 0, "Cap videos scanned (0 = no cap)")
	cmd.Flags().IntVar(&maxClipsPerVideo, "max-clips-per-video", 0, "Cap clips per video (0 = no cap)")
	return cmd
}

// listAllVideoIDs returns up to max video IDs (or all when max <= 0) by reading
// the live `GET /v1/videos` endpoint and extracting `id` from each entry.
// Walks every cursor page (via paginatedListIDs) — previously this issued
// a single c.Get, so transcripts sync silently missed every video past the
// first API page on larger workspaces.
func listAllVideoIDs(c *client.Client, max int) ([]string, error) {
	ids, err := paginatedListIDs(c, "/v1/videos", nil, "videos")
	if err != nil {
		return nil, err
	}
	if max > 0 && len(ids) > max {
		ids = ids[:max]
	}
	return ids, nil
}

func listClipIDs(c *client.Client, videoID string) ([]string, error) {
	return paginatedListIDs(c, fmt.Sprintf("/v1/videos/%s/clips", videoID), nil, "clips")
}

// paginatedListIDs walks the cursor-paged Tella response for a list endpoint
// and accumulates `id` strings across every page. Without this helper, the
// edit-pass family silently processed only the first API page on workspaces
// that exceeded the default page size — `total_clips` reflected the
// truncated count and the user got no warning. Uses the same extractPageItems
// envelope/cursor logic that sync.go uses, with a sticky-cursor guard and a
// 100-page hard cap (10k items at the default page size) so a misbehaving
// API can't burn unbounded calls.
//
// Caller passes a narrow Get-only interface so unit tests can stub the
// transport without standing up an httptest server.
func paginatedListIDs(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, path string, params map[string]string, arrayKey string) ([]string, error) {
	pageDefaults := determinePaginationDefaults()
	cursorParam := pageDefaults.cursorParam
	limitParam := pageDefaults.limitParam

	query := map[string]string{}
	for k, v := range params {
		query[k] = v
	}
	query[limitParam] = fmt.Sprintf("%d", pageDefaults.limit)

	const maxPages = 100
	var ids []string
	var cursor, lastCursor string
	for page := 0; page < maxPages; page++ {
		if cursor != "" {
			query[cursorParam] = cursor
		} else {
			delete(query, cursorParam)
		}

		data, err := c.Get(path, query)
		if err != nil {
			return nil, err
		}

		items, nextCursor, hasMore := extractPageItems(data, cursorParam)

		// Per-page ID extraction. extractPageItems hands us
		// []json.RawMessage; decode each one into an object to pull
		// `id`. Fall back to extractIDs for the (rare) case where the
		// page came back in an unrecognized envelope shape and items
		// is empty — better to surface the first page than nothing.
		if len(items) > 0 {
			for _, raw := range items {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				if id, ok := obj["id"].(string); ok && id != "" {
					ids = append(ids, id)
				}
			}
		} else if page == 0 {
			ids = append(ids, extractIDs(data, arrayKey)...)
		}

		if !hasMore || nextCursor == "" {
			break
		}
		if nextCursor == lastCursor {
			break
		}
		lastCursor = nextCursor
		cursor = nextCursor
	}
	return ids, nil
}

// extractIDs pulls "id" fields out of a JSON response that is either a bare
// array of objects or an envelope `{<arrayKey>: [...]}` (Tella uses the
// envelope shape for both `videos` and `clips`).
func extractIDs(data json.RawMessage, arrayKey string) []string {
	var out []string
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		for _, item := range arr {
			if id, ok := item["id"].(string); ok && id != "" {
				out = append(out, id)
			}
		}
		return out
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err == nil {
		if raw, ok := env[arrayKey]; ok {
			var inner []map[string]any
			if err := json.Unmarshal(raw, &inner); err == nil {
				for _, item := range inner {
					if id, ok := item["id"].(string); ok && id != "" {
						out = append(out, id)
					}
				}
			}
		}
	}
	return out
}

// extractTranscriptText pulls a flat text rendering from a Tella transcript
// payload. The API returns a structure with words/segments containing text +
// timings; the second return value is the raw word-timings JSON for callers
// that need word-level timestamps (clips captions).
func extractTranscriptText(data json.RawMessage) (string, string) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", ""
	}
	// Try "text" field first
	if t, ok := obj["text"].(string); ok && t != "" {
		wt, _ := json.Marshal(obj["words"])
		return t, string(wt)
	}
	// Try "transcript" string
	if t, ok := obj["transcript"].(string); ok && t != "" {
		wt, _ := json.Marshal(obj["words"])
		return t, string(wt)
	}
	// Try "words" array of {text|word, start, end}
	if wordsRaw, ok := obj["words"].([]any); ok && len(wordsRaw) > 0 {
		parts := make([]string, 0, len(wordsRaw))
		for _, w := range wordsRaw {
			if wm, ok := w.(map[string]any); ok {
				for _, k := range []string{"text", "word", "value"} {
					if s, ok := wm[k].(string); ok && s != "" {
						parts = append(parts, s)
						break
					}
				}
			}
		}
		wt, _ := json.Marshal(wordsRaw)
		return strings.Join(parts, " "), string(wt)
	}
	// Try "segments" array of {text, start, end}
	if segsRaw, ok := obj["segments"].([]any); ok && len(segsRaw) > 0 {
		parts := make([]string, 0, len(segsRaw))
		for _, s := range segsRaw {
			if sm, ok := s.(map[string]any); ok {
				if t, ok := sm["text"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
		}
		wt, _ := json.Marshal(segsRaw)
		return strings.Join(parts, " "), string(wt)
	}
	return "", ""
}
