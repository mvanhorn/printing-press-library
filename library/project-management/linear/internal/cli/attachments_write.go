package cli

// attachments_write.go closes GAP-045 (the attachments family was read-only:
// one promoted get leaf, no mutation, and no URL-keyed lookup) and GAP-049
// (reactionCreate and reactionDelete had no surface at all). The two rows ship
// together because reactions are the same size as a single attachment leaf and
// share every helper below.
//
// The attachment leaves are registered on the promoted `attachments` parent in
// promoted_attachments.go. Reactions have no natural parent in the tree, so
// `reactions` is a new top-level command and is the single AddCommand line
// added to root.go.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// attachmentIssueRef trims an --issue value and rejects an empty one.
// AttachmentCreateInput.issueId and the attachmentLinkURL issueId argument both
// document that they accept either a UUID or an issue identifier such as
// LIN-123, so the value goes to Linear verbatim and no resolution round trip is
// spent here.
func attachmentIssueRef(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", usageErr(fmt.Errorf("--issue is required (an issue UUID or identifier such as LIN-123)"))
	}
	return value, nil
}

// attachmentID validates the positional id the update and delete mutations
// take. Both are typed String! and key on the attachment's own UUID, so a
// mistyped issue identifier is caught here as a usage error instead of costing
// a round trip and returning a code-5 API error.
func attachmentID(args []string) (string, error) {
	if len(args) == 0 {
		return "", usageErr(fmt.Errorf("<attachment-id> is required"))
	}
	id := strings.TrimSpace(args[0])
	if !store.IsUUID(id) {
		return "", usageErr(fmt.Errorf("<attachment-id> expects an attachment UUID, got %q (list them with 'attachments for-url' or read the issue)", id))
	}
	return id, nil
}

// pp:data-source live
// Every leaf in this file is a GraphQL query or mutation against Linear. The
// only store reference here is store.IsUUID, a pure validator that reads
// nothing, so none of these commands has a local snapshot to serve from.
func newAttachmentsForURLCmd(flags *rootFlags) *cobra.Command {
	var url, after string
	var limit int
	var includeArchived bool
	cmd := &cobra.Command{
		Use:         "for-url",
		Short:       "List the attachments recorded against a URL",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List every attachment Linear holds for one URL, across issues, through the
attachmentsForURL query.

This is the dedupe primitive to call before creating. Both attachmentCreate and
attachmentLinkURL key on the url and issueId pair, so the same URL can
legitimately sit on several issues, and a create against a pair that already
exists updates that record rather than adding a second one.

attachmentsForURL is also the supported replacement for the deprecated
attachmentIssue query, which is why that one has no command here.`,
		Example: `  linear-pp-cli attachments for-url --url https://github.com/acme/api/pull/42 --agent
  linear-pp-cli attachments for-url --url https://example.com/ticket/9 --include-archived --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(url) == "" {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("--url is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if limit <= 0 {
				limit = 50
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"url": url, "first": limit, "after": nil, "includeArchived": includeArchived}
			if after != "" {
				vars["after"] = after
			}
			var resp struct {
				AttachmentsForURL struct {
					Nodes    []map[string]any `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"attachmentsForURL"`
			}
			if err := c.QueryInto(client.AttachmentsForURLQuery, vars, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			out, err := json.Marshal(map[string]any{
				"url":         url,
				"attachments": resp.AttachmentsForURL.Nodes,
				"pageInfo":    resp.AttachmentsForURL.PageInfo,
			})
			if err != nil {
				return err
			}
			// The attachments array is the whole answer this command exists to
			// give, and `attachments` is a key compactObjectFields strips, so the
			// compact projection must not run here or --agent reads back
			// {pageInfo, url} and concludes the URL carries nothing. Rendered the
			// same way `comments list` renders its nested array. A caller who
			// wants a narrower shape passes --select.
			return renderLivePayload(cmd, flags, out, "attachments", false)
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "The attachment URL to look up")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from pageInfo.endCursor for the next page")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum attachments to return")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "Include attachments on archived issues")
	return cmd
}

func newAttachmentsCreateCmd(flags *rootFlags) *cobra.Command {
	var issue, url, title, subtitle, iconURL, commentBody string
	var metadata, metadataFile string
	var groupBySource bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an attachment card on an issue",
		Long: `Create an attachment via the attachmentCreate mutation.
AttachmentCreateInput requires title, url, and issueId, and --issue accepts an
issue UUID or an identifier such as LIN-123.

Use "create" when you own the card's contents: --metadata carries arbitrary
string and number values that render on the card, which is the only way to put
your own structured payload on an issue. Use "link-url" instead when you want
Linear to interpret the URL, because that mutation is the one that produces a
rich integration attachment with status sync for a recognised provider.

attachmentCreate is an upsert: submitting a url and issueId pair that already
has an attachment updates that record rather than creating a second one, so this
command is naturally idempotent and never needs --idempotent. Run
"attachments for-url" first to see what a URL already carries.

AttachmentCreateInput also accepts createAsUser and displayIconUrl. Both are
restricted to OAuth applications operating in actor=app mode, which this
CLI's API-key auth is not, so neither is exposed. commentBodyData is the
internal Prosemirror twin of --comment-body and is likewise absent.`,
		Example: `  linear-pp-cli attachments create --issue LIN-123 --url https://example.com/run/9 --title "CI run 9" --agent
  linear-pp-cli attachments create --issue LIN-123 --url https://example.com/run/9 --title "CI run 9" --metadata '{"duration":412,"result":"pass"}' --agent
  linear-pp-cli attachments create --issue LIN-123 --url https://example.com/run/9 --title "CI run 9" --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, metaSet, err := wbJSONObject("metadata", metadata, "metadata-file", metadataFile, "metadata")
			if err != nil {
				return err
			}
			if strings.TrimSpace(issue) == "" || strings.TrimSpace(url) == "" || strings.TrimSpace(title) == "" {
				if dryRunOK(flags) {
					return nil
				}
				switch {
				case strings.TrimSpace(title) == "":
					return usageErr(fmt.Errorf("--title is required (AttachmentCreateInput.title is non-null)"))
				case strings.TrimSpace(url) == "":
					return usageErr(fmt.Errorf("--url is required"))
				default:
					return usageErr(fmt.Errorf("--issue is required (an issue UUID or identifier such as LIN-123)"))
				}
			}
			issueRef, err := attachmentIssueRef(issue)
			if err != nil {
				return err
			}
			input := map[string]any{
				"issueId": issueRef,
				"url":     url,
				"title":   title,
			}
			setOptionalString(input, "subtitle", subtitle)
			setOptionalString(input, "iconUrl", iconURL)
			setOptionalString(input, "commentBody", commentBody)
			if metaSet {
				input["metadata"] = meta
			}
			if cmd.Flags().Changed("group-by-source") {
				input["groupBySource"] = groupBySource
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_create_attachment", "attachmentCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.AttachmentCreateMutation, map[string]any{"input": input})
			if err != nil {
				return wbClassifyCreateError("attachmentCreate", err, flags)
			}
			attachment, err := extractMutationObject(resp, "attachmentCreate", "attachment")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, attachment, "attachments")
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "Issue UUID or identifier such as LIN-123")
	cmd.Flags().StringVar(&url, "url", "", "Attachment URL, which is also its dedupe key within the issue")
	cmd.Flags().StringVar(&title, "title", "", "Attachment title (required)")
	cmd.Flags().StringVar(&subtitle, "subtitle", "", "Attachment subtitle")
	cmd.Flags().StringVar(&iconURL, "icon-url", "", "Icon URL shown on the card (jpg or png, 20x20px renders best)")
	cmd.Flags().StringVar(&metadata, "metadata", "", "Attachment metadata as an inline JSON object of string and number values")
	cmd.Flags().StringVar(&metadataFile, "metadata-file", "", "Read the metadata JSON object from a file")
	cmd.Flags().StringVar(&commentBody, "comment-body", "", "Also post a linked comment with this markdown body")
	cmd.Flags().BoolVar(&groupBySource, "group-by-source", false, "Group attachments from the same source application in the Linear UI")
	return cmd
}

func newAttachmentsLinkURLCmd(flags *rootFlags) *cobra.Command {
	var issue, url, title string
	cmd := &cobra.Command{
		Use:   "link-url",
		Short: "Link a URL to an issue and let Linear unfurl it",
		Long: `Link a URL via the attachmentLinkURL mutation. Linear decides the resulting
attachment's shape: when the workspace has a matching integration configured
and recognises the URL (Zendesk, GitHub, Slack and friends) the result is a
rich attachment that carries integration features such as automated status
updates, and otherwise it is a basic one.

That is the whole difference from "attachments create". Create builds the card
you describe, including arbitrary --metadata, and never consults an
integration. link-url hands the URL to Linear and takes whatever card Linear
decides the URL deserves, which is why it accepts no metadata and why the
mutation types --title as optional here while create requires it.

Unlike attachmentCreate this mutation takes flat arguments rather than an input
object. Its createAsUser and displayIconUrl arguments are restricted to OAuth
applications in actor=app mode and are not exposed. Provider-specific siblings
(attachmentLinkGitHubPR, attachmentLinkSlack, attachmentLinkZendesk and the
rest) are deliberately absent: they need provider ids this CLI does not carry,
and this mutation reaches the same providers whenever their integration is
configured and the URL is one Linear recognises.`,
		Example: `  linear-pp-cli attachments link-url --issue LIN-123 --url https://github.com/acme/api/pull/42 --agent
  linear-pp-cli attachments link-url --issue LIN-123 --url https://example.com/ticket/9 --title "Support ticket 9" --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(issue) == "" || strings.TrimSpace(url) == "" {
				if dryRunOK(flags) {
					return nil
				}
				if strings.TrimSpace(url) == "" {
					return usageErr(fmt.Errorf("--url is required"))
				}
				return usageErr(fmt.Errorf("--issue is required (an issue UUID or identifier such as LIN-123)"))
			}
			issueRef, err := attachmentIssueRef(issue)
			if err != nil {
				return err
			}
			vars := map[string]any{"url": url, "issueId": issueRef, "title": nil}
			if strings.TrimSpace(title) != "" {
				vars["title"] = title
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_link_url", "attachmentLinkURL", vars)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.AttachmentLinkURLMutation, vars)
			if err != nil {
				return wbClassifyCreateError("attachmentLinkURL", err, flags)
			}
			attachment, err := extractMutationObject(resp, "attachmentLinkURL", "attachment")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, attachment, "attachments")
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "Issue UUID or identifier such as LIN-123")
	cmd.Flags().StringVar(&url, "url", "", "The URL to link")
	cmd.Flags().StringVar(&title, "title", "", "Title for the attachment (optional: Linear unfurls one when omitted)")
	return cmd
}

func newAttachmentsUpdateCmd(flags *rootFlags) *cobra.Command {
	var title, subtitle, iconURL string
	var metadata, metadataFile string
	cmd := &cobra.Command{
		Use:     "update <attachment-id>",
		Aliases: []string{"edit"},
		Short:   "Update an attachment's title, subtitle, metadata, or icon",
		Long: `Edit an attachment via the attachmentUpdate mutation.

AttachmentUpdateInput carries exactly four fields, title, subtitle, metadata,
and iconUrl, and title is non-null, so --title is required on every update even
when it is unchanged. Read the current value with "attachments <id>" first.
Neither url nor issueId is updatable: move an attachment by deleting it and
creating it again on the target issue.

--metadata is sent as the whole metadata value rather than as a patch, so
include every key you want the attachment to keep.`,
		Example: `  linear-pp-cli attachments update <attachment-uuid> --title "CI run 9 (rerun)" --agent
  linear-pp-cli attachments update <attachment-uuid> --title "CI run 9" --metadata-file /tmp/meta.json --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, metaSet, err := wbJSONObject("metadata", metadata, "metadata-file", metadataFile, "metadata")
			if err != nil {
				return err
			}
			if len(args) == 0 || strings.TrimSpace(title) == "" {
				if dryRunOK(flags) {
					return nil
				}
				if len(args) == 0 {
					return usageErr(fmt.Errorf("<attachment-id> is required"))
				}
				return usageErr(fmt.Errorf("--title is required (AttachmentUpdateInput.title is non-null, so restate it even when unchanged)"))
			}
			id, err := attachmentID(args)
			if err != nil {
				return err
			}
			input := map[string]any{"title": title}
			setChangedString(cmd, input, "subtitle", "subtitle", subtitle)
			setChangedString(cmd, input, "icon-url", "iconUrl", iconURL)
			if metaSet {
				input["metadata"] = meta
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_attachment", "attachmentUpdate", map[string]any{"id": id, "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.AttachmentUpdateMutation, map[string]any{"id": id, "input": input})
			if err != nil {
				return classifyMutationError("attachmentUpdate", err, flags, nil)
			}
			attachment, err := extractMutationObject(resp, "attachmentUpdate", "attachment")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, attachment, "attachments")
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Attachment title (required, even when unchanged)")
	cmd.Flags().StringVar(&subtitle, "subtitle", "", "Attachment subtitle (pass empty to clear)")
	cmd.Flags().StringVar(&iconURL, "icon-url", "", "Icon URL shown on the card (pass empty to clear)")
	cmd.Flags().StringVar(&metadata, "metadata", "", "Replacement metadata as an inline JSON object")
	cmd.Flags().StringVar(&metadataFile, "metadata-file", "", "Read the replacement metadata JSON object from a file")
	return cmd
}

func newAttachmentsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <attachment-id>",
		Short: "Delete an attachment",
		Long: `Delete an attachment via the attachmentDelete mutation, which takes the
attachment's own UUID. The command is scoped to that one attachment: the issue
keeps its others.

Deletion is confirmed interactively unless --yes is passed. --agent implies
--yes. With --ignore-missing an already-deleted attachment exits 0 as a no-op,
which makes a repeated cleanup run safe.`,
		Example: `  linear-pp-cli attachments delete <attachment-uuid> --yes --agent
  linear-pp-cli attachments delete <attachment-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<attachment-id> is required"))
			}
			id, err := attachmentID(args)
			if err != nil {
				return err
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_attachment", "attachmentDelete", map[string]any{"id": id})
			}
			if err := wbConfirm(cmd, flags, fmt.Sprintf("Delete attachment %s", id)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.AttachmentDeleteMutation, map[string]any{"id": id})
			if err != nil {
				return wbClassifyDeleteError("attachmentDelete", err, flags)
			}
			deleted, err := wbDecodeDeletePayload(resp, "attachmentDelete")
			if err != nil {
				return err
			}
			return wbRenderMutationEvent(cmd, flags, "attachment_deleted", map[string]any{"id": firstNonEmpty(deleted, id)},
				fmt.Sprintf("Deleted attachment %s", firstNonEmpty(deleted, id)))
		},
	}
	return cmd
}

// Reactions (GAP-049). reactionCreate and reactionDelete are the cheapest
// acknowledgement primitive Linear offers, and nothing in the tree owns them:
// a reaction hangs off a comment, an issue, a project update, or an initiative
// update, so none of those parents has a better claim than the others. Hence a
// new top-level command.

func newReactionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reactions",
		Short:       "List, add, and remove emoji reactions",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReactionsListCmd(flags))
	cmd.AddCommand(newReactionsAddCmd(flags))
	cmd.AddCommand(newReactionsRemoveCmd(flags))
	return cmd
}

// reactionTargetFlags carries the four public target ids ReactionCreateInput
// accepts. postId, pullRequestId, and pullRequestCommentId are marked
// [Internal] in the schema and are not exposed.
type reactionTargetFlags struct {
	comment          string
	issue            string
	projectUpdate    string
	initiativeUpdate string
}

func (t *reactionTargetFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&t.comment, "comment", "", "Comment UUID to react to")
	cmd.Flags().StringVar(&t.issue, "issue", "", "Issue UUID or identifier such as LIN-123 to react to")
	cmd.Flags().StringVar(&t.projectUpdate, "project-update", "", "Project update UUID to react to")
	cmd.Flags().StringVar(&t.initiativeUpdate, "initiative-update", "", "Initiative update UUID to react to")
}

// resolve returns the ReactionCreateInput field name and the value for the one
// target the caller named. Exactly one is required: the input object types all
// four as optional, so a call with none or several is a usage error this CLI
// catches rather than a shape Linear has to reject.
func (t *reactionTargetFlags) resolve() (string, string, error) {
	named := map[string]string{}
	for field, value := range map[string]string{
		"commentId":          t.comment,
		"issueId":            t.issue,
		"projectUpdateId":    t.projectUpdate,
		"initiativeUpdateId": t.initiativeUpdate,
	} {
		if strings.TrimSpace(value) != "" {
			named[field] = strings.TrimSpace(value)
		}
	}
	if len(named) != 1 {
		return "", "", usageErr(fmt.Errorf("pass exactly one target: --comment, --issue, --project-update, or --initiative-update"))
	}
	for field, value := range named {
		return field, value, nil
	}
	return "", "", nil
}

// empty reports whether no target flag was set, which lets --dry-run render
// nothing instead of erroring when the caller is only inspecting the shape.
func (t *reactionTargetFlags) empty() bool {
	return strings.TrimSpace(t.comment) == "" &&
		strings.TrimSpace(t.issue) == "" &&
		strings.TrimSpace(t.projectUpdate) == "" &&
		strings.TrimSpace(t.initiativeUpdate) == ""
}

func newReactionsListCmd(flags *rootFlags) *cobra.Command {
	target := reactionTargetFlags{}
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the reactions on a comment, issue, project update, or initiative update",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List reactions through the target's own reactions field.

reactionDelete keys on a reaction's own id, and nothing else in this CLI
surfaces one, so this is the command that makes "reactions remove" usable: read
the id here, then remove it.

Comment.reactions, Issue.reactions, ProjectUpdate.reactions, and
InitiativeUpdate.reactions are all plain lists with no pagination arguments, so
there is no --limit and no cursor. Each row carries the emoji and the user who
left it.`,
		Example: `  linear-pp-cli reactions list --issue LIN-123 --agent
  linear-pp-cli reactions list --comment <comment-uuid> --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target.empty() {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass exactly one target: --comment, --issue, --project-update, or --initiative-update"))
			}
			field, value, err := target.resolve()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var query, targetKey string
			id := value
			switch field {
			case "commentId":
				query, targetKey = client.CommentReactionsQuery, "comment"
			case "issueId":
				// The root issue query takes a UUID, so an identifier is
				// resolved first. The mutation input accepts either.
				resolved, err := resolveIssueID(c, value)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				id = resolved
				query, targetKey = client.IssueReactionsQuery, "issue"
			case "projectUpdateId":
				query, targetKey = client.ProjectUpdateReactionsQuery, "projectUpdate"
			case "initiativeUpdateId":
				query, targetKey = client.InitiativeUpdateReactionsQuery, "initiativeUpdate"
			}
			var resp map[string]*struct {
				ID         string           `json:"id"`
				Identifier string           `json:"identifier"`
				Reactions  []map[string]any `json:"reactions"`
			}
			if err := c.QueryInto(query, map[string]any{"id": id}, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			node := resp[targetKey]
			if node == nil {
				return notFoundErr(fmt.Errorf("%s %s not found", targetKey, id))
			}
			targetOut := map[string]any{"kind": targetKey, "id": node.ID}
			if node.Identifier != "" {
				targetOut["identifier"] = node.Identifier
			}
			out, err := json.Marshal(map[string]any{
				"target":    targetOut,
				"reactions": node.Reactions,
			})
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, "reactions", true)
		},
	}
	target.bind(cmd)
	return cmd
}

func newReactionsAddCmd(flags *rootFlags) *cobra.Command {
	target := reactionTargetFlags{}
	var emoji string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an emoji reaction to a comment, issue, project update, or initiative update",
		Long: `Add a reaction via the reactionCreate mutation. ReactionCreateInput requires
emoji plus exactly one target id, and --issue accepts an issue UUID or an
identifier such as LIN-123.

--emoji is passed through verbatim because Linear types it as a free-form
String, not an enum. Run "reactions list" on something that already carries a
reaction to see the exact form this workspace stores, then reuse that spelling.

ReactionCreateInput also carries postId, pullRequestId, and
pullRequestCommentId. All three are marked [Internal] in the schema and are not
exposed.`,
		Example: `  linear-pp-cli reactions add --issue LIN-123 --emoji +1 --agent
  linear-pp-cli reactions add --comment <comment-uuid> --emoji eyes --agent
  linear-pp-cli reactions add --project-update <update-uuid> --emoji tada --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target.empty() || strings.TrimSpace(emoji) == "" {
				if dryRunOK(flags) {
					return nil
				}
				if strings.TrimSpace(emoji) == "" {
					return usageErr(fmt.Errorf("--emoji is required"))
				}
				return usageErr(fmt.Errorf("pass exactly one target: --comment, --issue, --project-update, or --initiative-update"))
			}
			field, value, err := target.resolve()
			if err != nil {
				return err
			}
			input := map[string]any{"emoji": strings.TrimSpace(emoji), field: value}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_add_reaction", "reactionCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.ReactionCreateMutation, map[string]any{"input": input})
			if err != nil {
				return wbClassifyCreateError("reactionCreate", err, flags)
			}
			reaction, err := extractMutationObject(resp, "reactionCreate", "reaction")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, reaction, "reactions")
		},
	}
	target.bind(cmd)
	cmd.Flags().StringVar(&emoji, "emoji", "", "Emoji name as Linear stores it, for example +1, eyes, or tada")
	return cmd
}

func newReactionsRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <reaction-id>",
		Aliases: []string{"delete"},
		Short:   "Remove a reaction",
		Long: `Remove a reaction via the reactionDelete mutation.

The mutation keys on the reaction's own id, not on a target plus an emoji, so
find the id with "reactions list" first.

Removal is confirmed interactively unless --yes is passed. --agent implies
--yes. With --ignore-missing an already-removed reaction exits 0 as a no-op.`,
		Example: `  linear-pp-cli reactions remove <reaction-uuid> --yes --agent
  linear-pp-cli reactions remove <reaction-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<reaction-id> is required (find it with 'reactions list')"))
			}
			id := strings.TrimSpace(args[0])
			if !store.IsUUID(id) {
				return usageErr(fmt.Errorf("<reaction-id> expects a reaction UUID, got %q (find it with 'reactions list')", id))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_remove_reaction", "reactionDelete", map[string]any{"id": id})
			}
			if err := wbConfirm(cmd, flags, fmt.Sprintf("Remove reaction %s", id)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.ReactionDeleteMutation, map[string]any{"id": id})
			if err != nil {
				return wbClassifyDeleteError("reactionDelete", err, flags)
			}
			deleted, err := wbDecodeDeletePayload(resp, "reactionDelete")
			if err != nil {
				return err
			}
			return wbRenderMutationEvent(cmd, flags, "reaction_removed", map[string]any{"id": firstNonEmpty(deleted, id)},
				fmt.Sprintf("Removed reaction %s", firstNonEmpty(deleted, id)))
		},
	}
	return cmd
}
