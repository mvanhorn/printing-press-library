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
func listAllVideoIDs(c *client.Client, max int) ([]string, error) {
	data, err := c.Get("/v1/videos", nil)
	if err != nil {
		return nil, err
	}
	ids := extractIDs(data, "videos")
	if max > 0 && len(ids) > max {
		ids = ids[:max]
	}
	return ids, nil
}

func listClipIDs(c *client.Client, videoID string) ([]string, error) {
	data, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips", videoID), nil)
	if err != nil {
		return nil, err
	}
	return extractIDs(data, "clips"), nil
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
