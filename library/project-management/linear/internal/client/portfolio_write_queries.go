package client

// GraphQL documents for the portfolio write surface: initiative lifecycle,
// initiative-to-project links, project update edits, and the two document
// lifecycle verbs the CLI was missing.
//
// Every mutation name, argument name, and input-object field used below was
// read out of the introspection snapshot in
// .goal-linear-coverage/api-inventory.json before the document was written.
// Nothing here is guessed, and no deprecated field is selected.
// projectUpdateDelete exists in the schema but is deprecated in favour of
// projectUpdateArchive, so it is deliberately absent.

// Field projections shared by the documents in this file. Keeping them as
// constants means a create and its matching update return byte-identical
// shapes, so a caller can render either through the same code path.
const (
	// `content` is selected alongside `description` so a write through the
	// content field reads back in the mutation payload. `description` stays in
	// the projection because the field still exists and still renders whatever
	// a non-CLI writer put there, it is only the write side that is dead.
	initiativeFields = `id name description content status targetDate targetDateResolution
      color icon priority sortOrder slugId url trashed
      createdAt updatedAt archivedAt
      owner { id name displayName email }
      creator { id name displayName email }
      leadTeam { id key name }`

	// InitiativeArchivePayload.entity is nullable and carries the whole
	// Initiative, but an archive caller only needs the identity and the new
	// archivedAt, so the archive documents select a short projection.
	initiativeArchiveFields = `id name status archivedAt url`

	initiativeToProjectFields = `id sortOrder createdAt updatedAt
      initiative { id name status }
      project { id name }`

	projectUpdateFields = `id body health url isStale createdAt updatedAt editedAt archivedAt
      user { id name displayName email }
      project { id name }`

	documentArchiveFields = `id title slugId url trashed archivedAt updatedAt
      project { id name }
      initiative { id name }`
)

// InitiativeCreateMutation creates a workspace initiative. InitiativeCreateInput
// requires only name; every other field this CLI sends is optional.
//
// InitiativeCreateInput.description is a dead write: the schema accepts it, the
// mutation returns success, and nothing is persisted. Live probe goal-cov-r2 on
// 2026-08-11 created an initiative with a description and read back
// description: null on an independent re-read, while the same text sent as
// `content` persisted. The CLI therefore maps its --description flag to
// `content`, see initiativeWriteFlags in internal/cli/initiatives_write.go.
//
// verified live in api-inventory.json
const InitiativeCreateMutation = `mutation($input: InitiativeCreateInput!) {
  initiativeCreate(input: $input) {
    success
    initiative { ` + initiativeFields + ` }
  }
}`

// InitiativeUpdateMutation edits an initiative in place. InitiativeUpdateInput
// carries no name-requirement, so a caller may send a single field.
//
// InitiativeUpdateInput.description is dead the same way InitiativeCreateInput's
// is (live probe goal-cov-r2, 2026-08-11: update returned success, both the
// payload and an independent re-read gave description: null, while an update
// through `content` persisted and read back). Callers write the body through
// `content`.
//
// verified live in api-inventory.json
const InitiativeUpdateMutation = `mutation($id: String!, $input: InitiativeUpdateInput!) {
  initiativeUpdate(id: $id, input: $input) {
    success
    initiative { ` + initiativeFields + ` }
  }
}`

// InitiativeArchiveMutation returns InitiativeArchivePayload, whose entity is
// the archived initiative rather than a bare id.
//
// verified live in api-inventory.json
const InitiativeArchiveMutation = `mutation($id: String!) {
  initiativeArchive(id: $id) {
    success
    entity { ` + initiativeArchiveFields + ` }
  }
}`

// verified live in api-inventory.json
const InitiativeUnarchiveMutation = `mutation($id: String!) {
  initiativeUnarchive(id: $id) {
    success
    entity { ` + initiativeArchiveFields + ` }
  }
}`

// InitiativeDeleteMutation returns DeletePayload, which carries entityId rather
// than the deleted object. Unlike initiativeArchive this is not reversible from
// the CLI, because there is no initiativeUntrash counterpart in the schema.
//
// verified live in api-inventory.json
const InitiativeDeleteMutation = `mutation($id: String!) {
  initiativeDelete(id: $id) {
    success
    entityId
  }
}`

// InitiativeToProjectCreateMutation links a project to an initiative.
// InitiativeToProjectCreateInput requires both initiativeId and projectId.
//
// verified live in api-inventory.json
const InitiativeToProjectCreateMutation = `mutation($input: InitiativeToProjectCreateInput!) {
  initiativeToProjectCreate(input: $input) {
    success
    initiativeToProject { ` + initiativeToProjectFields + ` }
  }
}`

// InitiativeToProjectDeleteMutation takes the id of the link row, not the pair
// of endpoints, so an unlink has to resolve the link first through
// ProjectInitiativeLinksQuery.
//
// verified live in api-inventory.json
const InitiativeToProjectDeleteMutation = `mutation($id: String!) {
  initiativeToProjectDelete(id: $id) {
    success
    entityId
  }
}`

// ProjectInitiativeLinksQuery reads the initiative links of one project.
// Query.initiativeToProjects takes no filter argument, so the narrow path to a
// link id is Project.initiativeToProjects, which is scoped by construction.
//
// verified live in api-inventory.json
const ProjectInitiativeLinksQuery = `query($id: String!, $first: Int!, $after: String) {
  project(id: $id) {
    id
    name
    initiativeToProjects(first: $first, after: $after) {
      nodes {
        id
        initiative { id name }
      }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

// ProjectUpdateUpdateMutation edits a posted project update. The deprecated
// isDiffHidden field of ProjectUpdateUpdateInput is deliberately not exposed.
//
// verified live in api-inventory.json
const ProjectUpdateUpdateMutation = `mutation($id: String!, $input: ProjectUpdateUpdateInput!) {
  projectUpdateUpdate(id: $id, input: $input) {
    success
    projectUpdate { ` + projectUpdateFields + ` }
  }
}`

// ProjectUpdateArchiveMutation is the removal verb for a project update.
// projectUpdateDelete is deprecated in favour of it.
//
// verified live in api-inventory.json
const ProjectUpdateArchiveMutation = `mutation($id: String!) {
  projectUpdateArchive(id: $id) {
    success
    entity { id health url archivedAt updatedAt }
  }
}`

// verified live in api-inventory.json
const ProjectUpdateUnarchiveMutation = `mutation($id: String!) {
  projectUpdateUnarchive(id: $id) {
    success
    entity { id health url archivedAt updatedAt }
  }
}`

// DocumentDeleteMutation returns DocumentArchivePayload rather than
// DeletePayload: Linear trashes a document instead of erasing it, which is why
// documentUnarchive is the matching restore verb.
//
// verified live in api-inventory.json
const DocumentDeleteMutation = `mutation($id: String!) {
  documentDelete(id: $id) {
    success
    entity { ` + documentArchiveFields + ` }
  }
}`

// verified live in api-inventory.json
const DocumentUnarchiveMutation = `mutation($id: String!) {
  documentUnarchive(id: $id) {
    success
    entity { ` + documentArchiveFields + ` }
  }
}`
