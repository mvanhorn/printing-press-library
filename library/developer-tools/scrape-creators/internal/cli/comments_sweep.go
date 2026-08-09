// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: budget-gated bulk comment sweep for a handle.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

type sweepEnvelope struct {
	Handle         string         `json:"handle"`
	PostsScanned   int            `json:"posts_scanned"`
	CommentsStored int            `json:"comments_stored"`
	CreditsCharged int64          `json:"credits_charged"`
	MaxCredits     int64          `json:"max_credits"`
	StoppedEarly   bool           `json:"stopped_early"`
	Note           string         `json:"note,omitempty"`
	FetchFailures  []fetchFailure `json:"fetch_failures,omitempty"`
}

func newNovelCommentsSweepCmd(flags *rootFlags) *cobra.Command {
	var since string
	var maxCredits int64
	var maxPosts int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sweep [handle]",
		Short: "Pull recent posts and their comments in one budget-gated command",
		Long: strings.Trim(`
Use this command for bulk multi-post comment pulls with a credit budget: it
walks the handle's recent posts, pulls each post's comments, persists rows to
the local store, and stops cleanly when --max-credits is reached. Do NOT use
it for one post's complete threads; use 'comments thread' instead.`, "\n"),
		Example: strings.Trim(`
  scrape-creators-pp-cli comments sweep bracken.design --since 7d --max-credits 200 --agent
  scrape-creators-pp-cli comments sweep mkbhd --max-posts 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "handle=mock-value"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sweep recent posts and their comments under a credit budget")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a handle is required"))
			}
			handle := args[0]
			var sinceDur time.Duration
			if since != "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				sinceDur = d
			}
			if cliutil.IsDogfoodEnv() && (maxPosts == 0 || maxPosts > 2) {
				maxPosts = 2
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			postsRaw, err := c.Get(ctx, "/v2/instagram/user/posts", map[string]string{"handle": handle, "trim": "true"})
			if err != nil {
				return fmt.Errorf("fetching recent posts: %w", err)
			}
			if isErrorEnvelope(postsRaw) {
				return fmt.Errorf("posts endpoint returned an error envelope for %s", handle)
			}
			env := sweepEnvelope{Handle: handle, MaxCredits: maxCredits}
			env.CreditsCharged += payloadCredits(postsRaw)
			posts := extractPosts(postsRaw)

			cutoff := time.Time{}
			if sinceDur > 0 {
				cutoff = time.Now().Add(-sinceDur)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("scrape-creators-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := store.EnsureCommentCorpus(ctx, db.DB()); err != nil {
				return fmt.Errorf("comment corpus migration: %w", err)
			}

			// The posts feed is paginated via next_max_id (newest-first). Walk
			// pages under the same stops that bound the per-post loop, plus a
			// page safety cap for the doubly-unbounded (--max-credits 0
			// --max-posts 0) case.
			const maxSweepPages = 25
			cursor := ""
			// --max-credits stops BEFORE any fetch whose estimated cost would
			// push the charged total past the budget. The estimate is the
			// maximum per-fetch charge observed in this run (floor 1): request
			// cost is decided server-side and only known after the response,
			// so a run can exceed the budget by at most one fetch's
			// actual-minus-estimated cost. credits_charged always reports the
			// true total.
			var maxFetchCost int64 = 1
			if c0 := payloadCredits(postsRaw); c0 > maxFetchCost {
				maxFetchCost = c0
			}
			for page := 1; ; page++ {
				pastCutoff := false
				for _, p := range posts {
					if maxPosts > 0 && env.PostsScanned >= maxPosts {
						env.StoppedEarly = true
						env.Note = fmt.Sprintf("stopped at --max-posts %d", maxPosts)
						break
					}
					if maxCredits > 0 && env.CreditsCharged+maxFetchCost > maxCredits {
						env.StoppedEarly = true
						env.Note = fmt.Sprintf("stopped at --max-credits %d: the next fetch (est. %d cr) would exceed the budget; rerun with a higher budget to continue", maxCredits, maxFetchCost)
						break
					}
					if !cutoff.IsZero() && !p.takenAt.IsZero() && p.takenAt.Before(cutoff) {
						pastCutoff = true
						continue
					}
					if p.url == "" {
						continue
					}
					env.PostsScanned++
					raw, cerr := c.Get(ctx, "/v2/instagram/post/comments", map[string]string{"url": p.url})
					if cerr != nil {
						env.FetchFailures = append(env.FetchFailures, fetchFailure{Source: p.url, Error: sanitizeFetchErr(cerr)})
						continue
					}
					if isErrorEnvelope(raw) {
						env.FetchFailures = append(env.FetchFailures, fetchFailure{Source: p.url, Error: "error envelope from comments endpoint"})
						continue
					}
					pl := parseCommentsPayload(raw)
					env.CreditsCharged += pl.creditsCharged
					if pl.creditsCharged > maxFetchCost {
						maxFetchCost = pl.creditsCharged
					}
					rowsBatch := make([]store.CommentRow, 0, len(pl.comments))
					for _, cm := range pl.comments {
						rows, _ := commentToRows(cm, p.url, "")
						rowsBatch = append(rowsBatch, rows...)
					}
					if err := store.UpsertCommentRows(ctx, db.DB(), rowsBatch); err != nil {
						return fmt.Errorf("persisting comments for %s: %w", p.url, err)
					}
					reported := p.reportedCount
					if reported == 0 {
						if pl.reportedTotal > 0 {
							reported = pl.reportedTotal
						} else {
							reported = int64(len(rowsBatch))
						}
					}
					if err := store.UpsertPostMeta(ctx, db.DB(), p.url, handle, reported); err != nil {
						return fmt.Errorf("persisting post metadata for %s: %w", p.url, err)
					}
					env.CommentsStored += len(rowsBatch)
				}
				if env.StoppedEarly || pastCutoff {
					break
				}
				next, more := extractPostsCursor(postsRaw)
				if !more || next == cursor {
					break
				}
				if page >= maxSweepPages {
					env.StoppedEarly = true
					env.Note = fmt.Sprintf("stopped at the %d-page posts-feed safety cap; rerun with --since or --max-posts to bound the sweep", maxSweepPages)
					break
				}
				if maxCredits > 0 && env.CreditsCharged+maxFetchCost > maxCredits {
					env.StoppedEarly = true
					env.Note = fmt.Sprintf("stopped at --max-credits %d: the next posts page (est. %d cr) would exceed the budget; rerun with a higher budget to continue", maxCredits, maxFetchCost)
					break
				}
				cursor = next
				postsRaw, err = c.Get(ctx, "/v2/instagram/user/posts", map[string]string{"handle": handle, "trim": "true", "next_max_id": cursor})
				if err != nil {
					return fmt.Errorf("fetching recent posts (page %d): %w", page+1, err)
				}
				if isErrorEnvelope(postsRaw) {
					return fmt.Errorf("posts endpoint returned an error envelope for %s (page %d)", handle, page+1)
				}
				pageCost := payloadCredits(postsRaw)
				env.CreditsCharged += pageCost
				if pageCost > maxFetchCost {
					maxFetchCost = pageCost
				}
				posts = extractPosts(postsRaw)
				if len(posts) == 0 {
					break
				}
			}
			if err := allSourcesFailedErr("comments sweep", env.PostsScanned, env.FetchFailures); err != nil {
				return err
			}
			warnFetchFailures(cmd, "comment", env.FetchFailures)
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only sweep posts newer than this window (e.g. 7d, 24h, 1w)")
	cmd.Flags().Int64Var(&maxCredits, "max-credits", 100, "Credit budget: stop before any fetch estimated to exceed it; overshoot is bounded by one fetch (0 = no budget)")
	cmd.Flags().IntVar(&maxPosts, "max-posts", 0, "Maximum posts to sweep (0 = all returned)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type sweptPost struct {
	url           string
	takenAt       time.Time
	reportedCount int64
}

// extractPosts pulls post URLs (and timestamps when present) from the posts
// endpoint's envelope, tolerating items/posts/data wrappers.
func extractPosts(raw json.RawMessage) []sweptPost {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	var arr []json.RawMessage
	for _, key := range []string{"posts", "items", "data", "medias"} {
		if v, ok := obj[key]; ok {
			if err := json.Unmarshal(v, &arr); err == nil && len(arr) > 0 {
				break
			}
		}
	}
	out := make([]sweptPost, 0, len(arr))
	for _, it := range arr {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(it, &m); err != nil {
			continue
		}
		p := sweptPost{}
		for _, k := range []string{"url", "permalink", "link"} {
			if v, ok := m[k]; ok {
				var s string
				if err := json.Unmarshal(v, &s); err == nil && s != "" {
					p.url = s
					break
				}
			}
		}
		if p.url == "" {
			if v, ok := m["code"]; ok {
				var code string
				if err := json.Unmarshal(v, &code); err == nil && code != "" {
					p.url = "https://www.instagram.com/p/" + code
				}
			}
		}
		if v, ok := m["taken_at"]; ok {
			var ts int64
			if err := json.Unmarshal(v, &ts); err == nil && ts > 0 {
				p.takenAt = time.Unix(ts, 0)
			}
		}
		for _, k := range []string{"comment_count", "comments_count"} {
			if v, ok := m[k]; ok {
				if err := json.Unmarshal(v, &p.reportedCount); err == nil && p.reportedCount > 0 {
					break
				}
			}
		}
		if p.url != "" {
			out = append(out, p)
		}
	}
	return out
}

// extractPostsCursor pulls the next_max_id continuation cursor from a posts
// envelope. more is false when the envelope explicitly reports exhaustion
// (more_available/has_more false) or when no cursor is present.
func extractPostsCursor(raw json.RawMessage) (cursor string, more bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", false
	}
	more = true
	for _, k := range []string{"more_available", "has_more"} {
		if v, ok := obj[k]; ok {
			var b bool
			if json.Unmarshal(v, &b) == nil {
				more = b
				break
			}
		}
	}
	if v, ok := obj["next_max_id"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil && s != "" {
			cursor = s
		} else {
			var n json.Number
			if json.Unmarshal(v, &n) == nil && n.String() != "" && n.String() != "0" {
				cursor = n.String()
			}
		}
	}
	if cursor == "" {
		more = false
	}
	return cursor, more
}

// payloadCredits reads credits_charged from any endpoint envelope.
func payloadCredits(raw json.RawMessage) int64 {
	var obj struct {
		CreditsCharged int64 `json:"credits_charged"`
	}
	_ = json.Unmarshal(raw, &obj)
	return obj.CreditsCharged
}
