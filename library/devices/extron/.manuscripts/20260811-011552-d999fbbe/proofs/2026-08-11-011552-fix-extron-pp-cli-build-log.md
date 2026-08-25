# Extron CLI Build Log

Run: 20260811-011552-d999fbbe · Stamp: 2026-08-11-011552

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

## Planned
- literature updates (hand-code)
- catalog completeness (hand-code)
- literature recent (hand-code)
- literature rack (hand-code)
- literature family (hand-code)
- catalog verify (hand-code)

## Built
- Priority 0 (foundation): `internal/extron` sibling client (WAF-reset retry, adaptive pacing, typed 429), HTML index/category parser (headings→category, Rev/Date/Size/Type cells, pagination refs), PDF downloader; catalog stored in the generated `resources` table (resource_type='literature') so framework `search`/`sync`/hints work; download ledger (`.extron-downloads.json`) for rev/size tracking.
- Priority 1 (absorb): `literature list` (category/letter/limit filters, offline), `literature get <model>` (ranked resolution), `literature download <name|url> --dir` (batch, dry-run, ledger), framework `search` over the synced catalog.
- Priority 2 (transcend — all 6):
  1. `literature updates` — ledger rev vs catalog Rev diff, `--download` re-fetch, missing-file detection
  2. `catalog completeness` — per-model six-category cross-tab from `--bom`/`--model`
  3. `literature recent` — date-sorted what's-new with `--days`/`--category`
  4. `literature rack --bom` — BOM parse (line/CSV), per-model doc resolution, `--download`
  5. `literature family` — curated family-prefix grouping across letters
  6. `catalog verify` — on-disk size vs ledger bytes, missing/mismatch flags

## Deferred / Gaps
- `--full` catalog sync (per-category pagination) implemented but not run live end-to-end; default bounded sync (first page per letter) verified live.
- Machine bug: generator `cliutil_credentials_test.go.tmpl` emits `AuthHeader()`-containment asserts that fail for `auth: none` specs; worked around with a commented test-side patch (runtime code untouched). Retro candidate.
