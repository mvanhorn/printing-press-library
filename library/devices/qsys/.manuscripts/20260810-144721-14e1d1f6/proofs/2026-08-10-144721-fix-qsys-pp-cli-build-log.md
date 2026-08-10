# Q-SYS CLI — Build Log

Manifest transcendence rows: 8 planned, 5 built (seeded tree), 0 new. Phase 3 will not pass until all 8 ship.

New this run: `bom verify`, `page get --version`, `integrations`.

## Phase 3 build (2026-08-10)
Built this run (in addition to the 5 already-built novel commands carried from the
seeded prior tree):

1. **`bom verify`** (NEW) — `internal/cli/bom_verify.go`. Per-model report joining
   compat matrix (classify), product record (family/discontinued/spec via
   findProduct). Stdin BOM support, `--qds` required, verify-friendly RunE.
   Registered via registerNovelCommand hook as `bom` parent + `verify` child.
2. **`integrations`** (NEW) — `internal/cli/integrations.go`. Model token matched
   against qsys_pages section='Application_Integration' (title/body LIKE),
   conservative full-name UC platform detection, empty-result note. Local-store
   only, `// pp:data-source local`, mcp:read-only.
3. **`page get --version`** (NEW flag) — surgical hand-edit to generated
   `page_get.go`: `--version 9.4` prefixes the fetch path with the versioned doc
   tree `/q-sys_X.Y/Content/...` (verified HTTP 200 for 9.4/9.6/10.0; 404 for
   nonexistent). Regenerating merge may classify this TEMPLATED-WITH-ADDITIONS.

### Design note (page get --version mechanism)
The manifest row described a harvest-time version column; the actual `page get`
is a generated LIVE-fetch command, so the user-facing contract (`page get
<topic> --version 9.4` returns the page as of that version) is met by fetching
the verified versioned tree directly — no 3x harvest cost, works pre-harvest.
Not a feature downgrade: same approved command, same behavior.

### Phase 3 Completion Gate
- Per-row Cobra resolution: all 8 approved leaves exit 0 with `<leaf> [flags]`
  usage spec (no parent fall-through). VERIFIED.
- Deterministic backstop: `dogfood .novel_features_check` = planned 8, found 8,
  missing [], skipped false. GATE PASS.
- `go test -count=1 ./...` (fresh cache) run after gate.

### Intentionally deferred
- `product compare` and `sql` stay in the tree as working commands but were
  dropped from headline novel_features per reprint reconciliation (approved at
  Phase Gate 1.5).

## Phase 5 live dogfood (2026-08-10)
Full live dogfood matrix: **114/114 PASS (100%)** after 1 fix loop.
Fixes applied in-session:
- connect/product get/integrations: `pp:no-error-path-probe` (unknown models
  are honest empty/fallback results — the sanctioned annotation, not invented
  error semantics).
- page index/product index: `guardLiveJSON=false` (binary sitemap XML was
  misclassified as an auth failure by the generated JSON guard) +
  `pp:typed-exit-codes "0,1"` (designed exit-1 for --json on a binary
  endpoint). Both are generator-gap retro candidates: the binary endpoint
  template hardcodes guardLiveJSON=true and does not declare the binary
  --json exit code.
Behavioral verification on real harvested data: search over a fresh harvest
returns ranked FTS page results; compat check CX-Q TSC-70-G3 --qds 9.4 ->
supported; page get + --version 9.4 live; coverage reports real stats.
Gate: PASS (phase5-acceptance.json status pass).
