package client

// GraphQL documents for the tier-2 write families B surface: projects,
// project milestones, templates, custom views, and favorites.
//
// Every mutation name and every input field referenced here was checked
// against .goal-linear-coverage/api-inventory.json before being written.
// Deprecated inputs are deliberately absent: ProjectCreateInput.state and
// ProjectUpdateInput.state are superseded by statusId, and
// CustomView{Create,Update}Input.filters is superseded by filterData.

// projectSelection is the projection every project mutation returns. Project
// fields verified in api-inventory.json objects.Project. Project.state is
// deprecated in favour of Project.status and is therefore not selected.
const projectSelection = `
    id
    name
    description
    slugId
    url
    icon
    color
    priority
    progress
    startDate
    targetDate
    createdAt
    updatedAt
    status { id name type color }
    lead { id name displayName email }
    teams { nodes { id key name } }
    labels { nodes { id name color } }`

// projectMilestoneSelection projects ProjectMilestone. Fields verified in
// api-inventory.json objects.ProjectMilestone. status is the
// ProjectMilestoneStatus enum (unstarted, next, overdue, done).
const projectMilestoneSelection = `
    id
    name
    description
    targetDate
    sortOrder
    status
    progress
    createdAt
    updatedAt
    project { id name }`

// templateSelection projects Template. Fields verified in api-inventory.json
// objects.Template.
const templateSelection = `
    id
    name
    description
    type
    icon
    color
    templateData
    sortOrder
    lastAppliedAt
    createdAt
    updatedAt
    team { id key name }
    creator { id name displayName email }`

// customViewSelection projects CustomView. Fields verified in
// api-inventory.json objects.CustomView. CustomView.filters is deprecated in
// favour of filterData and is therefore not selected.
const customViewSelection = `
    id
    name
    description
    icon
    color
    shared
    slugId
    modelName
    filterData
    projectFilterData
    initiativeFilterData
    createdAt
    updatedAt
    team { id key name }
    owner { id name displayName email }
    creator { id name displayName email }`

// favoriteSelection projects Favorite. Fields verified in api-inventory.json
// objects.Favorite. Only the entity targets this CLI can create are selected.
const favoriteSelection = `
    id
    type
    title
    detail
    url
    folderName
    sortOrder
    projectTab
    initiativeTab
    pipelineTab
    predefinedViewType
    createdAt
    updatedAt
    parent { id }
    owner { id name displayName email }
    issue { id identifier title }
    project { id name }
    document { id title }
    label { id name }
    projectLabel { id name }
    initiative { id name }
    customView { id name }
    cycle { id name number }
    user { id name displayName email }
    team { id key name }`

// ProjectCreateMutation creates a project.
// projectCreate: verified live in api-inventory.json
const ProjectCreateMutation = `mutation($input: ProjectCreateInput!) {
  projectCreate(input: $input) {
    success
    project {` + projectSelection + `
    }
  }
}`

// ProjectUpdateMutation edits a project.
// projectUpdate: verified live in api-inventory.json
const ProjectUpdateMutation = `mutation($id: String!, $input: ProjectUpdateInput!) {
  projectUpdate(id: $id, input: $input) {
    success
    project {` + projectSelection + `
    }
  }
}`

// ProjectDeleteMutation trashes a project. projectDelete returns
// ProjectArchivePayload, whose entity field is the deleted project.
// projectDelete: verified live in api-inventory.json
const ProjectDeleteMutation = `mutation($id: String!) {
  projectDelete(id: $id) {
    success
    entity { id name url }
  }
}`

// ProjectAddLabelMutation attaches one project label. Linear exposes label
// attachment as its own mutation rather than through ProjectUpdateInput.
// labelIds, so this CLI calls the dedicated mutation and leaves the caller's
// other labels untouched.
// projectAddLabel: verified live in api-inventory.json
const ProjectAddLabelMutation = `mutation($id: String!, $labelId: String!) {
  projectAddLabel(id: $id, labelId: $labelId) {
    success
    project {` + projectSelection + `
    }
  }
}`

// ProjectRemoveLabelMutation detaches one project label.
// projectRemoveLabel: verified live in api-inventory.json
const ProjectRemoveLabelMutation = `mutation($id: String!, $labelId: String!) {
  projectRemoveLabel(id: $id, labelId: $labelId) {
    success
    project {` + projectSelection + `
    }
  }
}`

// ProjectMilestonesForProjectQuery lists one project's milestones. Read
// through Project.projectMilestones rather than the root projectMilestones
// connection so the project scope is part of the query rather than a
// comparator filter.
const ProjectMilestonesForProjectQuery = `query($id: String!, $first: Int!, $after: String) {
  project(id: $id) {
    id
    name
    projectMilestones(first: $first, after: $after) {
      nodes {` + projectMilestoneSelection + `
      }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

// ProjectMilestoneCreateMutation creates a milestone on a project.
// projectMilestoneCreate: verified live in api-inventory.json
const ProjectMilestoneCreateMutation = `mutation($input: ProjectMilestoneCreateInput!) {
  projectMilestoneCreate(input: $input) {
    success
    projectMilestone {` + projectMilestoneSelection + `
    }
  }
}`

// ProjectMilestoneUpdateMutation edits a milestone.
// projectMilestoneUpdate: verified live in api-inventory.json
const ProjectMilestoneUpdateMutation = `mutation($id: String!, $input: ProjectMilestoneUpdateInput!) {
  projectMilestoneUpdate(id: $id, input: $input) {
    success
    projectMilestone {` + projectMilestoneSelection + `
    }
  }
}`

// ProjectMilestoneDeleteMutation deletes a milestone.
// projectMilestoneDelete: verified live in api-inventory.json
const ProjectMilestoneDeleteMutation = `mutation($id: String!) {
  projectMilestoneDelete(id: $id) {
    success
    entityId
  }
}`

// ProjectMilestoneMoveMutation moves a milestone and its issues to another
// project. ProjectMilestoneMoveInput.projectId is required, and the optional
// newIssueTeamId plus addIssueTeamToProject cover the cross-team move.
// projectMilestoneMove: verified live in api-inventory.json
const ProjectMilestoneMoveMutation = `mutation($id: String!, $input: ProjectMilestoneMoveInput!) {
  projectMilestoneMove(id: $id, input: $input) {
    success
    projectMilestone {` + projectMilestoneSelection + `
    }
  }
}`

// TemplatesQuery lists every template in the workspace. Query.templates takes
// no arguments and returns a plain list rather than a connection, so per-team
// views filter client-side on Template.team.
const TemplatesQuery = `query {
  templates {` + templateSelection + `
  }
}`

// TemplateQuery reads one template by UUID.
const TemplateQuery = `query($id: String!) {
  template(id: $id) {` + templateSelection + `
  }
}`

// TemplateCreateMutation creates a template. TemplateCreateInput requires
// type and templateData alongside name.
// templateCreate: verified live in api-inventory.json
const TemplateCreateMutation = `mutation($input: TemplateCreateInput!) {
  templateCreate(input: $input) {
    success
    template {` + templateSelection + `
    }
  }
}`

// TemplateUpdateMutation edits a template. TemplateUpdateInput has no type
// field, so a template's type is fixed at creation.
// templateUpdate: verified live in api-inventory.json
const TemplateUpdateMutation = `mutation($id: String!, $input: TemplateUpdateInput!) {
  templateUpdate(id: $id, input: $input) {
    success
    template {` + templateSelection + `
    }
  }
}`

// TemplateDeleteMutation deletes a template.
// templateDelete: verified live in api-inventory.json
const TemplateDeleteMutation = `mutation($id: String!) {
  templateDelete(id: $id) {
    success
    entityId
  }
}`

// CustomViewsQuery lists custom views.
const CustomViewsQuery = `query($first: Int!, $after: String, $filter: CustomViewFilter) {
  customViews(first: $first, after: $after, filter: $filter) {
    nodes {` + customViewSelection + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// CustomViewQuery reads one custom view by UUID.
const CustomViewQuery = `query($id: String!) {
  customView(id: $id) {` + customViewSelection + `
  }
}`

// CustomViewCreateMutation creates a custom view. filterData carries the
// issue filter, projectFilterData the project filter, and
// initiativeFilterData the initiative filter.
// customViewCreate: verified live in api-inventory.json
const CustomViewCreateMutation = `mutation($input: CustomViewCreateInput!) {
  customViewCreate(input: $input) {
    success
    customView {` + customViewSelection + `
    }
  }
}`

// CustomViewUpdateMutation edits a custom view.
// customViewUpdate: verified live in api-inventory.json
const CustomViewUpdateMutation = `mutation($id: String!, $input: CustomViewUpdateInput!) {
  customViewUpdate(id: $id, input: $input) {
    success
    customView {` + customViewSelection + `
    }
  }
}`

// CustomViewDeleteMutation deletes a custom view.
// customViewDelete: verified live in api-inventory.json
const CustomViewDeleteMutation = `mutation($id: String!) {
  customViewDelete(id: $id) {
    success
    entityId
  }
}`

// FavoriteCreateMutation favorites exactly one entity.
// favoriteCreate: verified live in api-inventory.json
const FavoriteCreateMutation = `mutation($input: FavoriteCreateInput!) {
  favoriteCreate(input: $input) {
    success
    favorite {` + favoriteSelection + `
    }
  }
}`

// FavoriteUpdateMutation reorders or refolders a favorite. FavoriteUpdateInput
// carries only sortOrder, parentId, and folderName.
// favoriteUpdate: verified live in api-inventory.json
const FavoriteUpdateMutation = `mutation($id: String!, $input: FavoriteUpdateInput!) {
  favoriteUpdate(id: $id, input: $input) {
    success
    favorite {` + favoriteSelection + `
    }
  }
}`

// FavoriteDeleteMutation unfavorites an entity.
// favoriteDelete: verified live in api-inventory.json
const FavoriteDeleteMutation = `mutation($id: String!) {
  favoriteDelete(id: $id) {
    success
    entityId
  }
}`
