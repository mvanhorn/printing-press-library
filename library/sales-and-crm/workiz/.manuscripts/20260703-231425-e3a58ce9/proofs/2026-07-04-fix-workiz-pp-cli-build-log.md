Manifest transcendence rows: 6 planned, 6 built.

# Workiz CLI Build Log

## What was built
- Generated 18 absorbed endpoint commands across 5 resources (job, lead, team, customer, timeoff) from the hand-authored internal YAML spec. Zero generation quality-gate failures (go mod tidy, govulncheck, go vet, go build, binary smoke tests, doctor all PASS).
- Hand-authored auth plumbing for Workiz's non-standard credential shape:
  - `internal/config/config.go` `Load()`: folds the resolved API token into `BaseURL` (Workiz embeds the token as a URL path segment, not a header/query param), skipped when `WORKIZ_BASE_URL` is set so `printing-press verify`'s mock server still matches spec-declared paths.
  - `internal/client/client.go` `doInternal()`: injects `auth_secret` into the JSON body of every write call (Workiz requires the secret as a body field, not a header), only for map-shaped bodies and only when not already present.
  - Verified end-to-end via `--dry-run` smoke test: token correctly appears (masked) in the URL path, `auth_secret` correctly appears in the POST body.
- Implemented all 6 transcendence features as hand-written Go, wired automatically by the generator into their resource-parent commands (`job audit`, `job revenue`, `job search <term>`, `lead funnel`, `team bottleneck`, and top-level `digest`):
  - `team bottleneck --week`: joins locally synced job assignments, team roster, and time-off to compute per-crew scheduled hours plus double-booking and time-off conflicts.
  - `lead funnel --since`: matches leads to the job they likely became (by contact identity, since Workiz has no convert-link field) and reports conversion rate + average job value per lead source.
  - `job revenue --group-by`: rolls up total/outstanding job value by source or status.
  - `job audit`: flags jobs/leads/customers missing phone, email, address, crew, or price data.
  - `digest --since`: reports new/changed jobs and leads since a duration cutoff, using CreatedDate/LastStatusUpdate.
  - `job search <term>`: free-text search across job/lead notes and comments (parsing Workiz's irregular Comments shape — empty string or array of `{Comment}` objects).
- Added `internal/cli/novel_shared.go` with shared JSON-shape structs (tolerating Workiz's wire inconsistencies: numeric vs string Team[].id, stringly-typed prices, "null"-literal timestamps) and store-loading helpers used by all 6 novel commands.
- Fixed a generator misclassification: `job audit`'s auto-derived `mcp:read-only` annotation was `false`; corrected to `true` since the command only reads.
- Verified all 6 novel commands against a synthetic SQLite fixture (3 jobs, 2 leads, 2 team members, 1 time-off record, 2 customers) with hand-checked expected output — every command produced correct, non-trivial, verifiable results (double-booking + time-off conflict correctly detected for the overbooked tech; lead-to-job match correctly found for matching contact info and correctly absent for a non-matching lead; revenue correctly summed per source; audit correctly flagged the one incomplete job and one incomplete customer; search correctly found "leak" across job notes/comments/lead notes and returned empty for a nonsense term).
- Removed 5 auto-scaffolded `t.Skip` placeholder test files (job_audit, job_revenue, lead_funnel, team_bottleneck, digest — logic covered by behavioral smoke test above plus shared-helper unit tests) and replaced job_search's skip test with a real assertion. Added `internal/cli/novel_shared_test.go` with table-driven tests for `parseWorkizTime`, `parseMoney`, `wzComments` unmarshal (both wire shapes), `flexibleID` unmarshal (both wire shapes), and `snippetAround`.

## Resource naming
`jobs` collided with a reserved framework Cobra command; `client` collided with a reserved generator template. Both resolved by using Workiz's own singular wire-path naming (`job`, `lead`, `team`, `timeoff`) plus `customer` for the Client resource — confirmed via `generate --dry-run` before the real generation run.

## Intentionally deferred
- None. All 18 absorbed + 6 transcendence features from the approved Phase 1.5 manifest are built and verified.

## Skipped/complex fields
- None — no request bodies were skipped. All documented body fields for job/lead/customer create/update/assign/unassign were included per the SDK ground truth.

## Generator limitations found (candidates for retro)
- No `auth.in: path` support for URL-path-embedded credentials — confirmed via scratch dry-run that a `{token}` path placeholder becomes a required positional argument on every generated command rather than being auto-injected from config. Worked around via a 2-line hand-edit to `config.go`'s `Load()`.
- No spec-level mechanism for injecting a credential into a POST body field automatically — worked around via a shared injection point in `client.go`'s `doInternal()`.
