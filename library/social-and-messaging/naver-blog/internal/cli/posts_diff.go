// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `posts-diff` command. Compares the current engagement
// state of a post against the most recent snapshot at or before
// (now - --since). Records the fresh snapshot to engagement_history
// before computing the delta so the next invocation has a baseline.
//
// Wired as a top-level command rather than a subcommand of `posts`:
// the existing `posts` is a leaf that fetches a single post, and the
// task brief explicitly warns against refactoring it to a parent. The
// `posts diff` UX lives in the manifest; this command provides the
// underlying machinery, and a hyphenated name keeps the press's auto-
// promoted commands clean.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/store"
)

// postDiffOutput is the JSON shape emitted by `posts-diff`.
type postDiffOutput struct {
	URL    string         `json:"url"`
	BlogID string         `json:"blog_id"`
	LogNo  string         `json:"log_no"`
	Before *snapshotPoint `json:"before,omitempty"`
	After  snapshotPoint  `json:"after"`
	Delta  *snapshotDelta `json:"delta,omitempty"`
	Note   string         `json:"note,omitempty"`
}

type snapshotPoint struct {
	Likes      *int      `json:"likes"`
	Comments   *int      `json:"comments"`
	CapturedAt time.Time `json:"captured_at"`
	Source     string    `json:"source,omitempty"`
}

type snapshotDelta struct {
	Likes      int     `json:"likes"`
	Comments   int     `json:"comments"`
	TimeDelta  string  `json:"time_delta"`
	TimeDeltaH float64 `json:"time_delta_hours"`
}

func newPostsDiffCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "posts-diff <url>",
		Short: "Compute the engagement delta for a post since the last snapshot.",
		Long: `Compute the engagement delta (likes, comments) for a single Naver Blog post since the most recent snapshot at or before (now - --since). Records the fresh snapshot to ~/.local/share/naver-blog-pp-cli/data.db (engagement_history table) before computing the delta, so the next invocation has an updated baseline.

Accepts either two positional args (blog_id log_no) or a single URL in any of:
  - https://m.blog.naver.com/<blog_id>/<log_no>
  - https://blog.naver.com/<blog_id>/<log_no>
  - https://blog.naver.com/PostView.naver?blogId=<id>&logNo=<n>

When no prior snapshot exists at or before the cutoff but a newer one is cached, the command falls back to the most recent snapshot and emits a note explaining that the diff covers a shorter span than requested. When no snapshot exists at all, the output omits the 'before' and 'delta' fields and includes a note: "first snapshot".

--since accepts Go duration syntax with a trailing 'd' shortcut. Common values:
  - 1h, 6h, 12h    short-window diffs
  - 24h, 7d        canonical daily/weekly diffs (7d == 7*24h)
  - 30d, 90d       monthly/quarterly diffs

The current state is fetched live: it pulls the reaction-API like count and the PostView.naver comment count (same path as 'posts').`,
		Example: `  naver-blog-pp-cli posts-diff https://m.blog.naver.com/selly9401/224234460263
  naver-blog-pp-cli posts-diff selly9401 224234460263 --since 24h
  naver-blog-pp-cli posts-diff selly9401 224234460263 --since 7d`,
		Annotations: map[string]string{
			"pp:endpoint":   "posts.diff",
			"pp:method":     "GET",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before any arg validation so verify dry-run
			// probes succeed without forcing a sample URL or blog_id/log_no.
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			blogID, logNo, err := parsePostArgs(args)
			if err != nil {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": err.Error(),
						"usage": fmt.Sprintf("%s <blog_id> <log_no> | <url>", cmd.CommandPath()),
					}, flags)
				}
				return usageErr(err)
			}
			window, err := parseSinceWindow(flagSince)
			if err != nil {
				return usageErr(err)
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			current, err := fetchSinglePost(ctx, c, blogID, logNo, false)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := &postDiffOutput{
				BlogID: blogID,
				LogNo:  logNo,
				URL:    naverurl.MobileURL(blogID, logNo),
				After: snapshotPoint{
					Likes:      current.Likes,
					Comments:   intPtr(current.Comments),
					CapturedAt: now,
					Source:     current.EngagementSource,
				},
			}
			cutoff := now.Add(-window)
			prior, lookupErr := lookupBaseline(ctx, blogID, logNo, cutoff)
			// If no snapshot exists at or before the cutoff, try the most
			// recent snapshot regardless of age. Users commonly run
			// `--since 24h` expecting "diff vs yesterday" — if the only
			// cached baseline is 2h old, give them that with a clear note
			// rather than the misleading "first snapshot" message.
			fellBack := false
			if lookupErr == nil && prior == nil {
				prior, lookupErr = lookupBaseline(ctx, blogID, logNo, now)
				if prior != nil {
					fellBack = true
				}
			}
			// Record the fresh snapshot AFTER the baseline read so a
			// same-second race can't swap them.
			recordPostEngagement(ctx, blogID, logNo, current)
			if lookupErr != nil {
				out.Note = "baseline lookup failed: " + lookupErr.Error()
			}
			if prior == nil {
				if out.Note == "" {
					out.Note = "first snapshot (no prior engagement cached for this post)"
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if fellBack {
				age := now.Sub(prior.CapturedAt).Round(time.Minute)
				out.Note = fmt.Sprintf("baseline is %s old (newer than requested --since=%s window); diff covers a shorter span than requested", age, flagSince)
			}
			before := &snapshotPoint{
				CapturedAt: prior.CapturedAt,
				Source:     prior.Source,
			}
			if prior.Likes.Valid {
				v := int(prior.Likes.Int64)
				before.Likes = &v
			}
			if prior.Comments.Valid {
				v := int(prior.Comments.Int64)
				before.Comments = &v
			}
			out.Before = before
			out.Delta = computeDelta(before, &out.After)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Look-back window (Go duration; supports trailing 'd' for days, e.g. 7d = 7*24h)")
	return cmd
}

// parseSinceWindow accepts Go duration syntax plus the convenience
// suffix "d" meaning "days". The press's spec mentions 24h and 7d in
// the same breath, so we handle both shapes without forcing the
// caller to expand 7d to 168h by hand.
//
// Named differently from sync.go's parseSinceDuration (which returns
// a time.Time, not a time.Duration) to avoid collision while keeping
// both available.
func parseSinceWindow(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 7 * 24 * time.Hour, nil
	}
	if strings.HasSuffix(raw, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid --since %q: expected Go duration or Nd", raw)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: %w", raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid --since %q: must be positive", raw)
	}
	return d, nil
}

// lookupBaseline opens the local store and reads the most recent
// engagement snapshot at or before cutoff. Returns (nil, nil) when no
// snapshot exists — the caller treats that as "first snapshot, no
// delta to compute". Skipped (returns nil, nil) under verify env so
// the verifier's clean-room home dir is left alone.
func lookupBaseline(ctx context.Context, blogID, logNo string, cutoff time.Time) (*store.EngagementSnapshot, error) {
	if cliutil.IsVerifyEnv() {
		return nil, nil
	}
	dbPath := defaultDBPath("naver-blog-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return db.LatestEngagement(ctx, blogID, logNo, cutoff)
}

// computeDelta returns the differences between two snapshots. For
// nullable fields (likes/comments), a missing value on either side
// produces a 0 delta on that field; the absence is already visible in
// before.likes==nil so callers can distinguish "no change" from "we
// didn't sample this".
func computeDelta(before, after *snapshotPoint) *snapshotDelta {
	d := &snapshotDelta{}
	if before.Likes != nil && after.Likes != nil {
		d.Likes = *after.Likes - *before.Likes
	}
	if before.Comments != nil && after.Comments != nil {
		d.Comments = *after.Comments - *before.Comments
	}
	td := after.CapturedAt.Sub(before.CapturedAt)
	d.TimeDelta = td.String()
	d.TimeDeltaH = td.Hours()
	return d
}

func intPtr(n int) *int { return &n }
