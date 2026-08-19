// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: comment intelligence. Implemented body — regeneration preserves this file.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store"

	"github.com/spf13/cobra"
)

type minedComment struct {
	CommentID   string `json:"commentId"`
	VideoID     string `json:"videoId"`
	Author      string `json:"author"`
	Text        string `json:"text"`
	LikeCount   int64  `json:"likeCount"`
	PublishedAt string `json:"publishedAt"`
}

type keywordFreq struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

type commentsMineResult struct {
	Scope         string         `json:"scope"`
	Synced        int            `json:"commentsSynced"`
	TotalInStore  int            `json:"commentsInStore"`
	TopLiked      []minedComment `json:"topLiked"`
	Questions     []minedComment `json:"topQuestions"`
	Keywords      []keywordFreq  `json:"keywordFrequencies"`
	QuotaUnitsEst int            `json:"quotaUnitsEst"`
	Note          string         `json:"note,omitempty"`
	FetchFailures []string       `json:"fetch_failures,omitempty"`
}

var commentStopwords = map[string]struct{}{}

func init() {
	for _, w := range []string{
		"the", "a", "an", "and", "or", "but", "is", "are", "was", "were", "be", "been", "it", "its", "this", "that",
		"i", "you", "he", "she", "we", "they", "me", "my", "your", "his", "her", "our", "their", "of", "to", "in",
		"on", "for", "with", "at", "by", "from", "as", "so", "if", "not", "no", "yes", "do", "does", "did", "have",
		"has", "had", "just", "like", "what", "when", "who", "how", "why", "there", "here", "all", "can", "could",
		"would", "should", "will", "im", "its", "dont", "cant", "thats", "der", "die", "das", "und", "ist", "ich",
		"du", "wir", "sie", "ein", "eine", "nicht", "mit", "von", "auf", "auch", "aber", "was", "wie", "es", "zu",
		"den", "dem", "war", "sind", "very", "really", "video", "youtube",
	} {
		commentStopwords[w] = struct{}{}
	}
}

func newNovelCommentsMineCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxPages, topN int
	var noSync, rotate bool
	cmd := &cobra.Command{
		Use:     "comments-mine [@handle|channelId|videoId]",
		Short:   "Sync comments into a typed full-text-searchable table and report top-liked comments, keyword frequencies, and audience questions.",
		Long:    "Use this command for aggregate comment analysis from the local store.\nDo NOT use it to fetch one video's raw top comments live; use 'youtube videos-comments' instead.",
		Example: "  youtube-pp-cli comments-mine @mkbhd --json\n  youtube-pp-cli comments-mine dQw4w9WgXcQ --max-pages 3 --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "target=@GoogleDevelopers;--max-pages=1",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "comments mine")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a channel (@handle or UC… id) or a video id is required"))
			}
			target := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			pages := clampInt(maxPages, 1, 20)
			if cliutil.IsDogfoodEnv() && pages > 1 {
				pages = 1
			}
			db, path, err := openAnalystStore(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			res := commentsMineResult{Scope: target, TopLiked: make([]minedComment, 0), Questions: make([]minedComment, 0), Keywords: make([]keywordFreq, 0), FetchFailures: make([]string, 0)}
			isVideo := looksLikeVideoID(target)
			channelID, videoID := "", ""

			doSync := !noSync && flags.dataSource != "local"
			if isVideo {
				videoID = target
			} else {
				// Resolve channel: locally first, live only when syncing.
				if id, lerr := resolveLocalChannelID(ctx, db, target); lerr == nil {
					channelID = id
				} else if doSync {
					g, gerr := newRotatingGetter(flags, rotate)
					if gerr != nil {
						return gerr
					}
					ch, rerr := resolveChannelInput(ctx, g, target)
					if rerr != nil {
						return classifyAPIError(cmd.ErrOrStderr(), rerr, flags)
					}
					channelID = ch.ChannelID
				} else {
					return notFoundErr(lerr)
				}
			}

			if doSync {
				g, gerr := newRotatingGetter(flags, rotate)
				if gerr != nil {
					return gerr
				}
				if isVideo {
					n, serr := syncVideoComments(ctx, g, db, videoID, "", pages)
					res.Synced += n
					res.QuotaUnitsEst += pages
					if serr != nil {
						res.FetchFailures = append(res.FetchFailures, videoID+": "+serr.Error())
					}
				} else {
					n, serr := syncChannelComments(ctx, g, db, channelID, pages)
					res.Synced += n
					res.QuotaUnitsEst += pages
					if serr != nil {
						res.FetchFailures = append(res.FetchFailures, channelID+": "+serr.Error())
					}
				}
			}

			// ---- Report (store only) ----
			where, wargs := commentScope(channelID, videoID)
			row := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM yt_comments `+where, wargs...)
			if err := row.Scan(&res.TotalInStore); err != nil {
				return apiErr(err)
			}
			if res.TotalInStore == 0 {
				res.Note = fmt.Sprintf("no comments in the store for %s (databank %s); rerun without --no-sync or check the target", target, path)
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			top, err := queryComments(ctx, db, `SELECT comment_id, video_id, author, text, like_count, published_at FROM yt_comments `+where+` ORDER BY like_count DESC LIMIT ?`, append(wargs, topN)...)
			if err != nil {
				return apiErr(err)
			}
			res.TopLiked = top
			qs, err := queryComments(ctx, db, // The '?' inside the LIKE pattern is a literal question mark (finds question comments), not a bind slot.
				`SELECT comment_id, video_id, author, text, like_count, published_at FROM yt_comments `+where+` AND text LIKE '%?%' ORDER BY like_count DESC LIMIT ?`, append(wargs, topN)...)
			if err != nil {
				return apiErr(err)
			}
			res.Questions = qs
			res.Keywords = keywordFrequencies(ctx, db, where, wargs, 20)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d comments in store for %s (synced %d new, ~%d quota units)\n", res.TotalInStore, target, res.Synced, res.QuotaUnitsEst)
			for i, c := range res.TopLiked {
				fmt.Fprintf(cmd.OutOrStdout(), "%2d. (%d likes) %s\n", i+1, c.LikeCount, truncateStr(c.Text, 100))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "commentThreads pages of 100 to sync (1 quota unit each)")
	cmd.Flags().IntVar(&topN, "top", 10, "Rows per report section")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "Report from the store only; no network")
	cmd.Flags().BoolVar(&rotate, "rotate", false, "Fail over to the next stored API key on quota exhaustion")
	return cmd
}

func commentScope(channelID, videoID string) (string, []any) {
	if videoID != "" {
		return `WHERE video_id = ?`, []any{videoID}
	}
	return `WHERE channel_id = ?`, []any{channelID}
}

func queryComments(ctx context.Context, db *store.Store, q string, args ...any) ([]minedComment, error) {
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := make([]minedComment, 0)
	for rows.Next() {
		var c minedComment
		var author, text, pub sql.NullString
		var vid sql.NullString
		if err := rows.Scan(&c.CommentID, &vid, &author, &text, &c.LikeCount, &pub); err != nil {
			_ = rows.Close()
			return out, err
		}
		c.VideoID, c.Author, c.Text, c.PublishedAt = vid.String, author.String, text.String, pub.String
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return out, err
	}
	_ = rows.Close()
	return out, nil
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}']+`)

func keywordFrequencies(ctx context.Context, db *store.Store, where string, wargs []any, topN int) []keywordFreq {
	rows, err := db.DB().QueryContext(ctx, `SELECT text FROM yt_comments `+where+` LIMIT 5000`, wargs...)
	if err != nil {
		return []keywordFreq{}
	}
	counts := map[string]int{}
	texts := make([]string, 0, 1024)
	for rows.Next() {
		var t sql.NullString
		if err := rows.Scan(&t); err != nil {
			break
		}
		texts = append(texts, t.String)
	}
	_ = rows.Err()
	_ = rows.Close()
	for _, t := range texts {
		for _, w := range wordRe.FindAllString(strings.ToLower(t), -1) {
			if len(w) < 3 {
				continue
			}
			if _, stop := commentStopwords[w]; stop {
				continue
			}
			counts[w]++
		}
	}
	out := make([]keywordFreq, 0, len(counts))
	for w, c := range counts {
		if c > 1 {
			out = append(out, keywordFreq{Word: w, Count: c})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Word < out[j].Word
	})
	if len(out) > topN {
		out = out[:topN]
	}
	return out
}

// syncChannelComments pulls channel-wide comment threads via
// allThreadsRelatedToChannelId (1 unit per page of 100).
func syncChannelComments(ctx context.Context, g *rotatingGetter, db *store.Store, channelID string, pages int) (int, error) {
	total := 0
	pageToken := ""
	for p := 0; p < pages; p++ {
		params := map[string]string{
			"part":                         "snippet",
			"allThreadsRelatedToChannelId": channelID,
			"maxResults":                   "100",
			"order":                        "time",
			"textFormat":                   "plainText",
		}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := g.Get(ctx, "/youtube/v3/commentThreads", params)
		if err != nil {
			return total, err
		}
		n, next, err := storeCommentThreadsPage(ctx, db, data, channelID)
		total += n
		if err != nil {
			return total, err
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	return total, nil
}

// storeCommentThreadsPage parses one commentThreads page and upserts rows.
func storeCommentThreadsPage(ctx context.Context, db *store.Store, data []byte, channelID string) (int, string, error) {
	var resp struct {
		NextPageToken string `json:"nextPageToken"`
		Items         []struct {
			ID      string `json:"id"`
			Snippet struct {
				ChannelID       string `json:"channelId"`
				VideoID         string `json:"videoId"`
				TopLevelComment struct {
					Snippet struct {
						AuthorDisplayName string `json:"authorDisplayName"`
						TextDisplay       string `json:"textDisplay"`
						LikeCount         int64  `json:"likeCount"`
						PublishedAt       string `json:"publishedAt"`
					} `json:"snippet"`
				} `json:"topLevelComment"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, "", fmt.Errorf("parse commentThreads response: %w", err)
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, "", err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO yt_comments
		(comment_id, video_id, channel_id, author, text, like_count, published_at, is_reply)
		VALUES (?,?,?,?,?,?,?,0)
		ON CONFLICT(comment_id) DO UPDATE SET
			video_id=excluded.video_id,
			channel_id=CASE WHEN excluded.channel_id != '' THEN excluded.channel_id ELSE yt_comments.channel_id END,
			author=excluded.author,
			text=excluded.text, like_count=excluded.like_count, published_at=excluded.published_at`)
	if err != nil {
		_ = tx.Rollback()
		return 0, "", err
	}
	n := 0
	for _, it := range resp.Items {
		cs := it.Snippet.TopLevelComment.Snippet
		chID := it.Snippet.ChannelID
		if chID == "" {
			chID = channelID // fallback for responses omitting snippet.channelId
		}
		if _, err := stmt.ExecContext(ctx, it.ID, it.Snippet.VideoID, chID, cs.AuthorDisplayName, cs.TextDisplay, cs.LikeCount, cs.PublishedAt); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return n, "", err
		}
		n++
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		return n, "", err
	}
	return n, resp.NextPageToken, nil
}
