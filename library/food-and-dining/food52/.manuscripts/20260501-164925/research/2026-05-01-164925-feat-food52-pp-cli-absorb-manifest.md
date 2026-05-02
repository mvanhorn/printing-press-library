# Food52 Absorb Manifest (Reprint, v3.2.1)

This is a re-print of the food52 CLI built 2026-04-27. The prior manifest at
`~/printing-press/manuscripts/food52/20260426-230853/research/2026-04-26-230853-feat-food52-pp-cli-absorb-manifest.md`
established the absorbed-vs-transcendence map. The Food52 ecosystem has not
changed since then, so absorbed features remain identical. This document
records reprint reconciliation of the prior novel features against current
personas.

## Source Tools (unchanged)

The prior manifest's source-tools list still applies:

| # | Tool | Type | Status |
|---|------|------|--------|
| 1 | [imRohan/food52-cli](https://github.com/imRohan/food52-cli) | Ruby gem CLI | Broken (interactive `tty-prompt`, no `--json`, plain HTTP) |
| 2 | [hhursev/recipe-scrapers](https://github.com/hhursev/recipe-scrapers) | Python library | Active; single-recipe extraction only |

A check this run confirmed: no MCP server for Food52, no Claude Code plugin, no
Claude skill, no GitHub Action, no n8n integration. The market is unchanged.

## Absorbed Features (unchanged)

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| 1 | Search recipes by keyword | imRohan keywords mode | `recipes search <query> [--limit N]` via Typesense |
| 2 | Search recipes by ingredient | imRohan ingredients mode | `recipes search <query> --tag <ingredient>` |
| 3 | Browse recipes by tag | imRohan cuisine/meal modes | `recipes browse <tag>` against `/recipes/<tag>` SSR |
| 4 | Show single recipe | imRohan + recipe-scrapers | `recipes get <slug-or-url>` via `__NEXT_DATA__` |
| 5 | List recipe tags | imRohan | `tags list [--kind ingredient\|cuisine\|meal\|preparation\|...]` |
| 6 | Recipe data extraction (single URL) | hhursev/recipe-scrapers | Folded into `recipes get` |
| 7 | Article browsing | None | `articles browse <vertical>` and `articles get <slug>` |

Every absorbed feature ships with `--json`, `--limit` where applicable, typed
exit codes, and works against the local SQLite cache after `sync`.

## Transcendence (Reprint Reconciliation)

Each prior novel feature is re-scored against current personas (home cook +
agent doing weeknight planning) and tagged keep / reframe / drop:

| # | Feature | Command | Prior Score | Reprint Verdict | Justification |
|---|---------|---------|-------------|-----------------|---------------|
| 1 | Offline pantry → recipe matcher | `pantry add/list/remove`, `pantry match [--min-coverage N]` | 8/10 | **KEEP** | The single best agent-friendly differentiator. "What can I make right now" is the only Food52 query that genuinely requires local state. |
| 2 | Local FTS5 search across recipes and articles | `search <query> [--type recipe\|article]` | 7/10 | **KEEP** | Required for offline use; complements live `recipes search` (Typesense) for repeat queries. |
| 3 | Sync slice of catalog | `sync recipes <tag>...`, `sync articles <vertical>` | 7/10 | **KEEP** | Foundation for pantry and offline search; non-mutating, polite. |
| 4 | Test-Kitchen-only browse | `recipes top <tag> [--min-rating N] [--limit N]` | 7/10 | **KEEP** | Uses `testKitchenApproved` + `averageRating` editorial signals other tools throw away. The Food52 site has no first-class TK-only filter. |
| 5 | Recipe scaling via JSON-LD | `scale <slug-or-url> --servings N` | 6/10 | **KEEP** | Cleanly demonstrates the JSON-LD path; Food52 has no scaler. |
| 6 | Cooking-mode print view | `print <slug-or-url>` | 6/10 | **KEEP** | The site's "Print Recipe" still loads ad chrome; this strips to ingredients + numbered steps. |
| 7 | Article ↔ recipe cross-reference | `articles for-recipe <slug>` | 6/10 | **KEEP** | Reverse index of `relatedReading` — Food52 articles never link back. |
| 8 | Open in browser | `open <slug-or-url>` | 4/10 | **KEEP (REFRAMED)** | Now built as a side-effect-convention command per machine v3.2.1: print resolved URL by default, require `--launch` to actually open, short-circuit on `cliutil.IsVerifyEnv()`. Kept because UX matters for the human path. |

**No drops.** The personas are the same; the prior reasoning still holds.

**No new transcendence candidates.** The user-first review (home cook + agent
weeknight planner) didn't surface a feature the existing 7 don't already serve.

Total: 7 absorbed feature families + 8 transcendence commands (counting the
`pantry` family as 4 separate commands). Stub list is empty; everything is
buildable from the public read-only surface with the v3.2.1 generator.

## Stubs

None. Every feature in the manifest is buildable with the unauthenticated
surface. No paid API gates. No headless Chrome at runtime (Surf with Chrome
impersonation handles Vercel mitigation).

## Excluded (out of scope, user constraint)

Account/profile/saved-recipes (sign-in), comments and ratings write side
(sign-in), Shop / Storefront commerce (Shopify Storefront token discovery
deferred), Hotline community Q&A (no public surface in pageProps), recipe
contests (sign-in).

## What's different from the prior reprint

- Spec uses `html_extract.mode: embedded-json` for the four SSR-backed endpoints
  (recipes browse/get, articles browse/get) — eliminates the hand-built
  `internal/food52/nextdata.go` + 4 hand-replaced handlers from the prior run.
- `open` follows the side-effect convention (print-by-default, `--launch` to
  actually open, `cliutil.IsVerifyEnv()` short-circuit).
- All hand-written commands use `Example: strings.Trim(\`...\`, "\n")` and
  the verify-friendly RunE skeleton (no `cobra.MinimumNArgs`, no
  `MarkFlagRequired`, route through `dryRunOK(flags)`).
