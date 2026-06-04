# Handoff: jobber-pp-cli (paused press run)

**Created:** 2026-05-23 (initial pause)
**Updated:** 2026-05-24 (scope locked, codex mode enabled, awaiting emit-path decision)
**Previous session:** Claude Opus 4.7 (1M context), CWD `C:\Users\melan\printing-press`
**Skill in progress:** `/printing-press jobber codex`

## What the user is doing

Mark (Bouvier Advisory Partners, fractional CFO / M&A advisory) wants a read-only Go CLI for Jobber's GraphQL API, generated via the printing-press skill. CLI will live at `~/printing-press/library/jobber/` and be reusable across BAP clients on Jobber. Primary current client is Heritage; tenant-specific features are explicitly deferred to a later session.

Read these first — do **not** restart from scratch:

1. **Preservation folder:** `C:\Users\melan\printing-press\library\jobber\` (everything Phase 1.5 produced)
   - `README.md` — what's done, what's not, how to resume
   - `.resume-pointer.json` — machine-readable run state
   - `research/*-brief.md` — full Phase 1 brief
   - `research/*-absorb-manifest.md` — 20 absorbed + 8 novel features (USER APPROVED at Phase Gate 1.5)
   - `research/*-novel-features-brainstorm.md` — subagent audit trail
   - `research.json` — feature inventory for `generate --research-dir`
   - `gates/browser-browser-sniff-gate.json` — skip-silent marker
   - `state.json` — run id, paths
2. **Original research the user pre-built:** `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\schema\` — live 2026-05-15 GraphQL introspection: 18 endpoints, 22 objects, 431 fields. Brief is derived from this.
3. **Runstate:** `C:\Users\melan\printing-press\.runstate\jobber-cf0f9d34\runs\20260522-234028\` — same files as preservation folder; empty `working/jobber-pp-cli/` shell; no lock held.

## Phases completed (do not redo)

- Phase 0 Resolve (catalog empty, library empty, no lock)
- Phase 1 Research brief
- Phase 1.5 Absorb gate — 28 features approved
- Phase 1.6 Auth intel — OAuth2 with rotation, all 6 env vars set in User scope
- Phase 1.7 Browser-sniff gate — skip-silent
- Phase 1.9 Reachability — PASS (account query HTTP 200, throttle 9998/10000, version `2025-04-16`)

Mid-session: stale refresh token detected; OAuth re-authorized via `browser-use` skill against live Chrome (auto-approved, app already authorized). Fresh tokens persisted to Windows User env via `[Environment]::SetEnvironmentVariable(..., 'User')`. Callback tab closed. Tokens should be valid for the standard Jobber TTL from ~2026-05-23 00:08 UTC; if expired on resume, run the same dance.

## Scope (locked 2026-05-24)

User reviewed the full manifest, selected an AR-and-pipeline scope, and explicitly cut the labor/cost dimension (expenses are not recorded in their system, so timesheets, expenses, users, and `jobs pnl` are out).

**16 commands. Persona: Mark / BAP / Heritage.**

| Group | Commands |
|---|---|
| Chassis | `doctor`, `sync`, `search` (FTS5), `sql` (read-only passthrough) |
| Clients | `clients list`, `clients get` (+ `--expand`) |
| Properties | `properties list` |
| Quotes | `quotes list` |
| Jobs | `jobs list`, `jobs get` (+ `--expand`) |
| Visits | `visits list` |
| Invoices | `invoices list`, `invoices trace` |
| Payments | `payment-records list` |
| Novel | `ar aging`, `snapshot diff` |

Hand-code budget: ~300-500 LoC. Sync engine dominates; `ar aging`, `invoices trace`, `snapshot diff` are each ~100-150 LoC.

### Dropped (do not build)

- `A4 requests list` — funnel cut; if later added, start at quote.
- `A11 payout-records list` + `N3 payouts reconcile` — **open question.** Re-add if Heritage uses Jobber Payments and reconciles deposits to bank. A11 is XS, N3 is S-M.
- `A12 timesheets list`, `A13 expenses list`, `A14 users list`, `N4 jobs pnl` — labor/cost dimension cut wholesale.
- `A15 products list`, `A16 tax-rates list` — no line-item/tax analysis planned.
- `N5 jobs stale`, `N6 funnel`, `N8 clients 360` — not in this pass; reprint candidates for a later polish session.

## Why it's still paused

Scope is locked, but the press generator emits REST-shaped Go (`client.Get(path)` per endpoint) and Jobber is GraphQL-only. The emit-path decision is the next blocker before `printing-press generate` can usefully run:

| Route | Approach | Trade-off |
|---|---|---|
| A. GraphQL emit override | Replace REST templates with GraphQL-aware ones | Cleaner output, more upfront template work, may need a press plugin |
| B. Fake REST + post-rewrite | Treat each root surface as `/v1/{resource}`, rewrite transport layer after generate | Faster to start, more handwritten rewrite per file, easier to drift from generator output |

Codex mode is now enabled (`/printing-press jobber codex`); code-writing tasks (sync engine, novel commands, transport rewrite) get delegated to Codex with a 3-failure circuit breaker. Research, scope, and verification stay on Claude.

## Constraints carried forward (non-negotiable)

- **Read-only.** No mutations. Per `schema/discovery_plan.md`. The press will introspect mutations from the spec; the absorb manifest already filters them out.
- **No tenant data in tracked artifacts.** No tokens, tenant IDs, account names, customer rows, or raw GraphQL payloads in research/proofs/manuscripts. Use redacted shape summaries only. The Phase 1 reachability ping already followed this — `account.id` was masked, response body deleted after inspection.
- **Refresh-token rotation.** Every refresh MUST persist the new refresh token to Windows User env (`JOBBER_REFRESH_TOKEN`). Per `schema/auth.md`.
- **Output location accepted at `~/printing-press/library/jobber/`** — user knows this is outside the BAP workspace and accepted the trade-off. They mentioned post-generation symlink/copy bridging may be useful; not done yet.

## Auth context (env vars, names only — NO VALUES)

All six set in Windows User scope:

- `JOBBER_CLIENT_ID` (UUID, ~36 chars)
- `JOBBER_CLIENT_SECRET` (~64 chars) — **never log this**
- `JOBBER_CALLBACK_URL` = `http://localhost:8080/jobber/callback`
- `JOBBER_ACCESS_TOKEN` (JWT, ~651 chars, ~60 min TTL)
- `JOBBER_REFRESH_TOKEN` (~32 chars) — **rotated on every refresh**
- `JOBBER_GRAPHQL_VERSION` = `2025-04-16` (active version)

OAuth re-auth pattern (worked in this session):

1. Build authorize URL with `client_id`, `redirect_uri`, `response_type=code`
2. Open via `browser-use open <url>` against live Chrome (`browser-use connect` first)
3. App is pre-authorized for this BAP tenant — auto-redirects to callback. Read code from `browser-use tab list`.
4. POST to `https://api.getjobber.com/api/oauth/token` with `grant_type=authorization_code`, `client_id`, `client_secret`, `code`, `redirect_uri`
5. Persist new tokens via `pwsh -NoProfile -Command "[Environment]::SetEnvironmentVariable(..., 'User')"`
6. Close callback tab (`browser-use tab close <idx>`)
7. Delete response file
8. Verify with `query { account { id } }` against `https://api.getjobber.com/api/graphql`

## What to do next

1. **Re-read** the preservation folder README + `.resume-pointer.json` (now contains `scope_locked` and `open_decisions`).
2. **Verify the access token still works:**
   ```bash
   curl -s -X POST https://api.getjobber.com/api/graphql \
     -H "Authorization: Bearer $JOBBER_ACCESS_TOKEN" \
     -H "X-JOBBER-GRAPHQL-VERSION: $JOBBER_GRAPHQL_VERSION" \
     -H "Content-Type: application/json" \
     -d '{"query":"query { account { id } }"}' | jq '.data.account.id = "<redacted>" | .extensions.cost.throttleStatus'
   ```
   If 401, redo the OAuth dance (see Auth context section). Tokens last refreshed 2026-05-23 00:08 UTC — almost certainly expired by next session; have the re-auth ready.
3. **Ask the user the two open decisions** (do not assume):
   - **Payouts pair?** Does Heritage use Jobber Payments and need bank-deposit reconciliation? If yes, add A11 + N3 to scope.
   - **Emit-path route?** A (GraphQL emit override) or B (fake REST + post-rewrite). See "Why it's still paused" above.
4. **Author the internal YAML spec** per `~/go/pkg/mod/github.com/mvanhorn/cli-printing-press/v4@v4.6.1/skills/printing-press/references/spec-format.md`. Use only the in-scope root surfaces from `endpoint_inventory.csv`. Object joins in `object_inventory.csv`, fields in `field_inventory.csv`.
5. **Run generate:**
   ```
   printing-press generate \
     --spec <spec.yaml> \
     --research-dir C:\Users\melan\printing-press\library\jobber \
     --output C:\Users\melan\printing-press\.runstate\jobber-cf0f9d34\runs\<new-run-id>\working\jobber-pp-cli \
     --force --lenient --validate
   ```
   (Substitute the absolute path emitted by preflight as `PRINTING_PRESS_BIN` — never call bare `printing-press`.)
6. **Delegate hand-build to Codex** (codex mode is on). Order of attack:
   1. Transport layer rewrite or GraphQL override (whichever route the user picked).
   2. Sync engine (A17) — biggest single workitem; cursor + throttle + OAuth refresh + store-layer SQLite tables.
   3. `ar aging` (N1) — port from `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\tenants\heritage_builders\probes\jobber_ar_recreation.py` (1964 lines, Python — read targeted ranges per context-efficiency rules, not bulk).
   4. `invoices trace` (N2), `snapshot diff` (N7).
   5. `search` (A18), `sql` (A19).
   6. List/get absorbed commands — should mostly come from the generator; rewrite any GraphQL gaps.
   Circuit breaker: 3 consecutive Codex failures → fall back to writing locally.
7. **Phase 4 shipcheck**, **Phase 5 live dogfood** (tokens live, real Heritage tenant), **Phase 5.5 polish**, **Phase 5.6 promote**.

## Notes from this session worth keeping

- The user prefers honest scope checkpoints over silent over-promising. They corrected me twice when I tried to barrel past clarification asks ("the user wants to clarify these questions"). When in doubt, ask before generating thousands of LoC.
- The user has substantial pre-built schema research and BAP-tenant context in `assignments/CRMs/Jobber/schema/`. Use it before doing web research. The Phase 1.5a web sweep that ran during this session found 0 dedicated Jobber CLIs and one community MCP (`flutchai/mcp-server-jobber`, 10 read+write tools, most rejected per read-only stance).
- The auto-memory note about Heritage's Jobber AR inflation (`heritage-jobber-as-crm-not-accounting.md`) is **deliberately deferred** for v1. Future sessions can layer those features in via `/printing-press-polish` or as a fresh feature pass after the generic CLI ships.
- Browser-use skill works well on this machine. `browser-use connect` attaches to live Chrome cleanly; `tab list` shows callback URLs even when the page itself errored.
- PowerShell tool is broken on this Claude Code version (per `~/.claude/rules/windows-shell.md`). Use `pwsh -NoProfile -Command "..."` invoked via the Bash tool.

## Suggested skills

- **`/printing-press`** — only if the user wants to restart from scratch. NOT recommended; the preservation folder lets us resume mid-flow.
- **`browser-use`** — for the OAuth re-auth dance if tokens expire on resume. Already proven to work against this user's Chrome.
- **`/printing-press-polish`** — if the chosen scope was `minimal-viable` or `chassis-only` and the user later wants to add the remaining commands.
- **`/printing-press-amend`** — if there's a single named bug or a small feature add against an already-generated CLI (only useful AFTER Phase 2 completes).
- **`memory-management`** — there's a useful Heritage-specific note (`heritage-jobber-as-crm-not-accounting.md`) that's intentionally NOT applied in v1 but should be remembered for the next phase.

## Files referenced (do not duplicate; read at runtime)

- `C:\Users\melan\printing-press\library\jobber\README.md`
- `C:\Users\melan\printing-press\library\jobber\.resume-pointer.json`
- `C:\Users\melan\printing-press\library\jobber\research\*.md`
- `C:\Users\melan\printing-press\library\jobber\research.json`
- `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\schema\brief.md`
- `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\schema\auth.md`
- `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\schema\api_surface.md`
- `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\schema\discovery_plan.md`
- `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\schema\endpoint_inventory.csv`
- `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\schema\object_inventory.csv`
- `C:\Users\melan\Documents\Bouvier_Advisors\assignments\CRMs\Jobber\schema\field_inventory.csv`
- `~/go/pkg/mod/github.com/mvanhorn/cli-printing-press/v4@v4.6.1/skills/printing-press/references/spec-format.md`
- `~/.claude/skills/printing-press/SKILL.md` (the main skill)
- `~/.claude/projects/C--Users-melan-Documents-Bouvier-Advisors-assignments-CRMs-Jobber\memory\heritage-jobber-as-crm-not-accounting.md`

## Redaction notes

- All token values redacted from artifacts (length only, never value)
- Account ID redacted (`<redacted>`) from Phase 1.9 verification output
- Auth code from OAuth callback never logged; only character length emitted
- Tenant identifiers (BAP/Heritage names) appear in the file paths above; these are the user's own working directories and the user's chosen output location. No external party will read these unless the user shares this handoff file.
- The handoff file lives at `C:\Users\melan\printing-press\library\jobber\HANDOFF.md` (this file). Treat it as private to this machine — it sits inside the user's local printing-press library and is never published.
