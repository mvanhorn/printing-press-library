package client

// GraphQL documents for the issue batch mutations (GAP-034).
//
// Both mutation names, their argument names and their input shapes were read
// out of the introspection snapshot in
// .goal-linear-coverage/api-inventory.json before the documents were
// written:
//
//	issueBatchCreate(input: IssueBatchCreateInput!): IssueBatchPayload!
//	  IssueBatchCreateInput = { issues: [IssueCreateInput!]! }
//	issueBatchUpdate(input: IssueUpdateInput!, ids: [UUID!]!): IssueBatchPayload!
//
// Note the asymmetry, which is Linear's and not a transcription slip: create
// wraps its list in an input object, update takes the id list as a SEPARATE
// argument alongside a single IssueUpdateInput applied to all of them. Both
// return IssueBatchPayload { lastSyncId, issues, success }.
//
// Linear documents a ceiling of 50 on both: IssueBatchCreateInput is "Up to
// 50 issues can be created in a single batch" and issueBatchUpdate.ids is
// "Can't be more than 50 at a time". Callers enforce it before sending.

// batchIssueFields matches the projection issueCreate selects, so a batch
// create can feed the same pp_created ledger and local write-back path a
// single create does without a second round trip.
//
// verified live in api-inventory.json
const batchIssueFields = `id identifier title description url priority createdAt updatedAt
      team { id key }
      state { id name type }
      assignee { id name displayName }
      project { id name }
      parent { id identifier title }`

// IssueBatchCreateMutation creates up to 50 issues in one transaction.
//
// verified live in api-inventory.json
const IssueBatchCreateMutation = `mutation($input: IssueBatchCreateInput!) {
  issueBatchCreate(input: $input) {
    success
    issues { ` + batchIssueFields + ` }
  }
}`

// IssueBatchUpdateMutation applies one IssueUpdateInput to up to 50 issues.
// ids is [UUID!]!, so identifiers such as ENG-123 must be resolved to UUIDs
// before the call.
//
// verified live in api-inventory.json
const IssueBatchUpdateMutation = `mutation($input: IssueUpdateInput!, $ids: [UUID!]!) {
  issueBatchUpdate(input: $input, ids: $ids) {
    success
    issues { ` + batchIssueFields + ` }
  }
}`
