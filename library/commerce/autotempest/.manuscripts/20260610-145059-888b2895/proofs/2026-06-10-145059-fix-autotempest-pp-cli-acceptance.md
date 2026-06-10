# AutoTempest CLI — Phase 5 Acceptance Report

Level: Full Dogfood (live, no-auth API)
Tests: 56/56 passed
Gate: PASS

## What ran
Binary-owned live matrix across every leaf subcommand: help (realistic examples), happy-path, JSON-parse, output-mode fidelity, error paths. Live against autotempest.com.

## Failures fixed inline (Phase 5 + output-review)
- 11 commands missing `Example:` in --help -> added realistic examples (find/sources/drops/dedupe/deal/spread/auctions + watch add/ls/rm/run). [CLI fix]
- 5 cascade error_path failures on free-text positionals -> sanctioned `pp:no-error-path-probe` annotation (find/deal/spread/drops/watch rm/run). [CLI fix]
- watch rm/run now non-zero exit on missing saved-search name (genuine UX). [CLI fix]
- auctions emitted bids:-1 sentinel -> null when absent. [CLI fix]
- research.json --select examples used non-native keys (vin,price,sitecode / currentBid) -> native keys (vin,min_price,sources.*; current_bid; price_cents for find). [CLI fix]
- model-slug hyphen handling (f-150 vs f150) -> NormalizeSlug. [CLI fix]
- fbm/st link-only sources polluting fetch_failures -> default 7 inline sources + comparison_links. [CLI fix]

## Printing Press issues (for retro)
- generated internal/mcp/tools.go + .printing-press.json carried stale "through sync / and diff" watch description (hand-docs fixed; seed not). [generator]
- MCP server display name hardcoded "Autotempest" (lowercase t) vs "AutoTempest". [generator]
- dogfood flagged generated `sync` as no-op (defaultSyncResources empty) — expected for this CLI (listings sync via find/watch, not generated sync). [generator/spec]

## Live behavior confirmed
find across 7 inline sources (te/hem/cs/cv/cm/eb/ot) returns real listings; dedupe/deal/spread/auctions/drops/watch all functional on populated store; comparison_links for fbm/st; missing-mirror guard returns [] on empty store.
