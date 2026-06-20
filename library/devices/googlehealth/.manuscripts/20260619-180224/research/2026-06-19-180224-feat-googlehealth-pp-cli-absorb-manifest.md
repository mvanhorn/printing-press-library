# Google Health API CLI — Absorb / Transcend Manifest

Run ID: 20260619-180224

## Absorb layer (API surface)

The CLI mirrors the Google Health API v4 surface:

- `users data-types` — list, get, create, patch, batch-delete, roll-up,
  daily-roll-up, reconcile, and export-exercise-tcx over data points
  (`/v4/users/{usersId}/dataTypes/{dataTypesId}/dataPoints`).
- `users profile` / `users settings` — get and update.
- `users identity`, `users irn-profile` — get.
- `users paired-devices` — list and get.
- `projects subscribers` and `projects subscribers subscriptions` — webhook
  CRUD.

Auth is OAuth2 authorization_code via `auth login` (also surfaced as the
top-level `login`), with refresh-token persistence and automatic refresh. The
runtime bearer token is read from `GOOGLEHEALTH_OAUTH2C`.

## Transcend layer (novel features beyond the raw API)

The generator's default transcend commands were project-management-shaped
(`load`/`orphans`/`stale` — workload per assignee, items missing
assignee/project/priority) and domain-inappropriate for health data, which has
no assignees or work items. They were removed and replaced with genuine health
analytics authored in a new `internal/health` package:

| Feature | Command | What it adds |
|---------|---------|--------------|
| Local SQLite Sync | `sync` | Mirrors data points across data types into a local SQLite store |
| Full-Text Search | `search` | FTS5 full-text search over synced records |
| Metric Trend Lines | `trends` | Per-metric trailing rolling-average trend lines with net first→last delta, de-noising day-to-day whiplash |
| Goal Streaks | `streaks` | Current and longest consecutive-calendar-day streaks where a metric met a goal (gaps break the run) |
| Cross-Metric Correlation | `correlate` | Pearson correlation + best-lag scan between any two daily metrics (e.g. steps vs resting HR) |

`trends`, `streaks`, and `correlate` operate purely on the local store
(annotated `// pp:data-source local`) and are unit-tested in
`internal/health/health_test.go`. They are recorded as `novel_features` in
`.printing-press.json`. Cross-data-type correlation is the one capability even
garmy and oura-cli lack, on the one platform where nothing mature exists yet.

## Hand-edits (recorded as patches)

Two generated files were hand-edited, recorded under `.printing-press-patches/`:

- `replace-pm-slop-with-health-analytics` — deleted the three PM command files
  and re-pointed their registrations in `internal/cli/root.go` to the health
  commands.
- `wire-sync-googlehealth-datatypes` — populated the empty
  `defaultSyncResources` / `knownSyncResourceNames` / `syncResourcePath`
  functions in `internal/cli/sync.go` so `sync` populates data points (the
  generator left them empty because Google Health's data-point paths are
  parent-keyed and cannot be flat-derived).

## Disclosed gaps

- **webhook subscriptions** require a separate Google Cloud project identity and
  are outside the default OAuth-authenticated user-data read surface.
- **data-type fan-out**: a full `sync` issues one paginated request per data
  type, so the default resource set is curated to common metrics.
- **newly launched API**: field availability depends on the user's connected
  devices (Fitbit, Pixel Watch, third-party).

## MCP

25 MCP tools, readiness "full"; the Cloudflare orchestration pattern was not
triggered (25 endpoints < 50 threshold), so endpoint tools mirror the Cobra tree
at runtime.
