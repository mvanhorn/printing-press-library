# Inside the D CLI — Acceptance Report

  Level: Full Dogfood (binary-owned live matrix)
  Tests: 60/60 passed (37 skipped — commands with no live fixture)
  Gate: PASS

## What was tested
`printing-press dogfood --live --level full` enumerated the command tree and ran help, happy-path, JSON-fidelity, output-mode, and error-path checks against the live Algolia-backed store. Plus manual live verification: sync (748), search, cross-axis list, get, related (scored), coverage, categories (Dining=288), regions, recent, reading-list (md/json/csv/file), all error paths, dry-run, `--agent`/`--select`.

## Failures found & fixed inline (2, both CLI fixes)
1. **`blogs reading-list --json` emitted plain text, not JSON** (json_fidelity fail). reading-list used its own `--format` and ignored `--json`. Fix: `--json`/`--agent` now emits a JSON array to stdout (universal machine-output contract), bypassing the md/csv file-export path. The `--format`/`--output` path remains for human/file export.
2. **`blogs related __invalid__` exited 0 on an empty store** (error-path expected non-zero). A prior empty-store→`[]`-exit-0 tweak (added to appease the scorecard probe) broke the error-path contract and didn't even fix the probe (relevance fails without sync regardless). Reverted: `related` now returns exit 3 (with a "run sync first" hint) on an empty/unmatched store. On a synced store: valid slug → exit 0 ranked results; invalid → exit 3.

Re-ran full dogfood after fixes → **60/60 pass, 0 fail**.

## Printing Press issues (retro candidates)
- `store.List(type, 0)` defaults to **LIMIT 200**, not "all" — silently truncates "load all" callers (found & fixed in build).
- Generator emits **no `search`/`sql`** for a POST-search-only spec despite creating a store (hand-built `search`).
- Scorecard `--live-check` Sample Output Probe runs store-backed novel features against an **unsynced store**, so they can't pass the relevance check without a sync step the probe doesn't perform. Probe stays 3/4 for this CLI by design.

## Phase 4.85 (Agentic Output Review): PASS, no findings
Semantic relevance, format cleanliness (no entities/mojibake, canonical permalinks), no source drops, and descending-score ranking all verified on synced output. One reviewer note about `related` non-slug input was a misread — confirmed `related <non-slug>` → exit 3 (not silently-unrelated).

## PII
No PII in this report — all sampled data is public editorial content (article titles, URLs, public neighborhood/category tags). No user accounts, emails, or credentials are involved (read-only public content API).

Gate: **PASS** → proceed to Phase 5.5 (Polish).
