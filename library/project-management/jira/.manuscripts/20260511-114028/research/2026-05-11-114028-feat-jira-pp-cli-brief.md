# Jira CLI Brief

## API Identity
- Domain: Project management, issue tracking, sprint/agile planning, team collaboration
- Users: Software engineers, PMs, scrum masters, QA, DevOps, managers
- Data profile: Issues (high volume, rich metadata), projects (low volume), sprints/boards (medium), worklogs (medium), users (low)

## Reachability Risk
- None — official Atlassian REST API, well-documented, user has valid API token (confirmed HTTP 200)
- Auth: HTTP Basic (email:api_token base64), also OAuth 2.0 for integrations
- API base: https://verybigthings.atlassian.net (Cloud), /rest/api/3/ for primary endpoints, /rest/agile/1.0/ for boards/sprints

## Spec
- Source: https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json
- Format: OpenAPI 3.0.1, 420 paths, 2.4 MB
- Key note: Spec is massive; generator will truncate. Focus on core Issue, Project, Sprint, Comment, Worklog, User endpoints

## Top Workflows
1. **Daily standup prep** — "What did I work on? What's blocked? What's up next?" across my assigned issues
2. **Issue triage** — Quickly scan new/unassigned issues, assign, transition, comment in bulk
3. **Sprint planning** — View active sprint, backlog, velocity from prior sprints
4. **Worklog tracking** — Log time on issues, view personal/team worklog summaries
5. **JQL power search** — Run arbitrary JQL queries, save as local named filters, pipe to jq

## Table Stakes (from competitors)
- `issue list` — filter by assignee, status, priority, sprint, labels, JQL
- `issue create` — interactive or flag-driven, with type/priority/assignee/labels
- `issue edit` — update summary, description, assignee, priority, status
- `issue view` — rich single-issue view with comments, worklogs, links
- `issue transition` — move through workflow states
- `issue comment add` — add comment to issue
- `issue worklog add/list` — log and view time
- `issue link` — link issues (blocks/duplicates/relates-to)
- `project list` — list accessible projects
- `sprint list/view` — view sprints and their issues
- `board list` — list boards
- `search` — JQL search across all issues
- `me` — show current user info
- JSON/CSV/table output modes
- `--dry-run` for mutation commands

## Data Layer
- Primary entities: issues (with all fields), projects, sprints, users
- Sync cursor: `updated >= -Xd` JQL query for incremental sync
- FTS/search: SQLite FTS5 on issue summary, description, comments
- High-value offline queries: my open issues, overdue issues, issues I commented on, worklog by week

## Codebase Intelligence
- Source: MCP server analysis (cosmix/jira-mcp, jira-pilot)
- Auth: Basic auth header `Authorization: Basic <base64(email:token)>`, env vars JIRA_API_TOKEN + JIRA_BASE_URL + JIRA_EMAIL
- Description format: Atlassian Document Format (ADF) — text goes in as plain, comes back as ADF JSON
- Agile API: `/rest/agile/1.0/` separate from REST API v3 — boards, sprints live here
- Data Center vs Cloud: API v2 for Server/DC, v3 for Cloud; bearer token only on DC 8.14+
- X-Atlassian-Token: no-check header required for attachments

## User Vision
- User is Nikica Jokic at verybigthings.atlassian.net
- Use case: day-to-day Jira interaction from terminal, agent-native for AI workflows

## Product Thesis
- Name: jira-pp-cli
- Display name: Jira
- Why it should exist: Every Jira CLI today requires going online for everything. This one syncs your workspace locally, lets you search and query offline, and surfaces team insights no single API call can produce — sprint velocity, workload distribution, who's actually overloaded.

## Build Priorities
1. Issues: list/view/create/edit/transition/comment/worklog (core daily workflow)
2. Sync: incremental issue sync to SQLite with JQL-based cursor
3. Search: offline FTS on issue summary+description+comments
4. Sprint/board: view active sprint, sprint issues, basic agile ops
5. Power analytics: my-work summary, overdue, workload by assignee, velocity trends
6. Project list/view
7. JQL filter save/run
