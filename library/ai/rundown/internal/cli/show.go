// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// rdShowComment is one comment as rendered by `show`.
type rdShowComment struct {
	ID          string `json:"id"`
	Author      string `json:"author"`
	Body        string `json:"body"`
	UpvoteCount int    `json:"upvoteCount"`
	CreatedAt   string `json:"createdAt"`
	Age         string `json:"age"`
	IsReply     bool   `json:"isReply"`
}

// rdShowResult is the full workflow document `show` emits.
type rdShowResult struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Author       string          `json:"author"`
	AuthorLevel  string          `json:"authorLevel,omitempty"`
	Location     string          `json:"location,omitempty"`
	UpvoteCount  int             `json:"upvoteCount"`
	CommentCount int             `json:"commentCount"`
	CreatedAt    string          `json:"createdAt"`
	Age          string          `json:"age"`
	Tools        []string        `json:"tools"`
	Industries   []string        `json:"industries"`
	Tags         []string        `json:"tags"`
	Featured     bool            `json:"newsletterFeatured"`
	URL          string          `json:"url"`
	Comments     []rdShowComment `json:"comments"`
	Source       string          `json:"source"`
	ScannedPosts int             `json:"scannedPosts,omitempty"`
	Note         string          `json:"note,omitempty"`
}

func newNovelShowCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDBPath      string
		flagNoComments  bool
		flagMaxScanPage int
	)

	cmd := &cobra.Command{
		Use:   "show <post-id>",
		Short: "Print one workflow in full, with its tools, industries, author and comments",
		Long: strings.Trim(`
Print a single community workflow end to end.

The community API has no single-post endpoint, so this command reads the post
from the local mirror when it is synced, and otherwise scans the live feed to
find it. Comments come from the mirror when present and from the live comments
endpoint otherwise.

Use this command after 'use-cases' or 'top' to actually read a workflow rather
than a truncated feed card.
`, "\n"),
		Example: strings.Trim(`
  rundown-pp-cli show 89da5324-f822-4a4b-a30e-b33cfac60a95
  rundown-pp-cli show 89da5324-f822-4a4b-a30e-b33cfac60a95 --no-comments
  rundown-pp-cli show 89da5324-f822-4a4b-a30e-b33cfac60a95 --agent --select title,tools
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "post-id=89da5324-f822-4a4b-a30e-b33cfac60a95",
			"pp:data-source": "auto",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "show")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a post id is required, e.g. rundown-pp-cli show <post-id>"))
			}
			postID := strings.TrimSpace(args[0])
			if postID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("the post id must not be blank"))
			}
			if flagMaxScanPage <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--max-scan-pages must be greater than zero"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var (
				post     *rdPost
				comments []rdComment
				source   = "live"
				scanned  int
			)

			// --- prefer the local mirror ---
			dbPath := rdResolveDBPath(flagDBPath)
			if _, statErr := os.Stat(dbPath); statErr == nil {
				db, err := rdOpenMirrorStore(ctx, dbPath)
				if err == nil {
					defer db.Close()
					posts, err := rdLoadPosts(ctx, db)
					if err == nil {
						for i := range posts {
							if posts[i].ID == postID {
								post = &posts[i]
								source = "local"
								break
							}
						}
					}
					if post != nil && !flagNoComments {
						if local, err := rdLoadComments(ctx, db, postID); err == nil {
							comments = local
						}
					}
				}
			}

			// --- fall back to scanning the live feed ---
			if post == nil {
				found, n, err := rdFindPostLive(ctx, flags, postID, flagMaxScanPage)
				scanned = n
				if err != nil {
					return err
				}
				if found == nil {
					return notFoundErr(fmt.Errorf(
						"post %s not found after scanning %d live posts across up to %d pages; check the id, or raise --max-scan-pages",
						postID, n, flagMaxScanPage))
				}
				post = found
			}

			// --- comments: live only when the mirror has none ---
			//
			// Deliberately not keyed on post.CommentCount. That field routinely
			// exceeds what the comments endpoint actually serves (a post
			// reporting 21 returns 9 top-level + 7 replies and no nextCursor,
			// with every replyCount matching its replies exactly). Refetching on
			// a shortfall would fire a wasted call on most posts and still not
			// close the gap, because the missing comments are not served at all.
			// The gap is disclosed in the output instead.
			if !flagNoComments && len(comments) == 0 && post.CommentCount > 0 {
				if live, err := rdFetchComments(ctx, flags, postID); err == nil {
					comments = live
				}
			}

			tools := post.toolSlugs()
			industries := make([]string, 0, len(post.Industries))
			for _, ind := range post.Industries {
				industries = append(industries, ind.Slug)
			}
			shown := make([]rdShowComment, 0, len(comments))
			for _, c := range comments {
				shown = append(shown, rdShowComment{
					ID:          c.ID,
					Author:      strings.TrimSpace(c.Author.DisplayName),
					Body:        rdCleanBody(c.Body),
					UpvoteCount: c.UpvoteCount,
					CreatedAt:   c.CreatedAt,
					Age:         rdAgo(c.CreatedAt),
					IsReply:     strings.TrimSpace(c.ParentCommentID) != "",
				})
			}

			result := rdShowResult{
				ID:           post.ID,
				Title:        post.Title,
				Body:         rdCleanBody(post.Body),
				Author:       post.authorName(),
				AuthorLevel:  post.Author.Level,
				Location:     post.Author.Location,
				UpvoteCount:  post.UpvoteCount,
				CommentCount: post.CommentCount,
				CreatedAt:    post.CreatedAt,
				Age:          rdAgo(post.CreatedAt),
				Tools:        tools,
				Industries:   industries,
				Tags:         post.Tags,
				Featured:     post.featured(),
				URL:          rdPostURL(post.ID),
				Comments:     shown,
				Source:       source,
				ScannedPosts: scanned,
			}
			notes := make([]string, 0, 2)
			if source == "live" {
				notes = append(notes, "read from the live API; run 'rundown-pp-cli sync' to make this instant and offline.")
			}
			// Never let a shortfall pass silently: an agent comparing the two
			// numbers should be told the API withholds the difference rather
			// than concluding this command dropped them.
			if !flagNoComments && len(shown) < post.CommentCount {
				notes = append(notes, fmt.Sprintf(
					"the API reports %d comments but its comments endpoint serves only %d, so %d are withheld upstream (deleted or moderated); all %d served are shown here.",
					post.CommentCount, len(shown), post.CommentCount-len(shown), len(shown)))
			}
			result.Note = strings.Join(notes, " ")

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s\n", bold(result.Title))
			meta := fmt.Sprintf("%d upvotes · %d comments · %s · %s",
				result.UpvoteCount, result.CommentCount, result.Age, result.Author)
			if result.Location != "" {
				meta += " (" + result.Location + ")"
			}
			if result.Featured {
				meta += " · featured in the newsletter"
			}
			fmt.Fprintf(out, "%s\n%s\n\n", meta, result.URL)

			if len(result.Tools) > 0 {
				fmt.Fprintf(out, "Tools:      %s\n", strings.Join(result.Tools, ", "))
			}
			if len(result.Industries) > 0 {
				fmt.Fprintf(out, "Industries: %s\n", strings.Join(result.Industries, ", "))
			}
			if len(result.Tags) > 0 {
				fmt.Fprintf(out, "Tags:       %s\n", strings.Join(result.Tags, ", "))
			}
			fmt.Fprintf(out, "\n%s\n", rdWrap(result.Body, 88))

			if len(result.Comments) > 0 {
				fmt.Fprintf(out, "\n%s\n", bold(fmt.Sprintf("Comments (%d)", len(result.Comments))))
				for _, c := range result.Comments {
					prefix := "  "
					if c.IsReply {
						prefix = "    ↳ "
					}
					author := c.Author
					if author == "" {
						author = "anonymous"
					}
					fmt.Fprintf(out, "\n%s%s · %s · %d upvotes\n", prefix, author, c.Age, c.UpvoteCount)
					fmt.Fprintf(out, "%s\n", rdWrap(c.Body, 84))
				}
			}
			if result.Note != "" {
				fmt.Fprintf(out, "\n%s\n", result.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDBPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().BoolVar(&flagNoComments, "no-comments", false, "Skip the comment thread")
	cmd.Flags().IntVar(&flagMaxScanPage, "max-scan-pages", 8, "Maximum live feed pages to scan when the post is not in the local mirror")
	return cmd
}

// rdFindPostLive scans the live feed for a post id, since the API exposes no
// single-post endpoint. Returns the post (or nil), and how many posts it read.
func rdFindPostLive(ctx context.Context, flags *rootFlags, postID string, maxPages int) (*rdPost, int, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, 0, err
	}
	cursor := ""
	scanned := 0
	for page := 0; page < maxPages; page++ {
		params := map[string]string{
			"sort":  "new",
			"limit": strconv.Itoa(50),
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, err := c.Get(ctx, "/posts", params)
		if err != nil {
			return nil, scanned, err
		}
		var envelope struct {
			Posts      []rdPost `json:"posts"`
			NextCursor string   `json:"nextCursor"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, scanned, fmt.Errorf("parsing feed response: %w", err)
		}
		for i := range envelope.Posts {
			scanned++
			if envelope.Posts[i].ID == postID {
				return &envelope.Posts[i], scanned, nil
			}
		}
		if envelope.NextCursor == "" {
			break
		}
		cursor = envelope.NextCursor
	}
	return nil, scanned, nil
}

// rdFetchComments reads a post's comment thread from the live API.
func rdFetchComments(ctx context.Context, flags *rootFlags, postID string) ([]rdComment, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	// Follow nextCursor: a busy thread spans several pages, and stopping at the
	// first one renders fewer comments than the post's own commentCount without
	// saying so.
	const maxCommentPages = 20
	cursor := ""
	all := make([]rdComment, 0, 32)
	for page := 0; page < maxCommentPages; page++ {
		params := map[string]string{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, err := c.Get(ctx, "/posts/"+postID+"/comments", params)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Comments   []rdComment `json:"comments"`
			NextCursor string      `json:"nextCursor"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parsing comments response: %w", err)
		}
		all = append(all, envelope.Comments...)
		if envelope.NextCursor == "" {
			break
		}
		cursor = envelope.NextCursor
	}
	return rdFlattenComments(all), nil
}
