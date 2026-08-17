// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: budget-gated bulk comment sweep for a handle.
// pp:data-source live

package cli

import (
	"context"
	"database/sql"
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

			env, err := runCommentsSweep(ctx, c, db.DB(), sweepOpts{
				handle:   handle,
				cutoff:   cutoff,
				maxPosts: maxPosts,
				budget:   newSweepBudget(maxCredits),
			})
			if err != nil {
				return err
			}
			warnFetchFailures(cmd, "comment", env.FetchFailures)
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only sweep posts newer than this window (e.g. 7d, 24h, 1w)")
	cmd.Flags().Int64Var(&maxCredits, "max-credits", 100, "Credit budget checked before every fetch; the sweep halts immediately if a fetch exceeds it (0 = no budget)")
	cmd.Flags().IntVar(&maxPosts, "max-posts", 0, "Maximum posts to sweep (0 = all returned)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// sweepOpts carries one sweep's knobs. cutoff zero means no --since bound;
// maxPosts 0 means no post cap; budget must be non-nil (a zero-max budget
// disables enforcement).
type sweepOpts struct {
	handle   string
	cutoff   time.Time
	maxPosts int
	budget   *sweepBudget
}

// runCommentsSweep executes the paid part of `comments sweep` against the
// injectable client seam: it walks the handle's recent posts (paginated via
// next_max_id), pulls each post's comments, persists rows to db, and stops on
// the first of --max-posts, --since cutoff, the credit budget, or the page
// safety cap.
//
// Budget semantics: a fresh sweepBudget admits the first posts fetch by
// construction (charged 0 + floor estimate 1 fits any positive integer
// budget), so there is deliberately no pre-first-fetch gate here — one existed
// and was unreachable dead code. Enforcement starts at the first charge():
// every later fetch is gated on the running worst-case estimate, and a fetch
// that takes the charged total past the budget halts the sweep immediately.
func runCommentsSweep(ctx context.Context, c apiGetter, db *sql.DB, opts sweepOpts) (sweepEnvelope, error) {
	env := sweepEnvelope{Handle: opts.handle, MaxCredits: opts.budget.max}

	budgetAllows := func() bool { return opts.budget.allows() }
	noteBudgetStop := func(what string) {
		env.StoppedEarly = true
		env.Note = opts.budget.stopNote(what)
	}
	chargeFetch := func(cost int64) {
		if note, breached := opts.budget.charge(cost); breached {
			env.StoppedEarly = true
			env.Note = note
		}
		env.CreditsCharged = opts.budget.charged
	}

	postsRaw, err := c.Get(ctx, "/v2/instagram/user/posts", map[string]string{"handle": opts.handle, "trim": "true"})
	if err != nil {
		return env, fmt.Errorf("fetching recent posts: %w", err)
	}
	if isErrorEnvelope(postsRaw) {
		return env, fmt.Errorf("posts endpoint returned an error envelope for %s", opts.handle)
	}
	chargeFetch(payloadCredits(postsRaw))
	posts := extractPosts(postsRaw)

	// The posts feed is paginated via next_max_id (newest-first). Walk
	// pages under the same stops that bound the per-post loop, plus a
	// page safety cap for the doubly-unbounded (--max-credits 0
	// --max-posts 0) case.
	const maxSweepPages = 25
	cursor := ""
	for page := 1; ; page++ {
		pastCutoff := false
		for _, p := range posts {
			if env.StoppedEarly {
				break
			}
			if opts.maxPosts > 0 && env.PostsScanned >= opts.maxPosts {
				env.StoppedEarly = true
				env.Note = fmt.Sprintf("stopped at --max-posts %d", opts.maxPosts)
				break
			}
			if !budgetAllows() {
				noteBudgetStop("comments fetch")
				break
			}
			if !opts.cutoff.IsZero() && !p.takenAt.IsZero() && p.takenAt.Before(opts.cutoff) {
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
			chargeFetch(pl.creditsCharged)
			rowsBatch := make([]store.CommentRow, 0, len(pl.comments))
			for _, cm := range pl.comments {
				rows, _ := commentToRows(cm, p.url, "")
				rowsBatch = append(rowsBatch, rows...)
			}
			if err := store.UpsertCommentRows(ctx, db, rowsBatch); err != nil {
				return env, fmt.Errorf("persisting comments for %s: %w", p.url, err)
			}
			reported := p.reportedCount
			if reported == 0 {
				if pl.reportedTotal > 0 {
					reported = pl.reportedTotal
				} else {
					reported = int64(len(rowsBatch))
				}
			}
			if err := store.UpsertPostMeta(ctx, db, p.url, opts.handle, reported); err != nil {
				return env, fmt.Errorf("persisting post metadata for %s: %w", p.url, err)
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
		if !budgetAllows() {
			noteBudgetStop("posts page")
			break
		}
		cursor = next
		postsRaw, err = c.Get(ctx, "/v2/instagram/user/posts", map[string]string{"handle": opts.handle, "trim": "true", "next_max_id": cursor})
		if err != nil {
			return env, fmt.Errorf("fetching recent posts (page %d): %w", page+1, err)
		}
		if isErrorEnvelope(postsRaw) {
			return env, fmt.Errorf("posts endpoint returned an error envelope for %s (page %d)", opts.handle, page+1)
		}
		chargeFetch(payloadCredits(postsRaw))
		posts = extractPosts(postsRaw)
		if len(posts) == 0 {
			break
		}
	}
	if err := allSourcesFailedErr("comments sweep", env.PostsScanned, env.FetchFailures); err != nil {
		return env, err
	}
	return env, nil
}

// sweepBudget enforces --max-credits across every fetch a sweep makes.
//
// A fetch is admitted only when the already-charged total plus a worst-case
// estimate stays within the budget. The estimate is the largest per-fetch
// charge seen so far in this run (floor 1), so a request that prices above
// the previous one cannot slip through a stale estimate.
//
// Request cost is decided server-side and is known only after the response,
// so no client can guarantee a never-exceeded ceiling on the first surprise
// price alone. charge() closes that residue: the moment a fetch takes the
// charged total past the budget, the sweep is halted and the breach is
// reported, so the overshoot can never compound across further fetches.
//
// Invariant, stated so nobody re-adds a dead guard: a FRESH budget always
// admits the first fetch — charged is 0 and the floor estimate is 1, so
// allows() is true for every positive integer max (and non-positive max means
// no budget at all). The budget's protection is therefore entirely reactive:
// it begins with the first charge().
type sweepBudget struct {
	max     int64
	charged int64
	maxCost int64
}

func newSweepBudget(max int64) *sweepBudget {
	return &sweepBudget{max: max, maxCost: 1}
}

// allows reports whether the next fetch fits the budget in the worst case.
// A non-positive max means no budget.
func (b *sweepBudget) allows() bool {
	if b.max <= 0 {
		return true
	}
	return b.charged+b.maxCost <= b.max
}

// charge records an actual fetch cost, widens the worst-case estimate, and
// reports whether this fetch took the run past the budget.
func (b *sweepBudget) charge(cost int64) (note string, breached bool) {
	b.charged += cost
	if cost > b.maxCost {
		b.maxCost = cost
	}
	if b.max > 0 && b.charged > b.max {
		return fmt.Sprintf("budget exceeded: a fetch cost more than its estimate (charged %d of --max-credits %d); stopped immediately", b.charged, b.max), true
	}
	return "", false
}

// stopNote explains a pre-fetch budget stop for the named fetch kind.
func (b *sweepBudget) stopNote(what string) string {
	return fmt.Sprintf("stopped at --max-credits %d: the next %s (est. %d cr) would exceed the budget; rerun with a higher budget to continue", b.max, what, b.maxCost)
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
