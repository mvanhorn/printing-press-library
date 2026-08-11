package client

// GraphQL documents for the notification inbox (GAP-032).
//
// Every query name, mutation name, argument name and input-object field used
// below was read out of the introspection snapshot in
// .goal-linear-coverage/api-inventory.json before the document was written.
// Nothing here is guessed and no deprecated field is selected
// (NotificationEntityInput.projectId is deprecated and is deliberately
// absent, and notificationSubscriptionDelete is deprecated so it has no
// document here).
//
// Notification is an INTERFACE, not an object. Its 13 possible types
// (IssueNotification, ProjectNotification, InitiativeNotification,
// OauthClientApprovalNotification, DocumentNotification, PostNotification,
// CustomerNeedNotification, CustomerNotification, PullRequestNotification,
// WelcomeMessageNotification, UsageAlertNotification,
// ProductAnnouncementNotification, WorkflowDefinitionNotification) share the
// interface fields selected here. Only IssueNotification gets an inline
// fragment, because issue context is the only subtype payload this CLI acts
// on. Every other subtype still renders through the interface fields.
//
// The whole family is viewer-scoped: Query.notifications is documented as
// "The authenticated user's notifications", so these documents can never
// read or mutate another user's inbox.

// notificationFields is the shared projection. Keeping it in one constant
// means a list, a get and every mutation payload return byte-identical
// shapes, so one renderer handles all of them.
//
// verified live in api-inventory.json
const notificationFields = `id
      type
      category
      title
      subtitle
      url
      inboxUrl
      createdAt
      updatedAt
      readAt
      emailedAt
      snoozedUntilAt
      unsnoozedAt
      archivedAt
      issueStatusType
      groupingKey
      actor { id name displayName }
      ... on IssueNotification {
        issueId
        commentId
        issue { id identifier title url state { id name type } }
        team { id key name }
      }`

// NotificationsQuery reads the authenticated user's inbox.
//
// NotificationFilter carries id, createdAt, updatedAt, type, subscriptionType
// and archivedAt comparators plus and/or. It has NO readAt field, so unread
// is not expressible server side and callers filter readAt themselves.
//
// verified live in api-inventory.json
const NotificationsQuery = `query($first: Int!, $after: String, $filter: NotificationFilter, $includeArchived: Boolean, $orderBy: PaginationOrderBy) {
  notifications(first: $first, after: $after, filter: $filter, includeArchived: $includeArchived, orderBy: $orderBy) {
    nodes { ` + notificationFields + ` }
    pageInfo { hasNextPage endCursor }
  }
}`

// NotificationQuery reads one notification by id.
//
// verified live in api-inventory.json
const NotificationQuery = `query($id: String!) {
  notification(id: $id) { ` + notificationFields + ` }
}`

// NotificationsUnreadCountQuery reads the inbox badge count.
//
// Query.notificationsUnreadCount is live and not deprecated, but Linear
// describes it as "[Internal]". It is exposed because the unread filter is
// client side, so a cheap authoritative count is the only way to know an
// unread walk found everything.
//
// verified live in api-inventory.json
const NotificationsUnreadCountQuery = `query {
  notificationsUnreadCount
}`

// NotificationUpdateMutation is the single-notification read, unread, snooze
// and unsnooze path. NotificationUpdateInput carries exactly four fields:
// readAt, snoozedUntilAt, projectUpdateId and initiativeUpdateId. Both
// timestamp fields are nullable, so an explicit null clears them, which is
// how unread and unsnooze are expressed.
//
// verified live in api-inventory.json
const NotificationUpdateMutation = `mutation($id: String!, $input: NotificationUpdateInput!) {
  notificationUpdate(id: $id, input: $input) {
    success
    notification { ` + notificationFields + ` }
  }
}`

// NotificationArchiveMutation archives one notification. The payload is
// NotificationArchivePayload, whose object field is `entity`, not
// `notification`.
//
// verified live in api-inventory.json
const NotificationArchiveMutation = `mutation($id: String!) {
  notificationArchive(id: $id) {
    success
    entity { ` + notificationFields + ` }
  }
}`

// NotificationUnarchiveMutation reverses NotificationArchiveMutation.
//
// verified live in api-inventory.json
const NotificationUnarchiveMutation = `mutation($id: String!) {
  notificationUnarchive(id: $id) {
    success
    entity { ` + notificationFields + ` }
  }
}`

// The five batch mutations below all take NotificationEntityInput, which
// names the ENTITY whose related notifications are targeted, not a list of
// notification ids. Passing NotificationEntityInput.id targets one
// notification and everything grouped with it. They all return
// NotificationBatchActionPayload, whose object field is `notifications`.

// NotificationMarkReadAllMutation marks a notification and everything
// related to it as read. readAt is a required argument, not an input field.
//
// verified live in api-inventory.json
const NotificationMarkReadAllMutation = `mutation($input: NotificationEntityInput!, $readAt: DateTime!) {
  notificationMarkReadAll(input: $input, readAt: $readAt) {
    success
    notifications { ` + notificationFields + ` }
  }
}`

// NotificationMarkUnreadAllMutation is the inverse and takes no timestamp.
//
// verified live in api-inventory.json
const NotificationMarkUnreadAllMutation = `mutation($input: NotificationEntityInput!) {
  notificationMarkUnreadAll(input: $input) {
    success
    notifications { ` + notificationFields + ` }
  }
}`

// NotificationArchiveAllMutation archives a notification group.
//
// verified live in api-inventory.json
const NotificationArchiveAllMutation = `mutation($input: NotificationEntityInput!) {
  notificationArchiveAll(input: $input) {
    success
    notifications { ` + notificationFields + ` }
  }
}`

// NotificationSnoozeAllMutation snoozes a notification group until
// snoozedUntilAt, a required argument.
//
// verified live in api-inventory.json
const NotificationSnoozeAllMutation = `mutation($input: NotificationEntityInput!, $snoozedUntilAt: DateTime!) {
  notificationSnoozeAll(input: $input, snoozedUntilAt: $snoozedUntilAt) {
    success
    notifications { ` + notificationFields + ` }
  }
}`

// NotificationUnsnoozeAllMutation wakes a snoozed notification group.
// unsnoozedAt is a required argument and records when the wake happened.
//
// verified live in api-inventory.json
const NotificationUnsnoozeAllMutation = `mutation($input: NotificationEntityInput!, $unsnoozedAt: DateTime!) {
  notificationUnsnoozeAll(input: $input, unsnoozedAt: $unsnoozedAt) {
    success
    notifications { ` + notificationFields + ` }
  }
}`
