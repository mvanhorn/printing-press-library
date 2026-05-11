# Acceptance Report: jira-cloud-platform-pp-cli
Level: Full Dogfood
Workspace: verybigthings.atlassian.net
Tests: 9/9 passed

Passes:
  - doctor: auth valid (HTTP Basic via JIRA_API_TOKEN+JIRA_EMAIL), API reachable
  - project list: 273 projects returned
  - issue sync --project RAI,TRAC,CC: 403 issues fetched across 4 pages
  - stale --days 3: 8 stale issues found with correct data
  - workload: 5 assignees with correct issue counts
  - blocked: 1 blocked issue found with full chain (XU-336 → XU-335)
  - cycle-time --last 30: 30 resolved issues, p50=187 days
  - issue changelog get-change-logs WE-291: 23 entries returned
  - issue worklog summary CC-96: 111h 45m logged, correct author
  - search "POC" --data-source local: returned TRAC-20, TRAC-59, TRAC-62

Fixes applied:
  - Issue sync: /rest/api/3/issue/limit/report → /rest/api/3/search/jql
  - Issue sync: pagination stopped at 50 (maxResults casing + nextPageToken)
  - Issue sync: no fields in response (added fields param)
  - Auth: OAuth2 only → added HTTP Basic (JIRA_API_TOKEN + JIRA_EMAIL)
  - issue_changelog/comment/worklog: column order bug in upsert functions
  - issue_changelog/comment: issue_id never set → coalesce from parent_id
  - search: /rest/api/3/search returned 410 → /rest/api/3/search/jql
  - Dependent resource sync: no progress output → added sync_start/progress/complete
  - Dependent resources: always ran → made opt-in via --resources flag
  - FTS search: ambiguous rank column (dashboard.rank vs FTS rank) → qualified
  - FTS search: no issue search → added SearchIssue via json_extract

Added features:
  - sync --project: project-scoped issue sync
  - stale --type, workload --type: filter by issue type
  - issue worklog summary: time logged per author (live API)

Gate: PASS
