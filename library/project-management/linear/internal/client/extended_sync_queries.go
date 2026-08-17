package client

// GraphQL documents for the workspace-wide sync crawls that fill the shell
// tables the store has always created and never populated (GAP-038).
//
// Every root field and every selected field below was checked against
// .goal-linear-coverage/api-inventory.json before being written. The
// selections deliberately reuse the projections the write families already
// defined (projectMilestoneSelection, customViewSelection, favoriteSelection,
// templateSelection) so a synced row and a freshly mutated row carry the same
// shape. Deprecated fields are absent: CustomView.filters is superseded by
// filterData and is not selected anywhere.
//
// Each query pages with the standard first/after connection arguments so
// PaginatedQueryComplete can report whether the crawl reached the last page.
// That completeness flag is what lets the reconcile pass prune these tables.

// documentSyncSelection projects Document for the sync crawl. Fields verified
// in api-inventory.json objects.Document.
const documentSyncSelection = `
    id
    title
    content
    icon
    color
    slugId
    url
    sortOrder
    trashed
    createdAt
    updatedAt
    archivedAt
    creator { id name displayName email }
    updatedBy { id name displayName email }
    project { id name }
    initiative { id name }
    team { id key name }
    issue { id identifier title }`

// initiativeSyncSelection projects Initiative for the sync crawl. Fields
// verified in api-inventory.json objects.Initiative. status is the
// InitiativeStatus enum and health the InitiativeUpdateHealthType enum, so
// both are leaf selections. `content` rides along because Linear silently
// drops writes to Initiative.description (live probe goal-cov-r2,
// 2026-08-11), so a synced row without it holds none of the body text this
// CLI writes, and initiatives are counted in tens per workspace rather than
// in thousands.
const initiativeSyncSelection = `
    id
    name
    description
    content
    status
    health
    icon
    color
    slugId
    url
    sortOrder
    priority
    targetDate
    startedAt
    completedAt
    canceledAt
    createdAt
    updatedAt
    archivedAt
    owner { id name displayName email }
    creator { id name displayName email }
    leadTeam { id key name }`

// projectStatusSelection projects ProjectStatus. Fields verified in
// api-inventory.json objects.ProjectStatus. type is the ProjectStatusType
// enum (backlog, planned, started, paused, completed, canceled).
const projectStatusSelection = `
    id
    name
    description
    color
    position
    type
    indefinite
    createdAt
    updatedAt
    archivedAt`

// issueRelationSyncSelection projects IssueRelation. Fields verified in
// api-inventory.json objects.IssueRelation, whose entire field set is id,
// createdAt, updatedAt, archivedAt, type, issue and relatedIssue.
const issueRelationSyncSelection = `
    id
    type
    createdAt
    updatedAt
    archivedAt
    issue { id identifier title }
    relatedIssue { id identifier title }`

// DocumentsSyncQuery enumerates every document in the workspace.
// Query.documents: verified live in api-inventory.json
const DocumentsSyncQuery = `query($first: Int!, $after: String) {
  documents(first: $first, after: $after) {
    nodes {` + documentSyncSelection + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// InitiativesSyncQuery enumerates every initiative in the workspace.
// Query.initiatives: verified live in api-inventory.json
const InitiativesSyncQuery = `query($first: Int!, $after: String) {
  initiatives(first: $first, after: $after) {
    nodes {` + initiativeSyncSelection + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// CustomViewsSyncQuery enumerates every custom view in the workspace.
// Query.customViews: verified live in api-inventory.json
const CustomViewsSyncQuery = `query($first: Int!, $after: String) {
  customViews(first: $first, after: $after) {
    nodes {` + customViewSelection + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// FavoritesSyncQuery enumerates the authenticated user's favorites. Query
// favorites is scoped to the viewer by the API, which is the whole population
// of the local favorites table, so the crawl is still a complete enumeration.
// Query.favorites: verified live in api-inventory.json
const FavoritesSyncQuery = `query($first: Int!, $after: String) {
  favorites(first: $first, after: $after) {
    nodes {` + favoriteSelection + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// ProjectMilestonesSyncQuery enumerates every project milestone in the
// workspace, across all projects.
// Query.projectMilestones: verified live in api-inventory.json
const ProjectMilestonesSyncQuery = `query($first: Int!, $after: String) {
  projectMilestones(first: $first, after: $after) {
    nodes {` + projectMilestoneSelection + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// ProjectStatusesSyncQuery enumerates the workspace project statuses.
// Query.projectStatuses: verified live in api-inventory.json
const ProjectStatusesSyncQuery = `query($first: Int!, $after: String) {
  projectStatuses(first: $first, after: $after) {
    nodes {` + projectStatusSelection + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// IssueRelationsSyncQuery enumerates every issue relation in the workspace.
// This is the root-level connection, not the per-issue one in
// relations_queries.go: it pages the whole workspace, which is what makes the
// local issue_relations table prunable.
// Query.issueRelations: verified live in api-inventory.json
const IssueRelationsSyncQuery = `query($first: Int!, $after: String) {
  issueRelations(first: $first, after: $after) {
    nodes {` + issueRelationSyncSelection + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`
