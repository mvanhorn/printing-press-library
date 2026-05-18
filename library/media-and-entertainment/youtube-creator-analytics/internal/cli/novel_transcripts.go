package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/store"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/transcripts"
)

// ---- transcript get ----

func newTranscriptCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transcript",
		Short: "Fetch, sync, and search YouTube video transcripts (scraped, no OAuth)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTranscriptGetCmd(flags))
	cmd.AddCommand(newTranscriptSyncCmd(flags))
	cmd.AddCommand(newTranscriptSearchCmd(flags))
	return cmd
}

func newTranscriptGetCmd(flags *rootFlags) *cobra.Command {
	var lang string
	cmd := &cobra.Command{
		Use:         "get <video-id>",
		Short:       "Fetch the transcript for one video (no auth required)",
		Example:     "  youtube-creator-analytics-pp-cli transcript get dQw4w9WgXcQ --lang en --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			t, err := transcripts.Fetch(ctx, args[0], lang)
			if err != nil {
				return fmt.Errorf("transcript fetch failed for %s: %w", args[0], err)
			}
			return flags.printJSON(cmd, t)
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "", "Preferred caption language code (e.g. en, es)")
	return cmd
}

// ---- transcript sync (bulk scrape + persist to store) ----

func newTranscriptSyncCmd(flags *rootFlags) *cobra.Command {
	var channel, dbPath, lang string
	var limit int
	cmd := &cobra.Command{
		Use:         "sync",
		Short:       "Scrape transcripts for every cached video missing one (resource_type='transcripts')",
		Example:     "  youtube-creator-analytics-pp-cli transcript sync --limit 50 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("youtube-creator-analytics-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()
			vids, err := loadVideos(db, channel, 0)
			if err != nil {
				return err
			}
			done, failed := 0, 0
			for _, v := range vids {
				if limit > 0 && done >= limit {
					break
				}
				if _, err := db.Get("transcripts", v.ID); err == nil {
					continue
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 25*time.Second)
				t, err := transcripts.Fetch(ctx, v.ID, lang)
				cancel()
				if err != nil {
					failed++
					continue
				}
				raw, _ := json.Marshal(t)
				if err := db.Upsert("transcripts", v.ID, raw); err != nil {
					failed++
					continue
				}
				done++
			}
			return flags.printJSON(cmd, map[string]any{
				"synced":     done,
				"failed":     failed,
				"total_seen": len(vids),
				"resource":   "transcripts",
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Limit to one channel ID")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&lang, "lang", "", "Preferred caption language code")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max transcripts to fetch this run (0 = no cap)")
	return cmd
}

// ---- transcript search (FTS5 over scraped transcripts) ----

type transcriptHit struct {
	VideoID  string `json:"video_id"`
	Title    string `json:"title,omitempty"`
	Snippet  string `json:"snippet"`
	Language string `json:"language,omitempty"`
}

func newTranscriptSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search across scraped transcripts (offline)",
		Example:     "  youtube-creator-analytics-pp-cli transcript search 'sueño infantil' --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("youtube-creator-analytics-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()
			q := strings.Join(args, " ")
			hits, err := db.Search(q, limit)
			if err != nil {
				return fmt.Errorf("FTS search: %w", err)
			}
			out := make([]transcriptHit, 0, len(hits))
			for _, raw := range hits {
				var t transcripts.Transcript
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				snippet := t.PlainText()
				if len(snippet) > 240 {
					snippet = snippet[:240] + "…"
				}
				out = append(out, transcriptHit{
					VideoID:  t.VideoID,
					Snippet:  snippet,
					Language: t.Language,
				})
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results")
	return cmd
}
