// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
// transcript: fetch a video's inline WEBVTT captions as clean text and cache
// cue-level transcripts in the local store. `transcript sync` bulk-builds the
// corpus that `concordance` searches. Hand-built novel feature.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/client"
	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"

	"github.com/spf13/cobra"
)

// videoDetail is the subset of GET /video?id= we consume.
type videoDetail struct {
	ID        string   `json:"_id"`
	Title     string   `json:"title"`
	Subtitles string   `json:"subtitles"`
	Duration  int64    `json:"duration"`
	Guides    []string `json:"guides"`
	Tags      []string `json:"tags"`
	Sources   struct {
		Bunny   string `json:"bunny"`
		Youtube string `json:"youtube"`
	} `json:"sources"`
}

// fetchVideoDetail GETs /video?id= and unwraps the {video:{...}} envelope,
// falling back to the root object (yt-dlp-dreaming observed both shapes).
func fetchVideoDetail(ctx context.Context, c *client.Client, id string) (videoDetail, error) {
	raw, err := c.Get(ctx, "/video", map[string]string{"id": id})
	if err != nil {
		return videoDetail{}, err
	}
	var env struct {
		Video json.RawMessage `json:"video"`
	}
	var vd videoDetail
	if json.Unmarshal(raw, &env) == nil && len(env.Video) > 0 {
		if err := json.Unmarshal(env.Video, &vd); err == nil && (vd.Subtitles != "" || vd.Title != "" || vd.Sources.Bunny != "") {
			return vd, nil
		}
	}
	if err := json.Unmarshal(raw, &vd); err != nil {
		return videoDetail{}, fmt.Errorf("parsing video response: %w", err)
	}
	return vd, nil
}

func newNovelTranscriptCmd(flags *rootFlags) *cobra.Command {
	var dbOverride string
	cmd := &cobra.Command{
		Use:   "transcript <video-id>",
		Short: "Fetch a video's captions as clean, timestamp-free plain text (and cache the cue-level transcript).",
		Long: "Fetch a video's captions as clean, timestamp-free plain text and cache the\n" +
			"cue-level transcript in the local store. Run 'transcript sync' to bulk-build\n" +
			"the corpus that 'concordance' searches.",
		Example: strings.Trim(`
  dreaming-pp-cli transcript 5f3a1b2c4d5e6f7a8b9c0d1e
  dreaming-pp-cli transcript 5f3a1b2c4d5e6f7a8b9c0d1e --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vd, err := fetchVideoDetail(cmd.Context(), c, id)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if vd.Subtitles == "" {
				return notFoundErr(fmt.Errorf("video %s has no inline captions", id))
			}
			cues := parseVTT(vd.Subtitles)

			// Cache cue-level transcript for offline concordance search.
			if db, derr := openDreamingStoreAt(cmd.Context(), dbOverride); derr == nil {
				defer db.Close()
				stored := make([]store.TranscriptCue, 0, len(cues))
				for i, cue := range cues {
					stored = append(stored, store.TranscriptCue{VideoID: id, Index: i, StartMS: cue.StartMS, EndMS: cue.EndMS, Text: cue.Text})
				}
				_ = db.ReplaceTranscript(id, stored)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				out := map[string]any{
					"video_id": id,
					"title":    vd.Title,
					"cues":     len(cues),
					"text":     vttToPlainText(vd.Subtitles),
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), vttToPlainText(vd.Subtitles))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbOverride, "db", "", "Database path (default: ~/.local/share/dreaming-pp-cli/data.db)")
	cmd.AddCommand(newTranscriptSyncCmd(flags))
	cmd.AddCommand(newTranscriptExportCmd(flags))
	cmd.AddCommand(newTranscriptImportCmd(flags))
	return cmd
}

func newTranscriptSyncCmd(flags *rootFlags) *cobra.Command {
	var level, guide string
	var all bool
	var limit int
	var fromURL string
	var refresh bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Bulk-fetch and cache transcripts to build the searchable corpus",
		Long: "Fetch and cache cue-level transcripts for catalog videos so 'concordance'\n" +
			"can search them offline. Scope with --level/--guide; --all removes the cap.\n" +
			"Run the top-level 'sync' first to populate the video catalog.\n\n" +
			"Pass --from-url to import a shared corpus first (see 'transcript export');\n" +
			"only the transcripts that corpus is still missing are then fetched from\n" +
			"Dreaming, so a complete shared corpus turns a full sync into one download.",
		Example: strings.Trim(`
  dreaming-pp-cli transcript sync --level beginner
  dreaming-pp-cli transcript sync --guide "Agustina" --limit 50
  dreaming-pp-cli transcript sync --all
  dreaming-pp-cli transcript sync --all --from-url https://example.com/corpus.json.gz
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openDreamingStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			// Pull-merge first: import a shared corpus, then only fetch the gaps.
			if fromURL != "" {
				if imported, ierr := importCorpusInto(cmd.Context(), db, fromURL); ierr != nil {
					return ierr
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "Imported shared corpus from %s (%d transcripts added/updated); fetching only the gaps.\n", fromURL, imported)
				}
			}

			ids, err := candidateVideoIDs(cmd.Context(), db, level, guide)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return notFoundErr(fmt.Errorf("no videos in local store match that scope — run 'dreaming-pp-cli sync' first to populate the catalog"))
			}
			// Incremental by default: drop videos whose transcript is already
			// cached (e.g. just imported via --from-url) so the fetch targets only
			// the gaps, and so repeated --limit runs make forward progress instead
			// of re-fetching the same prefix. --refresh re-fetches everything.
			alreadyCached := 0
			if !refresh {
				fresh := ids[:0]
				for _, id := range ids {
					if db.HasTranscript(id) {
						alreadyCached++
						continue
					}
					fresh = append(fresh, id)
				}
				ids = fresh
			}
			if len(ids) == 0 {
				vids, cueCount, _ := db.TranscriptStats()
				out := map[string]any{"synced": 0, "already_cached": alreadyCached, "skipped_no_captions": 0, "failed": 0, "corpus_videos": vids, "corpus_cues": cueCount}
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Nothing to fetch — all %d matching videos already cached. Corpus: %d videos, %d cues.\n", alreadyCached, vids, cueCount)
				return nil
			}
			if !all && limit > 0 && len(ids) > limit {
				fmt.Fprintf(cmd.ErrOrStderr(), "Limiting to %d of %d uncached matching videos (use --all to sync every match).\n", limit, len(ids))
				ids = ids[:limit]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			total := len(ids)
			progress := !flags.asJSON && !flags.quiet
			if progress {
				fmt.Fprintf(cmd.ErrOrStderr(), "Fetching %d transcripts from Dreaming (one request each)...\n", total)
			}

			limiter := cliutil.NewAdaptiveLimiter(flags.rateLimit)
			synced, skipped, failed := 0, 0, 0
			var firstErr error
			for i, id := range ids {
				limiter.Wait()
				vd, ferr := fetchVideoDetail(cmd.Context(), c, id)
				if ferr != nil {
					if rl, ok := ferr.(*cliutil.RateLimitError); ok {
						return rateLimitErr(rl)
					}
					if firstErr == nil {
						firstErr = ferr
					}
					// Fail fast on systemic errors instead of grinding through
					// thousands of doomed requests. Auth failures (expired/missing
					// token) never recover mid-run, so abort on the very first one;
					// any other error that kills the first 5 requests with zero
					// successes is also systemic (host moved, network down).
					if isAuthError(ferr) || (synced == 0 && skipped == 0 && failed+1 >= 5) {
						return classifyAPIError(fmt.Errorf("transcript sync aborted after %d failed fetch(es) with no successes: %w", failed+1, firstErr), flags)
					}
					limiter.OnSuccess()
					failed++
					continue
				}
				limiter.OnSuccess()
				if vd.Subtitles == "" {
					skipped++
				} else {
					cues := parseVTT(vd.Subtitles)
					stored := make([]store.TranscriptCue, 0, len(cues))
					for ci, cue := range cues {
						stored = append(stored, store.TranscriptCue{VideoID: id, Index: ci, StartMS: cue.StartMS, EndMS: cue.EndMS, Text: cue.Text})
					}
					if err := db.ReplaceTranscript(id, stored); err != nil {
						if firstErr == nil {
							firstErr = err
						}
						failed++
					} else {
						synced++
					}
				}
				// Live progress every 10 processed (and on the final item): show
				// running counts so a long sync is visibly making progress.
				if progress {
					done := i + 1
					if done%10 == 0 || done == total {
						fmt.Fprintf(cmd.ErrOrStderr(), "  %d/%d  (synced %d, no-captions %d, failed %d)\n", done, total, synced, skipped, failed)
					}
				}
			}
			vids, cueCount, _ := db.TranscriptStats()
			out := map[string]any{"synced": synced, "already_cached": alreadyCached, "skipped_no_captions": skipped, "failed": failed, "corpus_videos": vids, "corpus_cues": cueCount}
			// Surface why fetches failed so a partial/empty sync isn't a black box.
			if failed > 0 && firstErr != nil {
				out["first_error"] = cliutil.SanitizeErrorBody(firstErr.Error())
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d transcripts (%d already cached, %d skipped, %d failed). Corpus now: %d videos, %d cues.\n", synced, alreadyCached, skipped, failed, vids, cueCount)
			if failed > 0 && firstErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "First failure: %s\n", cliutil.SanitizeErrorBody(firstErr.Error()))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&level, "level", "", "Only videos at this catalog level (superbeginner, beginner, intermediate, advanced)")
	cmd.Flags().StringVar(&guide, "guide", "", "Only videos by this guide/teacher")
	cmd.Flags().BoolVar(&all, "all", false, "Sync every matching video (ignore --limit)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max videos to sync unless --all is set")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Re-fetch transcripts even when already cached (default: skip cached)")
	cmd.Flags().StringVar(&fromURL, "from-url", "", "Import a shared corpus from this URL first, then fetch only the gaps")
	return cmd
}

// isAuthError reports whether err is a 401/403 from the API — a credential
// problem that will never recover mid-run, so transcript sync should abort
// immediately rather than retry it across thousands of videos.
func isAuthError(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 401 || apiErr.StatusCode == 403
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP 401") || strings.Contains(msg, "HTTP 403")
}

// candidateVideoIDs returns video ids from the local catalog, optionally
// filtered by level and guide.
func candidateVideoIDs(ctx context.Context, db *store.Store, level, guide string) ([]string, error) {
	q := `SELECT id FROM videos WHERE 1=1`
	var args []any
	if level != "" {
		q += " AND level = ?"
		args = append(args, level)
	}
	if guide != "" {
		q += " AND guide = ?"
		args = append(args, guide)
	}
	q += " ORDER BY difficulty"
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
