package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"

	"github.com/spf13/cobra"
)

// pp:data-source live
// Comments are not part of 'sync', so every leaf in this file goes straight to
// the API through flags.newClient; there is no local snapshot to read.
func newCommentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "comments",
		Short:       "List, add, edit, delete, and resolve Linear comments",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCommentsListCmd(flags))
	cmd.AddCommand(newCommentsAddCmd(flags))
	cmd.AddCommand(newCommentsEditCmd(flags))
	cmd.AddCommand(newCommentsDeleteCmd(flags))
	cmd.AddCommand(newCommentsResolveCmd(flags))
	cmd.AddCommand(newCommentsUnresolveCmd(flags))
	return cmd
}

// Comment lifecycle (GAP-030). commentDelete, commentResolve, and
// commentUnresolve are all live in api-inventory.json. Without them an agent
// that opens a comment thread has no way to close it.

func newCommentsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <comment-id>",
		Short: "Delete a Linear comment",
		Long: `Delete a Linear comment via the commentDelete mutation.

Pass --ignore-missing to treat an already-deleted comment as a successful
no-op, which makes repeated cleanup runs safe. Requires confirmation unless
--yes is set.`,
		Example: `  linear-pp-cli comments delete <comment-uuid> --yes --agent
  linear-pp-cli comments delete <comment-uuid> --ignore-missing --yes --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_comment", "commentDelete", map[string]any{"input": map[string]any{"id": args[0]}})
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Delete comment %s?", args[0])); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.CommentDeleteMutation, map[string]any{"id": args[0]})
			if err != nil {
				return classifyGraphQLMutationError("commentDelete", err, flags)
			}
			entityID, err := extractDeletedEntityID(resp, "commentDelete")
			if err != nil {
				return err
			}
			return renderMutationEvent(cmd, flags, "comment_deleted", map[string]any{"entity_id": firstNonEmpty(entityID, args[0])})
		},
	}
	return cmd
}

// newCommentsResolveCmd marks a comment thread resolved by the current user.
// resolvingCommentId is a top-level optional argument on the mutation, not an
// input-object field, and names the child comment that settled the thread.
func newCommentsResolveCmd(flags *rootFlags) *cobra.Command {
	var resolvingComment string
	cmd := &cobra.Command{
		Use:   "resolve <comment-id>",
		Short: "Resolve a comment thread",
		Long: `Resolve a Linear comment thread via the commentResolve mutation.

Pass the root comment of the thread. --resolving-comment optionally names the
child comment that settled it, which is what Linear surfaces as the answer.`,
		Example: `  linear-pp-cli comments resolve <root-comment-uuid> --agent
  linear-pp-cli comments resolve <root-comment-uuid> --resolving-comment <child-comment-uuid> --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vars := map[string]any{"id": args[0]}
			if resolvingComment != "" {
				vars["resolvingCommentId"] = resolvingComment
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_resolve_comment", "commentResolve", map[string]any{"input": vars})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.CommentResolveMutation, vars)
			if err != nil {
				return classifyGraphQLMutationError("commentResolve", err, flags)
			}
			comment, err := extractMutationObject(resp, "commentResolve", "comment")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, comment, "comments")
		},
	}
	cmd.Flags().StringVar(&resolvingComment, "resolving-comment", "", "UUID of the child comment that resolves the thread")
	return cmd
}

func newCommentsUnresolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unresolve <comment-id>",
		Short:   "Reopen a previously resolved comment thread",
		Long:    "Unresolve a Linear comment thread via the commentUnresolve mutation, clearing the resolved state on the root comment.",
		Example: `  linear-pp-cli comments unresolve <root-comment-uuid> --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_unresolve_comment", "commentUnresolve", map[string]any{"input": map[string]any{"id": args[0]}})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.CommentUnresolveMutation, map[string]any{"id": args[0]})
			if err != nil {
				return classifyGraphQLMutationError("commentUnresolve", err, flags)
			}
			comment, err := extractMutationObject(resp, "commentUnresolve", "comment")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, comment, "comments")
		},
	}
	return cmd
}

// positionalIssueRef folds a positional issue argument into whatever --issue
// carried. `comments list` and `comments add` both accept either shape, so the
// two verbs share one rule and one message instead of drifting apart: equal
// values are fine, a genuine disagreement is a usage error, and an absent
// positional leaves the flag alone.
func positionalIssueRef(args []string, flagValue string) (string, error) {
	if len(args) == 0 {
		return flagValue, nil
	}
	if flagValue != "" && flagValue != args[0] {
		return "", usageErr(fmt.Errorf("conflicting issue references: positional %q vs --issue %q; pass just one", args[0], flagValue))
	}
	return args[0], nil
}

func newCommentsListCmd(flags *rootFlags) *cobra.Command {
	var issue string
	var after string
	var limit int
	cmd := &cobra.Command{
		Use:   "list [issue]",
		Short: "List comments on an issue",
		Long: `List comments on an issue. The issue accepts an identifier (ENG-123) or UUID,
positionally or via --issue; the positional form is the preferred agent shape.`,
		Example: `  linear-pp-cli comments list ENG-123 --agent
  linear-pp-cli comments list --issue ENG-123 --limit 100 --select comments.id,comments.body`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedIssue, err := positionalIssueRef(args, issue)
			if err != nil {
				return err
			}
			issue = resolvedIssue
			if issue == "" {
				return usageErr(fmt.Errorf("issue is required; pass it positionally (comments list ENG-123) or via --issue"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// A dry run must not reach the network to resolve an identifier, so
			// take the reference as given and let the printed query carry it.
			issueID := issue
			if !flags.dryRun {
				issueID, err = resolveIssueID(c, issue)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
			}
			if limit <= 0 {
				limit = 50
			}
			const query = `query($issueId: String!, $first: Int!, $after: String) {
				issue(id: $issueId) {
					id identifier title
					comments(first: $first, after: $after) {
						nodes {
							id body createdAt updatedAt url quotedText
							user { id name displayName email }
							parent { id }
						}
						pageInfo { hasNextPage endCursor }
					}
				}
			}`
			var resp struct {
				Issue struct {
					ID         string `json:"id"`
					Identifier string `json:"identifier"`
					Title      string `json:"title"`
					Comments   struct {
						Nodes    []json.RawMessage `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"comments"`
				} `json:"issue"`
			}
			vars := map[string]any{"issueId": issueID, "first": limit, "after": nil}
			if after != "" {
				vars["after"] = after
			}
			if err := c.QueryInto(query, vars, &resp); err != nil {
				return classifyAPIError(err, flags)
			}
			out, err := json.Marshal(map[string]any{
				"issue": map[string]any{
					"id":         resp.Issue.ID,
					"identifier": resp.Issue.Identifier,
					"title":      resp.Issue.Title,
				},
				"comments": resp.Issue.Comments.Nodes,
				"pageInfo": resp.Issue.Comments.PageInfo,
			})
			if err != nil {
				return err
			}
			// A comment's body is the payload agents asked for, so do not strip
			// it under --agent/--compact unless the caller explicitly selects.
			return renderLivePayload(cmd, flags, out, "comments", false)
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "Issue identifier or UUID")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from pageInfo.endCursor for the next page")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum comments to return")
	return cmd
}

func newCommentsAddCmd(flags *rootFlags) *cobra.Command {
	var bodyFlag, bodyFile string
	var bodyStdin bool
	var mediaFlag []string
	var mediaPublic bool
	var quotedText string
	targets := commentTargetFlags{}
	cmd := &cobra.Command{
		Use:     "add [issue]",
		Aliases: []string{"create"},
		Short:   "Add a Linear comment",
		Long: `Add a comment to exactly one target. Use --body-file or --body-stdin
for Markdown so shell snippets, backticks, and GraphQL variables stay literal.

An issue target may be given positionally (comments add ENG-123) or with
--issue, the same shape 'comments list' takes. Every other target stays a flag,
because only the issue form is common enough to earn the argument slot.

"comments create" is accepted as a compatibility alias; "comments add" is canonical.`,
		Example: `  linear-pp-cli comments add ENG-123 --body-file /tmp/comment.md --agent
  linear-pp-cli comments add --issue ENG-123 --body-file /tmp/comment.md --agent
  linear-pp-cli comments add --issue ENG-123 --body-file /tmp/comment.md --media /tmp/screenshot.png --agent
  linear-pp-cli comments add --document-content <document-content-id> --body-stdin --agent < /tmp/comment.md`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueRef, err := positionalIssueRef(args, targets.Issue)
			if err != nil {
				return err
			}
			targets.Issue = issueRef
			body, bodySet, err := readMarkdownBody(cmd, markdownBodySpec{
				InlineFlag: "body",
				Inline:     bodyFlag,
				FileFlag:   "body-file",
				File:       bodyFile,
				StdinFlag:  "body-stdin",
				Stdin:      bodyStdin,
				Label:      "body",
			})
			if err != nil {
				return err
			}
			if !bodySet && len(mediaFlag) == 0 {
				return usageErr(fmt.Errorf("comment body is required; pass --body-file, --body-stdin, --body, or --media"))
			}
			input, err := targets.inputRaw()
			if err != nil {
				return err
			}
			if bodySet {
				input["body"] = body
			}
			if quotedText != "" {
				input["quotedText"] = quotedText
			}
			if flags.dryRun {
				out := map[string]any{"input": input}
				if len(mediaFlag) > 0 {
					out["media"] = mediaFlag
					out["media_public"] = mediaPublic
				}
				return renderMutationDryRun(cmd, flags, "would_create_comment", "commentCreate", out)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			input, err = targets.input(c)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			body, uploaded, err := uploadMediaAndAppend(c, body, mediaFlag, mediaPublic)
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			input["body"] = body
			if quotedText != "" {
				input["quotedText"] = quotedText
			}
			const mutation = `mutation($input: CommentCreateInput!) {
				commentCreate(input: $input) {
					success
					comment {
						id body createdAt updatedAt url quotedText
						user { id name displayName email }
						issue { id identifier title }
						documentContent { id }
						project { id name }
						initiative { id name }
						parent { id }
					}
				}
			}`
			resp, err := c.Mutate(mutation, map[string]any{"input": input})
			if err != nil {
				return classifyMutationError("commentCreate", err, flags, uploaded)
			}
			comment, err := extractMutationObject(resp, "commentCreate", "comment")
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			return renderLiveObject(cmd, flags, comment, "comments")
		},
	}
	targets.bind(cmd)
	cmd.Flags().StringVar(&bodyFlag, "body", "", "Comment body markdown")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read comment body markdown from file")
	cmd.Flags().BoolVar(&bodyStdin, "body-stdin", false, "Read comment body markdown from stdin")
	cmd.Flags().StringSliceVar(&mediaFlag, "media", nil, "Upload file and append it to the comment markdown (repeatable)")
	cmd.Flags().BoolVar(&mediaPublic, "media-public", false, "Request public Linear asset URLs for uploaded media")
	cmd.Flags().StringVar(&quotedText, "quoted-text", "", "Original selected text for an inline comment")
	return cmd
}

func newCommentsEditCmd(flags *rootFlags) *cobra.Command {
	var bodyFlag, bodyFile string
	var bodyStdin bool
	var mediaFlag []string
	var mediaPublic bool
	var quotedText string
	cmd := &cobra.Command{
		Use:   "edit <comment-id>",
		Short: "Edit a Linear comment",
		Example: `  linear-pp-cli comments edit <comment-id> --body-file /tmp/comment.md --agent
  linear-pp-cli comments edit <comment-id> --media /tmp/screenshot.png --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, bodySet, err := readMarkdownBody(cmd, markdownBodySpec{
				InlineFlag: "body",
				Inline:     bodyFlag,
				FileFlag:   "body-file",
				File:       bodyFile,
				StdinFlag:  "body-stdin",
				Stdin:      bodyStdin,
				Label:      "body",
			})
			if err != nil {
				return err
			}
			input := map[string]any{}
			if bodySet {
				input["body"] = body
			}
			if quotedText != "" {
				input["quotedText"] = quotedText
			}
			if len(input) == 0 && len(mediaFlag) == 0 {
				return usageErr(fmt.Errorf("no comment fields supplied; pass --body-file, --body-stdin, --body, --media, or --quoted-text"))
			}
			if flags.dryRun {
				out := map[string]any{"comment": args[0], "input": input}
				if len(mediaFlag) > 0 {
					out["media"] = mediaFlag
					out["media_public"] = mediaPublic
				}
				return renderMutationDryRun(cmd, flags, "would_update_comment", "commentUpdate", out)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if len(mediaFlag) > 0 && !bodySet {
				existing, err := fetchCommentLive(c, args[0])
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				var comment struct {
					Body string `json:"body"`
				}
				if err := json.Unmarshal(existing, &comment); err != nil {
					return fmt.Errorf("parsing existing comment body: %w", err)
				}
				body = comment.Body
				bodySet = true
			}
			body, uploaded, err := uploadMediaAndAppend(c, body, mediaFlag, mediaPublic)
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			input = map[string]any{}
			if bodySet {
				input["body"] = body
			}
			if quotedText != "" {
				input["quotedText"] = quotedText
			}
			if len(input) == 0 {
				return usageErr(fmt.Errorf("no comment fields supplied; pass --body-file, --body-stdin, --body, --media, or --quoted-text"))
			}
			const mutation = `mutation($id: String!, $input: CommentUpdateInput!) {
				commentUpdate(id: $id, input: $input) {
					success
					comment {
						id body createdAt updatedAt url quotedText
						user { id name displayName email }
						issue { id identifier title }
						documentContent { id }
						project { id name }
						initiative { id name }
						parent { id }
					}
				}
			}`
			resp, err := c.Mutate(mutation, map[string]any{"id": args[0], "input": input})
			if err != nil {
				return classifyMutationError("commentUpdate", err, flags, uploaded)
			}
			comment, err := extractMutationObject(resp, "commentUpdate", "comment")
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			return renderLiveObject(cmd, flags, comment, "comments")
		},
	}
	cmd.Flags().StringVar(&bodyFlag, "body", "", "Comment body markdown")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read comment body markdown from file")
	cmd.Flags().BoolVar(&bodyStdin, "body-stdin", false, "Read comment body markdown from stdin")
	cmd.Flags().StringSliceVar(&mediaFlag, "media", nil, "Upload file and append it to the comment markdown (repeatable)")
	cmd.Flags().BoolVar(&mediaPublic, "media-public", false, "Request public Linear asset URLs for uploaded media")
	cmd.Flags().StringVar(&quotedText, "quoted-text", "", "Original selected text for an inline comment")
	return cmd
}

type commentTargetFlags struct {
	Issue            string
	DocumentContent  string
	Parent           string
	Project          string
	ProjectUpdate    string
	Initiative       string
	InitiativeUpdate string
	Post             string
}

func (t *commentTargetFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&t.Issue, "issue", "", "Issue identifier or UUID")
	cmd.Flags().StringVar(&t.DocumentContent, "document-content", "", "Document content UUID")
	cmd.Flags().StringVar(&t.Parent, "parent", "", "Parent comment UUID")
	cmd.Flags().StringVar(&t.Project, "project", "", "Project UUID")
	cmd.Flags().StringVar(&t.ProjectUpdate, "project-update", "", "Project update UUID")
	cmd.Flags().StringVar(&t.Initiative, "initiative", "", "Initiative UUID")
	cmd.Flags().StringVar(&t.InitiativeUpdate, "initiative-update", "", "Initiative update UUID")
	cmd.Flags().StringVar(&t.Post, "post", "", "Post UUID")
}

type graphqlQueryer interface {
	QueryInto(string, map[string]any, any) error
}

func (t commentTargetFlags) input(c graphqlQueryer) (map[string]any, error) {
	input, err := t.inputRaw()
	if err != nil {
		return nil, err
	}
	if t.Issue != "" {
		issueID, err := resolveIssueID(c, t.Issue)
		if err != nil {
			return nil, err
		}
		input["issueId"] = issueID
	}
	return input, nil
}

func (t commentTargetFlags) inputRaw() (map[string]any, error) {
	values := []struct {
		key   string
		value string
	}{
		{"issueId", t.Issue},
		{"documentContentId", t.DocumentContent},
		{"parentId", t.Parent},
		{"projectId", t.Project},
		{"projectUpdateId", t.ProjectUpdate},
		{"initiativeId", t.Initiative},
		{"initiativeUpdateId", t.InitiativeUpdate},
		{"postId", t.Post},
	}
	count := 0
	input := map[string]any{}
	for _, v := range values {
		if v.value == "" {
			continue
		}
		count++
		input[v.key] = v.value
	}
	if count != 1 {
		return nil, usageErr(fmt.Errorf("pass exactly one comment target: --issue, --document-content, --parent, --project, --project-update, --initiative, --initiative-update, or --post"))
	}
	return input, nil
}

func fetchCommentLive(c graphqlQueryer, id string) (json.RawMessage, error) {
	const query = `query($id: String!) {
		comment(id: $id) {
			id body createdAt updatedAt url quotedText
			user { id name displayName email }
			issue { id identifier title }
			documentContent { id }
			project { id name }
			initiative { id name }
			parent { id }
		}
	}`
	var resp struct {
		Comment json.RawMessage `json:"comment"`
	}
	if err := c.QueryInto(query, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Comment) == 0 || string(resp.Comment) == "null" {
		return nil, notFoundErr(fmt.Errorf("comment %q not found", id))
	}
	return resp.Comment, nil
}

func extractMutationObject(resp json.RawMessage, mutationKey, objectKey string) (json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(resp, &root); err != nil {
		return nil, fmt.Errorf("parsing %s response: %w", mutationKey, err)
	}
	raw, ok := root[mutationKey]
	if !ok {
		return nil, fmt.Errorf("%s response missing %q", mutationKey, mutationKey)
	}
	var payload struct {
		Success bool `json:"success"`
	}
	var payloadMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payloadMap); err != nil {
		return nil, fmt.Errorf("parsing %s payload: %w", mutationKey, err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parsing %s success: %w", mutationKey, err)
	}
	if !payload.Success {
		return nil, apiErr(fmt.Errorf("Linear reported %s success=false", mutationKey))
	}
	obj, ok := payloadMap[objectKey]
	if !ok || len(obj) == 0 || string(obj) == "null" {
		return nil, fmt.Errorf("%s response missing %q", mutationKey, objectKey)
	}
	return obj, nil
}
