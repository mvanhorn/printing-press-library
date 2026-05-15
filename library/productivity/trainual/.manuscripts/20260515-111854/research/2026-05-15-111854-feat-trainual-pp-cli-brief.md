# Trainual CLI Brief

## API Identity
- Domain: Employee training & SOP documentation platform
- Users: HR managers, L&D leads, ops managers, franchise operators (SMB/mid-market)
- Data profile: ~140 users, ~75 curriculums, ~300 courses, ~66 surveys per typical org. Moderate volume, high read frequency, low write frequency.

## Reachability Risk
- None — official documented REST API, no reported 403/blocking issues on GitHub

## API Surface
- Base URL: `https://app.trainual.com/api/v1`
- Auth: Bearer token (generated in Settings > Integrations > API) OR Basic HTTP Auth (email&account_id:password)
- Response pattern: `{ "users": [...], "meta": {...} }` for collections, `{ "user": {...} }` for singles
- Pagination: offset-based (`page`, `per_page`)
- Rate limits: undocumented, exponential backoff on 429/5xx recommended
- Key naming: API uses "subjects/topics/tests", UI uses "curriculums/courses/surveys"

## Endpoints (combined from Stitchflow, Ibexa Connect, brain-mcp)

### Users
- GET /users — list (supports ?curriculums_assigned=true&roles_assigned=true for completion data)
- GET /users/{id} — get
- POST /users — create/invite (email, first_name, last_name required)
- PATCH /users/{id} — update
- DELETE /users/{id} — archive
- POST /users/{id}/unarchive — restore
- POST /users/{id}/assign_subjects — assign subjects (comma-separated IDs)
- POST /users/{id}/unassign_subjects — remove subjects
- POST /users/{id}/assign_roles — assign roles
- POST /users/{id}/unassign_roles — remove roles

### Subjects (Curriculums)
- GET /subjects — list (?assigned_users=true to include user assignments)
- GET /subjects/{id} — get

### Topics (Courses)
- GET /subjects/{id}/topics — list topics for subject
- GET /subjects/{id}/topics/{topic_id} — get

### Tests (Surveys)
- GET /subjects/{id}/tests — list tests for subject
- GET /subjects/{id}/tests/{test_id} — get

### Roles
- GET /roles — list (?assigned_users=true)

## Top Workflows
1. Training compliance audit — who's behind on completion? Which curriculums have gaps?
2. Onboarding tracking — new hires progressing through assigned training?
3. Content quality audit — empty curriculums, curriculums without tests, untitled surveys
4. Role-based progress reports — completion by venue/role
5. Bulk assignment management — assign/unassign subjects and roles across users

## Table Stakes
- List/get/create/update/archive users
- List/get subjects, topics, tests
- Assign/unassign subjects and roles to users
- Role management
- Pagination handling
- Zapier-equivalent triggers (completion events)

## Data Layer
- Primary entities: users, subjects (curriculums), topics (courses), tests (surveys), roles
- Sync cursor: updated_at on users; full sync for subjects/topics/tests (no cursor available)
- FTS/search: users by name/email, subjects by name, cross-entity training status

## Key Quirks (from brain-mcp / production experience)
- Use completion_percentage NOT avg_completion (avg_completion is broken)
- URL encode all nested params with encodeURIComponent
- Batch operations — never N+1, use ?curriculums_assigned=true&roles_assigned=true
- Paginate all endpoints (per_page=50 max)
- Cache TTL: users=15min, curriculums=30min, roles=60min
- Use role ID never name (names can change)
- Response pattern: { data: [...] } for arrays, { user: {...} } for single resource

## Product Thesis
- Name: trainual-pp-cli
- Why it should exist: No CLI exists for Trainual. Zero competing tools. The API is small but the power-user workflows (compliance audit, onboarding tracking, content quality analysis) require correlating users + subjects + completion data + roles — something no single API call provides. A local SQLite store enables offline compliance dashboards, historical tracking, and cross-entity queries impossible through the web UI or Zapier.

## Build Priorities
1. Data layer — sync all entities to SQLite, FTS search
2. All CRUD operations with agent-native output (--json, --dry-run, --select)
3. Transcendence: compliance audit, onboarding tracker, content quality analyzer, completion trends, role coverage gaps
