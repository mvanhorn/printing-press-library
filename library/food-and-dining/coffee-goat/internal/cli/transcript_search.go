// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// transcriptHit is one excerpt from a YouTube review.
type transcriptHit struct {
	VideoID         string `json:"video_id"`
	Creator         string `json:"creator"`
	VideoTitle      string `json:"video_title"`
	PublishedAt     string `json:"published_at,omitempty"`
	URL             string `json:"url"`
	Excerpt         string `json:"excerpt"`
	ApproxStartMs   int    `json:"approx_start_ms,omitempty"`
	ApproxStartTime string `json:"approx_start,omitempty"`
}

func newTranscriptSearchCmd(flags *rootFlags) *cobra.Command {
	var creator string
	var limit int
	var excerptChars int
	cmd := &cobra.Command{
		Use:   "transcript-search <query>",
		Short: "FTS5 search over Hoffmann + Hedrick transcripts. Returns matching excerpts with video URL and approximate timestamp",
		Long: `Searches the local youtube_reviews_fts virtual table populated by 'sync'.
Matches are ranked by FTS5 BM25; each row returns the first containing
excerpt with an approximate start position (character offset converted
to seconds at an average words-per-minute rate, since transcripts are
stored as flat text not timestamped lines).`,
		Example: `  coffee-goat-pp-cli transcript-search "puck prep" --creator hedrick --agent
  coffee-goat-pp-cli transcript-search "espresso WDT" --limit 5`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("transcript-search requires a query"))
			}
			if creator != "" && creator != "hoffmann" && creator != "hedrick" {
				return usageErr(fmt.Errorf("--creator must be hoffmann or hedrick (got %q)", creator))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			hits, err := runTranscriptSearch(db, query, creator, limit, excerptChars)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no transcript matches (run 'sync --source youtube' first)")
				return nil
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s @ %s\n    %s\n    %s\n",
					h.Creator, h.VideoTitle, h.ApproxStartTime, h.Excerpt, h.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&creator, "creator", "", "Restrict to one creator (hoffmann or hedrick)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max excerpts to return")
	cmd.Flags().IntVar(&excerptChars, "excerpt-chars", 240, "Excerpt window length around the match in characters")
	return cmd
}

func runTranscriptSearch(db *store.Store, query, creator string, limit, excerptChars int) ([]transcriptHit, error) {
	if limit <= 0 {
		limit = 10
	}
	if excerptChars <= 0 {
		excerptChars = 240
	}
	q := `SELECT yr.video_id, COALESCE(yr.creator,''), COALESCE(yr.video_title,''),
	             COALESCE(yr.video_published_at,''), COALESCE(yr.transcript_text,'')
	      FROM youtube_reviews yr
	      JOIN youtube_reviews_fts ON youtube_reviews_fts.rowid = yr.rowid
	      WHERE youtube_reviews_fts MATCH ?`
	args := []any{query}
	if creator != "" {
		q += ` AND LOWER(yr.creator) = ?`
		args = append(args, strings.ToLower(creator))
	}
	q += ` ORDER BY rank LIMIT ?`
	args = append(args, limit)
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("transcript search: %w", err)
	}
	defer rows.Close()
	needles := lowerWords(query)
	var out []transcriptHit
	for rows.Next() {
		var videoID, creatorRow, title, publishedAt, transcript string
		if err := rows.Scan(&videoID, &creatorRow, &title, &publishedAt, &transcript); err != nil {
			return nil, err
		}
		excerpt, startChar := bestExcerpt(transcript, needles, excerptChars)
		approxMs := approxMillisecondsForCharOffset(transcript, startChar)
		out = append(out, transcriptHit{
			VideoID:         videoID,
			Creator:         creatorRow,
			VideoTitle:      title,
			PublishedAt:     publishedAt,
			URL:             youtubeURL(videoID, approxMs),
			Excerpt:         excerpt,
			ApproxStartMs:   approxMs,
			ApproxStartTime: formatHMS(approxMs),
		})
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return out, nil
}

func lowerWords(q string) []string {
	parts := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == '"'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 {
			out = append(out, p)
		}
	}
	return out
}

// bestExcerpt returns a window of about width chars around the first
// query token match in the transcript. Falls back to the start of the
// transcript when no token matches (rare since FTS already matched the
// row).
func bestExcerpt(text string, needles []string, width int) (string, int) {
	lower := strings.ToLower(text)
	bestIdx := -1
	for _, n := range needles {
		if i := strings.Index(lower, n); i >= 0 {
			if bestIdx < 0 || i < bestIdx {
				bestIdx = i
			}
		}
	}
	if bestIdx < 0 {
		end := width
		if end > len(text) {
			end = len(text)
		}
		return strings.TrimSpace(text[:end]), 0
	}
	start := bestIdx - width/2
	if start < 0 {
		start = 0
	}
	end := start + width
	if end > len(text) {
		end = len(text)
		if end-width > 0 {
			start = end - width
		}
	}
	excerpt := strings.TrimSpace(text[start:end])
	if start > 0 {
		excerpt = "…" + excerpt
	}
	if end < len(text) {
		excerpt += "…"
	}
	return excerpt, bestIdx
}

// approxMillisecondsForCharOffset estimates a video timestamp given a
// character offset into the transcript. Uses an average speaking rate
// of ~150 words per minute and ~5.5 characters per word — these
// constants land excerpts in the right minute of typical Hoffmann /
// Hedrick narration, which is the granularity the URL fragment
// supports anyway.
func approxMillisecondsForCharOffset(text string, charOffset int) int {
	if charOffset <= 0 || len(text) == 0 {
		return 0
	}
	const charsPerWord = 5.5
	const wordsPerMinute = 150.0
	words := float64(charOffset) / charsPerWord
	minutes := words / wordsPerMinute
	return int(minutes * 60 * 1000)
}

func youtubeURL(videoID string, approxMs int) string {
	base := "https://www.youtube.com/watch?v=" + videoID
	if approxMs > 0 {
		base += fmt.Sprintf("&t=%ds", approxMs/1000)
	}
	return base
}

func formatHMS(ms int) string {
	if ms <= 0 {
		return "00:00"
	}
	totalSec := ms / 1000
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
