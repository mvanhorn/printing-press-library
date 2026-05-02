# Food52 CLI Brief (Re-validation Pass, v3.2.1)

This is a redo of the food52 CLI built 2026-04-27 against printing-press v2.3.10.
The prior brief at
`~/printing-press/manuscripts/food52/20260426-230853/research/2026-04-26-230853-feat-food52-pp-cli-brief.md`
remains the canonical product analysis. This document records re-validation findings
against v3.2.1 of the machine and the live food52.com site.

## API Identity (unchanged)

Food52 LLC, founded 2009 by Amanda Hesser and Merrill Stubbs. Curated, editorially
reviewed recipes; long-form articles; community Hotline; home/kitchen Shop. The
"unauthenticated only" constraint excludes accounts, saved recipes, comments,
contests, and the Shop's authenticated paths.

## Reachability (re-validated 2026-05-01)

`printing-press probe-reachability https://food52.com/ --json`:

- stdlib HTTP returns `429` with `x-vercel-mitigated: challenge` and the Vercel
  Security Checkpoint HTML — same as April.
- Surf with Chrome TLS impersonation returns `200` for the homepage in 71ms with
  no clearance cookie state required.

**Verdict:** `mode: browser_http`, confidence 0.85. The runtime is settled —
ship Surf transport. No clearance cookie capture needed in the printed CLI.

The prior food52-pp-cli binary was sanity-tested against the live site:

- `recipes search "brownies" --json --limit 3` returned 175 hits, top result
  "Lunch Lady Brownies" with `test_kitchen_approved: true`.
- `recipes browse chicken --limit 2 --json` returned two chicken recipes from
  the SSR `__NEXT_DATA__` payload with full structured data.
- `doctor` reported `api: reachable, auth: not required`.

The Typesense search-only key auto-discovery from `_app-<hash>.js` still works.
The Vercel mitigation is unchanged. The Next.js SSR data shape is unchanged.

## Reachability Risk

**Low** — the protection is passive TLS-fingerprint mitigation, not an active JS
challenge. Surf clears it transparently. No re-challenge has been observed during
sustained polite-rate usage in either prior or current testing.

## Top Workflows (unchanged)

The prior list of 8 read-only public workflows still applies. No new public
endpoints have appeared on food52.com that would change priorities.

## Data Layer (unchanged)

Recipes, articles, hotline questions (unreachable), shop products (deferred),
plus tags/cuisines/ingredients taxonomy. Sync cursor by published_at. SQLite
FTS5 across recipes (title + summary + ingredients + tags) and articles
(title + summary + body).

## What Changed In The Machine (v2.3.10 → v3.2.1)

These deltas affect generation of this CLI specifically:

| Retro finding | v3.2.1 status | Effect on this run |
|---|---|---|
| **F1**: HTML extractor only had `mode: page`/`links` — Next.js `__NEXT_DATA__` required hand-replacing 4 generated handlers | **Resolved.** New `mode: embedded-json` with `script_selector` (default `script#__NEXT_DATA__`) and `json_path` walk. | Spec updates the four SSR endpoints (recipes browse/get, articles browse/get) to use `embedded-json`. The generator now emits handlers that return the structured JSON directly; no hand-replacement. |
| **F2**: `Example: strings.TrimSpace(...)` silently broke dogfood example detection | Convention-level guidance is in AGENTS.md (use `strings.Trim(..., "\n")`). Generator templates use the safe form. | Apply the safe pattern to all hand-written transcendence command Examples. |
| **F3**: Side-effect commands (`open`) launched browser during verify mock-mode runs | **Resolved.** `cliutil.IsVerifyEnv()` helper emitted into every CLI; convention is print-by-default + opt-in `--launch` flag, short-circuit on `IsVerifyEnv()`. | `open` follows the convention. |
| **F4**: `.printing-press.json` missing `novel_features` after generation | Dogfood now syncs `novel_features_built` into the manifest and into README/SKILL `## Unique Features` blocks. | No manual manifest patching. |
| **F5**: `traffic-analysis.json` schema rejected `browser_http` mode + string evidence | **Resolved.** `browser_http` is in the enum; `EvidenceRef.UnmarshalJSON` accepts strings via sentinel index. | Prior traffic-analysis.json copies straight into the run with no edits. |
| **F6**: 30 dead helpers when handlers were hand-replaced | Should largely disappear because F1 means we don't hand-replace handlers. Any residual dead code addressed by polish. | Watch for it in shipcheck; address in polish if it surfaces. |
| **F7**: Verify mock-mode dispatched required-positional commands and they "failed" | Status uncertain in v3.2.1; will surface in shipcheck if still present. | Not a redo blocker — false positives in mock-mode are noise, not behavioral failures. |

## Source Priority

Single source: `food52.com`. No combo CLI; the priority gate does not apply.

## User Vision (carried forward)

Unauthenticated only. Public read-only surface. Discovery, planning, and storage
all happen locally. Same constraint as the prior run.

## Product Thesis

Unchanged. The `food52-pp-cli` is the only first-class agent-native Food52
surface; the existing Ruby CLI remains broken against today's Vercel-protected
site. The redo's value is mainly machine-level — eliminate the hand-built
`__NEXT_DATA__` extraction layer (which lived in `internal/food52/nextdata.go`),
collapse onto generator-emitted handlers, and pick up the verify and manifest
fixes that came with v3.2.1.

## Build Priorities (re-validated)

1. **Foundation.** Surf+Chrome HTTP transport (`http_transport: browser-chrome`),
   SQLite store with FTS5, polite rate limiting, response caching.
2. **Absorb.** `recipes search/browse/get/top`, `articles browse/browse-sub/get`,
   `tags list`. All read-only, all `--json`/`--select`/`--limit`/typed exit codes.
3. **Transcend.** `pantry add/list/remove/match`, `sync recipes`, `sync articles`,
   local `search`, `scale`, `print`, `articles for-recipe`, `open` (with
   print-by-default + `--launch` opt-in).
4. **Polish.** README, SKILL.md, agent-friendly recipes including a `--select`
   exemplar, descriptive recipes, anti-triggers.

## What's NOT in scope

Same exclusions as the prior run: account/profile/saved recipes/comments/
contests (sign-in), Hotline (no public surface in pageProps), Shop (Shopify
Storefront token discovery deferred). All sign-in surfaces are out by user
request.
