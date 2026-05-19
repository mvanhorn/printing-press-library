// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written implementation of the `posts` command. Replaces the
// generated GET-and-passthrough body with the Naver-specific flow:
//
//  1. Canonicalize input (accepts both "posts <blog_id> <log_no>" and
//     "posts <url>" pasted as a single arg).
//  2. Fetch mobile post HTML, parse OpenGraph + body via postparse.
//  3. Fetch engagement through the shared primitive: reactions for
//     likes, cbox for comments, and PostView.naver for publish date.
//  4. Optionally detect KFTC sponsored-disclosure markers.
//  5. Record an engagement snapshot to engagement_history.
//  6. Emit through the press's standard output helpers so --json /
//     --select / --compact / --csv / --quiet / --dry-run all behave
//     the same as on generated commands.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/commentapi"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/engagement"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/postparse"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/sponsored"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/store"
)

// postOutput is the shape emitted by `posts`. JSON tags drive --select
// field matching: keep this struct stable across releases.
type postOutput struct {
	BlogID           string    `json:"blog_id"`
	LogNo            string    `json:"log_no"`
	URL              string    `json:"url"`
	PostViewURL      string    `json:"post_view_url"`
	Title            string    `json:"title"`
	Description      string    `json:"description,omitempty"`
	ThumbnailURL     string    `json:"thumbnail_url,omitempty"`
	Tags             []string  `json:"tags,omitempty"`
	Images           []string  `json:"images"`
	BodyText         string    `json:"body_text,omitempty"`
	BodyHTML         string    `json:"body_html,omitempty"`
	Likes            *int      `json:"likes"`
	Comments         int       `json:"comments"`
	PublishedAtUTC   time.Time `json:"published_at_utc,omitempty"`
	PublishDateRaw   string    `json:"publish_date_raw,omitempty"`
	EngagementSource string    `json:"engagement_source,omitempty"`
	Sponsored        *bool     `json:"sponsored,omitempty"`
	SponsoredMarkers []string  `json:"sponsored_markers,omitempty"`
	FetchedAtUTC     time.Time `json:"fetched_at_utc"`

	CommentItems        []commentapi.Comment `json:"-"`
	IncludeCommentItems bool                 `json:"-"`
	IncludeImages       bool                 `json:"-"`
}

func (p postOutput) MarshalJSON() ([]byte, error) {
	type postOutputAlias postOutput
	base, err := json.Marshal(postOutputAlias(p))
	if err != nil {
		return nil, err
	}
	if p.IncludeImages && !p.IncludeCommentItems {
		return base, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(base, &obj); err != nil {
		return nil, err
	}
	if !p.IncludeImages {
		delete(obj, "images")
	}
	if !p.IncludeCommentItems {
		return json.Marshal(obj)
	}
	commentCount, err := json.Marshal(p.Comments)
	if err != nil {
		return nil, err
	}
	comments, err := json.Marshal(p.CommentItems)
	if err != nil {
		return nil, err
	}
	obj["comment_count"] = commentCount
	obj["comments"] = comments
	return json.Marshal(obj)
}

func newPostsPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagSponsored bool
	var flagIncludeComments bool
	var flagNoImages bool

	cmd := &cobra.Command{
		Use:   "posts <blog_id> <log_no>",
		Short: "Fetch a single Naver Blog post with title, body, tags, likes, comments, publish date.",
		Long: `Fetch a single Naver Blog post and return its title, snippet, body text, tags, like count, comment count, publish date, and (optionally) sponsored-content detection.

Accepts either two positional args (blog_id log_no) or a single URL in any of:
  - https://m.blog.naver.com/<blog_id>/<log_no>
  - https://blog.naver.com/<blog_id>/<log_no>
  - https://blog.naver.com/PostView.naver?blogId=<id>&logNo=<n>

Engagement counts are recorded to ~/.local/share/naver-blog-pp-cli/data.db
(engagement_history table) on each successful fetch so 'posts diff --since'
can compute deltas.

Pass --flag-sponsored to additionally scan the body for KFTC-required
sponsorship-disclosure phrases (협찬, 체험단, 광고 포함, 유료광고 포함, and the
"본 포스팅은 ... 받아 ... 작성" sentence form).

Pass --include-comments to replace the default numeric comments field with a
detailed comments array and emit the original count as comment_count.

Body image URLs are included as images by default. Pass --no-images to omit
that array from minimal responses.`,
		Example: `  naver-blog-pp-cli posts selly9401 224234460263
  naver-blog-pp-cli posts https://m.blog.naver.com/selly9401/224234460263
  naver-blog-pp-cli posts selly9401 224234460263 --flag-sponsored
  naver-blog-pp-cli posts perfect62 224286416663 --include-comments --json`,
		Annotations: map[string]string{
			"pp:endpoint":   "posts.get",
			"pp:method":     "GET",
			"pp:path":       "/{blog_id}/{log_no}",
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
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			out, err := fetchSinglePostWithOptions(ctx, c, blogID, logNo, postFetchOptions{
				FlagSponsored:   flagSponsored,
				IncludeComments: flagIncludeComments,
				NoImages:        flagNoImages,
			})
			if err != nil {
				return classifyCommentAPIError(err, flags)
			}
			// Record engagement snapshot to local DB. Best-effort: a
			// store failure does NOT block the response — the user
			// asked for live data and they got it. We log a warning
			// to stderr for human runs so an operator notices a
			// silently-broken database.
			recordPostEngagement(ctx, blogID, logNo, out)
			cachePost(ctx, out)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&flagSponsored, "flag-sponsored", false, "Scan the body for KFTC-required sponsored-content disclosure phrases (협찬, 체험단, 광고 포함, 유료광고 포함, sentence form)")
	cmd.Flags().BoolVar(&flagIncludeComments, "include-comments", false, "Fetch actual comment content and emit comments as an array")
	cmd.Flags().BoolVar(&flagNoImages, "no-images", false, "Omit extracted body image URLs from the output")
	return cmd
}

// parsePostArgs accepts either a single URL arg or two positional
// args (blog_id, log_no). Returns the canonical (blog_id, log_no) or
// a usage error.
func parsePostArgs(args []string) (string, string, error) {
	switch len(args) {
	case 1:
		blogID, logNo, ok := naverurl.CanonicalKey(args[0])
		if !ok {
			return "", "", fmt.Errorf("could not parse %q as a Naver Blog URL; pass two positional args or a recognized URL shape", args[0])
		}
		return blogID, logNo, nil
	case 2:
		blogID, logNo := args[0], args[1]
		if blogID == "" {
			return "", "", fmt.Errorf("blog_id is required")
		}
		if logNo == "" {
			return "", "", fmt.Errorf("log_no is required")
		}
		// Re-normalize through CanonicalKey so a user pasting a
		// mixed-shape arg still gets the canonical key.
		blogID2, logNo2, ok := naverurl.CanonicalKey(naverurl.MobileURL(blogID, logNo))
		if ok {
			return blogID2, logNo2, nil
		}
		return blogID, logNo, nil
	default:
		return "", "", fmt.Errorf("expected 1 (URL) or 2 (blog_id log_no) positional args, got %d", len(args))
	}
}

// fetchSinglePost runs the full retrieval pipeline for one post. The
// body comes from mobile HTML; engagement comes from the shared
// best-effort engagement primitive.
type postFetchOptions struct {
	FlagSponsored   bool
	IncludeComments bool
	NoImages        bool
}

func fetchSinglePost(ctx context.Context, c *client.Client, blogID, logNo string, flagSponsored bool) (*postOutput, error) {
	return fetchSinglePostWithOptions(ctx, c, blogID, logNo, postFetchOptions{FlagSponsored: flagSponsored})
}

func fetchSinglePostWithOptions(ctx context.Context, c *client.Client, blogID, logNo string, opts postFetchOptions) (*postOutput, error) {
	mobileURL := naverurl.MobileURL(blogID, logNo)
	postViewURL := naverurl.PostViewURL(blogID, logNo)

	out := &postOutput{
		BlogID:        blogID,
		LogNo:         logNo,
		URL:           mobileURL,
		PostViewURL:   postViewURL,
		FetchedAtUTC:  time.Now().UTC(),
		IncludeImages: !opts.NoImages,
	}

	mobileHTML, err := getHTMLBytes(c, mobileURL)
	if err != nil {
		return nil, fmt.Errorf("fetching mobile post: %w", err)
	}
	postMeta, err := postparse.ParseMobilePost(mobileHTML)
	if err != nil {
		return nil, fmt.Errorf("parsing mobile post: %w", err)
	}
	out.Title = postMeta.Title
	out.Description = postMeta.Description
	out.ThumbnailURL = postMeta.ThumbnailURL
	out.Tags = postMeta.Tags
	if !opts.NoImages {
		out.Images = postMeta.Images
		if out.Images == nil {
			out.Images = []string{}
		}
	}
	out.BodyHTML = postMeta.BodyHTML
	out.BodyText = postMeta.BodyText

	snap := engagement.Fetch(ctx, c, blogID, logNo)
	out.Likes = snap.Likes
	out.Comments = snap.Comments
	out.PublishDateRaw = snap.PublishDateStr
	out.PublishedAtUTC = snap.PublishedAtUTC
	out.EngagementSource = engagementSourceForSnapshot(snap)
	for _, err := range snap.Errors {
		fmt.Fprintf(os.Stderr, "warning: engagement fetch failed: %v\n", err)
	}

	if opts.FlagSponsored {
		isSponsored, markers := sponsored.DetectSponsored(out.BodyText)
		out.Sponsored = &isSponsored
		out.SponsoredMarkers = markers
	}

	if opts.IncludeComments {
		comments, total, err := commentapi.GetComments(ctx, c.HTTPClient, blogID, logNo, commentapi.GetOptions{
			PageSize: 100,
			All:      true,
			Limiter:  c.Limiter(),
			Pacing:   200 * time.Millisecond,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching comments: %w", err)
		}
		stripCommentRaw(comments)
		out.CommentItems = comments
		out.IncludeCommentItems = true
		out.EngagementSource = "cbox"
		if total > 0 {
			out.Comments = total
		}
	}

	return out, nil
}

// getHTMLBytes wraps client.Client.Get with the BinaryResponseHeader
// so the client returns raw HTML bytes (not parsed JSON). The
// response is passed through the JSON-encoded RawMessage type for
// transport but contains the HTML bytes verbatim.
func getHTMLBytes(c *client.Client, absURL string) ([]byte, error) {
	headers := map[string]string{
		client.BinaryResponseHeader: "true",
	}
	raw, err := c.GetWithHeaders(absURL, nil, headers)
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

// recordPostEngagement persists a snapshot to engagement_history.
// Best-effort: failures log to stderr and are otherwise swallowed.
// engagement_source is the source that supplied the comment count when
// known ("cbox" in the normal path, "post-view-html" for the legacy
// fallback), otherwise the like-count source when only reactions landed.
func recordPostEngagement(ctx context.Context, blogID, logNo string, out *postOutput) {
	if cliutil.IsVerifyEnv() {
		// Skip DB writes under verify so the verifier's clean-room
		// home dir doesn't end up with persisted state.
		return
	}
	dbPath := defaultDBPath("naver-blog-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: engagement record skipped — opening db: %v\n", err)
		return
	}
	defer db.Close()
	likes := -1
	source := "cbox"
	if out.Likes != nil {
		likes = *out.Likes
	}
	if out.EngagementSource != "" {
		source = out.EngagementSource
	}
	if err := db.RecordEngagement(ctx, blogID, logNo, likes, out.Comments, source); err != nil {
		fmt.Fprintf(os.Stderr, "warning: engagement record failed: %v\n", err)
	}
}

func engagementSourceForSnapshot(snap engagement.Snapshot) string {
	if snap.CommentsSource != "" {
		return snap.CommentsSource
	}
	if snap.LikesSource != "" {
		return snap.LikesSource
	}
	return ""
}

// cachePost persists a successfully fetched post into the local posts cache.
// Best-effort: cache failures must not block the live post response.
func cachePost(ctx context.Context, out *postOutput) {
	if out == nil || cliutil.IsVerifyEnv() {
		return
	}
	data, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: post cache skipped: marshal post: %v\n", err)
		return
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		fmt.Fprintf(os.Stderr, "warning: post cache skipped: decode post: %v\n", err)
		return
	}
	obj["id"] = postCacheID(out.URL, out.BlogID, out.LogNo)
	obj["tags"] = joinPostTags(out.Tags)
	if out.Likes != nil {
		obj["likes"] = *out.Likes
	}
	if !out.PublishedAtUTC.IsZero() {
		obj["published_at"] = out.PublishedAtUTC.Format(time.RFC3339)
	}
	cachePostObject(ctx, obj)
}

func cachePostObject(ctx context.Context, obj map[string]any) {
	if obj == nil || cliutil.IsVerifyEnv() {
		return
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("naver-blog-pp-cli"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: post cache skipped: opening db: %v\n", err)
		return
	}
	defer db.Close()
	if err := upsertPostCacheObject(db, obj); err != nil {
		fmt.Fprintf(os.Stderr, "warning: post cache failed: %v\n", err)
	}
}

func upsertPostCacheObject(db *store.Store, obj map[string]any) error {
	ensurePostCacheID(obj)
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal post: %w", err)
	}
	return db.UpsertPosts(data)
}

func ensurePostCacheID(obj map[string]any) {
	if s, ok := obj["id"].(string); ok && strings.TrimSpace(s) != "" {
		return
	}
	if s, ok := obj["url"].(string); ok && strings.TrimSpace(s) != "" {
		obj["id"] = s
		return
	}
	blogID, _ := obj["blog_id"].(string)
	logNo, _ := obj["log_no"].(string)
	obj["id"] = postCacheID("", blogID, logNo)
}

func postCacheID(url, blogID, logNo string) string {
	if strings.TrimSpace(url) != "" {
		return url
	}
	if strings.TrimSpace(blogID) != "" && strings.TrimSpace(logNo) != "" {
		return naverurl.MobileURL(blogID, logNo)
	}
	return ""
}

func joinPostTags(tags []string) string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return strings.Join(out, ",")
}

// Quiet unused-import warning when go vet runs against a build that
// stripped the only json reference above (kept for future fields).
var _ = json.RawMessage(nil)
