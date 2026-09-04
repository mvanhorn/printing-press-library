# Splitwise CLI Brief

## API Identity

Domain: shared-expense tracking and settling. Concrete personas: **Riley** (roommate-household
bill-runner splitting rent/utilities/groceries in a standing group, wants to know how *long*
a balance has gone unpaid); **Sam** (trip organizer / treasurer who fronts trip costs and
reconciles at the end, wants an auditable ledger and a minimal-transfer settle-up plan); **Devon**
(couple / personal-finance tracker with joint spend and scattered old IOUs, wants search and
trend/normalization across currencies); **Avery** (agent operator driving the CLI from an LLM,
penalized by live-wrapper fan-out and nested-array context bloat). Data profile: personal
financial/relationship data — users, groups, friends, expenses with per-member shares, comments,
notifications — all PII-adjacent (names, balances, spend history) and single-tenant per API key.

## Reachability Risk

None. Evidence: official OpenAPI 3.0.1 spec at `dev.splitwise.com` (local copy verified below);
`GET /get_current_user` returns 401 without a key (auth-required shape, not a dead/renamed
endpoint). 2026-09-03 preflight probe of `https://secure.splitwise.com/api/v3.0/get_current_user`
returned 401 — PASS. `SPLITWISE_API_KEY` is present in the environment (ask before using for
Phase 5 live checks).

## Top Workflows

1. Who owes whom right now, and settle a trip in the fewest transfers (`balances`, `settle-up`).
2. Monthly/category shared spend, offline, without paging the live API (`spend`, `report`).
3. Fairness — who's carrying cost vs riding, and collection risk on stale debts (`fairness`,
   `debts --aged`, `net`).
4. Cross-group netting — one person's balance spans many groups + non-group expenses; collapse
   to the minimum real-world transfers, cancelling cycles (`net`).
5. Audit before settling — catch duplicate "Settle all balances" rows and cost outliers so a
   settle-up isn't built on bad data (`audit`).

## Table Stakes

All 27 Splitwise API endpoints, generator-emitted as typed commands (absorb manifest,
prior print v4.16.0→v4.19.0, unchanged this reprint):
- **users**: get-current-user, get-user, update-user
- **groups**: get-groups, get-group, create-group, delete-group, undelete-group,
  add-user-to-group, remove-user-from-group
- **friends**: get-friends, get-friend, create-friend, create-friends, delete-friend
- **expenses**: get-expenses, get-expense, create-expense, update-expense, delete-expense,
  undelete-expense
- **comments**: get-comments, create-comment, delete-comment
- **notifications**: get-notifications
- **currencies**: get-currencies
- **categories**: get-categories
- **resolve**: hand-coded fuzzy name→ID helper, shared infra reused by create/add/ledger/
  settle-up/split/fairness-nudge (not a transcendence row)
- **search**: framework FTS command (spec-emits, not hand-code) — recipe is `search "term"
  --type <endpoint-keyed resource>`, never a bare flagword

## Data Layer

Primary entities: users, groups, friends, expenses (with per-user paid_share/owed_share),
comments, notifications, currencies, categories. Sync cursor: **offset pagination with no
`has_more`/cursor signal from the API** — sync must advance offset until it gets a short page;
a prior patch (`splitwise-sync-offset-pagination-past-first-page`) fixed page-1-only sync that
silently dropped 43 of 143 expenses. Incremental sync also uses `updated_after` for expenses.
`Store.List` must not default to a 200-row cap (a prior patch, `analytics-read-200-cap`, fixed
this). FTS/search: expense descriptions, comments, group/friend names — scan-based relevance
over extracted meaningful text (word-boundary matching), not the raw-JSON FTS index
(`search-scan-based-relevance-and-fuzzy`).

## Auth

API key via `Authorization: Bearer <key>` (spec: `ApiKeyAuth`, `type: http`, `scheme: bearer`).
Canonical env var (spec-declared `x-auth-env-vars`): **`SPLITWISE_API_KEY`**. OAuth2
(authorization-code) also exists in the spec (`OAuth` security scheme) for multi-user apps, but
the personal API key is the fast path for a single-user power CLI — unchanged from prior print.

## User Vision

> The point of this reprint is to lift the CLI onto press 4.27.0+ (many minor versions of
> machine improvements since 4.19.0), adopt the hub's **canonical MCP output-bounding
> contract** (mandatory on any reprint — `docs/specs/2026-06-20-canonical-mcp-output-bounding-
> spec.md`; reference impl `library/readwise/internal/mcp/envelope`), and re-evaluate the prior
> novel features (the 9 in research.json PLUS the 8 that shipped only as post-publish patches:
> fairness, net, audit, forecast, normalize, report, balances --by-group, which) against current
> personas — keep, reframe, or drop with reasons, never silently. Better MCP tool design and
> token efficiency are the named weak spots. **NO scope-cutting**: the reprint must not regress
> any shipped novel command or any API truth recorded in the patches.

**MUST-SURVIVE — 17 already-shipped novel commands:**

*9 from prior research (research.json + absorb manifest):*
1. `balances` — net position across every friend and group
2. `debts --aged` — who owes whom, sorted by staleness
3. `ledger "<group>"` — running per-member balance, replayed from synced expenses
4. `spend --group-by category|group|month` — offline spend rollups
5. `search "term" --type <resource>` — FTS across expenses/comments/names
6. `settle-up "<group>"` — min-transfer settle plan, `--record` opt-in
7. `activity` — diff since last sync (new/edited/deleted expenses)
8. `split` — share-builder / create_expense preview, `--record` opt-in
9. `recurring` — repeating-charge detector with a regularity gate

*8 shipped only as post-publish patches:*
10. `fairness` — risk / contribution / collectability lenses, carrier-vs-rider classification
11. `fairness nudge <friend>` — reminder-as-comment write action, preview default / `--send`
12. `fairness --by collectability` projected settle date (`projected_days_out`)
13. `net` — cross-group debt netting (min real-world transfers across the whole network,
    cancels cycles)
14. `audit` — duplicate detection + robust median/MAD cost-outlier detection
15. `forecast` — upcoming shared-obligation projection off the recurring cadence model
16. `normalize` — multi-currency normalization to a base currency, unconverted currencies
    surfaced (never dropped), user-supplied rates only (no historical/auto FX)
17. `report` — offline trip/period export to md/csv/json (summary + per-person + per-category)
18. `balances --by-group` — one row per group per currency, non-zero only, offline
19. `which` — keyword-alias command finder matching the SKILL's trigger phrases

(19 commands/modes; the hand-off brief's "17" names the two headline batches — 9 + 8 — above.)

## Prior Patches (watch-list)

29 patches, condensed:

- **MCP numeric-param correctness** — large IDs must never render as scientific notation in
  path/query params (`get-friend 1925035` sent as `1.925035e+06` → spurious 403s); fixing the
  format bug also cleared the 403 (it was malformed input, not a permission wall).
- **MCP list-tool pagination** — GET list tools byte-budget-paginate by default; `get-groups_list`
  was an uncapped ~119KB firehose the host rejected.
- **MCP `sql` tool schema honesty** — description must say the real store shape: one `resources`
  table keyed by resource type with JSON in `data`, not "tables match resource names."
  Command-mirror MCP tools (cobratree) also had a args-vs-structured-params mismatch bug
  (`settle_up` positional not surfaced through freeform `args`).
  Also: `--compact` drops `balance` from a friend record — arguably the most useful field.
- **MCP discoverability** — tool descriptions front-load "Splitwise"; server reports real build
  version at startup (not a stale banner).
- **Output-format honesty** — `--csv`/`--plain` must be honored by every analytics command
  (balances, recurring, ledger, spend, debts) — several silently fell back to JSON.
- **Local-read correctness** — signal dropped filters via `meta.unscoped`; honor `--limit`/
  `--offset`; `recurring` needs a real regularity gate (trips/settlements aren't recurring);
  `spend`/`audit` need `--since`/`--until` date windows (ledger/recurring deliberately skip
  windowing — cadence detection needs the full history, running-balance needs a fixed start).
- **Name resolution** — multi-word positional names (settle-up, resolve, split, ledger,
  fairness nudge) must rejoin; resolver prefers exact match and **errors on ambiguity** instead
  of silently guessing — load-bearing for `settle-up --record`.
- **`debts --aged` episode model** — age from the friend's last-settled point forward (shared
  `friendOpenDebt` model with `fairness`), not from the oldest-ever expense; residual balances
  after cross-group simplification can be un-reconstructable to a date (shows "-" honestly
  rather than a wrong number).
- **Sync correctness** — offset pagination must advance past page 1 (no `has_more` from the API).
- **Toolchain / annotations** — Go floor 1.26.5 (crypto/tls vuln fix, superseded — check current
  floor at build time); `get-group` carries `pp:no-error-path-probe`.

## Product Thesis

Name: **splitwise-pp-cli**. Why: "Every Splitwise feature, plus an offline SQLite ledger that
powers balance, debt-aging, spend analytics, and full-text search no other Splitwise tool has."
splitwise-pp-cli wraps the full Splitwise API — expenses, groups, friends, comments, settle-ups —
and keeps a local copy of the whole ledger. That local store powers a net `balances` view,
`debts --aged`, `spend` rollups, offline `search`, a group `ledger` with running balances, and a
`settle-up` plan that minimizes transfers. Fuzzy name resolution means no pasted numeric IDs.

## Build Priorities

1. Data layer + sync correctness first — offset-pagination-past-page-1, `updated_after`
   incremental sync, no 200-row `Store.List` cap. Every novel command reads this store; get it
   right before anything built on top of it.
2. Absorb everything — all 27 endpoints as typed commands, `resolve` fuzzy-match infra, `search`
   as the framework FTS command.
3. Rebuild the 17 must-survive novel commands (9 research-born + 8 patch-born), carrying forward
   every correctness fix in the watch-list (name-resolution ambiguity errors, date-window scope
   per command, `debts --aged` episode model, output-format honesty, regularity gate).
4. MCP surface + the canonical output-bounding contract — apply Pre-Generation MCP Enrichment
   (intents for settle-up/fairness/net/split workflows, not endpoint-mirror tools alone); every
   tool uses the shared output-bounding envelope; fix the numeric-scientific-notation and
   sql-schema-description classes of bug at the generator level if not already upstreamed.
5. Polish — `--csv`/`--plain` honored everywhere, `which` keyword aliases match SKILL trigger
   phrases, tool descriptions front-load "Splitwise", real version banner.

## Spec Facts

- **Paths:** 27. **Operations:** 27 (one operation per path — no path serves multiple methods).
- **Security schemes:** `OAuth` (type `oauth2`), `ApiKeyAuth` (type `http`, scheme `bearer`,
  `bearerFormat: API key`).
- **Extension keys:** `x-mcp` — absent (0 occurrences). `x-learn` — absent (0 occurrences).
  `x-auth-env-vars` — present, 1 occurrence, on `components.securitySchemes.ApiKeyAuth`, value
  `[SPLITWISE_API_KEY]`. `x-category` — absent. Top-level `category` — absent.
- **Tags / resources:** `users`, `groups`, `friends`, `expenses`, `comments`, `notifications`,
  `other`.
