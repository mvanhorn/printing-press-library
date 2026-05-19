// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `posts-comments` command. Fetches actual comment content from
// Naver's public cbox endpoint, using the comments-info endpoint to resolve
// the numeric blogNo needed for the cbox objectId.

package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/commentapi"
)

type commentTreeNode struct {
	commentapi.Comment
	Replies []*commentTreeNode `json:"replies,omitempty"`
}

func newPostsCommentsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit      int
		flagAll        bool
		flagIncludeRaw bool
		flagTree       bool
	)

	cmd := &cobra.Command{
		Use:   "posts-comments <url>",
		Short: "Fetch the actual comments on a Naver Blog post.",
		Long: `Fetch actual comment content for a single Naver Blog post through Naver's public cbox endpoint.

Accepts either two positional args (blog_id log_no) or a single URL in any of:
  - https://m.blog.naver.com/<blog_id>/<log_no>
  - https://blog.naver.com/<blog_id>/<log_no>
  - https://blog.naver.com/PostView.naver?blogId=<id>&logNo=<n>

The output is a flat array by default. Nested replies are inlined and carry
reply_level plus parent_comment_no. Pass --tree to reconstruct a replies array
under each parent comment.

reply_level is 1-indexed against Naver's cbox convention: top-level comments
have reply_level=1, first-level replies have reply_level=2, and so on. This
matches Naver's wire format rather than re-numbering to a 0-indexed shape.

Nested reply bodies are returned inline in the same response as their parent
top-level comment (one cbox call covers the whole thread). reply_count and
reply_all_count on each comment let you cross-check that every claimed reply
came through.`,
		Example: `  naver-blog-pp-cli posts-comments https://m.blog.naver.com/perfect62/224286416663 --json
  naver-blog-pp-cli posts-comments perfect62 224286416663 --all --select user_name,contents,reg_time_utc,sympathy_count
  naver-blog-pp-cli posts-comments perfect62 224286416663 --tree --json`,
		Annotations: map[string]string{
			"pp:endpoint":         "posts.comments",
			"pp:method":           "GET",
			"pp:path":             "https://apis.naver.com/commentBox/cbox/web_naver_list_jsonp.json",
			"pp:typed-exit-codes": "0,4",
			"mcp:read-only":       "true",
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
			if flagLimit <= 0 {
				return usageErr(fmt.Errorf("--limit must be positive"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			pageSize := flagLimit
			if pageSize > 100 {
				pageSize = 100
			}
			comments, _, err := commentapi.GetComments(ctx, c.HTTPClient, blogID, logNo, commentapi.GetOptions{
				PageSize: pageSize,
				All:      flagAll,
				Limiter:  c.Limiter(),
				Pacing:   200 * time.Millisecond,
			})
			if err != nil {
				return classifyCommentAPIError(err, flags)
			}
			if len(comments) > flagLimit {
				comments = comments[:flagLimit]
			}
			if !flagIncludeRaw {
				stripCommentRaw(comments)
			}
			if flagTree {
				return printJSONFiltered(cmd.OutOrStdout(), buildCommentTree(comments), flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), comments, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max number of comments to return")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch every page until exhausted")
	cmd.Flags().BoolVar(&flagIncludeRaw, "include-raw", false, "Include original HTML in contents_raw")
	cmd.Flags().BoolVar(&flagTree, "tree", false, "Return nested replies under their parent comments")
	return cmd
}

func stripCommentRaw(comments []commentapi.Comment) {
	for i := range comments {
		comments[i].ContentsRaw = ""
	}
}

func buildCommentTree(comments []commentapi.Comment) []*commentTreeNode {
	nodes := make(map[string]*commentTreeNode, len(comments))
	order := make([]string, 0, len(comments))
	for _, c := range comments {
		node := &commentTreeNode{Comment: c}
		nodes[c.CommentNo] = node
		order = append(order, c.CommentNo)
	}
	roots := make([]*commentTreeNode, 0, len(comments))
	for _, id := range order {
		node := nodes[id]
		if node == nil {
			continue
		}
		parentNo := node.ParentCommentNo
		if parentNo != "" && parentNo != node.CommentNo {
			if parent := nodes[parentNo]; parent != nil {
				parent.Replies = append(parent.Replies, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots
}

func classifyCommentAPIError(err error, flags *rootFlags) error {
	var unsuccessful *commentapi.UnsuccessfulError
	if errors.As(err, &unsuccessful) {
		classified := &cliError{code: 4, err: err}
		writeAPIErrorEnvelope(flags, classified, ExitCode(classified))
		return classified
	}
	return classifyAPIError(err, flags)
}
