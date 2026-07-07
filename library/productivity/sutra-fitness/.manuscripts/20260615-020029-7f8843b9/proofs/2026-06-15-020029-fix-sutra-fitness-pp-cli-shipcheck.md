# Sutra Fitness CLI — Shipcheck Report

## Shipcheck umbrella (6/6 legs PASS)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative (--strict --full-examples) | PASS |
| dogfood | PASS |
| workflow-verify | PASS (no manifest; skipped) |
| verify-skill (+ canonical-sections) | PASS |
| scorecard (--live-check) | PASS |

**Verdict: PASS (6/6).** Sample output probe: 8/8 (100%).

## Scorecard: 91/100 — Grade A
Output Modes 10, Auth 10, Error Handling 10, Terminal UX 10, README 10, Doctor 10,
Agent Native 10, MCP Quality 10, MCP Desc 10, MCP Remote Transport 10, Local Cache 10,
Vision 10, Workflows 10, Insight 10, Agent Workflow 9, Breadth 9.
Domain Correctness: Path Validity 8, Auth Protocol 10, Data Pipeline 10, Sync Correctness 10, Dead Code 5/5.

### Scorecard gaps (non-blocking, above 65 ship floor)
- **mcp_token_efficiency 4/10** — endpoint-mirror MCP surface (12 tools) is verbose. Polish target; would need spec MCP `orchestration: code` + regeneration, which risks the hand-fixed sync.go. Left for polish/future reprint.
- **cache_freshness 5/10** — cache intentionally disabled (operator-controlled sync + stale-read hints; pre-read auto-refresh would surprise users and the API is rate-limited).
- **type_fidelity 2/5** — loose generated types (the create-reservation oneOf body fell back to --body-json). Acceptable.

## Phase 4.8/4.9 — Doc correctness audit
Clean except 1 WARNING (SKILL frontmatter description concatenated a truncated headline into "Trigger phrases:"). **Fixed**: shortened `narrative.headline` in research.json and hand-corrected SKILL.md:3 + root.go Short/Long to a clean complete sentence. All commands, flags, examples, auth narrative, anti-triggers verified accurate; no dropped-feature claims; no `--group-by location` claims.

## Phase 4.95 — Local code review (hand-written Go)
0 errors, 1 warning. Clean on SQL injection (group-by fragments from fixed switches; dates via `?` placeholders), NULL-safety (no silent row-drops; sql.Null* / COALESCE throughout), resource leaks, divide-by-zero, context propagation, and dependent-sync parent-key injection / pagination / fan-out cap / partner-id handling.
- **WARNING (fixed)**: missing `rows.Err()` checks after `for rows.Next()` loops in the 8 analytics commands — a mid-iteration driver error could silently truncate. Added `rows.Err()` checks to scorecard (x2), no-shows, utilization, expiring, churn, referral-funnel, ltv. Re-built, re-tested, shipcheck re-run PASS.

## Phase 4.7 — sync-param-drop: skipped (no traffic-analysis; spec-source run, no browser-sniff).

## Generator defect found & worked around (retro candidate)
A spec with a universal leading `{partnerId}` path param on every endpoint caused the
generator to emit an empty sync registry (`syncResourcePath`/`defaultSyncResources`/
`knownSyncResourceNames`), default the wrong cursor param (`after` vs `start_after`), and
generate no dependent-resource iteration. Hand-fixed (registry + pagination + dependent
reservations/rooms fetch in `internal/cli/sutra_sync_deps.go`); verified path resolution +
live 401 reachability. Logged for Phase 6 retro.

## Behavioral correctness
All 8 transcendence analytics verified against a designed seeded dataset (math checked
per command — see build log) plus a Go behavioral test (`sutra_analytics_test.go`).
Found & fixed a `round2` negative-rounding bug during this testing.

## Ship recommendation: **ship**
All ship-threshold conditions met: shipcheck 6/6, scorecard 91 (>65), no broken flagship
feature, doc + code reviews clean (warnings fixed). Live smoke testing skipped (user chose
build-without-key); verified against mocks, dry-run, and a seeded local store.

---

## ADDENDUM — Live testing ran (operator provided a key at publish time)

The "live smoke testing skipped" note above is superseded. The operator supplied
a Partner API key during the publish flow, so full live testing ran against the
real Sutra/Arketa API. See the live-smoke proof for detail. Summary:

- Live sync pulled 34,523 records (6,995 clients, 4,461 purchases, 865 classes,
  22,143 reservations); all 8 analytics verified on real data.
- **Two correctness bugs found and fixed:** (1) pagination cursor double-encoding
  that capped sync at the first page; (2) reservation status enum mismatch
  (real data uses ATTENDED/no NO_SHOW vs spec CHECKED_IN/NO_SHOW). A third fix
  made the framework `tail` command partner-scoped + validate its argument.
- Publish live gate (`dogfood --live --level full`): 83 passed, 0 failed, 61 skipped.
- Re-ran shipcheck after each fix: 6/6 PASS throughout.

Final ship recommendation remains **ship** — now backed by live verification.
