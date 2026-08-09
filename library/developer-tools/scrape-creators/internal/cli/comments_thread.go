// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cost-aware complete comment threads for one post.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

const includeRepliesFlatCredits = 15

type threadEnvelope struct {
	PostURL        string            `json:"post_url"`
	Route          string            `json:"route"`
	RouteReason    string            `json:"route_reason"`
	TopLevel       int               `json:"top_level_comments"`
	ReportedTotal  int64             `json:"reported_comment_count,omitempty"`
	Replies        int               `json:"replies"`
	CreditsCharged int64             `json:"credits_charged"`
	Truncated      bool              `json:"truncated,omitempty"`
	Note           string            `json:"note,omitempty"`
	Comments       []json.RawMessage `json:"comments"`
	FetchFailures  []fetchFailure    `json:"fetch_failures,omitempty"`
}

func newNovelCommentsThreadCmd(flags *rootFlags) *cobra.Command {
	var route string
	var dbPath string
	var noStore bool

	cmd := &cobra.Command{
		Use:   "thread [post-url]",
		Short: "Fetch one post's complete comment threads with cost-aware routing",
		Long: strings.Trim(`
Use this command to fetch one post's complete comment threads with cost-aware
routing between include_replies (15 credits flat) and per-comment reply calls
(1 credit each). Do NOT use it for bulk multi-post pulls; use 'comments sweep'
instead. Do NOT use it to audit already-synced posts; use 'comments coverage'
instead.`, "\n"),
		Example: strings.Trim(`
  scrape-creators-pp-cli comments thread https://www.instagram.com/reel/C8rKmYvsrck --agent
  scrape-creators-pp-cli comments thread https://www.instagram.com/p/DEF456 --route flat`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "post-url=https://www.instagram.com/reel/C8rKmYvsrck"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch complete comment threads with cost-aware routing")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("post URL is required"))
			}
			postURL := args[0]
			if route != "auto" && route != "flat" && route != "per-comment" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--route must be auto, flat, or per-comment"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var credits int64
			var env commentsPayload
			var truncatedTop bool
			chosen := route
			reason := "forced by --route"
			if route != "flat" {
				// First page of top-level comments decides the route: N
				// per-comment reply calls cost N credits, the flat route costs
				// 15 regardless. A forced --route flat skips this probe so the
				// flat path costs exactly 15 credits.
				raw, err := c.Get(ctx, "/v2/instagram/post/comments", map[string]string{"url": postURL})
				if err != nil {
					return fmt.Errorf("fetching top-level comments: %w", err)
				}
				if isErrorEnvelope(raw) {
					return fmt.Errorf("comments endpoint returned an error envelope for %s", postURL)
				}
				env = parseCommentsPayload(raw)
				credits = env.creditsCharged
				truncatedTop = envelopeHasMore(raw)
				if route == "auto" {
					if len(env.comments) > includeRepliesFlatCredits {
						chosen = "flat"
						reason = fmt.Sprintf("%d top-level comments: include_replies (15 cr flat) beats %d per-comment calls", len(env.comments), len(env.comments))
					} else if len(env.comments) == includeRepliesFlatCredits {
						chosen = "flat"
						reason = "15 top-level comments: per-comment and flat both cost 15 cr; flat wins for completeness"
					} else {
						chosen = "per-comment"
						reason = fmt.Sprintf("%d top-level comments: per-comment replies (<=%d cr) beat the 15 cr flat call", len(env.comments), len(env.comments))
					}
				}
			}

			out := threadEnvelope{PostURL: postURL, Route: chosen, RouteReason: reason, TopLevel: len(env.comments)}
			storeRows := make([]store.CommentRow, 0, len(env.comments)*2)

			switch chosen {
			case "flat":
				fullRaw, err := c.Get(ctx, "/v2/instagram/post/comments", map[string]string{"url": postURL, "include_replies": "true"})
				if err != nil {
					return fmt.Errorf("fetching threads with include_replies: %w", err)
				}
				if isErrorEnvelope(fullRaw) {
					return fmt.Errorf("comments endpoint returned an error envelope for %s", postURL)
				}
				full := parseCommentsPayload(fullRaw)
				credits += full.creditsCharged
				truncatedTop = envelopeHasMore(fullRaw)
				if full.reportedTotal > 0 {
					out.ReportedTotal = full.reportedTotal
				}
				out.TopLevel = len(full.comments)
				out.Comments = full.comments
				for _, cm := range full.comments {
					rows, replyCount := commentToRows(cm, postURL, "")
					storeRows = append(storeRows, rows...)
					out.Replies += replyCount
				}
			case "per-comment":
				if env.reportedTotal > 0 {
					out.ReportedTotal = env.reportedTotal
				}
				out.Comments = env.comments
				attemptedReplies := 0
				for _, cm := range env.comments {
					rows, _ := commentToRows(cm, postURL, "")
					storeRows = append(storeRows, rows...)
					id := commentField(cm, "id")
					if id == "" {
						continue
					}
					attemptedReplies++
					repRaw, rerr := c.Get(ctx, "/v1/instagram/post/comment/replies", map[string]string{"url": postURL, "comment_id": id})
					if rerr != nil {
						out.FetchFailures = append(out.FetchFailures, fetchFailure{Source: id, Error: sanitizeFetchErr(rerr)})
						continue
					}
					rep := parseCommentsPayload(repRaw)
					credits += rep.creditsCharged
					out.Replies += len(rep.comments)
					out.Comments = append(out.Comments, rep.comments...)
					for _, rm := range rep.comments {
						rows, _ := commentToRows(rm, postURL, id)
						storeRows = append(storeRows, rows...)
					}
				}
				if err := allSourcesFailedErr("comments thread replies", attemptedReplies, out.FetchFailures); err != nil {
					return err
				}
			}
			out.CreditsCharged = credits
			if truncatedTop {
				// The comments endpoint reports more pages beyond the one
				// fetched. Say so instead of presenting a partial thread as
				// complete; a cursor traversal needs a credit-budget design
				// first (each further page is a paid call).
				out.Truncated = true
				out.Note = "comments endpoint reports has_more=true: only the first page of top-level comments is included"
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", out.Note)
			}
			warnFetchFailures(cmd, "reply", out.FetchFailures)

			if !noStore {
				if dbPath == "" {
					dbPath = defaultDBPath("scrape-creators-pp-cli")
				}
				if db, derr := store.OpenWithContext(ctx, dbPath); derr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open local store, nothing persisted: %v\n", derr)
				} else {
					if err := store.EnsureCommentCorpus(ctx, db.DB()); err == nil {
						if err := store.UpsertCommentRows(ctx, db.DB(), storeRows); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to persist %d comment rows: %v\n", len(storeRows), err)
						}
						reported := out.ReportedTotal
						if reported == 0 {
							// No API-reported total in the payload: fall back to
							// what this complete-thread fetch actually returned.
							reported = int64(out.TopLevel + out.Replies)
						}
						if err := store.UpsertPostMeta(ctx, db.DB(), postURL, "", reported); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to persist post metadata: %v\n", err)
						}
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: comment corpus migration failed: %v\n", err)
					}
					_ = db.Close()
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&route, "route", "auto", "Thread route: auto (cost-based), flat (include_replies, 15 cr), per-comment (1 cr per comment)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for persisting the fetched rows")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Skip persisting fetched comments to the local store")
	return cmd
}

type commentsPayload struct {
	comments       []json.RawMessage
	creditsCharged int64
	reportedTotal  int64
}

// parseCommentsPayload tolerates the two envelope shapes the comments and
// replies endpoints return: a top-level array field named comments/replies, or
// a data wrapper around one.
func parseCommentsPayload(raw json.RawMessage) commentsPayload {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return commentsPayload{}
	}
	var out commentsPayload
	if cc, ok := obj["credits_charged"]; ok {
		_ = json.Unmarshal(cc, &out.creditsCharged)
	}
	for _, key := range []string{"comment_count", "total_comments", "count"} {
		if v, ok := obj[key]; ok {
			if json.Unmarshal(v, &out.reportedTotal) == nil && out.reportedTotal > 0 {
				break
			}
		}
	}
	for _, key := range []string{"comments", "replies", "child_comments", "data"} {
		v, ok := obj[key]
		if !ok {
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(v, &arr); err == nil && len(arr) > 0 {
			out.comments = arr
			return out
		}
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(v, &nested); err == nil {
			for _, nkey := range []string{"comments", "replies", "child_comments"} {
				if nv, ok := nested[nkey]; ok {
					if err := json.Unmarshal(nv, &arr); err == nil {
						out.comments = arr
						return out
					}
				}
			}
		}
	}
	return out
}

// envelopeHasMore reports whether a comments envelope explicitly says more
// pages exist (has_more/more_available true). Absent flags mean "no evidence
// of truncation" -- this is deliberately conservative so a trailing cursor
// field on the last page cannot false-positive a truncation warning.
func envelopeHasMore(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	for _, k := range []string{"has_more", "more_available", "has_more_comments"} {
		if v, ok := obj[k]; ok {
			var b bool
			if json.Unmarshal(v, &b) == nil && b {
				return true
			}
		}
	}
	return false
}

// commentField extracts a string-or-number field from a raw comment object.
func commentField(raw json.RawMessage, field string) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	v, ok := obj[field]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(v, &n); err == nil {
		return n.String()
	}
	return ""
}

// commentToRows converts one raw comment (plus any inline replies array) into
// store rows; returns the rows and how many inline replies it found.
func commentToRows(raw json.RawMessage, postURL, parentID string) ([]store.CommentRow, int) {
	id := commentField(raw, "id")
	if id == "" {
		id = commentField(raw, "pk")
	}
	if id == "" {
		return nil, 0
	}
	var obj map[string]json.RawMessage
	_ = json.Unmarshal(raw, &obj)
	text := commentField(raw, "text")
	var likes int64
	if lv, ok := obj["like_count"]; ok {
		_ = json.Unmarshal(lv, &likes)
	}
	rows := []store.CommentRow{{CommentID: id, PostURL: postURL, ParentID: parentID, Text: text, LikeCount: likes, Platform: "instagram"}}
	replyCount := 0
	for _, key := range []string{"replies", "child_comments"} {
		if rv, ok := obj[key]; ok {
			var arr []json.RawMessage
			if err := json.Unmarshal(rv, &arr); err == nil {
				for _, r := range arr {
					childRows, _ := commentToRows(r, postURL, id)
					rows = append(rows, childRows...)
					replyCount += len(childRows)
				}
			}
		}
	}
	return rows, replyCount
}
