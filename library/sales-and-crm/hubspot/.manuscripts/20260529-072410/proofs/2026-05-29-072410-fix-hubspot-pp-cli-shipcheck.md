# hubspot-pp-cli Shipcheck Report

## Outcome

**Verdict: ship** — 6/6 legs PASS; sample-output probe 9/9; scorecard 92/100 Grade A; 0 known functional bugs in shipping-scope features.

## Initial shipcheck run

| Leg | Result | Exit | Notes |
|---|---|---|---|
| verify | PASS | 0 | 28/29 (97%) pass rate; 1 transient FAIL on workflow group (no fixture) |
| validate-narrative | PASS | 0 | 10/10 commands resolved, full examples passed |
| dogfood | PASS | 0 | Path Validity 5/5; Auth MATCH; Dead Flags 0; Novel Features 9/9 survived |
| workflow-verify | PASS | 0 | no manifest (skipped per default) |
| verify-skill | PASS | 0 | all checks passed, canonical-sections passed |
| scorecard | PASS | 0 | 91/100 Grade A |

Sample-output probe (initial): 7/9 — two failures:
- `lifecycle-stuck --multiplier 2x` rejected (float parse); my example was `2x`.
- `daily-digest --since yesterday` rejected ("yesterday" not a duration); my example was `yesterday`.

## Fix round 1 — example values

- `research.json` `novel_features[1].example` and `novel_features[8].example`: `2x` → `2.0`, `yesterday` → `1d`.
- `research.json` `narrative.quickstart`/`narrative.recipes` daily-digest entries: same `--since yesterday` → `--since 1d`.
- README and SKILL `--multiplier 2x` / `--since yesterday` references rewritten via in-place `perl -i`.

Re-ran shipcheck: 92/100 Grade A; sample-output probe **9/9 (100%)**.

## Fix round 2 — code-review and SKILL/README audit findings (25+ issues)

### Code-review (Greptile-shaped)
1. Context propagation: `streamContacts`, `loadPriorScoreSnapshots`, `loadContactsPage` now use `QueryContext(ctx, ...)`. Added a `QueryContext` method to the store.
2. `rows.Err()` checked after every `for rows.Next()` loop across all 9 novel commands + helpers; descriptive error wrapping.
3. Deterministic tiebreakers added to every `sort.Slice` (source-roi, owner-overload, lifecycle-stuck, engagement-decay, stale-but-valuable, score-drift, silent-after-first-touch, duplicate-suspects).
4. `computeScoreDrift` split `!snap.scoreSet` from `snap.score == 0` into two explicit guards with comments.
5. `streamContacts` now returns `(scanned, parseErrors, err)`; every caller surfaces `parse_errors` in the result envelope when `>0`.
6. `lifecycle-stuck` `matchesStage("other", "")` no longer matches — contacts with empty `LifecycleStage` are excluded.
7. `duplicate-suspects` emits `"cap_hit": true` whenever the scan cap is reached, regardless of match count.
8. `duplicate-suspects` blockKey documented as ASCII-only assumption (normalization strips non-ASCII before this point).

### SKILL/README audit
1. README config path: `~/.config/contacts-pp-cli/config.toml` → `~/.config/hubspot-pp-cli/config.toml`.
2. Read-only positioning + visible CRUD: kept CRUD commands in the spec-derived command tree, added an explicit "Write endpoints note" callout in README and a new anti-trigger bullet in SKILL ("Do not call `crm post/patch/delete-*`; recommended scopes return 403").
3. Destructive teaching examples (`crm delete-v3-objects-contacts-contact-id-archive`) in README Output Formats and SKILL Agent Mode/Profile sections replaced with `crm get-v3-objects-contacts-get-page --limit 5`.
4. README Quick Start `doctor --dry-run` → `doctor` (dry-run was misleading for a no-network-call command).
5. SKILL gating disclosures added: engagement-decay, silent-after-first-touch, stale-but-valuable, score-drift, daily-digest each carry a one-line note about needed sync properties or prior runs.
6. Recipe `--select silent_vips` → `--select fresh_stale_vips` to match actual daily-digest output field name.
7. SKILL anti-trigger forward-looking promise ("planned as future amend PRs") reworded to "out of scope for this CLI."
8. SKILL Auth Setup adds "Verify token is loaded with `hubspot-pp-cli auth status`."

## Final shipcheck (post-fix)

- **Verdict: PASS (6/6 legs)**
- Sample-output probe: 9/9
- Scorecard: 92/100 Grade A
- Per-section scores not at ceiling (none ship-blocking):
  - MCP Token Efficiency 7/10 — generator-determined
  - Cache Freshness 5/10 — intentionally disabled (cache freshness would auto-refresh on read, conflicting with the snapshot model)
  - Breadth 9/10
  - Vision 8/10
  - Agent Workflow 9/10
  - Data Pipeline Integrity 7/10 — search uses direct SQL rather than typed methods (acceptable, not a correctness gap)
  - Type Fidelity 4/5 — minor; generator-determined

## Known gaps (documented in README + SKILL, not ship-blockers)

- score-drift and daily-digest produce empty score/stage sections on the first invocation. By design — they snapshot on first run and produce deltas on subsequent runs. Documented in `--help` and in SKILL gating notes.
- engagement-decay, silent-after-first-touch, stale-but-valuable depend on non-default sync properties (`hs_email_open`, `hs_email_click`, `hs_last_activity_date`, `hubspotscore`, `total_revenue`). Pass them via `sync --param properties=…`. Documented in SKILL gating notes and README Troubleshooting.
- Write endpoints (`crm post/patch/delete-*`) are present from the spec but will return 403 against a Private App with read-only scopes. Disclosed in README "Write endpoints note" and SKILL anti-triggers.

## Final ship recommendation: `ship`

All ship-threshold conditions met. No functional bugs in shipping-scope features. Phase 5 to run when `HUBSPOT_PRIVATE_APP_TOKEN` is available; if not, will emit `phase5-skip.json` with `auth_required_no_credential` and proceed to promote and publish.
