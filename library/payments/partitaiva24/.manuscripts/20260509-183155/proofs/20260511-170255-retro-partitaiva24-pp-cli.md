# Printing Press Retro: partitaiva24 (post-polish)

This retro follows a `/printing-press-polish partitaiva24-pp-cli` run on 2026-05-11.
The original generation retro is in the same directory (`20260509-194500-retro-partitaiva24-pp-cli.md`) and covered four SKILL-instruction findings from generation. This retro covers only what surfaced during polish.

## Session Stats
- API: partitaiva24
- Spec source: agent-authored internal YAML (unchanged from generation)
- Scorecard: 86/100 (Grade A) — up 1 from the 85/100 at generation
- Verify pass rate: 100%
- Dogfood: PASS
- Publish-validate: PASS
- Fix loops during polish: 0
- Manual code edits during polish: 0
- Manual filesystem edits during polish: 1 (removed local `internal/pdfgen/node_modules/`)

Polish was effectively a no-op on the code surface. The single hand action was deleting a vendored Node directory the agent had placed under `internal/pdfgen/` for puppeteer-based PDF generation. Without that removal, `pii-audit` flagged hundreds of address-shape matches inside upstream npm packages' README/JSON files.

## Findings

### 1. pii-audit's `skippedDirs` map only fires at depth-1 from the CLI root (template/scorer)
- **What happened:** `internal/artifacts/pii.go:166-176` defines `skippedDirs = {.git, node_modules, vendor, build}`, but the walker at lines 190-198 only consults that map when `parent == root`. `internal/pdfgen/node_modules/` sits at depth-3 from the CLI root, so the walker descended into it. `isHighRiskFile` (line 239) then matched every `package.json` and `*.md` inside upstream npm packages by basename, producing a large pile of postal-address false positives that blocked `pii-audit`. The workaround was to delete the directory locally; the directory itself is legitimate runtime infrastructure for puppeteer-based PDF generation.
- **Scorer correct?** Partially. The scorer correctly scans `*.json`/`*.md` anywhere by basename — that's by design for the README/manuscripts leak path. The bug isn't the scan rule; it's the assumption baked into `skippedDirs` that infrastructure directory names only appear as direct children of the CLI root. Once `node_modules` exists at any depth, the scan rule sweeps it in.
- **Root cause:** `internal/artifacts/pii.go:171-176` and `pii.go:194-198`. The comment at line 166 explicitly defends depth-1 scoping: ".git and friends as direct children of the cli-dir are infrastructure; the same names nested inside .manuscripts/ or testdata/ are captured content and must be scanned." That defense holds for `.git` (a captured `.git` snapshot under `testdata/` is plausible content) but it overweights the captured-fixture case for `node_modules`, which is almost never a legitimate captured fixture.
- **Cross-API check:** Tested mentally across the user's local library (genius, skyscanner, substack, partitaiva24). Only partitaiva24 vendors a `node_modules`. The shape — a Go CLI shelling out to a Node helper for a capability Go can't easily provide (PDF rendering via headless Chromium, JS-bundled SDKs, MDX rendering) — is real but rare.
- **Frequency:** This API only, in the user's library today. Subclass: Go CLIs that embed a Node helper for a capability without a clean Go equivalent. Plausible candidates in future catalog work: any CLI wrapping a service that publishes JS-only SDKs (most Web3/crypto APIs), any CLI that needs server-side rendering, any CLI building PDFs/screenshots from HTML. None of these exist in the catalog today.
- **Fallback if the Printing Press doesn't fix it:** Agent deletes `node_modules` before polish runs, or adds it to a manual exclusion list. Per-CLI choice; cheap to fix once noticed; obvious from the pii-audit output. Same fallback would apply on every future CLI that goes this route.
- **Worth a Printing Press fix?** Not at this priority and not without a guard. See Step B / C / G below.
- **Inherent or fixable:** Fixable — but the cheapest fix needs a guard that protects the captured-fixture case the comment defends.
- **Durable fix (sketched, not proposed for filing):** Two viable approaches.
  1. Special-case `node_modules` in `skippedDirs` to skip at *any* depth, keeping `.git`/`vendor`/`build` at depth-1. Rationale: a captured `testdata/node_modules` fixture is far less plausible than a captured `testdata/.git` snapshot. Risk: still wrong if any future CLI captures node_modules as test data.
  2. Add an opt-in skip list (e.g., a `.pii-audit-ignore` file at the CLI root, or a `pii.skip_dirs` annotation in `tools-manifest.json`) and have generation emit it for known Node-helper patterns. Risk: more machinery; only pays off if more CLIs vendor Node.
- **Test (if filed):** Positive: a CLI with `internal/<pkg>/node_modules/` passes pii-audit without manual deletion. Negative: a CLI with `testdata/node_modules/` containing a fixture intentionally exercising PII detection still scans it.
- **Evidence:** Polish session 2026-05-11. `pii-audit` flagged hundreds of false positives inside upstream npm package READMEs. Removing the directory cleared all of them.
- **Related prior retros:** None matched on `node_modules` or `pii-audit` across `~/printing-press/manuscripts/*/proofs/*-retro-*.md`.
- **Step B (3 APIs with evidence):** Cannot name three. Only partitaiva24 vendors `node_modules` in the local library today. Pure speculation on future APIs.
- **Step C (counter-check):** Yes — "skip node_modules at any depth" would hurt a CLI that captures node_modules under `testdata/` for a fixture purpose. Low likelihood, but the design comment in `pii.go:166-170` explicitly defends scanning captured-content fixtures. Any fix needs a guard or an opt-in surface; a blanket default change is unsafe.
- **Step G case-against:** Embedding `node_modules` inside a Go module's `internal/` is an unusual choice the agent made for one CLI. The depth-1 scoping is deliberate, documented, and protects a real case (captured `testdata/.git`). One CLI, one workaround (`rm -rf`), zero future CLIs that demonstrably need this today. Filing would be wishlist territory until a second CLI hits the same wall.
- **Why Step G survives or doesn't:** Step G stands. Case-against (single CLI, deliberate design, fix needs a guard not available today) is stronger than case-for (saves one `rm -rf` per future Node-helper CLI). Re-raise if/when a second CLI in the library independently lands on a `node_modules` subtree.

## Prioritized Improvements

*No findings made it past Phase 3 Step B.*

### Skip

| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| F1 | pii-audit `skippedDirs` only fires at depth-1 | Step B: only 1 named API with evidence; Step C: blanket fix without a guard would regress the captured-fixture case the design comment explicitly defends; Step G: case-against stronger — single CLI, unusual vendoring choice |

### Dropped at triage

*None. The single candidate from the polish session was carried through full Phase 3 analysis and rejected at Step B/C/G rather than at triage.*

## Work Units

*None. No findings reached the Do bucket.*

## Anti-patterns
- **Vendoring `node_modules` inside a Go module's `internal/` tree.** It makes `pii-audit`, `tools-audit`, `pii-cleanup`, `secrets`, and any other walk-the-CLI-root scanner descend into upstream npm package READMEs. If a printed CLI genuinely needs a Node helper, keep the Node dependency out of the CLI module — invoke it from the user's PATH (`puppeteer` via `npx` or a separate side-installed module under `$XDG_DATA_HOME`), not as committed/vendored content inside the Go source tree.

## What the Printing Press Got Right
- **Polish exit was clean.** `dogfood`, `verify`, `scorecard`, `pii-audit` (post-cleanup), and `publish-validate` all passed in one pass with zero code edits required during polish itself. The single intervention was a filesystem operation outside the Go module — exactly the boundary polish should not have to defend.
- **The 1-point scorecard bump (85 → 86)** came from polish-time normalization that the agent didn't have to think about. The scorer caught what the generator left rough, the polish skill knew what to fix, and the work didn't bleed back into the source tree.
- **`pii-audit`'s file-scoping rule (`isHighRiskFile`) is the right shape.** Scanning `*.json`/`*.md` anywhere by basename is what catches the README/manuscripts leak path. The depth-1 limit on `skippedDirs` is the surgical place where this finding lives; the file-scoping itself is correctly broad.
