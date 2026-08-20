# Printing Press Retro: OfferUp

## Session Stats
- API: offerup (local marketplace; no public API)
- Spec source: browser-sniffed (Next.js SSR `__NEXT_DATA__`) + hand-authored internal YAML
- Scorecard: 70/100 (B)
- Verify pass rate: shipcheck 6/6 PASS; dogfood --live 58/58 PASS
- Fix loops: ~2 (shipcheck), ~3 (dogfood-auth)
- Manual code edits: substantial — this is a from-website CLI (hand-built SSR extraction + 6 transcendence features + a full authenticated cookie surface)
- Features built from scratch: 6 public transcendence + 4 absorbed read commands + 8 authenticated commands

## Findings

### F1. dogfood/docsync novel-feature + narrative sync skips which.go and mcp/tools.go (recurring friction / scorer)
- **What happened:** After generation I renamed all 6 novel features and corrected a description in `research.json`. dogfood synced the new copy into README `## Unique Features`, SKILL `## Unique Capabilities`, `root.go` Highlights, and `.printing-press.json` — but `internal/cli/which.go` (the capability index) and `internal/mcp/tools.go` (the MCP `context` novel-feature list + playbook) kept the *generate-time* names/descriptions, leaving a stale `--markdowns` claim and old mechanical names in two agent-facing surfaces. Separately, hand-edits to the rendered README/SKILL narrative sections were silently reverted by docsync (the edit source is `research.json`'s narrative, which isn't obvious).
- **Scorer correct?** N/A (not a score penalty) — it's a sync-coverage gap in the dogfood/docsync pipeline.
- **Root cause:** `internal/pipeline/docsync.go` (`SyncCLINarrativeDocs`) rewrites README/SKILL narrative blocks; the dogfood novel-feature sync rewrites README/SKILL/root.go/`.printing-press.json` from `novel_features_built`. Neither touches `which.go` or `mcp/tools.go`, both of which embed the same novel-feature copy at generate time.
- **Cross-API check:** Recurs on any CLI whose novel-feature names/descriptions change after generation — common via the polish skill's description cleanup or an explicit rename (this run). The gap is structural (the sync target list provably omits two files), not API-shape-dependent.
- **Frequency:** every CLI with novel features that gets any post-generation copy edit.
- **Fallback if the Printing Press doesn't fix it:** the agent must remember to hand-edit `which.go` + `mcp/tools.go` (generated files) on every copy change — easy to miss; agent-facing MCP/`which` output silently drifts from README/SKILL.
- **Worth a Printing Press fix?** Yes — two agent-facing surfaces silently disagree with the canonical novel-feature copy.
- **Inherent or fixable:** Fixable. Extend the existing sync to `which.go` + `mcp/tools.go`; the copy already lives in `novel_features_built`/narrative.
- **Durable fix:** In the dogfood novel-feature sync (and/or docsync), add `which.go`'s `whichIndex` entries and `mcp/tools.go`'s `command_mirror_capabilities` + `playbook` to the rewrite set, keyed off `novel_features_built` + narrative. Also: emit a one-line note when docsync overwrites a hand-edited narrative section (so the "edit research.json, not the rendered file" rule is discoverable), or document it in the SKILL.
- **Test:** Generate a CLI with novel features; change a novel feature's `name`/`description` in `research.json`; run dogfood; assert `which.go` + `mcp/tools.go` reflect the new copy (positive) and that an unrelated CLI with no novel features is untouched (negative).
- **Evidence:** This run — `which.go:31` kept `--markdowns` after I removed that mode; `mcp/tools.go` kept "Below-median deal flagging" etc. after the rename. Required hand-edits to both.
- **Related prior retros:** None.

### F2. html_extract embedded-json can't filter or flatten feed-shaped item arrays (template gap / generator)
- **What happened:** OfferUp's search SSR embeds results as `props.pageProps.searchFeedResponse.looseTiles[]` — ad tiles interleaved with listing tiles, and each listing nested under `.listing`. The generated `response_format: html` + `html_extract{mode: embedded-json, json_path}` command extracts the `looseTiles` array but can't drop `tileType != LISTING` or flatten `.listing`, so it returns noisy tiles. Forced hand-built extraction for the headline read.
- **Scorer correct?** N/A.
- **Root cause:** `internal/spec/spec.go` `HTMLExtract` supports `mode`/`script_selector`/`json_path` only — no per-item filter or nested item path. `json_path` is a single dot-walk to an array; it can't express "keep elements where field==value" or "map each element to its `.listing` child."
- **Cross-API check:** Recurs on Next.js/SSR feed pages that interleave non-items (ads, modules) or nest items under a wrapper key. Step B named embedded-json CLIs with evidence: kayak (`catalog/kayak.yaml`), food52 (`library/food52/spec.yaml`), allrecipes (`library/allrecipes/internal/cli/html_extract.go`); OfferUp exhibited the ad+nested shape specifically.
- **Frequency:** subclass — embedded-json feed pages with interleaved non-items or per-item wrapper nesting.
- **Fallback if the Printing Press doesn't fix it:** hand-build the extractor (a `// pp:client-call` source package), as done here. Works, but the generated baseline command ships noisy.
- **Worth a Printing Press fix?** Yes, low priority — a clean, opt-in extension that several embedded-json CLIs could use.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Add optional `html_extract.item_filter` (e.g. `field: tileType, equals: LISTING`) and `html_extract.item_path` (e.g. `listing`) so the runtime extractor filters + flattens each element after the `json_path` walk. Spec-driven, opt-in, no effect on existing clean-array extractors.
- **Counter-check:** Opt-in spec fields — zero effect on CLIs that don't set them. No guard needed beyond "only applies when present."
- **Test:** embedded-json fixture page with an array of mixed-type wrapper objects; with `item_filter`+`item_path` set, assert only matching, flattened items come back; without them, assert current behavior is byte-identical.
- **Evidence:** This run — generated `listings search` returned `looseTiles` (Google ad tile first, listing under `.listing`); reimplemented as a hand-built SSR extractor.
- **Related prior retros:** None.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F1 | dogfood/docsync sync skips which.go + mcp/tools.go | scorer | every CLI w/ novel features + post-gen copy edit | low (easy to forget the 2 files) | small | none needed |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F2 | html_extract item_filter + item_path for feed pages | generator | subclass: embedded-json feeds w/ ads/nesting | medium (hand-build works) | medium | opt-in spec fields |

### Skip
| Finding | Title | Why it didn't make it |
|---|---|---|
| C1 | Two-tier auth / hand-built auth on a no-auth spec | Step G: the machine already supports cookie/composed auth in the spec (allrecipes, pagliacci, substack); generating with `auth.type: none` + hand-building auth was a scope-evolution artifact (auth was added after generation, not knowable at generate time). The regen-collision is a known hand-edit-durability tradeoff already documented in AGENTS.md. Per-workflow, not a machine gap. |
| C2 | verify-skill flag-commands false-positive on shared flag helpers | Step B: can't name 3 other CLIs with evidence (it bit this run's hand-built shared-helper style). Step G: inlining flag registration with a literal `Use` matches the generated-command convention, so verify-skill is arguably enforcing a good pattern rather than mis-firing. |
| C3 | press-auth login completion heuristic fails for avatar-hidden logout | Step B: can't confirm 3 cookie/composed-auth CLIs whose sites hide logout AND that hit the timeout (allrecipes/pagliacci/substack unverified). `--complete-selector` is a documented escape hatch. Component is press-auth (`cmd/press-auth/`), outside the core six; the auth-companion ref already covers press-auth usage. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|---|---|---|
| (docsync clobbers hand-edited narrative) | Folded into F1 — same sync-coverage root cause. | merged-into-F1 |

## Work Units

### WU-1: Sync novel-feature copy to which.go + mcp/tools.go (from F1)
- **Priority:** P2
- **Component:** scorer (dogfood/docsync sync; `internal/pipeline/`)
- **Goal:** Post-generation novel-feature copy changes (names/descriptions in `research.json`/`novel_features_built`) propagate to `which.go` and `mcp/tools.go`, the two agent-facing surfaces the sync currently misses.
- **Target:** `internal/pipeline/dogfood.go` (novel-feature sync) + `internal/pipeline/docsync.go`; the emitted `internal/cli/which.go` (`whichIndex`) and `internal/mcp/tools.go` (`command_mirror_capabilities`, `playbook`).
- **Acceptance criteria:**
  - positive: rename a novel feature in `research.json`, run dogfood; `which.go` + `mcp/tools.go` reflect the new name/description.
  - negative: a CLI with no novel-feature copy change shows zero diff in those files.
- **Scope boundary:** Just extend the existing sync target set; don't redesign the sync. Optionally add a stderr note when docsync overwrites a hand-edited narrative block.
- **Dependencies:** none
- **Complexity:** small

### WU-2: html_extract item_filter + item_path for feed-shaped embedded-json (from F2)
- **Priority:** P3
- **Component:** generator (`internal/spec/` schema + `internal/generator/` html-extract runtime)
- **Goal:** Generated embedded-json read commands can filter interleaved non-items and flatten per-item wrapper keys, producing clean items for Next.js/SSR feed pages.
- **Target:** `internal/spec/spec.go` `HTMLExtract` (+ defaults), the html_extract runtime template in `internal/generator/`.
- **Acceptance criteria:**
  - positive: with `item_filter: {field, equals}` + `item_path`, a mixed-type wrapper array yields only matching, flattened items.
  - negative: specs without the new fields produce byte-identical output to today.
- **Scope boundary:** Single-field equals filter + single-key flatten; no general predicate language.
- **Dependencies:** none
- **Complexity:** medium

## Anti-patterns
- None observed in the machine. (Run-level: my own initial over-restriction of "prefer unauthenticated" to "unauthenticated only" was a reading error, corrected by the user — not a machine issue.)

## What the Printing Press Got Right
- `response_format: html` + `html_extract{mode: embedded-json}` is purpose-built for Next.js `__NEXT_DATA__` and made the SSR approach viable.
- `probe-reachability` cleanly settled the runtime tier (standard_http) and saved a needless browser-transport build.
- The novel-feature scaffolding (stubs + `novel_features_built` sync to README/SKILL/root/manifest) is excellent — F1 is just two missed files in an otherwise strong sync.
- `cliutil` (AdaptiveLimiter, CleanText, RateLimitError, ParseDurationLoose) covered exactly the hand-built-code needs.
- Write-safety conventions (IsVerifyEnv / IsDogfoodEnv short-circuits, `pp:no-error-path-probe`) made the authenticated mutation commands dogfood-clean without bespoke logic.
