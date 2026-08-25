# Printing Press Retro: Productive

## Session Stats
- API: Productive (productive.io v2 JSON:API)
- Spec source: internal YAML/JSON (authored from MCP `describe_*` introspection + live probes)
- Scorecard: 84/100 (A)
- Verify pass rate: PASS (7/7 shipcheck legs)
- Fix loops: 2 (validate-narrative recipe; Phase 5 live-dogfood fixes)
- Manual code edits: ~10 files (auth enrichment, 4 novel commands, generic writes, 2 dogfood fixes)
- Features built from scratch: 6 novel (recognized-revenue, invoiced, reconcile, aging + generic create/update/delete)

## Findings

### 1. Multi-word resource examples use the underscore resource key instead of the hyphenated command path (Bug)
- **What happened:** For every multi-word (snake_case) resource, the generated Cobra `Example` used the raw resource key — `productive-pp-cli line_items list` — but the actual command is kebab-cased (`line-items`). Running the example yields `unknown command "line_items"`. Live dogfood flagged all 11 multi-word resources (`line_items`, `invoice_attributions`, `time_entries`, `deal_statuses`, `service_types`, `task_lists`, `tax_rates`, `workflow_statuses`, `contact_entries`, `custom_fields`, `holiday_calendars`) as `happy_path: missing runnable example` — 22 failing checks.
- **Scorer correct?** Yes — dogfood correctly reported the examples don't resolve. The defect is in the generator's example synthesis, not the scorer.
- **Root cause:** Generator (`internal/generator/`) Cobra `Example` synthesis emits the command path from the resource key without applying the same snake_case→kebab-case transform it already applies to the command `Use:` field. The transform exists (commands are correctly hyphenated); it just isn't applied to the example string.
- **Cross-API check:** Recurs on any API with snake_case / multi-word resource names — ubiquitous in REST/JSON:API (Rails-style resources, Stripe `payment_intents`, etc.). Deterministic, not probabilistic.
- **Frequency:** every API with ≥1 multi-word resource name.
- **Fallback if the Printing Press doesn't fix it:** Agent must notice during dogfood and hand-fix every example (I did — 22 examples across 11 resources). Easy to miss for resources whose list happens to return empty (no dogfood happy_path signal).
- **Worth a Printing Press fix?** Yes — one-line transform fix prevents N broken examples per CLI and the associated dogfood failures.
- **Inherent or fixable:** Fixable. Apply the existing command-name kebab transform when rendering the example command path.
- **Durable fix:** In the generator's example rendering, derive the command path from the resolved Cobra command name (already hyphenated), not from the raw resource key. Add a generator test with a multi-word resource asserting the example resolves via `<binary> <cmd> --help`.
- **Test:** Positive — generate a CLI with a `multi_word_thing` resource; assert `Example` contains `multi-word-thing`, not `multi_word_thing`, and that `<binary> multi-word-thing list --help` exits 0. Negative — single-word resources unchanged.
- **Evidence:** Phase 5 dogfood: 22 `missing runnable example` failures, all multi-word resources; single-word resources (deals, invoices, tasks) had valid examples. Confirmed by grepping generated `*_list.go`/`*_get.go` `Example:` lines.
- **Case-against (Step G):** "You authored the spec with underscore keys, so it's your input." — Fails: snake_case resource types are the JSON:API norm and the correct input; the generator already kebab-cases the command name from the same key, so the example inconsistency is unambiguously a generator bug, not user error.
- **Related prior retros:** None.

### 2. `workflow archive` does not curtail work under live-dogfood, so it times out on APIs with many resources (Bug / Template gap)
- **What happened:** The generated `workflow archive` command (channel_workflow.go) full-syncs every resource (48 for Productive) with no dogfood curtailment. Under `dogfood --live` (flat 30s per-command timeout) it was killed → `exit -1` on both happy_path and json_fidelity. It works correctly outside the harness.
- **Scorer correct?** Yes — the command genuinely exceeds 30s live; dogfood is right to time it out. The gap is the generated command not honoring the documented `IsDogfoodEnv` curtail contract.
- **Root cause:** Generator template for the `workflow archive` compound command iterates all syncable resources without a `cliutil.IsDogfoodEnv()` guard. The framework `sync` command was NOT flagged (it handles this), so the archive template diverges from sync's dogfood behavior.
- **Cross-API check:** Recurs on any printed CLI whose resource count × per-resource latency (worsened by rate limits) exceeds 30s. Common for medium/large APIs.
- **Frequency:** subclass — APIs with enough resources/latency to blow 30s (most non-trivial APIs, especially rate-limited ones).
- **Fallback if the Printing Press doesn't fix it:** Agent hand-adds the `IsDogfoodEnv` curtail (I added `resources = resources[:3]` under dogfood). Easy to miss; ships a permanent dogfood failure otherwise.
- **Worth a Printing Press fix?** Yes — mirror sync's existing dogfood curtail in the archive template.
- **Inherent or fixable:** Fixable. The curtail pattern already exists for `sync`; apply it to `workflow archive`.
- **Durable fix:** In the `workflow archive` template, curtail the resource list (or per-resource page count) when `cliutil.IsDogfoodEnv()` is true, matching the `sync` command's behavior. Add a generator/dogfood assertion that `workflow archive` completes under the dogfood timeout for a many-resource spec.
- **Test:** Positive — many-resource spec; `PRINTING_PRESS_DOGFOOD=1 <binary> workflow archive` completes < 30s. Negative — without the env var, archives all resources.
- **Evidence:** Phase 5 dogfood: `workflow archive` exit -1 (timeout) while `sync` passed; manual run completed in full (exit 0) but >30s.
- **Case-against (Step G):** "It's a long-running command; timeout is expected." — Fails: the framework already solved this for `sync` via `IsDogfoodEnv`; the archive command is a framework-emitted sibling that should inherit the same contract, so this is a template consistency bug, not inherent behavior.
- **Related prior retros:** None.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|----------------------|------------|--------|
| 1 | Multi-word example uses underscore key not hyphen command | generator | every API w/ multi-word resource | low (easy to miss on empty-list resources) | small | none needed |
| 2 | `workflow archive` missing IsDogfoodEnv curtail | generator | subclass: many-resource APIs | medium | small | curtail only under dogfood env |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| S1 | Spec can't express a second env-var-driven auth header (Productive needs X-Auth-Token + X-Organization-Id) | Step B: only 1 API (Productive) named with concrete evidence. Real gap — required hand-wiring config.go + client.go + doctor.go — but can't name 3 library APIs with two dynamic auth headers. Record for future corroboration; a second sighting should promote it. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| D1 | reconcile used `date` field; real report field is `date_period` | printed-CLI (my hand-code assumption; generator can't know report field names) |
| D2 | scorecard insight 4/10 despite reconcile/aging | unproven-one-off (did not trace scorer logic; no evidence scorer is wrong) |
| D3 | `deals --stage-type budget` rejected by API (needed numeric enum) | printed-CLI (my example value choice; fixed with a per-CLI mapping) |
| D4 | report subcommand keys kebab-cased fine (`reports financial-item`) | not-a-bug (worked correctly) |

## Work Units

### WU-1: Apply command-name kebab transform to synthesized Cobra examples (from F1)
- **Priority:** P2
- **Component:** generator
- **Goal:** Generated `Example` strings for multi-word resources use the hyphenated command path so they resolve and pass dogfood.
- **Target:** Generator Cobra command emission (`internal/generator/`), example-string rendering for list/get/endpoint commands.
- **Acceptance criteria:**
  - positive: a spec with resource key `multi_word_thing` emits `Example` containing `multi-word-thing ...` and `<binary> multi-word-thing list --help` exits 0.
  - negative: single-word resource examples unchanged.
- **Scope boundary:** Only the example command-path rendering; does not touch flag or arg synthesis.
- **Dependencies:** none
- **Complexity:** small

### WU-2: Curtail `workflow archive` under IsDogfoodEnv (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** The generated `workflow archive` command completes within the live-dogfood timeout by curtailing work when `PRINTING_PRESS_DOGFOOD=1`, mirroring `sync`.
- **Target:** `workflow archive` template (channel_workflow.go equivalent in `internal/generator/`).
- **Acceptance criteria:**
  - positive: many-resource spec; `PRINTING_PRESS_DOGFOOD=1 <binary> workflow archive` completes < 30s.
  - negative: without the env var, all resources are archived.
- **Scope boundary:** Only the dogfood curtail; full behavior unchanged in normal use.
- **Dependencies:** none
- **Complexity:** small

## Anti-patterns
- Emitting example command paths from the raw resource key when the command name uses a different (kebab) casing — the two must be derived from the same source.
- A framework-emitted compound command (`workflow archive`) diverging from its sibling (`sync`) on the dogfood-curtail contract.

## What the Printing Press Got Right
- The snake→kebab transform on command names, JSON:API `filter[x][eq]` flag mapping, provenance envelope, and `--json/--select/--csv` helpers all worked out of the box — the entire 38-resource read surface generated and passed with zero hand-editing.
- The MCP Cloudflare pattern auto-applied correctly for the 85-endpoint surface.
- Hand-authored novel-command files (no "Generated by" header) were preserved cleanly; `dryRunOK`/`boundCtx`/`printOutputWithFlagsMeta` helpers made the financial commands straightforward.
- verify-skill and validate-narrative caught real issues (the `&&` compound recipe) before ship.
