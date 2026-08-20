# Printing Press Retro: GISIS (gisis-pp-cli)

## Session Stats
- API: gisis (IMO Global Integrated Shipping Information System — public Ship Particulars)
- Spec source: browser-sniffed (HAR with bodies stripped by Brave) → hand-authored internal YAML spec (`response_format: html`, `mode: page`, `auth.type: cookie`)
- Scorecard: 77/100 (B) post-polish
- Verify pass rate: 100% (22/22 probes); umbrella verdict FAIL on the data-pipeline "sync crashed" sub-check
- Fix loops: 1 (polish pass — cleared 3 gosec findings in hand-authored code)
- Manual code edits: high — all 6 remaining novel features hand-authored (expected; novel features are the agent's value layer)
- Features built from scratch: 6 novel commands (`ship history/list/stale/batch/pin/unpin/refresh`, `owner fleet`) + a local SQLite cache layer

## Context

This was Run 2 of a multi-run build (Phase 1a of an internal Vessel-MCP project). Run 1/1.5 generated the scaffold, parser, `ship get`, and `auth ping`. Run 2 implemented the remaining novel features + cache. The CLI is functionally correct (all commands smoke-tested; live GISIS path validated end-to-end — login-wall detection + typed errors; a successful fetch is gated only on a fresh GISIS session). The shipcheck residue is doc/manifest/scorer, not code defects.

The notable retro signal: **three of the four issues I'd have filed are already open** (#2396, #2444, #2445), filed from recent revenuecat / Amazon-Product / plan-driven runs. GISIS is another data point for the same HTML-page-mode CLI class. The right move is reinforcing comments, not duplicates.

## Findings

### 1. shipcheck verify leg FAILs page-mode CLIs on the data-pipeline "sync crashed" sub-check (scorer)
- **What happened:** `shipcheck`'s `verify` leg ran spec-mode against the HTML page-mode spec; the data-pipeline gate reported `Data Pipeline: FAIL: sync crashed` and flipped the umbrella verdict to FAIL, even though verify pass-rate was 100% (22/22) and the CLI is correct. GISIS's generated `sync` deliberately returns an explicit not-applicable error ("sync is not implemented for this CLI; the generic spec-driven sync template does not fit predominantly HTML page-mode endpoints") — so the *generator itself* knows sync is N/A, yet the *scorer* fails the CLI for it.
- **Scorer correct?** No — penalizes an intentional, generator-emitted design for a CLI class with no enumerable JSON list endpoint.
- **Root cause:** scorer — the `shipcheck` umbrella's verify-leg invocation runs spec-mode/data-pipeline unconditionally; the documented-correct mode (`verify --no-spec`) is unreachable through the umbrella.
- **Cross-API check:** recurs on every HTML-page-mode / plan-driven / browser-sniffed CLI whose only spec resource is an HTML page placeholder. **Already filed** as #2396 (Amazon Product) and adjacent #2445 (revenuecat). GISIS is a third confirming API.
- **Dedup:** `same` as #2396. → **Comment on #2396**, don't file new.
- **Evidence:** `shipcheck` summary `verify FAIL exit 1 / Data Pipeline: FAIL: sync crashed`; standalone behavior confirmed correct; generator-emitted disabled-sync message proves intent.
- **Related prior retros:** #2396 (`aligned` — same finding, Amazon Product), #2445 (`extends` — HTML-source verify-mode, revenuecat).

### 2. extractObjectID silently mis-keys renameable page-mode resources by `name` (generator)
- **What happened:** the generated `store.UpsertShip` derives the primary key via `extractObjectID`, whose fallback chain is `id → Id → ID → uuid → slug → name`. The GISIS ship payload has no `id`/`slug` but **does** have `name`, so every ship would be keyed by its **name**. Vessel names change (GISIS even ships inline name-history), so name-keying silently collides/duplicates on rename. I worked around it with a hand-authored `UpsertShipByIMO` keyed by the domain identity (IMO).
- **Scorer correct?** N/A (not a score penalty) — it's a silent generator data-integrity bug; no scorer caught it because the typed payload *has* a `name` so nothing crashes.
- **Root cause:** generator — `extractObjectID`'s `name` fallback is unsafe for page-mode resources where the only present field is a mutable display name. Distinct from #2444 (which addresses the *crash* when no id/slug/name exists at all, via url-synthesis); even with #2444's fix, the name-present case here would still mis-key.
- **Cross-API check:** any page-mode resource whose stable identity is a domain field the fallback chain doesn't know (IMO, ISIN, ISBN, VIN…) and whose payload also carries a mutable `name`. The durable fix is spec-declared key (`x-resource-id`) for page-mode resources rather than guessing from the payload.
- **Dedup:** `related-area` to #2444 (same component, `extractID`/`extractObjectID` for page-mode; distinct facet). → **Comment on #2444** with the mutable-name sub-case.
- **Evidence:** `internal/store/store.go` `extractObjectID` order; the GISIS payload shape (`imo_number`, `name`, no `id`); the hand-authored `UpsertShipByIMO`.
- **Related prior retros:** #2444 (`extends` — same component, crash-on-no-id facet; mine adds the silent-mis-key-on-mutable-name facet).

## Prioritized Improvements

### Comment (reinforce existing open issues)
| Finding | Title | Plan | Component |
|---------|-------|------|-----------|
| 1 | verify leg FAILs page-mode CLIs on "sync crashed" | Comment on #2396 | scorer |
| 2 | extractObjectID mis-keys renameable page-mode resources by name | Comment on #2444 | generator |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| A | dogfood/verify-skill judge novel features by grepping generated stub source (TODO bodies, placeholder `Use:`/flags) and miss hand-authored impls registered at runtime; the MCP-surface scorer already walks the runtime cobra tree | Step G: case-against stronger. The trigger is this CLI's own `ResetCommands` + `*_impl.go` override pattern (chosen for regen-safety); the common path implements features in-place in the stub, where source-grep is accurate. Adjacent active work exists (#1004 verify-skill resolution). Real but self-inflicted + borderline — don't file new. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| kooky in detectCookieTool | CLI's `auth.go` cookie-tool detection (pycookiecheat/cookies/cookie-scoop-cli) doesn't include kooky; getting cookies into the jar is manual for Cloudflare-gated cookie-auth sites | printed-CLI / amend candidate; thin cross-API evidence (only GISIS + planned Equasis) |
| mcp:read-only on local-write commands | `ship pin/unpin/refresh/batch` carry `mcp:read-only` despite writing the local cache/watchlist | printed-CLI; defensible author choice (no remote mutation) |

## Work Units

### WU-1: Comment on #2396 — GISIS as a third confirming API for the spec-mode-verify-FAILs-page-mode-CLIs bug
- **Priority:** P2
- **Component:** scorer
- **Goal:** Reinforce #2396 with a second/third concrete API and the generator-intent angle.
- **Acceptance:** comment posted with GISIS evidence + the "generator emits disabled-sync message, scorer then fails on it" contradiction.

### WU-2: Comment on #2444 — mutable-name mis-key facet of page-mode resource keying
- **Priority:** P2
- **Component:** generator
- **Goal:** Add the distinct facet (name present but mutable → silent wrong key) and the spec-declared-key fix direction, so the #2444 fix doesn't stop at url-synthesis.
- **Acceptance:** comment posted with the `extractObjectID` fallback-chain analysis + IMO/ISIN/VIN class of stable-domain-key resources.

## Anti-patterns avoided
- Did not file duplicates of #2396 / #2444 / #2445 — the maintainers are actively working the HTML-page-mode CLI class; reinforcing comments beat noise.
- Did not file the borderline dogfood/verify-skill source-grep finding as a new issue — its trigger is partly self-inflicted and adjacent work exists.

## What the Printing Press Got Right
- The generator pre-emitted a typed `ship` table + `UpsertShip`/`upsertShipTx`/`UpsertBatch` and novel-feature stubs from `research.json` — a real floor-raise that saved boilerplate even though the key derivation needed a domain override.
- `auth.type: cookie` + `response_format: html` + `mode: page` worked end-to-end for an ASP.NET WebForms scraper; the Surf transport reached the live site where curl was TLS-fingerprinted.
- The MCP-surface scorer already walks the runtime cobra tree — the right model the other scorers should follow.
