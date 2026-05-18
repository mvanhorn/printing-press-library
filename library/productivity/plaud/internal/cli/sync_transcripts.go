// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

// `sync-transcripts` — walks the local recordings_typed table and fetches
// the speaker-diarized transcript + AI summary for each recording that
// doesn't yet have transcripts in the store. Uses /ai/transsumm/{id} as
// the primary path, with an S3 fallback through /file/detail/{id} for
// pre-March-2026 recordings.
//
// This is the command that populates the substrate the transcendence
// commands (commitments, topic, about, forgotten, themes, cross-meeting,
// silence, mentioned-me) read from. Without it, those queries return
// empty results — the typed transcripts table is the source of truth.

package cli

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newSyncTranscriptsCmd(flags *rootFlags) *cobra.Command {
	var flagAll bool
	var flagIDs, flagSince string
	var flagLimit int
	var flagFilenameContains string

	cmd := &cobra.Command{
		Use:   "sync-transcripts",
		Short: "Fetch transcripts + summaries for recordings in the local store",
		Long: "Walks the local recordings_typed table and fetches transcripts +\n" +
			"AI summaries for each recording missing them. Uses /ai/transsumm/{id}\n" +
			"with an S3 fallback for older recordings.\n\n" +
			"Run after `plaud-pp-cli sync` populates the recordings list. The\n" +
			"transcendence commands (commitments, topic, about, etc.) read from\n" +
			"the typed transcripts + summaries tables populated by this command.",
		Example: `  plaud-pp-cli sync-transcripts --all
  plaud-pp-cli sync-transcripts --ids abc123,def456
  plaud-pp-cli sync-transcripts --since 30d --limit 50`,
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			s, err := openPlaudStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			ids, err := pickRecordingIDsForEnrich(cmd.Context(), s, flagIDs, flagAll, flagSince, flagFilenameContains, flagLimit)
			if err != nil {
				return apiErr(fmt.Errorf("picking recordings to enrich: %w", err))
			}
			if len(ids) == 0 {
				if !flags.quiet {
					fmt.Fprintln(cmd.ErrOrStderr(), "No recordings to enrich. Run `plaud-pp-cli sync` first to populate recordings_typed.")
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"enriched": 0,
					"skipped":  0,
					"failed":   0,
				}, flags)
			}

			enriched, skipped, failed := 0, 0, 0
			failures := []map[string]any{}
			for i, id := range ids {
				if !flags.quiet && i%5 == 0 && i > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "  ... %d/%d (%d enriched, %d failed)\n", i, len(ids), enriched, failed)
				}
				ok, ferr := enrichOneTranscript(cmd.Context(), c, s, id)
				if ferr != nil {
					failed++
					failures = append(failures, map[string]any{"id": id, "error": ferr.Error()})
					continue
				}
				if ok {
					enriched++
				} else {
					skipped++
				}
			}

			// Refresh speaker aggregation now that new transcript rows exist.
			if err := s.RefreshSpeakerAgg(); err != nil {
				return apiErr(fmt.Errorf("refreshing speakers: %w", err))
			}

			out := map[string]any{
				"enriched": enriched,
				"skipped":  skipped,
				"failed":   failed,
				"total":    len(ids),
				"failures": failures,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&flagAll, "all", false, "Enrich every recording in the store that lacks a transcript")
	cmd.Flags().StringVar(&flagIDs, "ids", "", "Comma-separated recording IDs to enrich (overrides --all/--since)")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Only recordings whose start_time falls in this window (default 30d)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max recordings to enrich per run")
	cmd.Flags().StringVar(&flagFilenameContains, "name-contains", "", "Filter to recordings whose filename contains this substring")
	return cmd
}

// pickRecordingIDsForEnrich resolves the set of recording IDs based on the
// flag combination. Precedence: --ids > --all > default (since-windowed).
func pickRecordingIDsForEnrich(ctx context.Context, s *store.Store, idsFlag string, all bool, sinceFlag, nameContains string, limit int) ([]string, error) {
	if idsFlag != "" {
		raw := strings.Split(idsFlag, ",")
		out := make([]string, 0, len(raw))
		for _, r := range raw {
			r = strings.TrimSpace(r)
			if r != "" {
				out = append(out, r)
			}
		}
		return out, nil
	}

	query := `
		SELECT r.id FROM recordings_typed r
		LEFT JOIN transcripts t ON t.recording_id = r.id
		WHERE r.is_trash = 0 AND t.recording_id IS NULL
	`
	args := []any{}
	if !all {
		since, err := parseSinceFlag(sinceFlag)
		if err != nil {
			return nil, err
		}
		if since > 0 {
			query += " AND r.start_time >= ?"
			args = append(args, since)
		}
	}
	if nameContains != "" {
		query += " AND r.filename LIKE ?"
		args = append(args, "%"+nameContains+"%")
	}
	query += " ORDER BY r.start_time DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// enrichOneTranscript fetches transcript + summary for a single recording.
// Returns (true, nil) when transcripts were written, (false, nil) when the
// recording is genuinely empty (no transcript content available, e.g., very
// short memo), or (false, err) on failure.
type clientLike interface {
	Post(path string, body any) (json.RawMessage, int, error)
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func enrichOneTranscript(ctx context.Context, c clientLike, s *store.Store, id string) (bool, error) {
	// Primary path: /ai/transsumm/{id}
	body, _, err := c.Post(fmt.Sprintf("/ai/transsumm/%s", id), map[string]any{})
	if err != nil {
		return false, fmt.Errorf("POST /ai/transsumm/%s: %w", id, err)
	}

	var envelope struct {
		Status        int                            `json:"status"`
		Msg           string                         `json:"msg"`
		DataResult    []store.PlaudTranscriptSegment `json:"data_result"`
		DataResultSum json.RawMessage                `json:"data_result_summ"`
	}
	if jerr := json.Unmarshal(body, &envelope); jerr != nil {
		return false, fmt.Errorf("parsing transsumm response: %w", jerr)
	}

	// status -12 means recording predates the new transcript path; fall back
	// to /file/detail/{id} → content_list[] → S3.
	if envelope.Status == -12 || (envelope.DataResult == nil && len(envelope.DataResultSum) == 0) {
		return enrichViaS3Fallback(ctx, c, s, id)
	}

	if envelope.Status != 0 {
		msg := envelope.Msg
		if msg == "" {
			msg = fmt.Sprintf("status %d", envelope.Status)
		}
		return false, fmt.Errorf("transsumm failed: %s", msg)
	}

	if len(envelope.DataResult) > 0 {
		if err := s.UpsertTranscriptSegments(id, envelope.DataResult); err != nil {
			return false, fmt.Errorf("storing transcripts: %w", err)
		}
	}

	if len(envelope.DataResultSum) > 0 {
		summary := store.NormalizeSummaryShape(envelope.DataResultSum)
		if err := s.UpsertSummary(id, summary, envelope.DataResultSum); err != nil {
			return false, fmt.Errorf("storing summary: %w", err)
		}
	}

	return true, nil
}

// enrichViaS3Fallback handles pre-March-2026 recordings whose transcript
// lives in S3 rather than the /ai/transsumm endpoint. Reads content_list[]
// from /file/detail/{id} and GETs the unauthenticated S3 data_link URLs.
func enrichViaS3Fallback(ctx context.Context, c clientLike, s *store.Store, id string) (bool, error) {
	detail, err := c.Get(fmt.Sprintf("/file/detail/%s", id), nil)
	if err != nil {
		return false, fmt.Errorf("GET /file/detail/%s: %w", id, err)
	}

	var dEnvelope struct {
		Data struct {
			ContentList []struct {
				DataType string `json:"data_type"`
				DataLink string `json:"data_link"`
			} `json:"content_list"`
		} `json:"data"`
	}
	if jerr := json.Unmarshal(detail, &dEnvelope); jerr != nil {
		return false, fmt.Errorf("parsing detail: %w", jerr)
	}

	wroteAny := false
	for _, item := range dEnvelope.Data.ContentList {
		switch item.DataType {
		case "transaction", "transaction_polish":
			body, err := fetchS3Body(ctx, item.DataLink)
			if err != nil {
				continue
			}
			segments, err := parseTranscriptFromS3(body)
			if err != nil || len(segments) == 0 {
				continue
			}
			if err := s.UpsertTranscriptSegments(id, segments); err == nil {
				wroteAny = true
			}
		case "auto_sum_note":
			body, err := fetchS3Body(ctx, item.DataLink)
			if err != nil {
				continue
			}
			// S3 summary is typically markdown wrapped in JSON.
			summary := store.PlaudSummary{Markdown: string(body)}
			if err := s.UpsertSummary(id, summary, body); err == nil {
				wroteAny = true
			}
		}
	}
	return wroteAny, nil
}

// fetchS3Body GETs an unauthenticated S3 URL. Tries gunzip first, falls back
// to the raw body when the bytes aren't a valid gzip stream.
func fetchS3Body(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("empty S3 URL")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Try gunzip
	if len(raw) >= 2 && raw[0] == 0x1F && raw[1] == 0x8B {
		gz, err := gzip.NewReader(strings.NewReader(string(raw)))
		if err == nil {
			defer gz.Close()
			out, err := io.ReadAll(gz)
			if err == nil {
				return out, nil
			}
		}
	}
	return raw, nil
}

// parseTranscriptFromS3 tries the common shapes Plaud writes to S3:
//   - JSON array of {start_time, end_time, content, speaker, original_speaker}
//   - JSON object { data: [...] } wrapping the array
//   - Plain text fallback (split on newlines into one-segment-per-line)
func parseTranscriptFromS3(body []byte) ([]store.PlaudTranscriptSegment, error) {
	// Direct array
	var direct []store.PlaudTranscriptSegment
	if err := json.Unmarshal(body, &direct); err == nil && len(direct) > 0 {
		return direct, nil
	}
	// Wrapped: { data: [...] }
	var wrap struct {
		Data []store.PlaudTranscriptSegment `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err == nil && len(wrap.Data) > 0 {
		return wrap.Data, nil
	}
	// Plain text fallback
	txt := string(body)
	if strings.TrimSpace(txt) == "" {
		return nil, fmt.Errorf("empty body")
	}
	lines := strings.Split(txt, "\n")
	out := make([]store.PlaudTranscriptSegment, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, store.PlaudTranscriptSegment{Content: line})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no segments parseable")
	}
	return out, nil
}
