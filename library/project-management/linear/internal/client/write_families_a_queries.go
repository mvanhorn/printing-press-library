package client

// GraphQL documents for the tier-2 write families: issue labels, workflow
// states, cycles, comment lifecycle, and issue lifecycle.
//
// Every mutation name, argument name, and input-object field used below was
// read out of the introspection snapshot in
// .goal-linear-coverage/api-inventory.json before the document was written.
// Nothing here is guessed, and no deprecated field is selected (IssueLabel
// .organization is deprecated and is deliberately absent).

// Field projections shared by the documents in this file. Keeping them as
// constants means a create and its matching update return byte-identical
// shapes, so a caller can render either through the same code path.
const (
	issueLabelFields = `id name color description isGroup retiredAt lastAppliedAt createdAt updatedAt
      team { id key name }
      parent { id name }`

	workflowStateFields = `id name type color description position createdAt updatedAt archivedAt
      team { id key name }`

	cycleFields = `id number name description startsAt endsAt completedAt autoArchivedAt
      isActive isFuture isPast progress createdAt updatedAt
      team { id key name }`

	commentFields = `id body url quotedText createdAt updatedAt resolvedAt
      user { id name displayName email }
      issue { id identifier title }
      parent { id }
      resolvingUser { id name displayName }
      resolvingComment { id }`

	issueLifecycleFields = `id identifier title url archivedAt trashed updatedAt
      state { id name type }
      team { id key name }`
)

// IssueLabelCreateMutation creates a workspace label (teamId omitted) or a
// team label (teamId set).
//
// verified live in api-inventory.json
const IssueLabelCreateMutation = `mutation($input: IssueLabelCreateInput!) {
  issueLabelCreate(input: $input) {
    success
    issueLabel { ` + issueLabelFields + ` }
  }
}`

// verified live in api-inventory.json
const IssueLabelUpdateMutation = `mutation($id: String!, $input: IssueLabelUpdateInput!) {
  issueLabelUpdate(id: $id, input: $input) {
    success
    issueLabel { ` + issueLabelFields + ` }
  }
}`

// IssueLabelDeleteMutation returns DeletePayload, which carries entityId
// rather than the deleted object.
//
// verified live in api-inventory.json
const IssueLabelDeleteMutation = `mutation($id: String!) {
  issueLabelDelete(id: $id) {
    success
    entityId
  }
}`

// IssueLabelRetireMutation is the soft-delete half of the modern pair: a
// retired label stays visible on existing issues but cannot be applied to
// new ones. Distinct from issueLabelDelete.
//
// verified live in api-inventory.json
const IssueLabelRetireMutation = `mutation($id: String!) {
  issueLabelRetire(id: $id) {
    success
    issueLabel { ` + issueLabelFields + ` }
  }
}`

// verified live in api-inventory.json
const IssueLabelRestoreMutation = `mutation($id: String!) {
  issueLabelRestore(id: $id) {
    success
    issueLabel { ` + issueLabelFields + ` }
  }
}`

// WorkflowStateCreateMutation requires type, name, color, and teamId.
// WorkflowStateCreateInput.type accepts only backlog, unstarted, started,
// completed, and canceled: triage and duplicate states are provisioned by
// Linear itself and cannot be created through the API.
//
// verified live in api-inventory.json
const WorkflowStateCreateMutation = `mutation($input: WorkflowStateCreateInput!) {
  workflowStateCreate(input: $input) {
    success
    workflowState { ` + workflowStateFields + ` }
  }
}`

// WorkflowStateUpdateMutation carries no type field. WorkflowStateUpdateInput
// exposes only name, color, description, and position, so a state's type is
// immutable once created.
//
// verified live in api-inventory.json
const WorkflowStateUpdateMutation = `mutation($id: String!, $input: WorkflowStateUpdateInput!) {
  workflowStateUpdate(id: $id, input: $input) {
    success
    workflowState { ` + workflowStateFields + ` }
  }
}`

// WorkflowStateArchiveMutation returns WorkflowStateArchivePayload, whose
// object field is named entity, not workflowState.
//
// verified live in api-inventory.json
const WorkflowStateArchiveMutation = `mutation($id: String!) {
  workflowStateArchive(id: $id) {
    success
    entity { ` + workflowStateFields + ` }
  }
}`

// verified live in api-inventory.json
const CycleCreateMutation = `mutation($input: CycleCreateInput!) {
  cycleCreate(input: $input) {
    success
    cycle { ` + cycleFields + ` }
  }
}`

// verified live in api-inventory.json
const CycleUpdateMutation = `mutation($id: String!, $input: CycleUpdateInput!) {
  cycleUpdate(id: $id, input: $input) {
    success
    cycle { ` + cycleFields + ` }
  }
}`

// CycleArchiveMutation returns CycleArchivePayload, whose object field is
// named entity, not cycle.
//
// verified live in api-inventory.json
const CycleArchiveMutation = `mutation($id: String!) {
  cycleArchive(id: $id) {
    success
    entity { ` + cycleFields + ` }
  }
}`

// CycleShiftAllMutation takes CycleShiftAllInput, whose only two fields are
// id (the cycle to start shifting from) and daysToShift (a Float).
//
// verified live in api-inventory.json
const CycleShiftAllMutation = `mutation($input: CycleShiftAllInput!) {
  cycleShiftAll(input: $input) {
    success
    cycle { ` + cycleFields + ` }
  }
}`

// CycleStartUpcomingCycleTodayMutation takes a bare id and only accepts the
// team's next not-yet-started cycle.
//
// verified live in api-inventory.json
const CycleStartUpcomingCycleTodayMutation = `mutation($id: String!) {
  cycleStartUpcomingCycleToday(id: $id) {
    success
    cycle { ` + cycleFields + ` }
  }
}`

// verified live in api-inventory.json
const CommentDeleteMutation = `mutation($id: String!) {
  commentDelete(id: $id) {
    success
    entityId
  }
}`

// CommentResolveMutation takes resolvingCommentId as a top-level optional
// argument, not as an input object.
//
// verified live in api-inventory.json
const CommentResolveMutation = `mutation($id: String!, $resolvingCommentId: String) {
  commentResolve(id: $id, resolvingCommentId: $resolvingCommentId) {
    success
    comment { ` + commentFields + ` }
  }
}`

// verified live in api-inventory.json
const CommentUnresolveMutation = `mutation($id: String!) {
  commentUnresolve(id: $id) {
    success
    comment { ` + commentFields + ` }
  }
}`

// IssueArchiveMutation accepts an optional trash argument. Note that
// issueArchive returns IssueArchivePayload, whose object field is entity.
//
// verified live in api-inventory.json
const IssueArchiveMutation = `mutation($id: String!, $trash: Boolean) {
  issueArchive(id: $id, trash: $trash) {
    success
    entity { ` + issueLifecycleFields + ` }
  }
}`

// verified live in api-inventory.json
const IssueUnarchiveMutation = `mutation($id: String!) {
  issueUnarchive(id: $id) {
    success
    entity { ` + issueLifecycleFields + ` }
  }
}`

// IssueDeleteMutation trashes an issue. permanentlyDelete skips the 30-day
// grace period and is admin-only, so it stays behind an explicit flag.
//
// verified live in api-inventory.json
const IssueDeleteMutation = `mutation($id: String!, $permanentlyDelete: Boolean) {
  issueDelete(id: $id, permanentlyDelete: $permanentlyDelete) {
    success
    entity { ` + issueLifecycleFields + ` }
  }
}`

// IssueSubscribeMutation defaults to the current user when neither userId nor
// userEmail is supplied. Returns IssuePayload, whose object field is issue.
//
// verified live in api-inventory.json
const IssueSubscribeMutation = `mutation($id: String!, $userId: String, $userEmail: String) {
  issueSubscribe(id: $id, userId: $userId, userEmail: $userEmail) {
    success
    issue {
      ` + issueLifecycleFields + `
      subscribers { nodes { id name displayName email } }
    }
  }
}`

// verified live in api-inventory.json
const IssueUnsubscribeMutation = `mutation($id: String!, $userId: String, $userEmail: String) {
  issueUnsubscribe(id: $id, userId: $userId, userEmail: $userEmail) {
    success
    issue {
      ` + issueLifecycleFields + `
      subscribers { nodes { id name displayName email } }
    }
  }
}`
