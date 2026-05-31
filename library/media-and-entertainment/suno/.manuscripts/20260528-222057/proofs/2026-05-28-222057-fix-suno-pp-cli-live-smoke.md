# Suno CLI — Live Smoke / Acceptance Report

Level: Quick Check (read-only; no generation, no mutations, no cost)
Auth: Clerk session captured via `auth login --chrome` from the user's logged-in Chrome.

## Gate: PASS
Binary-owned `dogfood --live --level quick`: status **pass**, 5/5 tests, 0 failures (phase5-acceptance.json).

## Manual read-only live results (real Suno API)
- `auth login --chrome` — resolved Clerk session, minted JWT, saved to config. doctor: Auth configured, API reachable. ✓
- `credits --json` — live billing: <N> credits, "year" plan (tolerant parse handles real schema). ✓
- `sync --max-pages 2` — 40 clips + 20 workspaces synced into SQLite via correct opaque-cursor walk. ✓
- `analytics --type clips --group-by model_name` — 40 clips, avg_duration 390.6s, avg_bpm 140.3, sum_play_count 11. ✓
- `top --by duration` — ranks by real duration (top clip 479.4s). ✓
- `sql "SELECT ... FROM clips"` — read-only SQL over synced data; tags/duration/bpm populated. ✓
- `credits --forecast` — live billing + local 7d window: "<N> credits left; 40 generations in last 7d; throttle ~200". ✓
- `workspace list` — 28 real workspaces (My Workspace 212 clips, etc.). ✓

## Bugs found and fixed inline (CLI fixes)
1. **`auth login --chrome` failed** — kooky aborted on the first unreadable cookie store (Chrome Canary / wrong `Network/Cookies` path) before reaching the real `Default/Cookies`. Fixed: use `kooky.TraverseCookies(...).Collect()` which skips per-store errors. Now works.
2. **Clip metadata fidelity** — `tags/duration/avg_bpm/has_stem/is_remix/make_instrumental` live under `metadata.*` in Suno's clip JSON but the typed-column upsert only read top-level keys, so top/analytics/grep/lineage showed 0/empty. Fixed: added `sunoClipField` (top-level then metadata fallback). Re-sync confirmed real values.

## Printing Press issues for retro
- Generic `upsertClipsTx` uses top-level-only `lookupFieldValue`; nested-metadata APIs need a metadata-aware fallback (or spec hint for nested field paths).
- `auth login --chrome` cookie-store discovery should default to error-tolerant traversal (Collect) in the foundation pattern.

## Workspace list warning (minor, non-blocking)
`workspace list` prints `warning: 1/1 workspace items skipped (no extractable ID field found)` from the generic list ID-extractor; the data still renders correctly (envelope includes all 28 projects). Cosmetic.

## Gate = PASS → promote.
