# Canvas LMS CLI Brief

## API Identity
- Domain: Learning Management System — course management, assignments, grades, files, discussions, messaging
- Users: Students checking deadlines/grades, instructors managing submissions/grading, admins running reports
- Data profile: Courses → Assignments → Submissions → Grades; heavy relational, paginated list endpoints, bearer-token auth

## Reachability Risk
- **None** — Official Swagger 1.2 spec with 140 resource files at canvas.txstate.edu (verified 200). Token auth works.
- Minor: New Quizzes (v2) engine is LTI-only, not REST. Skip quiz submission; legacy quizzes covered.
- Minor: Canvas paginates via `Link` header (not offset); must implement RFC 5988 link-header pagination.

## Auth
- Type: Bearer token (`Authorization: Bearer $CANVAS_API_TOKEN`)
- Token: per-user access token generated in Canvas Settings → Approved Integrations
- No OAuth PKCE for simple installs; auth setup = "export CANVAS_API_TOKEN=..."

## Top Workflows
1. **Student deadline dashboard** — aggregate all due dates across all enrolled courses; most-used workflow, impossible from UI in a glanceable format
2. **Grade check** — see current scores across all courses without clicking into each individually
3. **Assignment detail + submission status** — check if submitted, what was submitted, score received
4. **Announcement digest** — compile all recent announcements across all courses (professors bury important info here)
5. **File download** — grab lecture slides, rubrics, assignment templates; very high frequency

## Table Stakes (from competing tools)
- fuller (Rust): list courses/todos/assignments, view assignment detail, ignore todo
- canvas-cli-and-mcp (TS): courses, upcoming, todos, assignments, submissions, submit (text/url/file), discussions, files, modules, pages, announcements
- canvasctl (Python): sync, today (due today), upcoming, diff (change tracking), announcements
- canvas-mcp vishalsachdev: 88 tools covering full student+instructor+admin surface
- canvas-mcp DMontgomery40: 54 tools, Docker/K8s support

## Data Layer
- Primary entities: Course, Assignment, Submission, Announcement, Module, ModuleItem, File, DiscussionTopic, Enrollment, CalendarEvent, User
- Sync cursor: `updated_since` param on most endpoints (ISO8601)
- FTS/search: assignment titles, course names, file names, announcement bodies
- SQLite snapshot enables: cross-course joins, deadline diffs, grade history, workload scoring

## Codebase Intelligence
- ucfopen/canvasapi (Python, 660★): de facto SDK. Pagination via PaginatedList with `Link` header. Rate limiting: sleep+retry on 403/throttle.
- vishalsachdev/canvas-mcp (TS, 129★): 88 tools across student/instructor/admin. Uses `CANVAS_API_TOKEN` + `CANVAS_BASE_URL`. Confirmed working auth pattern.
- node-canvas-api (JS, 72★): `node-canvas-api` npm, 54+ functions, handles link-header pagination transparently
- atomicjolt/canvasapi (Go, 3★): Go wrapper, pagination support

## Product Thesis
- **Name:** canvas-pp-cli
- **Why it should exist:** Every Canvas CLI is a student side-project with 0-2 stars and no package distribution. No tool handles pagination transparently, provides instructor workflows, or gives a polished auth setup flow. The de facto tools are MCP servers (rich tool surface but no terminal UX). A Go CLI with transparent pagination, SQLite snapshot layer, and both student+instructor workflows would immediately be the best thing in the category.

## Build Priorities
1. Core student loop: sync courses → list assignments with due dates → check submission status → view grades
2. Instructor grading loop: list submissions → grade with rubric → download/upload files
3. Transcendence: cross-course deadline scoring, grade history tracking, announcement digest, overdue detector
