package client

// GraphQL query constants for the Linear API.

// IssuesQuery is the workspace issue collection, used by `issues list` and by
// the sync issues pass.
//
// $filter is an IssueFilter. Incremental sync narrows the crawl by passing
// {updatedAt: {gt: <RFC3339>}} through it: IssueFilter.updatedAt is a
// DateComparator carrying gt/gte/lt/lte/eq/neq/in/nin, verified live in
// api-inventory.json.
//
// $includeArchived maps to the issues connection's includeArchived Boolean
// argument, verified live in api-inventory.json. Declaring the variable is
// safe for every existing caller: an unprovided variable makes the argument
// absent rather than null, so the server default (exclude archived) still
// applies to anyone who does not pass it.
//
// archivedAt (Issue.archivedAt: DateTime, verified live in api-inventory.json)
// is selected unconditionally. It is null on a live issue, so it costs nothing
// on the default path and is the only thing that makes an archived row
// distinguishable once includeArchived is on.
const IssuesQuery = `query($first: Int!, $after: String, $filter: IssueFilter, $includeArchived: Boolean) {
  issues(first: $first, after: $after, filter: $filter, includeArchived: $includeArchived) {
    nodes {
      id
      identifier
      title
      description
      priority
      estimate
      dueDate
      createdAt
      updatedAt
      archivedAt
      state { id name type color }
      assignee { id name displayName email }
      team { id name key }
      project { id name }
      cycle { id name number }
      labels { nodes { id name color } }
      parent { id identifier title }
      children { nodes { id identifier title } }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const IssueQuery = `query($id: String!) {
  issue(id: $id) {
    id
    identifier
    title
    description
    priority
    estimate
    dueDate
    url
    createdAt
    updatedAt
    state { id name type color }
    assignee { id name displayName email }
    team { id name key }
    project { id name }
    cycle { id name number }
    labels { nodes { id name color } }
    parent { id identifier title }
    children { nodes { id identifier title } }
    comments { nodes { id body createdAt user { id name } } }
    relations { nodes { id type relatedIssue { id identifier title } } }
  }
}`

const IssueSearchQuery = `query($first: Int!, $query: String!) {
  issueSearch(first: $first, query: $query) {
    nodes {
      id
      identifier
      title
      priority
      state { id name type }
      assignee { id name }
      team { id name key }
      updatedAt
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const ProjectsQuery = `query($first: Int!, $after: String) {
  projects(first: $first, after: $after) {
    nodes {
      id
      name
      description
      state
      targetDate
      startDate
      progress
      createdAt
      updatedAt
      lead { id name }
      members { nodes { id name } }
      teams { nodes { id name key } }
      projectMilestones(first: 50) {
        nodes { id name targetDate sortOrder }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const TeamsQuery = `query {
  teams {
    nodes {
      id
      name
      key
      description
      createdAt
    }
  }
}`

const CyclesQuery = `query($first: Int!, $after: String, $filter: CycleFilter) {
  cycles(first: $first, after: $after, filter: $filter) {
    nodes {
      id
      name
      number
      startsAt
      endsAt
      completedAt
      progress
      progress
      createdAt
      updatedAt
      team { id name key }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const UsersQuery = `query($first: Int!) {
  users(first: $first) {
    nodes {
      id
      name
      displayName
      email
      active
      admin
      createdAt
      updatedAt
    }
  }
}`

const WorkflowStatesQuery = `query {
  workflowStates(first: 200) {
    nodes {
      id
      name
      type
      color
      position
      team { id name key }
    }
  }
}`

const IssueLabelsQuery = `query($first: Int!, $after: String) {
  issueLabels(first: $first, after: $after) {
    nodes {
      id
      name
      color
      createdAt
      team { id name key }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const ViewerQuery = `query {
  viewer {
    id
    name
    displayName
    email
    active
    admin
    organization { id name urlKey }
  }
}`

const IssueCreateMutation = `mutation($input: IssueCreateInput!) {
  issueCreate(input: $input) {
    success
    issue {
      id
      identifier
      title
      url
      state { name }
      team { key }
    }
  }
}`

const IssueUpdateMutation = `mutation($id: String!, $input: IssueUpdateInput!) {
  issueUpdate(id: $id, input: $input) {
    success
    issue {
      id
      identifier
      title
      url
      state { name }
    }
  }
}`

const CommentCreateMutation = `mutation($input: CommentCreateInput!) {
  commentCreate(input: $input) {
    success
    comment {
      id
      body
      createdAt
      user { id name }
    }
  }
}`

const DocumentsQuery = `query($first: Int!, $after: String) {
  documents(first: $first, after: $after) {
    nodes {
      id
      title
      content
      createdAt
      updatedAt
      creator { id name }
      project { id name }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const InitiativesQuery = `query($first: Int!, $after: String) {
  initiatives(first: $first, after: $after) {
    nodes {
      id
      name
      description
      status
      createdAt
      updatedAt
    }
    pageInfo { hasNextPage endCursor }
  }
}`
