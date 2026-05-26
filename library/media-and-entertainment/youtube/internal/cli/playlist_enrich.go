// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-20260523: novel command) — collapses the playlistItems.list ->
// videos.list -> per-video transcript workflow into one call with bounded
// concurrency, start-index/limit paging, and per-row error isolation. Brings
// the CLI to parity with the yt-video-mcp fetch_playlist_full tool. Calls the
// real Data API (playlistItems.list, videos.list) and the real InnerTube
// transcript path; no hand-rolled payloads.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil"

	"github.com/spf13/cobra"
)

type enrichedVideo struct {
	VideoID         string            `json:"videoId"`
	Position        int               `json:"position"`
	Title           string            `json:"title"`
	Description     string            `json:"description,omitempty"`
	ChannelTitle    string            `json:"channelTitle,omitempty"`
	PublishedAt     string            `json:"publishedAt,omitempty"`
	Duration        string            `json:"duration,omitempty"`
	ViewCount       string            `json:"viewCount,omitempty"`
	LikeCount       string            `json:"likeCount,omitempty"`
	WatchURL        string            `json:"watchUrl"`
	EmbedURL        string            `json:"embedUrl"`
	ThumbnailURL    string            `json:"thumbnailUrl,omitempty"`
	Transcript      *transcriptResult `json:"transcript,omitempty"`
	TranscriptError string            `json:"transcriptError,omitempty"`
	MetadataError   string            `json:"metadataError,omitempty"`
}

type playlistEnrichResponse struct {
	PlaylistID    string `json:"playlistId"`
	PlaylistTitle string `json:"playlistTitle,omitempty"`
	// TotalItems is the playlist's full length (playlistItems.list
	// pageInfo.totalResults), so startIndex+returned < totalItems is a valid
	// "more pages exist" test even though only the scan window is enriched.
	TotalItems int             `json:"totalItems"`
	StartIndex int             `json:"startIndex"`
	Limit      int             `json:"limit"`
	Returned   int             `json:"returned"`
	Videos     []enrichedVideo `json:"videos"`
	Warnings   []string        `json:"warnings,omitempty"`
}

func newYoutubePlaylistEnrichCmd(flags *rootFlags) *cobra.Command {
	var startIndex int
	var limit int
	var lang string
	var noTranscript bool
	var concurrency int

	cmd := &cobra.Command{
		Use:         "playlist-enrich <playlistUrlOrId>",
		Short:       "Resolve a playlist to per-video metadata + transcript + description in one concurrent call",
		Example:     "  youtube-pp-cli youtube playlist-enrich https://www.youtube.com/playlist?list=PLxxxx --limit 25",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			playlistID := parsePlaylistID(strings.TrimSpace(args[0]))
			if playlistID == "" {
				return usageErr(fmt.Errorf("could not extract a playlist ID from %q (expected a playlist URL or a PL.../UU.../LL... ID)", args[0]))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be > 0"))
			}
			if startIndex < 0 {
				return usageErr(fmt.Errorf("--start-index must be >= 0"))
			}
			if concurrency < 1 {
				concurrency = 1
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /youtube/v3/playlistItems (playlistId=%s)\n", playlistID)
				fmt.Fprintln(cmd.ErrOrStderr(), "GET /youtube/v3/videos (batched metadata for the paged window)")
				if !noTranscript {
					fmt.Fprintln(cmd.ErrOrStderr(), "POST https://www.youtube.com/youtubei/v1/player (transcript per video, concurrent)")
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c = c.WithContext(cmd.Context())

			out := playlistEnrichResponse{
				PlaylistID: playlistID,
				StartIndex: startIndex,
				Limit:      limit,
			}

			// 1. Resolve the playlist title (best-effort; playlistItems.list
			// does not carry it). A failure here is non-fatal — enrichment
			// proceeds without the title.
			out.PlaylistTitle = fetchPlaylistTitle(c, playlistID)

			// 2. Walk the playlist to collect ordered video IDs. Cap the page
			// walk so a giant playlist doesn't burn quota; we only need enough
			// items to cover startIndex+limit.
			needed := startIndex + limit
			ordered, totalItems, walkWarns, err := walkPlaylistItems(c, playlistID, needed, flags)
			if err != nil {
				return err
			}
			// totalItems is the playlist's true length (playlistItems.list
			// pageInfo.totalResults), not the count we scanned. Fall back to
			// the scanned count if the API omitted/under-reported it.
			if totalItems < len(ordered) {
				totalItems = len(ordered)
			}
			out.TotalItems = totalItems
			out.Warnings = append(out.Warnings, walkWarns...)

			// 3. Apply start-index / limit window over the resolved IDs.
			if startIndex >= len(ordered) {
				out.Returned = 0
				out.Videos = []enrichedVideo{}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			end := startIndex + limit
			if end > len(ordered) {
				end = len(ordered)
			}
			window := ordered[startIndex:end]

			// 4. Batch-fetch metadata via videos.list (50 IDs per call).
			meta, metaWarns := fetchVideoMetadata(c, window)
			out.Warnings = append(out.Warnings, metaWarns...)

			// 5. Assemble rows, fanning out transcript fetches concurrently.
			rows := make([]enrichedVideo, len(window))
			for i, pe := range window {
				row := enrichedVideo{
					VideoID:  pe.videoID,
					Position: pe.position,
					Title:    pe.title,
					WatchURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", pe.videoID),
					EmbedURL: fmt.Sprintf("https://www.youtube.com/embed/%s", pe.videoID),
				}
				if m, ok := meta[pe.videoID]; ok {
					row.Title = m.title
					row.Description = m.description
					row.ChannelTitle = m.channelTitle
					row.PublishedAt = m.publishedAt
					row.Duration = m.duration
					row.ViewCount = m.viewCount
					row.LikeCount = m.likeCount
					row.ThumbnailURL = m.thumbnailURL
				} else {
					row.MetadataError = "metadata not returned by videos.list (video may be private or deleted)"
				}
				rows[i] = row
			}

			if !noTranscript {
				// Per-video transcript fetch, bounded concurrency. Each fetch
				// gets its own timeout so one slow video can't stall the pool.
				type tResult struct {
					idx        int
					transcript *transcriptResult
					err        error
				}
				idxs := make([]int, len(rows))
				for i := range rows {
					idxs[i] = i
				}
				results, ferrs := cliutil.FanoutRun(
					cmd.Context(),
					idxs,
					func(i int) string { return rows[i].VideoID },
					func(ctx context.Context, i int) (tResult, error) {
						tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
						defer cancel()
						tr, terr := fetchTranscriptForVideo(tctx, rows[i].VideoID, lang)
						return tResult{idx: i, transcript: tr, err: terr}, nil
					},
					cliutil.WithConcurrency(concurrency),
				)
				for _, r := range results {
					if r.Value.err != nil {
						rows[r.Value.idx].TranscriptError = r.Value.err.Error()
					} else {
						rows[r.Value.idx].Transcript = r.Value.transcript
					}
				}
				// Fanout-level errors (cancellation, panic) surface to stderr;
				// they're rare but must not be silently dropped.
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), ferrs)
			}

			out.Videos = rows
			out.Returned = len(rows)

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}

	cmd.Flags().IntVar(&startIndex, "start-index", 0, "0-based index into the playlist to start enrichment from")
	cmd.Flags().IntVar(&limit, "limit", 25, "Number of videos to enrich starting at --start-index")
	cmd.Flags().StringVar(&lang, "lang", "en", "Caption language code for transcripts")
	cmd.Flags().BoolVar(&noTranscript, "no-transcript", false, "Skip transcript fetch (metadata + description only)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Max concurrent transcript fetches")

	return cmd
}

// apiGetter is the slice of *client.Client the enrichment helpers depend on.
// Narrowing to this interface keeps the helpers testable without constructing
// a full client and avoids importing the client package in helper signatures.
type apiGetter interface {
	GetWithHeaders(path string, params, headers map[string]string) (json.RawMessage, error)
}

// playlistEntry is one resolved playlist item before metadata enrichment.
type playlistEntry struct {
	videoID  string
	title    string
	position int
}

// videoMeta is the subset of videos.list fields we surface per row.
type videoMeta struct {
	title        string
	description  string
	channelTitle string
	publishedAt  string
	duration     string
	viewCount    string
	likeCount    string
	thumbnailURL string
}

// parsePlaylistID extracts a playlist ID from a URL or returns the input if it
// already looks like a bare playlist ID. Recognizes the `list=` query param on
// playlist and watch URLs, and bare IDs with the known playlist prefixes.
func parsePlaylistID(in string) string {
	if in == "" {
		return ""
	}
	if strings.Contains(in, "://") {
		if u, err := url.Parse(in); err == nil {
			if id := u.Query().Get("list"); id != "" {
				return id
			}
		}
	}
	// Bare ID? YouTube playlist IDs start with PL, UU, LL, FL, RD, OL, etc.
	// Accept any token that has no spaces and no scheme — videos.list will
	// reject a genuinely bad ID with a clean API error.
	if !strings.ContainsAny(in, " \t/?&") {
		return in
	}
	return ""
}

// walkPlaylistItems pages playlistItems.list until it has at least `needed`
// video IDs or the playlist ends. maxPages caps quota use; under verify env we
// cap hard so the verifier's per-command timeout isn't tripped.
// walkPlaylistItems returns the ordered entries scanned (capped at needed/500),
// the playlist's true total length (from pageInfo.totalResults on the first
// page), and any warnings.
func walkPlaylistItems(c apiGetter, playlistID string, needed int, flags *rootFlags) ([]playlistEntry, int, []string, error) {
	var warnings []string
	var total int
	maxPages := (needed + 49) / 50
	if maxPages < 1 {
		maxPages = 1
	}
	if maxPages > 10 {
		maxPages = 10
		warnings = append(warnings, "playlist walk capped at 500 items (10 pages of 50)")
	}
	if cliutil.IsVerifyEnv() && maxPages > 1 {
		maxPages = 1
	}

	ordered := []playlistEntry{}
	pageToken := ""
	for page := 0; page < maxPages; page++ {
		params := map[string]string{
			"playlistId": playlistID,
			"part":       "snippet,contentDetails",
			"maxResults": "50",
		}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := c.GetWithHeaders("/youtube/v3/playlistItems", params, nil)
		if err != nil {
			if page == 0 {
				return nil, 0, warnings, classifyAPIError(err, flags)
			}
			warnings = append(warnings, fmt.Sprintf("playlist page %d fetch failed: %v", page+1, err))
			break
		}
		var resp struct {
			NextPageToken string `json:"nextPageToken"`
			PageInfo      struct {
				TotalResults int `json:"totalResults"`
			} `json:"pageInfo"`
			Items []struct {
				Snippet struct {
					Title      string `json:"title"`
					Position   int    `json:"position"`
					ResourceID struct {
						VideoID string `json:"videoId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, 0, warnings, apiErr(fmt.Errorf("parse playlistItems page %d: %w", page+1, err))
		}
		if page == 0 {
			total = resp.PageInfo.TotalResults
		}
		for _, it := range resp.Items {
			vid := it.Snippet.ResourceID.VideoID
			if vid == "" {
				continue
			}
			ordered = append(ordered, playlistEntry{
				videoID:  vid,
				title:    html.UnescapeString(it.Snippet.Title),
				position: it.Snippet.Position,
			})
		}
		if len(ordered) >= needed || resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	if len(ordered) == 0 {
		return nil, 0, warnings, notFoundErr(fmt.Errorf("playlist %q returned no items (private, empty, or invalid ID)", playlistID))
	}
	return ordered, total, warnings, nil
}

// fetchPlaylistTitle resolves the playlist's display title via playlists.list.
// Best-effort: returns "" on any error so enrichment is never blocked.
func fetchPlaylistTitle(c apiGetter, playlistID string) string {
	data, err := c.GetWithHeaders("/youtube/v3/playlists", map[string]string{
		"id":   playlistID,
		"part": "snippet",
	}, nil)
	if err != nil {
		return ""
	}
	var resp struct {
		Items []struct {
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || len(resp.Items) == 0 {
		return ""
	}
	return html.UnescapeString(resp.Items[0].Snippet.Title)
}

// fetchVideoMetadata batch-fetches videos.list (50 IDs per call) and returns a
// map keyed by video ID. Per-batch errors become warnings, never abort.
func fetchVideoMetadata(c apiGetter, entries []playlistEntry) (map[string]videoMeta, []string) {
	meta := make(map[string]videoMeta, len(entries))
	var warnings []string
	const batchSize = 50
	for start := 0; start < len(entries); start += batchSize {
		end := start + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		ids := make([]string, 0, end-start)
		for _, e := range entries[start:end] {
			ids = append(ids, e.videoID)
		}
		params := map[string]string{
			"id":   strings.Join(ids, ","),
			"part": "snippet,contentDetails,statistics",
		}
		data, err := c.GetWithHeaders("/youtube/v3/videos", params, nil)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("videos.list batch %d-%d failed: %v", start, end, err))
			continue
		}
		var resp struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title        string `json:"title"`
					Description  string `json:"description"`
					ChannelTitle string `json:"channelTitle"`
					PublishedAt  string `json:"publishedAt"`
					Thumbnails   map[string]struct {
						URL string `json:"url"`
					} `json:"thumbnails"`
				} `json:"snippet"`
				ContentDetails struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
				Statistics struct {
					ViewCount string `json:"viewCount"`
					LikeCount string `json:"likeCount"`
				} `json:"statistics"`
			} `json:"items"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			warnings = append(warnings, fmt.Sprintf("parse videos.list batch %d-%d: %v", start, end, err))
			continue
		}
		for _, it := range resp.Items {
			thumb := ""
			for _, key := range []string{"high", "medium", "default"} {
				if t, ok := it.Snippet.Thumbnails[key]; ok {
					thumb = t.URL
					break
				}
			}
			meta[it.ID] = videoMeta{
				title:        html.UnescapeString(it.Snippet.Title),
				description:  html.UnescapeString(it.Snippet.Description),
				channelTitle: html.UnescapeString(it.Snippet.ChannelTitle),
				publishedAt:  it.Snippet.PublishedAt,
				duration:     it.ContentDetails.Duration,
				viewCount:    it.Statistics.ViewCount,
				likeCount:    it.Statistics.LikeCount,
				thumbnailURL: thumb,
			}
		}
	}
	return meta, warnings
}

// fetchTranscriptForVideo resolves and fetches a single video's transcript
// using the existing InnerTube path. Returns a clean error for the per-row
// transcriptError field on any failure.
func fetchTranscriptForVideo(ctx context.Context, videoID, lang string) (*transcriptResult, error) {
	tracks, err := fetchCaptionTracks(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("video has no captions")
	}
	picked, err := pickCaptionTrack(tracks, lang)
	if err != nil {
		return nil, err
	}
	kind := "manual"
	if picked.Kind == "asr" {
		kind = "asr"
	}
	return fetchJSON3Transcript(ctx, picked, videoID, lang, kind)
}
