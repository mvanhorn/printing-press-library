// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cost-aware complete comment threads for one post.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

const includeRepliesFlatCredits = 15

// maxThreadPages is a safety net behind the credit budget for --max-credits
// traversal: even a large budget never walks more pages than this.
const maxThreadPages = 50

type threadEnvelope struct {
	PostURL        string            `json:"post_url"`
	Route          string            `json:"route"`
	RouteReason    string            `json:"route_reason"`
	TopLevel       int               `json:"top_level_comments"`
	ReportedTotal  int64             `json:"reported_comment_count,omitempty"`
	Replies        int               `json:"replies"`
	CreditsCharged int64             `json:"credits_charged"`
	MaxCredits     int64             `json:"max_credits,omitempty"`
	PagesFetched   int               `json:"pages_fetched"`
	Truncated      bool              `json:"truncated,omitempty"`
	Note           string            `json:"note,omitempty"`
	Comments       []json.RawMessage `json:"comments"`
	FetchFailures  []fetchFailure    `json:"fetch_failures,omitempty"`
}

// threadFetchOpts carries the user-facing knobs of one thread fetch.
type threadFetchOpts struct {
	postURL string
	route   string // auto|flat|per-comment (validated by the caller)
	// maxCredits > 0 enables budgeted traversal of further top-level comment
	// pages; 0 keeps the single-page behavior and reports truncation. This is
	// deliberately opt-in (unlike sweep, where 0 disables the budget): page
	// traversal is new spend on a command that used to make 1-2 calls.
	maxCredits int64
}

// fetchCommentThread performs the whole paid part of `comments thread`:
// route probe, route decision, flat/per-comment fetch, and (when a budget is
// given) cursor traversal of further top-level comment pages. Every fetch —
// probe, include_replies call, per-comment reply calls, and page fetches — is
// charged to one sweepBudget, so --max-credits bounds the command's total
// spend. It returns the output envelope plus the rows to persist.
func fetchCommentThread(ctx context.Context, c apiGetter, opts threadFetchOpts) (threadEnvelope, []store.CommentRow, error) {
	const commentsPath = "/v2/instagram/post/comments"

	out := threadEnvelope{PostURL: opts.postURL, MaxCredits: opts.maxCredits}
	budget := newSweepBudget(opts.maxCredits)
	traverse := opts.maxCredits > 0
	budgetStopped := false
	halt := func(note string) {
		budgetStopped = true
		if note != "" {
			out.Note = note
		}
	}
	charge := func(cost int64) {
		if note, breached := budget.charge(cost); breached {
			halt(note)
		}
	}

	var env commentsPayload
	var lastRaw json.RawMessage
	var truncatedTop bool
	attemptedReplies := 0
	// Reply failures are tracked separately from page failures so the
	// all-replies-failed check can never be tripped (or masked) by a failed
	// page fetch; out.FetchFailures aggregates both for the envelope.
	var replyFailures []fetchFailure
	var storeRows []store.CommentRow

	// addPerCommentPage stores a page of top-level comments and fetches each
	// comment's replies (1 credit per call), gating every reply fetch on the
	// budget so a long comment list cannot overrun --max-credits.
	addPerCommentPage := func(comments []json.RawMessage) {
		for _, cm := range comments {
			rows, _ := commentToRows(cm, opts.postURL, "")
			storeRows = append(storeRows, rows...)
			id := commentField(cm, "id")
			if id == "" {
				continue
			}
			if budgetStopped {
				continue
			}
			if !budget.allows() {
				halt(budget.stopNote("reply fetch"))
				continue
			}
			attemptedReplies++
			repRaw, rerr := c.Get(ctx, "/v1/instagram/post/comment/replies", map[string]string{"url": opts.postURL, "comment_id": id})
			if rerr != nil {
				f := fetchFailure{Source: id, Error: sanitizeFetchErr(rerr)}
				replyFailures = append(replyFailures, f)
				out.FetchFailures = append(out.FetchFailures, f)
				continue
			}
			rep := parseCommentsPayload(repRaw)
			charge(rep.creditsCharged)
			out.Replies += len(rep.comments)
			out.Comments = append(out.Comments, rep.comments...)
			for _, rm := range rep.comments {
				rows, _ := commentToRows(rm, opts.postURL, id)
				storeRows = append(storeRows, rows...)
			}
		}
	}

	// addFlatPage stores a page returned by include_replies (replies inline).
	addFlatPage := func(comments []json.RawMessage) {
		for _, cm := range comments {
			rows, replyCount := commentToRows(cm, opts.postURL, "")
			storeRows = append(storeRows, rows...)
			out.Replies += replyCount
		}
	}

	chosen := opts.route
	reason := "forced by --route"
	if opts.route != "flat" {
		// First page of top-level comments decides the route: N per-comment
		// reply calls cost N credits, the flat route costs 15 regardless. A
		// forced --route flat skips this probe so the flat path costs exactly
		// 15 credits.
		raw, err := c.Get(ctx, commentsPath, map[string]string{"url": opts.postURL})
		if err != nil {
			return out, nil, fmt.Errorf("fetching top-level comments: %w", err)
		}
		if isErrorEnvelope(raw) {
			return out, nil, fmt.Errorf("comments endpoint returned an error envelope for %s", opts.postURL)
		}
		env = parseCommentsPayload(raw)
		charge(env.creditsCharged)
		truncatedTop = envelopeHasMore(raw)
		lastRaw = raw
		out.PagesFetched++
		if opts.route == "auto" {
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

	out.Route = chosen
	out.RouteReason = reason
	out.TopLevel = len(env.comments)

	switch chosen {
	case "flat":
		flatOK := !budgetStopped
		if flatOK && !budget.allows() {
			halt(budget.stopNote("include_replies fetch"))
			flatOK = false
		}
		if flatOK {
			fullRaw, err := c.Get(ctx, commentsPath, map[string]string{"url": opts.postURL, "include_replies": "true"})
			if err != nil {
				return out, nil, fmt.Errorf("fetching threads with include_replies: %w", err)
			}
			if isErrorEnvelope(fullRaw) {
				return out, nil, fmt.Errorf("comments endpoint returned an error envelope for %s", opts.postURL)
			}
			full := parseCommentsPayload(fullRaw)
			charge(full.creditsCharged)
			truncatedTop = envelopeHasMore(fullRaw)
			lastRaw = fullRaw
			out.PagesFetched++
			if full.reportedTotal > 0 {
				out.ReportedTotal = full.reportedTotal
			}
			out.TopLevel = len(full.comments)
			out.Comments = full.comments
			addFlatPage(full.comments)
		} else if len(env.comments) > 0 {
			// The probe already paid for the first page of top-level comments;
			// return it rather than discarding paid data. Replies are missing,
			// which the note (set by halt) explains.
			out.Comments = env.comments
			addFlatPage(env.comments)
		}
	case "per-comment":
		if env.reportedTotal > 0 {
			out.ReportedTotal = env.reportedTotal
		}
		out.Comments = env.comments
		addPerCommentPage(env.comments)
	}

	// Budgeted traversal of further top-level comment pages (F3): only when
	// the caller set --max-credits, so the default single-page cost profile is
	// unchanged. The cursor comes from the envelope's cursor field (the same
	// contract the generated `instagram list-post-2 --cursor` flag documents).
	if traverse {
		params := map[string]string{"url": opts.postURL}
		if chosen == "flat" {
			params["include_replies"] = "true"
		}
		// Cursors already submitted: a cyclic cursor (the API reporting
		// has_more with a cursor it has served before) would otherwise re-buy
		// the same page until the budget or the page cap intervened.
		seenCursors := map[string]bool{}
		for page := out.PagesFetched + 1; truncatedTop && !budgetStopped; page++ {
			if page > maxThreadPages {
				out.Note = fmt.Sprintf("stopped at the %d-page safety cap with pages remaining; the thread is still truncated", maxThreadPages)
				break
			}
			cursor := extractCommentsCursor(lastRaw)
			if cursor == "" {
				// has_more with no cursor: nothing to continue on.
				break
			}
			if seenCursors[cursor] {
				out.Note = fmt.Sprintf("comments endpoint returned a cursor it already served (page %d): stopped to avoid paying for the same page twice; the thread is still truncated", page)
				break
			}
			seenCursors[cursor] = true
			if !budget.allows() {
				halt(budget.stopNote("comments page"))
				break
			}
			pageParams := map[string]string{"cursor": cursor}
			for k, v := range params {
				pageParams[k] = v
			}
			raw, err := c.Get(ctx, commentsPath, pageParams)
			if err != nil {
				out.FetchFailures = append(out.FetchFailures, fetchFailure{Source: fmt.Sprintf("page %d", page), Error: sanitizeFetchErr(err)})
				out.Note = fmt.Sprintf("page %d fetch failed (%s): traversal stopped; the thread is still truncated", page, sanitizeFetchErr(err))
				break
			}
			if isErrorEnvelope(raw) {
				out.FetchFailures = append(out.FetchFailures, fetchFailure{Source: fmt.Sprintf("page %d", page), Error: "error envelope from comments endpoint"})
				out.Note = fmt.Sprintf("page %d fetch failed (error envelope from comments endpoint): traversal stopped; the thread is still truncated", page)
				break
			}
			pl := parseCommentsPayload(raw)
			charge(pl.creditsCharged)
			truncatedTop = envelopeHasMore(raw)
			lastRaw = raw
			out.PagesFetched++
			if len(pl.comments) == 0 {
				break
			}
			out.TopLevel += len(pl.comments)
			if chosen == "per-comment" {
				out.Comments = append(out.Comments, pl.comments...)
				addPerCommentPage(pl.comments)
			} else {
				out.Comments = append(out.Comments, pl.comments...)
				addFlatPage(pl.comments)
			}
		}
	}

	// Finalize the envelope BEFORE the all-replies-failed check so the
	// partial envelope the caller serializes on the error path is complete
	// (credits, truncation, note, fetch_failures).
	out.CreditsCharged = budget.charged
	out.Truncated = truncatedTop

	if err := allSourcesFailedErr("comments thread replies", attemptedReplies, replyFailures); err != nil {
		// An earlier stop already diagnosed in the note (a failed page fetch,
		// a cursor cycle, a budget stop) must not vanish behind the aggregate
		// reply error: carry both diagnoses in the hard error. The %w wrap
		// keeps the underlying cliError in the chain, so the auth exit-code
		// mapping of allSourcesFailedErr survives unchanged. The caller also
		// serializes the partial envelope to stdout before exiting non-zero.
		if out.Note != "" {
			err = fmt.Errorf("%w\nadditionally: %s", err, out.Note)
		}
		return out, nil, err
	}

	if out.Truncated && out.Note == "" {
		if traverse {
			out.Note = "comments endpoint still reports more pages; raise --max-credits to continue the traversal"
		} else {
			out.Note = "comments endpoint reports has_more=true: only the first page of top-level comments is included; rerun with --max-credits to traverse further pages under a credit budget"
		}
	}
	return out, storeRows, nil
}

// extractCommentsCursor pulls the continuation cursor from a comments
// envelope. The comments endpoint names it cursor (see the generated
// list-post-2 command's --cursor flag); next_max_id is accepted as a fallback
// for older envelope shapes.
func extractCommentsCursor(raw json.RawMessage) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, k := range []string{"cursor", "next_cursor", "next_max_id"} {
		v, ok := obj[k]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) == nil && s != "" {
			return s
		}
		var n json.Number
		if json.Unmarshal(v, &n) == nil && n.String() != "" && n.String() != "0" {
			return n.String()
		}
	}
	return ""
}

// printPartialThreadEnvelope serializes a partially-populated thread envelope
// to stdout on a hard-error exit, so structured diagnoses (note,
// fetch_failures, any partial comments already paid for) are not lost to
// stderr-only error text. A bare failure with nothing to report (e.g. the
// very first probe errored) prints nothing, preserving the plain
// error-only behavior for empty envelopes.
func printPartialThreadEnvelope(w io.Writer, out threadEnvelope, flags *rootFlags) error {
	if len(out.Comments) == 0 && len(out.FetchFailures) == 0 && out.Note == "" {
		return nil
	}
	return printJSONFiltered(w, out, flags)
}

func newNovelCommentsThreadCmd(flags *rootFlags) *cobra.Command {
	var route string
	var dbPath string
	var noStore bool
	var maxCredits int64

	cmd := &cobra.Command{
		Use:   "thread [post-url]",
		Short: "Fetch one post's complete comment threads with cost-aware routing",
		Long: strings.Trim(`
Use this command to fetch one post's complete comment threads with cost-aware
routing between include_replies (15 credits flat) and per-comment reply calls
(1 credit each). By default only the first page of top-level comments is
fetched and 'truncated' reports when more exist; pass --max-credits to keep
traversing further pages under a credit budget. Do NOT use it for bulk
multi-post pulls; use 'comments sweep' instead. Do NOT use it to audit
already-synced posts; use 'comments coverage' instead.`, "\n"),
		Example: strings.Trim(`
  scrape-creators-pp-cli comments thread https://www.instagram.com/reel/C8rKmYvsrck --agent
  scrape-creators-pp-cli comments thread https://www.instagram.com/p/DEF456 --route flat
  scrape-creators-pp-cli comments thread https://www.instagram.com/p/DEF456 --max-credits 60 --agent`, "\n"),
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
			if maxCredits < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--max-credits must be zero or positive"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			out, storeRows, err := fetchCommentThread(ctx, c, threadFetchOpts{postURL: postURL, route: route, maxCredits: maxCredits})
			if err != nil {
				// Serialize the partial envelope BEFORE failing so the
				// structured diagnoses (note, fetch_failures, partial
				// comments) reach stdout consumers; the returned error still
				// drives the non-zero exit (authErr mapping included). This
				// return path never reaches the success-path print below, so
				// the envelope is emitted exactly once.
				if perr := printPartialThreadEnvelope(cmd.OutOrStdout(), out, flags); perr != nil {
					return perr
				}
				return err
			}
			if out.Note != "" {
				// Truncation and budget stops both land here: never present a
				// partial thread as complete without saying so on stderr.
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", out.Note)
			}
			warnFetchFailures(cmd, "thread", out.FetchFailures)

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
	cmd.Flags().Int64Var(&maxCredits, "max-credits", 0, "Credit budget for traversing further top-level comment pages (0 = fetch only the first page and report truncated; unlike sweep, 0 does NOT mean unlimited)")
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
