# Phase 5 Acceptance Report — Bandsintown

**Level:** Skipped (auth required, no partner credential)
**Tests:** 0/0
**Failures:** none
**Fixes applied:** 0
**Gate:** SKIP (per Phase 0 reachability decision)

## Why skipped

Bandsintown REST API is partner-gated since 2025. The legacy free `app_id` model
has been deprecated; the API returns HTTP 403 ("explicit deny", AWS IAM-style)
for every unauthenticated request.

The user explicitly chose "Generate anyway from spec" at the Phase 1.9
reachability gate, matching their Double Deer `docs/printing-press-queue.md`
plan that defers live key acquisition until after generation.

## Offline verification carried this run

- Shipcheck verdict: **PASS (6/6 legs)**
  - dogfood, verify, workflow-verify, verify-skill, validate-narrative, scorecard
- Scorecard: **82/100 — Grade A**
- Sample Output Probe: **86% pass (6/7)**
  - Only failure: `pull` (the one transcendence command that requires a live API
    call). This is expected and documented in the auth narrative.
- `go test ./...`: **all packages pass** (cliutil, dd, store, mcp, cobratree, cli)
- `dd` package tests: behavioral coverage for route, gaps, lineup co-bill,
  snapshot+trend, sea-radar, watchlist add/list/remove, region filter

## What the user gets when a partner key arrives

1. `export BANDSINTOWN_APP_ID=<partner-key>`
2. `bandsintown-pp-cli doctor` — auth check should return OK
3. `bandsintown-pp-cli track add "Phoenix" "Beach House" "Tame Impala"`
4. `bandsintown-pp-cli pull --tracked --snapshot` — populates dd_artists,
   dd_events, dd_venues, dd_lineup_members, dd_offers, dd_artist_snapshots
5. `bandsintown-pp-cli route --to "Jakarta,ID" --on 2026-08-15 --window 7d
   --tracked --score --json` — first real Double Deer tour-routing query

## Acceptance verdict

**SKIP (expected).** Generated CLI is the Double Deer Bandsintown adapter
foundation; live behavioral testing is post-gen, owned by whoever provisions
the partner credential.
