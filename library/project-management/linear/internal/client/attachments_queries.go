package client

// GraphQL documents for the attachment write family (GAP-045) and the reaction
// pair (GAP-049).
//
// Every mutation name, argument name, input-object field, and selected object
// field below was read out of the introspection snapshot in
// .goal-linear-coverage/api-inventory.json before the document was written.
// Nothing here is guessed. No deprecated field is selected: Attachment exposes
// no deprecated fields, and the deprecated attachmentIssue query is
// deliberately absent in favour of attachmentsForURL, which is the supported
// URL-keyed lookup.

// Field projections shared by the documents in this file. Keeping them as
// constants means a create, an update, and a URL lookup return byte-identical
// attachment shapes, so one renderer serves all three.
const (
	attachmentFields = `id title subtitle url sourceType groupBySource metadata source bodyData
      createdAt updatedAt archivedAt
      creator { id name displayName email }
      externalUserCreator { id name displayName email }
      issue { id identifier title }
      originalIssue { id identifier title }`

	// Reaction.post, .pullRequest, and .pullRequestComment are marked
	// [Internal] in the schema description and are not selected, matching
	// the target flags reactions add exposes.
	reactionFields = `id emoji createdAt updatedAt archivedAt
      user { id name displayName email }
      externalUser { id name displayName email }
      issue { id identifier title }
      comment { id }
      projectUpdate { id }
      initiativeUpdate { id }`
)

// AttachmentCreateMutation creates a metadata-bearing attachment card.
// AttachmentCreateInput requires title, url, and issueId, and issueId accepts
// either a UUID or an issue identifier such as LIN-123, so no client-side
// resolution is needed. Linear updates the existing record instead of creating
// a second one when the same url and issueId pair is submitted again.
//
// verified live in api-inventory.json
const AttachmentCreateMutation = `mutation($input: AttachmentCreateInput!) {
  attachmentCreate(input: $input) {
    success
    attachment { ` + attachmentFields + ` }
  }
}`

// AttachmentLinkURLMutation links a URL to an issue and lets Linear decide the
// attachment shape: a configured, recognised integration URL becomes a rich
// attachment with status sync, anything else becomes a basic one. Unlike
// attachmentCreate this mutation takes flat arguments rather than an input
// object, and title is optional because Linear can unfurl one.
//
// verified live in api-inventory.json
const AttachmentLinkURLMutation = `mutation($url: String!, $issueId: String!, $title: String) {
  attachmentLinkURL(url: $url, issueId: $issueId, title: $title) {
    success
    attachment { ` + attachmentFields + ` }
  }
}`

// AttachmentUpdateMutation edits an existing attachment. AttachmentUpdateInput
// carries exactly four fields (title, subtitle, metadata, iconUrl) and title is
// non-null, so an update always restates the title.
//
// verified live in api-inventory.json
const AttachmentUpdateMutation = `mutation($id: String!, $input: AttachmentUpdateInput!) {
  attachmentUpdate(id: $id, input: $input) {
    success
    attachment { ` + attachmentFields + ` }
  }
}`

// AttachmentDeleteMutation returns DeletePayload, which carries entityId
// rather than the deleted object.
//
// verified live in api-inventory.json
const AttachmentDeleteMutation = `mutation($id: String!) {
  attachmentDelete(id: $id) {
    success
    entityId
  }
}`

// AttachmentsForURLQuery lists the attachments already recorded against a URL,
// across issues. It is the supported replacement for the deprecated
// attachmentIssue query and the dedupe primitive to call before creating: the
// create mutation keys on url plus issueId, so the same URL can legitimately
// exist on several issues.
//
// verified live in api-inventory.json
const AttachmentsForURLQuery = `query($url: String!, $first: Int, $after: String, $includeArchived: Boolean) {
  attachmentsForURL(url: $url, first: $first, after: $after, includeArchived: $includeArchived) {
    nodes { ` + attachmentFields + ` }
    pageInfo { hasNextPage endCursor }
  }
}`

// ReactionCreateMutation adds an emoji reaction. ReactionCreateInput requires
// emoji and carries one optional id per target type: commentId,
// projectUpdateId, initiativeUpdateId, and issueId are the public ones, while
// postId, pullRequestId, and pullRequestCommentId are marked [Internal] and are
// not exposed. issueId accepts a UUID or an issue identifier.
//
// verified live in api-inventory.json
const ReactionCreateMutation = `mutation($input: ReactionCreateInput!) {
  reactionCreate(input: $input) {
    success
    reaction { ` + reactionFields + ` }
  }
}`

// ReactionDeleteMutation removes a reaction by its own id, not by target plus
// emoji. Returns DeletePayload.
//
// verified live in api-inventory.json
const ReactionDeleteMutation = `mutation($id: String!) {
  reactionDelete(id: $id) {
    success
    entityId
  }
}`

// The reaction read documents. reactionDelete keys on the reaction's own id,
// which nothing else in the CLI surfaces, so "reactions list" is what makes
// "reactions remove" usable. Comment.reactions, Issue.reactions,
// ProjectUpdate.reactions, and InitiativeUpdate.reactions are all plain
// [Reaction!]! lists that take no pagination arguments, so there is no cursor
// to thread through.
//
// verified live in api-inventory.json
const CommentReactionsQuery = `query($id: String!) {
  comment(id: $id) {
    id
    reactions { ` + reactionFields + ` }
  }
}`

// IssueReactionsQuery takes an issue UUID. Unlike the reaction and attachment
// mutation inputs, the root issue query does not document identifier support,
// so callers resolve TEAM-NUMBER to a UUID first.
//
// verified live in api-inventory.json
const IssueReactionsQuery = `query($id: String!) {
  issue(id: $id) {
    id
    identifier
    reactions { ` + reactionFields + ` }
  }
}`

// verified live in api-inventory.json
const ProjectUpdateReactionsQuery = `query($id: String!) {
  projectUpdate(id: $id) {
    id
    reactions { ` + reactionFields + ` }
  }
}`

// verified live in api-inventory.json
const InitiativeUpdateReactionsQuery = `query($id: String!) {
  initiativeUpdate(id: $id) {
    id
    reactions { ` + reactionFields + ` }
  }
}`
