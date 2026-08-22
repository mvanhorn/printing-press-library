// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the hand-written Rundown commands (top, use-cases, show,
// digest, tools rank, stack). Kept in its own file so `generate --force`
// preserves it.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/rundown/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/rundown/internal/store"
	"github.com/spf13/cobra"
)

// rdPostBaseURL is the public permalink prefix for a community workflow.
const rdPostBaseURL = "https://app.therundown.ai/community/posts/"

// rdNamed is the {name, slug} shape the API uses for tools and industries.
type rdNamed struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// rdAuthor is the subset of the author envelope worth showing in a terminal.
type rdAuthor struct {
	DisplayName string `json:"displayName"`
	Location    string `json:"location"`
	Level       string `json:"level"`
	LinkedinURL string `json:"linkedinUrl"`
	Verified    bool   `json:"verified"`
}

// rdPost is one community workflow post.
type rdPost struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	UpvoteCount  int       `json:"upvoteCount"`
	CommentCount int       `json:"commentCount"`
	CreatedAt    string    `json:"createdAt"`
	Tags         []string  `json:"tags"`
	Tools        []rdNamed `json:"tools"`
	Industries   []rdNamed `json:"industries"`
	Author       rdAuthor  `json:"author"`

	// NewsletterFeature is non-null when The Rundown featured the post in the
	// newsletter. Kept raw because only its presence matters here.
	NewsletterFeature json.RawMessage `json:"newsletterFeature"`
}

// rdComment is one comment on a workflow post.
type rdComment struct {
	ID              string   `json:"id"`
	PostID          string   `json:"postId"`
	ParentCommentID string   `json:"parentCommentId"`
	Body            string   `json:"body"`
	UpvoteCount     int      `json:"upvoteCount"`
	CreatedAt       string   `json:"createdAt"`
	Author          rdAuthor `json:"author"`
	// Replies holds the nested child comments. The community API returns a
	// thread as top-level comments each carrying their own replies[], never as
	// a flat sibling list, so this is the only place replies exist.
	Replies []rdComment `json:"replies"`
}

// rdFlattenComments walks a comment thread depth-first, emitting each comment
// immediately before its own replies so the thread still reads in order.
//
// Sync stores the parent comments verbatim, nested replies and all, and never
// writes a reply as its own row. A reader that only looks at the top level
// therefore drops every reply while still reporting the post's full
// commentCount - and the replies are usually the half of the thread where the
// author answers the questions.
func rdFlattenComments(in []rdComment) []rdComment {
	out := make([]rdComment, 0, len(in))
	var walk func(c rdComment, parentID string)
	walk = func(c rdComment, parentID string) {
		replies := c.Replies
		c.Replies = nil
		if strings.TrimSpace(c.ParentCommentID) == "" {
			c.ParentCommentID = parentID
		}
		out = append(out, c)
		sort.SliceStable(replies, func(i, j int) bool { return replies[i].CreatedAt < replies[j].CreatedAt })
		for _, r := range replies {
			walk(r, c.ID)
		}
	}
	for _, c := range in {
		walk(c, "")
	}
	return out
}

// rdPostURL returns the public permalink for a post id.
func rdPostURL(id string) string { return rdPostBaseURL + id }

// featured reports whether the post was picked up by the newsletter.
func (p rdPost) featured() bool {
	s := strings.TrimSpace(string(p.NewsletterFeature))
	return s != "" && s != "null"
}

// created parses createdAt, returning the zero time when it is absent or
// malformed so callers treat it as "outside every window".
func (p rdPost) created() time.Time {
	t, err := time.Parse(time.RFC3339, p.CreatedAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

// toolSlugs returns the post's tool slugs, lowercased.
func (p rdPost) toolSlugs() []string {
	out := make([]string, 0, len(p.Tools))
	for _, t := range p.Tools {
		if s := strings.ToLower(strings.TrimSpace(t.Slug)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// authorName returns a display name, falling back to "anonymous".
func (p rdPost) authorName() string {
	if n := strings.TrimSpace(p.Author.DisplayName); n != "" {
		return n
	}
	return "anonymous"
}

// rdResolveDBPath applies the standard default when --db was not passed.
func rdResolveDBPath(dbPath string) string {
	if strings.TrimSpace(dbPath) != "" {
		return dbPath
	}
	return defaultDBPath("rundown-pp-cli")
}

// rdMirrorMissing handles the "user has not synced yet" case. It reports
// whether the caller should stop, plus the error to return when it should.
//
// Machine formats get a valid empty payload rather than a SQLite open failure;
// humans get a stderr hint naming the exact sync command.
func rdMirrorMissing(cmd *cobra.Command, flags *rootFlags, dbPath string, empty any) (bool, error) {
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		return false, nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"no local mirror at %s\nrun: rundown-pp-cli sync --resources posts,tools --db %s\n", dbPath, dbPath)
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return true, printJSONFiltered(cmd.OutOrStdout(), empty, flags)
	}
	return true, nil
}

// rdLoadPosts reads every mirrored workflow out of the local store.
//
// Reads the generic `resources` table rather than the typed `posts` table so
// the query keeps working whether the resource was synced flat or parent-scoped.
func rdLoadPosts(ctx context.Context, db *store.Store) ([]rdPost, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, data FROM resources
		WHERE resource_type IN ('posts')`)
	if err != nil {
		return nil, fmt.Errorf("querying posts: %w", err)
	}

	type rawRow struct {
		id   string
		data []byte
	}
	raws := make([]rawRow, 0, 320)
	for rows.Next() {
		var r rawRow
		if err := rows.Scan(&r.id, &r.data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning post: %w", err)
		}
		raws = append(raws, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating posts: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing posts rows: %w", err)
	}

	posts := make([]rdPost, 0, len(raws))
	for _, r := range raws {
		var p rdPost
		if err := json.Unmarshal(r.data, &p); err != nil {
			// A single unparseable row must not sink the whole query.
			continue
		}
		if p.ID == "" {
			p.ID = r.id
		}
		// Workflow posts always carry a title; comment rows never do. Guards
		// against a mixed store left behind by an older sync.
		if p.Title == "" {
			continue
		}
		posts = append(posts, p)
	}
	return posts, nil
}

// rdLoadComments reads mirrored comments for one post, oldest first.
func rdLoadComments(ctx context.Context, db *store.Store, postID string) ([]rdComment, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT data FROM resources
		WHERE resource_type IN ('comments')`)
	if err != nil {
		return nil, fmt.Errorf("querying comments: %w", err)
	}
	raws := make([][]byte, 0, 64)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning comment: %w", err)
		}
		raws = append(raws, data)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating comments: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing comment rows: %w", err)
	}

	out := make([]rdComment, 0, 8)
	for _, data := range raws {
		var c rdComment
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		if c.PostID == postID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return rdFlattenComments(out), nil
}

// rdWindowStart converts a "7d" / "2w" / "48h" window into an absolute cutoff.
// An empty window (or "all") means no cutoff and returns the zero time.
func rdWindowStart(since string) (time.Time, error) {
	since = strings.TrimSpace(since)
	if since == "" || strings.EqualFold(since, "all") {
		return time.Time{}, nil
	}
	d, err := cliutil.ParseDurationLoose(since)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q: use a window like 7d, 2w, or 48h", since)
	}
	if d <= 0 {
		return time.Time{}, fmt.Errorf("invalid --since %q: the window must be positive", since)
	}
	return time.Now().UTC().Add(-d), nil
}

// rdInWindow reports whether a post falls inside the cutoff. A zero cutoff
// admits everything, including posts with an unparseable timestamp.
func rdInWindow(p rdPost, cutoff time.Time) bool {
	if cutoff.IsZero() {
		return true
	}
	created := p.created()
	if created.IsZero() {
		return false
	}
	return created.After(cutoff)
}

// rdAgo renders an RFC3339 timestamp as a compact relative age.
func rdAgo(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// rdWrap soft-wraps prose to width columns, preserving existing line breaks.
func rdWrap(s string, width int) string {
	if width <= 0 {
		width = 88
	}
	var out strings.Builder
	for i, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		col := 0
		for j, word := range strings.Fields(line) {
			if j > 0 {
				if col+1+len(word) > width {
					out.WriteByte('\n')
					col = 0
				} else {
					out.WriteByte(' ')
					col++
				}
			}
			out.WriteString(word)
			col += len(word)
		}
	}
	return out.String()
}

// rdOpenMirrorStore opens the local store so the shared migration path runs.
func rdOpenMirrorStore(ctx context.Context, dbPath string) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return db, nil
}

// rdSortByUpvotes orders posts by upvotes desc, breaking ties on recency so
// the ranking is deterministic across runs.
func rdSortByUpvotes(posts []rdPost) {
	sort.Slice(posts, func(i, j int) bool {
		if posts[i].UpvoteCount != posts[j].UpvoteCount {
			return posts[i].UpvoteCount > posts[j].UpvoteCount
		}
		return posts[i].CreatedAt > posts[j].CreatedAt
	})
}

// rdCleanBody normalizes API prose for terminal display.
func rdCleanBody(s string) string { return cliutil.CleanText(s) }
