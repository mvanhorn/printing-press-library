# Printing Press Retro: coffee-goat (Session 2 vertical slice)

## Session Stats
- API: coffee-goat
- Spec source: synthetic (internal YAML; `kind: synthetic`, `base_url: https://coffee-goat.local`)
- Scorecard: 70/100 (Grade B)
- Verify pass rate: 100%
- Dogfood: 61/61 (live, post-fix)
- Shipcheck legs: 6/6 PASS
- Fix loops: 2 dogfood loops + 2 shipcheck loops
- Manual code edits: ~10 (root.go AddCommands, store.go EnsureCoffeeSchema hook, Example field adds, error-path fixes, narrative trim, products-fallback)
- Features built from scratch: 7 transcendence + 3 source adapters + extract + roasters registry + refdata + coffee_schema + sync rewrite + doctor rewrite + products-synthetic-fallback (~4,500 LOC)

## Findings

### 1. `printing-press generate --force` wipes hand-written code; recovery is purely manual (Bug / Generator)

- **What happened:** Twice in the same session, running `printing-press generate --force` (to refresh README/SKILL after editing `research.json` or the spec's `description`) deleted every hand-written package (`internal/source/`, `internal/refdata/`, `internal/roasters/`, `internal/extract/`), 8 hand-written commands in `internal/cli/`, the extended store schema, and reverted `internal/cli/sync.go` + `doctor.go` to template versions. The press correctly creates a timestamped `preserve-<unix-ns>/` snapshot and refuses subsequent `--force` runs until that preserve is recovered — but the recovery itself is fully manual (cp -r 4 packages, copy 8 command files, copy 2 store files, re-edit root.go to add 7 AddCommand calls, re-edit store.go to add the `EnsureCoffeeSchema` hook).
- **Scorer correct?** N/A (not a scoring finding).
- **Root cause:** `--force` in `internal/generator/generator.go` (or whoever owns the output-dir overwrite) deletes the existing dir wholesale. The `regenmerge` package exists but appears to not apply to `--force`; the preserve safety net is one-shot manual recovery.
- **Cross-API check:** Recurs for every CLI that combines generator templates with substantial hand-written extensions — which is every transcendence-heavy CLI (recipe-goat, yahoo-fantasy, agent-capture, youtube, every catalog entry with novel commands). The recovery workflow is identical: copy packages, copy commands, re-stitch root.go AddCommand registrations, re-stitch store.go schema hooks.
- **Frequency:** Triggered any time the agent re-runs `generate` to refresh template-rendered surfaces (README, SKILL) after editing research.json or spec metadata — a documented workflow in the skill itself when narrative.headline / cli_description / extra_commands change.
- **Fallback if the Printing Press doesn't fix it:** Agent must memorize the recovery checklist (which packages, which root.go anchors, which store.go hook) and execute it each time. In this session, recovery took ~6 file ops + 2 Python patches + a build, executed twice. Across the catalog this costs maintainer-time on every regen-after-template-edit cycle.
- **Worth a Printing Press fix?** Yes. Step B evidence: coffee-goat (this run), recipe-goat (prior cookbook CLI with hand-written extensions; named in the skill's own example shape), agent-capture (per the skill's macOS framework guidance; also has hand-written packages). Step G case-against: "the agent should not --force when they have hand-written code." Counter: the press skill **itself instructs** to re-run generate to refresh README/SKILL after research.json edits; the user-facing workflow IS the regen-cycle.
- **Inherent or fixable:** Fixable. Several options at different levels:
  1. `--force` could detect non-generator-emitted Go files (anything not matching the template-emit set or `roaster_products`-style hand-written packages under `internal/`) and either (a) automatically restore them from preserve, (b) refuse `--force` and require an explicit `--wipe`, or (c) emit a one-shot "regenerate-merge-restore" command that re-applies preserved hand-edits in one step.
  2. The `regenmerge` package already exists for incremental regens — extend it to apply to `--force` too, so README/SKILL refresh doesn't require nuking the hand-written file tree.
  3. At minimum, `--force` could write a single-shot recovery script alongside the preserve dir (`preserve/restore.sh` that does the cp + sed patches automatically).
- **Durable fix:** Most durable is option 2 (regenmerge applies to `--force` for non-template-emitted paths). Cheaper: option 3 (auto-generated restore.sh in preserve dir). Both are template/binary fixes, not skill fixes.
- **Test:** Positive: regenerate a CLI with hand-written `internal/source/foo/foo.go`, hand-edited `internal/cli/root.go` (AddCommand for a hand-written command), and a hand-written test file. After `generate --force`, assert all three files remain at their hand-written content. Negative: ensure generator-emitted template files (e.g., `internal/cli/sync.go.tmpl`-emitted files) DO refresh — preservation is for non-template paths only.
- **Evidence:** Session timestamps: first force regen at ~12:08 (preserve dir `preserve-1779304357924715000`), second at ~12:12 (`preserve-1779304894527934000`). Both required manual restore.
- **Related prior retros:** None matching this specific finding (two prior retros mention "preserve" but in unrelated contexts).

### 2. Synthetic-spec anchor commands fail 100% because generator emits live-HTTP endpoint commands against a `.local` base_url (Template gap / Generator)

- **What happened:** coffee-goat's synthetic spec declares `kind: synthetic`, `base_url: https://coffee-goat.local`, and a single anchor `products` resource (so the generator has SOMETHING to scaffold the store/types layer around). The generator dutifully emitted `internal/cli/promoted_products.go` with a `coffee-goat-pp-cli products` command that calls `resolveRead` → `client.Get("/products")` → tries to dial `coffee-goat.local` → DNS lookup fails. This is the dogfood happy-path + json-fidelity tests' failure mode for every synthetic CLI. Phase 5 was blocked from writing the acceptance marker until I hand-wrote `products_synthetic_fallback.go` (~90 LOC) to fall through to the locally-synced `roaster_products` table when the API path fails.
- **Scorer correct?** Yes — the dogfood happy-path test correctly flagged "this command does not work." The bug is in the generator, not the scorer.
- **Root cause:** The generator emits the endpoint-mirror command template (`command_endpoint.go.tmpl` / `promoted_products.go`) regardless of `kind: synthetic`. For synthetic specs there is no live API; the typed endpoint command has no way to succeed.
- **Cross-API check:** Every synthetic CLI hits this. From this run's evidence: coffee-goat Session 1 handoff explicitly notes "Doctor gate fails because `base_url: https://coffee-goat.local` is a synthetic placeholder" (same root cause — synthetic spec, generator-emitted code tries to dial a non-existent host); Session 2 reproduces it. recipe-goat is referenced in the main skill as the synthetic-CLI pattern reference (its sync.go is similarly hand-written, suggesting it likely has the same generator-emit issue or had to work around it). The skill's own example list of synthetic shapes (combo CLIs, browser-sniffed CLIs, scraped catalogs) means this affects every CLI with `kind: synthetic`.
- **Frequency:** Every synthetic-kind CLI; the spec author cannot avoid this without omitting the anchor resource (which removes the store/types scaffolding the synthetic CLI needs).
- **Fallback if the Printing Press doesn't fix it:** Hand-write a `_synthetic_fallback.go` shim per CLI that intercepts `resolveRead` failures and falls through to the locally-synced table. ~90 LOC per CLI. The author has to know which local table backs the anchor resource (`roaster_products` in coffee-goat; would be `products` for recipe-goat, `videos` for some other shape, etc.) and write the SQL.
- **Worth a Printing Press fix?** Yes. Step B evidence: coffee-goat (this run + prior session, both blocked by the same issue), recipe-goat (skill references it as the canonical synthetic-CLI), and any future browser-sniffed CLI that uses a synthetic spec. Step G case-against: "synthetic CLIs are rare." Counter: synthetic specs are the only path for browser-sniffed, crowd-sniffed, and combo CLIs — three of the five generation paths in the press skill.
- **Inherent or fixable:** Fixable in the generator. Several options:
  1. When `kind: synthetic` is set (or `base_url` ends in `.local`), generate the endpoint-mirror command with a built-in fallback: try the API, on dial-tcp failure fall through to a `SELECT * FROM <anchor_table>` query against the synced corpus. The anchor table name is already known to the generator (it's the resource name; coffee-goat uses `products`-derived `roaster_products`).
  2. When `kind: synthetic`, suppress endpoint-mirror command emission for the anchor resource — let the agent's hand-written commands (search, twin, etc.) be the user-facing surface, and only emit the resource as a store table + types.
  3. The scorer (dogfood happy-path) could skip endpoint tests for resources whose spec sets `synthetic: true` or whose CLI is marked `kind: synthetic`. But this is the wrong layer — the dogfood is correctly catching a real bug.
- **Durable fix:** Option 1 (auto-fallback) is the cleanest because it keeps the typed endpoint command working as users expect AND the local store is the canonical source for synthetic CLIs anyway. Option 2 is simpler but loses the typed endpoint surface entirely.
- **Test:** Positive: generate a synthetic-kind CLI, run `sync --source <adapter>` to populate the local table, run `<cli> <anchor_resource> --limit 3 --agent`, assert JSON output with `meta.source: "local"` and 3 results. Negative: for a non-synthetic spec (live API), the same command must still hit the API, not the local fallback.
- **Evidence:** Pre-fix dogfood output: `[happy_path] products: Error: API unreachable and no local data. Run 'coffee-goat-pp-cli sync' to enable offline access. | Original error: GET /products: Get "https://coffee-goat.local/products"`. Post-fix (with hand-written shim): `{"meta":{"source":"local","reason":"synthetic_anchor_fallback","resource_type":"products"},"results":[...real Onyx data...]}`.
- **Related prior retros:** None matching this specific finding.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | regen --force wipes hand-written code; recovery is manual | generator | Every regen-after-template-edit | Low (manual recovery; easy to miss a file) | medium | Guard: only preserve non-template-emitted paths; template-emitted files MUST refresh |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | Synthetic-spec anchor commands try `.local` and fail | generator | Every `kind: synthetic` CLI | Medium (agent can hand-write fallback once they recognize the pattern) | small | Guard: detect via `kind: synthetic` OR `base_url` `.local` suffix; fall through to local store only when both spec-side conditions met |

### Skip
*(none — both findings cleared all three Phase 3 Step F conditions and Step G case-against was weaker than case-for)*

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| `allowPartialFailure` dead flag on all CLIs | Persistent flag emitted by root.go template, dead in 90% of CLIs | `printed-CLI` (low-impact, dogfood already classifies as `dead`; mentioned in shipcheck output. Not worth its own retro finding — single-line template cleanup) |
| Subagent-written commands missing Cobra `Example:` field | 10/61 dogfood failures because build subagent's starter template usage didn't carry `Example:` through | `iteration-noise` (the SKILL.md Phase 3 starter templates DO include Example; the subagent dropped it during code-gen. Fixing this is subagent prompting, not generator/skill — and prior retros haven't surfaced this) |
| `browser-browser-sniff-gate.json` filename has "browser" doubled | The marker file name appears to be `browser-browser-sniff-gate.json` rather than `browser-sniff-gate.json` | `printed-CLI` (cosmetic typo in skill SKILL.md; affects no behavior) |

## Work Units

### WU-1: Preserve hand-written code across `generate --force` (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Make `generate --force` refresh template-emitted files while preserving non-template paths (hand-written packages under `internal/<custom>/`, hand-written `.go` files in `internal/cli/` whose names don't match a template-emit pattern, and the hand-written edits to `root.go` + `store.go` that wire those in).
- **Target:** `internal/generator/generator.go` (the file-emit + output-dir-wipe path) + `internal/pipeline/regenmerge/` (extend to `--force` runs).
- **Acceptance criteria:**
  - positive test: regenerate a CLI with hand-written `internal/source/foo/foo.go`, hand-edited `root.go` (AddCommand for `newFooCmd`), and `internal/cli/foo.go`. After `generate --force`, all three files remain at hand-written content; the template-emitted README/SKILL refresh to reflect the new research.json.
  - negative test: generator-emitted template files (e.g., `internal/cli/sync.go` when sync.go.tmpl is the canonical source) DO refresh on `--force` so spec edits propagate.
- **Scope boundary:** Does NOT touch the preserve-dir safety net (keep it as a belt-and-suspenders). Does NOT auto-resolve merge conflicts in template-emitted files that have been hand-edited — those remain as preserve-dir candidates.
- **Dependencies:** None.
- **Complexity:** medium

### WU-2: Auto-fallback for synthetic-spec anchor commands (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** When the spec declares `kind: synthetic` (or the parser detects `base_url` ending in `.local`), emit endpoint-mirror commands that auto-fall through to the locally-synced table on API failure, with `meta.source: "local"` and `meta.reason: "synthetic_anchor_fallback"` in the output envelope.
- **Target:** `internal/generator/templates/command_endpoint.go.tmpl` (or the promoted-resource emit path) + `internal/generator/templates/data_source.go.tmpl` (the `resolveRead` helper) + parser changes in `internal/spec/spec.go` or `internal/openapi/parser.go` to surface the `synthetic` flag.
- **Acceptance criteria:**
  - positive test: generate a synthetic-kind CLI with anchor resource `widgets`. After `sync --source <adapter>` populates `widgets`, run `<cli> widgets --limit 3 --agent` and assert JSON output `{meta: {source: "local", reason: "synthetic_anchor_fallback"}, results: [...3 widgets...]}`.
  - negative test: generate a live-API CLI (non-synthetic spec). Run the same command — it must hit the API path (`meta.source: "live"`), not fall through to local; if API is down, the existing error message stands.
- **Scope boundary:** Only the anchor/promoted resource gets the fallback. Other commands (the agent's hand-written transcendence commands) already use direct `store.OpenWithContext` — no change needed there.
- **Dependencies:** None.
- **Complexity:** small

## Anti-patterns spotted
- Building a vertical slice and then re-running `generate --force` to refresh templates: this is the canonical regen-wipe trap. Until WU-1 lands, agents must back up hand-written paths to a manual location before any `--force`.
- Spec-author authoring a synthetic spec WITHOUT seeing the generator emit a broken anchor command: there's no warning at generate-time that the anchor will be unreachable. WU-2 fixes the runtime; a generate-time warning would also help.

## What the Printing Press Got Right
- The preserve-dir snapshot **worked**. Even after two consecutive `--force` runs, every hand-written file was preserved at a timestamped path. The user can recover; the fix in WU-1 is about making that recovery automatic, not about adding new safety nets.
- The press's check that refuses `--force` while an unrecovered preserve dir exists fired correctly and prevented a chain-wipe (the second --force would have nuked the first preserve dir, then the third would have nuked the second's, etc.). This is a real safety win.
- The `printing-press dogfood --live` matrix correctly surfaced the synthetic-spec products bug (it's not a scorer false-positive). Without dogfood I would have shipped the CLI with a broken `products` command.
- The Multi-Source Priority Gate in Phase 0.5 of the main skill worked perfectly — caught coffee-goat as a combo CLI on the first prompt, captured roaster-sites-as-primary, and Phase 1.7's per-source `browser-sniff-gate.json` marker correctly recorded each source's decision with a real `asked_at` timestamp.
- The novel-features subagent's 3-pass output (customer model → candidates → adversarial cut) produced 32 candidates from 4 well-grounded personas, then surfaced 30 survivors at ≥5/10. The user trimmed to 21 at the gate; that's exactly what the gate exists to enable.
