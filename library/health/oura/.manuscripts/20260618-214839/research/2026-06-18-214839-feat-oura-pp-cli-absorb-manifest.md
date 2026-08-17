# Oura CLI — Absorb / Transcend Manifest

Run ID: 20260618-214839

## Absorb layer (API surface)

The CLI mirrors the Oura Cloud API V2 read-only surface under `usercollection`,
with a list (`multiple-*-documents`) and single (`single-*-document`) command
for each data type: daily sleep, sleep, sleep time, daily readiness, daily
activity, heart rate, workouts, sessions, daily SpO2, daily stress, daily
resilience, VO2 max, daily cardiovascular age, tags, enhanced tags, rest mode
period, ring configuration, ring battery level, and personal info. Webhook
subscription CRUD is exposed under `webhook`; `sandbox` mirrors the Oura
sandbox routes that serve synthetic data for testing.

Auth is OAuth2 authorization_code via `auth login` (also surfaced as the
top-level `login`), with refresh-token persistence and automatic refresh.

## Transcend layer (novel features beyond the raw API)

| Feature | Command | What it adds |
|---------|---------|--------------|
| Local SQLite Sync | `sync` | Mirrors usercollection data into a local SQLite store for offline use |
| Full-Text Search | `search` | FTS5 full-text search over synced records |
| Local Analytics | `analytics` | count / group-by / summary queries over synced data |
| Compound Workflows | `workflow` | chains multiple API operations into one agent-friendly invocation |

These four commands are absent from every existing Oura tool surveyed (all of
which are stateless API clients). They are the CLI's differentiation and are
recorded as `novel_features` in `.printing-press.json`.

## Disclosed gaps

- **sync default resources**: `defaultSyncResources` is empty out of the box,
  so `sync` is a no-op until per-resource population is wired. This is the
  primary follow-up; the transcend layer (`search`/`analytics`) has no default
  data path until then.
- **insight**: scorecard insight scored 4/10; `analytics` ships generic
  placeholder examples that are not Oura-specific yet.
- **webhook auth**: webhook subscription routes require a separate
  `x-client-id` + `x-client-secret` pair and are not part of the default
  OAuth-authenticated read surface.

## MCP

75 MCP tools, readiness "full"; the Cloudflare orchestration pattern applies
(75 endpoints > 50 threshold), so endpoint tools are hidden behind orchestration
with stdio + http transports.
