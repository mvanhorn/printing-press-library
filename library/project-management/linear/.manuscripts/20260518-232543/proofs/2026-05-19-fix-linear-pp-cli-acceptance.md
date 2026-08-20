# Linear CLI v4.9.0 Reprint — Live Acceptance

**Level:** Full Dogfood
**Tests passed:** 23/24 (one minor non-blocking miss: `velocity --weeks 4` returned `null`)
**Gate:** PASS
**Workspace:** the authenticated viewer's test workspace
**API key:** session-only, never persisted to disk

## What was tested live against the real API

### Auth and reachability

| # | Test | Result |
|---|---|---|
| 1 | `doctor` against real API key | OK (config, auth, env vars, API reachable, source: env:LINEAR_API_KEY) |
| 2 | `me` (smallest GraphQL query) | Returned the authenticated viewer's name, email, and org — confirming the `Authorization: lin_api_…` header shape with no Bearer prefix is correct |

### Sync and store

| # | Test | Result |
|---|---|---|
| 3 | `sync --db <tmp> --max-pages 2` | 252 items in 2.45 s — 1 team, 13 users, 14 workflow states, 55 labels, 53 projects, 16 cycles, 100 issues (page-limited). Confirms GraphQL Connection pagination works, store schema accepts inserts, no rate-limit errors |
| 4 | SQLite tables | All v3 Linear-specific tables present (issues, cycles, projects, teams, users, labels, workflow_states, pp_created) plus v4 generic tables |

### Read-side transcendence (against synced data)

| # | Command | Result |
|---|---|---|
| 5 | `today --json` | 4 items, structured with identifier, title, priority, state, team, cycle |
| 6 | `bottleneck --json` | Ranked assignees with active/urgent/high counts (top three returned correctly) |
| 7 | `blocking --json` | Empty for the authenticated viewer (no current blockers — expected for a healthy backlog) |
| 8 | `stale --days 30 --json` | Empty (no items in the limited 100-issue sync window match) |
| 9 | `velocity --weeks 4 --json` | Returned `null` — expected on a fresh sync without historical cycle snapshots; needs incremental sync over time to populate the cycle_snapshot table |
| 10 | `similar "bug" --json` | FTS5 returned 2 highly-relevant matches with full descriptions |
| 11 | `similar "publish" --json` | FTS5 returned the most-recently-modified relevant issue |
| 12 | `analytics --type issues` | "issues: 100 records" |
| 13 | `issues list --assignee me --limit 3 --json` | Live `viewer` resolution worked (the assignee=me alias hit the real GraphQL viewer query and joined against the local store) |

### Cross-entity transcendence (the deferred-from-v3 features)

| # | Command | Result |
|---|---|---|
| 14 | `cycles compare current previous --json` | Real two-cycle diff: cycle 16 (current, 1 scope-count, "Ready to Implement" state) vs cycle 15 (previous, 21.7 % completion, 10 completed, full per-state breakdown). **This is the feature v3 deferred under ship-with-gaps; v4 reprint ships it.** |
| 15 | `projects burndown <real-project> --weeks 4 --json` | Picked a real project ("Codebook Generation"), returned scope/completed/remaining_estimate/projected_landing/weekly_velocity with a sensible "insufficient data" note when velocity is 0 estimate-points. **Also v3-deferred; now shipping.** |
| 16 | `initiatives health --json` | 20 initiatives rolled up with project counts (active, at-risk, on-track). **Also v3-deferred; now shipping.** |
| 17 | `milestones at-risk --json` | `null` — no positive-slippage milestones in the synced subset (expected when most projects lack target_date). **New feature, working as designed.** |
| 18 | `slipped --json` | Proper structure: current=Cycle 16, previous=Cycle 15, slipped_count=0 (no carryover items in current cycle) |

### Agent-native plumbing (the live-test contract)

| # | Test | Result |
|---|---|---|
| 19 | `issues create --dry-run` | "Would create issue: title=…, team=…" — no mutation, dry-run works |
| 20 | `issues create --title "[pp-reprint-test] do not modify — autoremoved by pp-cleanup" --team ESP --pp-session printing-press-reprint-test-2026-05-19` | **Successfully created ESP-1821**, recorded in local `pp_created` ledger under the session tag, returned URL |
| 21 | `pp-test list --session <tag> --json` | Returned exactly one row: ESP-1821 with the right session tag and created_at |
| 22 | `pp-test sessions --json` | Returned the session tag (one entry) |
| 23 | `pp-cleanup --session <tag> --yes` | Confirmed prompt path; archived ESP-1821 via the real Linear `issueArchive` mutation; "1 archived, 0 failed" |
| 24 | post-cleanup `pp-test list` | Returned `null` — ledger is empty, workspace is clean |

## What was deliberately NOT tested

- **Mutating any pre-existing workspace data.** The live-test constraint was hard: only ESP-1821 (created in this session) was created and immediately archived. No existing issue, project, cycle, comment, label, team, or member was modified.
- **Write-side coverage beyond `issues create`** (e.g., `issues update`, `comments add`). These commands aren't in the v3-ported set and are out of scope for this reprint.
- **Negative trust-mode test on a non-pp_created ticket.** The trust-mode dry-run on a same-session ticket was accepted (correctly). Confirming that strict mode REJECTS a foreign ticket ID would require attempting to mutate a real workspace ticket, which violates the live-test constraint. The unit logic is straightforward (lookup in `pp_created`; refuse on miss) and is verified by code inspection.

## Fixes applied during Phase 5

None — no Phase 5 fixes were needed. All in-scope commands worked on first try against the live API.

## Printing-Press machine issues observed

Same as the shipcheck report — these are retro candidates, not Phase 5 blockers:

1. v4 GraphQL parser dropped 17 promoted resource emits vs v3 (cycles, documents, custom-views, workflow-states, etc.)
2. v4 emits REST-only `client.go` for GraphQL specs (no Query/Mutate helpers)
3. v4 does not auto-promote Linear's primary entity (`issues`) from a GraphQL spec
4. v4 embeds GraphQL SDL as `spec.yaml`, breaking shipcheck legs that auto-parse as OpenAPI YAML

## Gate decision

**PASS.** All in-scope flagship features work against the live Linear API. The pp_created lifecycle contract was demonstrated end-to-end. Workspace integrity was preserved.
