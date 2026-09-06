# seats-aero reprint — peer-review evidence (2026-09-06)

Independent review lenses run against the promoted tree before publish, with every finding verified against the code before it was accepted (the durable log with verify-the-reviewer dispositions lives in the workshop's `docs/agent-reviews/`). No personal data appears in any finding; output samples are route/availability data only.

## Round 1 — Phase 4.95 (peer-code, peer-test, peer-security) + Greptile r1/r2 on PR #1953
- Blocker: `recheck` compared a Go `time.Time` against RFC3339 text → `datetime()` normalisation, mutation-verified.
- Major: `reach` local mode faked evidence (destinations has no origin column) → removed.
- Medium: quota guard failed open on unknown quota → fail-closed with explicit `--ignore-quota`.
- Six test false-greens fixed (fixtures that did not match real store shapes); Greptile r1: eight findings (legacy credential fallback, date-only bounds, first-seen backfill, local take limit, reach current-only evidence, recheck partial-refresh reporting, MCP HTTP timeouts, `which` discoverability); r2: `CredentialSource` for the legacy TOML path. All fixed, all recorded in `.printing-press-patches/`.

## Round 2 — Tier 2/3 gate (privacy, architecture, database, performance, interface, reliability)
Three Blockers and twelve Majors, all fixed in the same head with mutation-checked tests:
| Lens | Finding | Fix |
|---|---|---|
| interface | flagship rename `seats-aero-partner-search`→`awards` shipped with no alias or upgrade note | hidden deprecated alias forwarding to `awards` (`--cabin`→`--cabins`), "Upgrading from 2026.8.1" in README/SKILL |
| performance | generated local list loaded the whole `resources` partition (7 s / 3.9 GB at 300k rows) | typed-table SQL push-down + streaming fallback with early stop |
| performance | `reach` full-scanned per destination (0.56 s each) | expression indexes on route airports + date (4 ms) |
| architecture / database | `availability_all` preferred the older `availability` copy over a fresher `awards` write-through | freshest-wins by `synced_at` (`NOT EXISTS` both ways) |
| reliability / database | `immutable=1` read DSN on a TRUNCATE-journal store → torn reads (reproduced) | `mode=ro` without `immutable` |
| database / performance | `(route_id,date)` indexes indexed only NULLs (generator never matches `RouteID`) | dropped; real expression indexes; first-seen + source indexes |
| performance | first-seen backfill + view rebuild on every read-write open (+0.57 s per command) | one-time work gated by `store_extras_meta` |
| interface | framework `search <query>` POSTed a fabricated body to the metered `/live` endpoint | local-only |
| reliability | auto-refresh issued a guaranteed-400 call per read after the stale window | skip with `missing_required_params`, honest `meta.freshness` |
| reliability | `doctor` migrated the store one-way as a side effect | read-only open + `migration_pending` |
| interface | MCP intents annotated destructive; `find_best_award` promised a trips step | read-only/non-destructive annotations; honest description; cabin default |
| interface | novel commands returned `results: []` exit 0 on a never-synced store; usage errors had no stdout envelope | `meta.synced`/`meta.last_synced_at`; `{error,usage}` envelope |
| reliability | `reach --confirm-live` discarded the whole result on one live-check error | per-item `live_check_error` + `warnings[]`, ≤10-call cap intact |
| architecture | novel commands relied on a generated side-effect open to create the view | schema probe in the read-only open; upgrade-window guard |
| privacy | pipeline logs and home-directory paths in the shipped manuscripts | removed / placeholdered |

Tier-1 re-review of the fix diff (peer-code + peer-test) then found a nil cobra context in the alias, a skip reported as "refreshed", and a read-only upgrade-window gap, plus eleven tests that were not load-bearing; all fixed and mutation-verified. A harness regression report (typed `routes` table empty on pre-2.0 stores → silent empty local read) was fixed by falling back to the legacy table with a hint.

## Confirmed sound by the reviewers
Vendor data carries no personal data; API-key lifecycle (header-only, stdin entry, 0600 files, masked dumps); invocation journal minimisation; the explicit 78-column view; trigger semantics (`DO UPDATE` does not re-fire `AFTER INSERT`); per-page sync transactions; the fail-closed quota guard; the HTTP client's retry/`Retry-After` policy; MCP output bounding.
