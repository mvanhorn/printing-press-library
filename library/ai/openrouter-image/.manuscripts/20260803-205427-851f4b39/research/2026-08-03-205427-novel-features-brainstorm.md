## Customer model

**Relay — the cron pipeline agent.** An AI agent (run by its operator's scheduled pipelines) that generates images on demand for nightly social assets, doc diagrams, and cover images.
- **Today (without this CLI):** The pipeline calls `POST /images` via hand-written curl scripts with model slugs copy-pasted from the OpenRouter docs site. When OpenRouter retires a model, the pipeline 404s silently until a human notices. Relay cannot answer "which models do image-to-image and 4K under $0.10" without paging through `GET /images/models` JSON and eyeballing `supported_parameters`.
- **Weekly ritual:** Generate a scheduled batch, re-run failed generations with identical parameters, and check the credit balance before the batch starts.
- **Frustration:** Model churn breaks pinned pipelines silently; there is no offline, cheapest-first capability query; a runaway batch can burn the whole balance with no pre-spend gate.

**Maya — the prompt-iteration designer.** A human who needs on-demand image generation with explicit model choice and iterates on prompts across models.
- **Today (without this CLI):** Keeps a notes file of prompts plus the exact flags that produced each winner, because the API has no memory. Re-types the full model/seed/resolution/quality flag set for every variant, and loses track of which seed produced which output.
- **Weekly ritual:** Takes one prompt through 3–5 models/providers, tweaks seed/resolution/quality, compares outputs, saves winners.
- **Frustration:** Reproducing a past generation exactly (same seed, same params) requires perfect record-keeping; there is no "re-run that one" and no per-variant cost visibility.

**Sam — the budget-conscious builder.** An indie/startup developer shipping AI imagery in a product, watching spend.
- **Today (without this CLI):** Learns the cost of a generation only after the fact from the `usage` block of each response; totals spend by pasting JSON into a spreadsheet; cannot answer "what would this batch cost" before running it.
- **Weekly ritual:** Monday spend review — audits which models/providers cost the most, checks remaining credits, trims the expensive ones from the pipeline.
- **Frustration:** No pre-spend estimate, no local ledger analytics, no period-over-period trend; every answer requires scraping API responses by hand.

## Candidates (pre-cut)

**1. cost-estimate** — `cost-estimate --model <slug> [--resolution 1K] [--quality high] [--n 4] [--json]`
- One-line: Estimate USD cost of a generation before spending, computed offline from synced per-endpoint pricing lines (unit=image|megapixel) × resolution tiers.
- Persona served: Sam, Maya. Source: (c) cross-entity local queries + (a) persona-driven.
- Long Description: "Use this command to estimate the cost of a future generation before spending credits. Do NOT use it to preview the request payload; use 'generate --dry-run' instead."
- Kill/keep: PASS — no LLM, no external service, same auth, one command, verifiable against `GET /generation` actual cost, computes from real synced pricing (not a fake API call). `pp:data-source local`; drain-first (single pricing scan, then compute).

**2. models rank** — `models rank [--image-to-image] [--resolution 4K] [--max-cost 0.10] [--limit 5] [--json]`
- One-line: Constraint query over the synced catalog returning cheapest-first (model, provider) combos that satisfy capability + budget filters.
- Persona served: Relay, Maya. Source: (c) cross-entity join (`image_models.supported_parameters` × `endpoints` pricing) + (b) service-specific content pattern (dynamic 40+ model catalog with per-provider pricing).
- Long Description: "Use this command to choose a model+provider combo from capability and budget constraints. Do NOT use it to inspect a specific model's providers; use 'models endpoints <model>' instead. Do NOT use it to estimate cost for a model you already picked; use 'cost-estimate' instead."
- Kill/keep: PASS. `pp:data-source local`; drain-first (scan candidate rows into structs, close, then resolve provider pricing).

**3. regenerate** — `regenerate <generation-id> [--tweak "<prompt edit>"] [--output <file>] [--dry-run] [--json]`
- One-line: Re-run a past generation with its exact stored parameters (model, seed, resolution, quality, output_format, references) read from the local history ledger.
- Persona served: Maya, Relay (deterministic re-runs). Source: (b) service-specific content pattern (seed determinism / reproducible generation) + (a) persona-driven.
- Long Description: "Use this command to re-run a past generation with its exact stored parameters. Do NOT use it for a brand-new prompt; use 'generate' instead."
- Kill/keep: PASS. `pp:data-source auto` (params from local ledger, request via live POST /images); drain-first (read generation row, close, then build flags).

**4. usage top** — `usage top --group-by model --since 7d [--limit 10]`
- One-line: Rank models/providers by spend, count, and average cost from the local generation ledger.
- Persona served: Sam. Source: (c) + (a).
- Long Description: "Use this command to rank models by spend over a window. Do NOT use it for a period-over-period summary; use 'usage digest' instead."
- Kill/keep: REFRAME-FLAG — the generic framework `analytics --type generations --group-by model` likely emits the same grouped table from the synced resource, and provider attribution already lives on generation records, so no cross-source join survives. Likely Pass 3 kill.

**5. credits** — `credits [--json] [--agent]`
- One-line: Show remaining credits and rate limits from GET /key + GET /credits in agent-shaped output.
- Persona served: Relay, Sam. Source: (e) user briefing (agents generate on demand; need budget state) + (b).
- Long Description: none.
- Kill/keep: PASS checks but flagged — thin wrapper over two GET endpoints with no local join; the planned `doctor` command already covers key/reachability. Likely Pass 3 kill.

**6. models diff** — `models diff [--since 7d] [--json]`
- One-line: Diff the current catalog snapshot against the previous sync: newly added, retired, and price-changed models/providers.
- Persona served: Relay (must know when a pinned model retires), Maya. Source: (b) service-specific content pattern (dynamic catalog) + (c) snapshot-vs-snapshot local query.
- Long Description: "Use this command to see what changed in the model catalog between syncs. Do NOT use it to browse the current catalog; use 'models list' instead."
- Kill/keep: PASS (needs a minor snapshot-table addition — build feasibility 1). `pp:data-source local`; drain-first (scan current rows, close, then compare to stored snapshot).

**7. batch** — `batch --spec <csv> [--budget <usd>] [--dry-run] [--json]`
- One-line: Run N generations from a CSV (prompt, model, resolution, quality per row) with a hard budget gate: estimate all rows from local pricing first, abort before any spend if total exceeds `--budget`, then execute and append each cost to the ledger.
- Persona served: Relay, Sam. Source: (a) persona-driven (cron pipelines) + (e) user briefing (agent generates on demand with explicit model selection; defer to agent recommendations).
- Long Description: "Use this command to plan and run a budgeted batch of generations from a CSV. Do NOT use it for a single ad-hoc image; use 'generate' instead. Do NOT use it to estimate one model's cost interactively; use 'cost-estimate' instead."
- Kill/keep: PASS after descope to one command (plan-then-execute inside one invocation, no TUI). `pp:data-source auto` (estimates from local pricing, execution via live API); drain-first (read CSV + pricing rows fully, then execute).

**8. usage digest** — `usage digest [--since 7d] [--json] [--agent]`
- One-line: Weekly spend/volume summary with period-over-period trend (images generated, USD spent, top models, avg cost/image vs the prior window).
- Persona served: Sam, Relay (emits weekly cost report). Source: (c) cross-entity local query (two-window self-join on the generation ledger) + (a).
- Long Description: "Use this command for a period-over-period spend and volume summary. Do NOT use it to estimate a future generation's cost; use 'cost-estimate' instead."
- Kill/keep: PASS — trend requires a two-window comparison the generic analytics command does not emit. `pp:data-source local`; drain-first (scan window 1 into structs, close, then run window 2 query).

**9. models audit** — `models audit [--json]`
- One-line: Flag local history entries whose model no longer exists in the synced catalog, or whose provider pricing changed materially.
- Persona served: Relay. Source: (c) cross-entity join (generations × image_models × endpoints) + (b).
- Long Description: "Use this command to check your generation history against the current catalog. Do NOT use it to see catalog changes generally; use 'models diff' instead."
- Kill/keep: PASS checks but flagged — periodic safety-net, not a weekly ritual; drift is also caught by models diff + generate-time model validation. Likely Pass 3 kill.

**10. reference** — `reference <image-file>`
- One-line: Emit a base64 data URL for a local image so it can be passed to `generate --reference`.
- Persona served: Maya, Relay. Source: (a) + (b) `input_references` content pattern.
- Long Description: "Use this command to prepare a local image for image-to-image. Do NOT use it to run an edit; use 'generate --reference' instead."
- Kill/keep: FLAGGED — `base64 -w0` one-liner, no API involvement, and generate --reference already accepts local data URLs. Likely Pass 3 kill.

**11. plan** — `plan --spec <csv> [--budget <usd>] [--json]`
- One-line: Estimate-and-print pass over a batch CSV spec: per-row cost + total, no generation.
- Persona served: Sam, Relay. Source: (a) + (e).
- Long Description: "Use this command to preview a batch's cost. Do NOT use it to execute the batch; use 'batch' instead."
- Kill/keep: FLAGGED — `batch --dry-run` performs the identical estimate pass; duplicate command surface. Likely Pass 3 kill.

**12. endpoints cheapest** — `endpoints cheapest --model <slug> [--resolution 1K]`
- One-line: Show the single cheapest provider for one model at a given resolution.
- Persona served: Maya, Sam. Source: (c) + (b).
- Long Description: "Use this command for the cheapest provider of one model. Do NOT use it to compare providers side by side; use 'models endpoints <model>' instead. Do NOT use it to choose among models; use 'models rank' instead."
- Kill/keep: FLAGGED — one row of models rank's output space, and its model-slug input overlaps `models endpoints`. Likely Pass 3 kill.

**13. history find** — `history find --hash <sha256>` / `history find --seed <n>`
- One-line: Locate past generations by output-file hash or seed from the local ledger.
- Persona served: Maya. Source: (c) + (a).
- Long Description: "Use this command to locate a past run by file hash or seed. Do NOT use it to re-run a run; use 'regenerate' instead."
- Kill/keep: FLAGGED — plain index lookup with no join; `search --type generations` and history listing already locate past runs. Likely Pass 3 kill.

## Survivors and kills

### Survivors

**cost-estimate** — (1) Weekly use: Sam estimates a new model/resolution combo weekly before adopting it; Maya checks per-variant cost while iterating — yes, weekly. (2) Wrapper vs leverage: not a wrapper — no API call at all; computes from synced pricing lines × resolution tiers, which a generic client cannot do offline. (3) Transcendence proof: local SQLite pricing computation (service-specific pricing-line content pattern). (4) Sibling kill: plan — its CSV estimation duplicated this path and was absorbed into `batch --dry-run`, leaving cost-estimate as the interactive single-row probe. (5) Buildability: hand-code. (6) Long-description validity: references `generate --dry-run` (survives as absorbed headline command) — valid.

**models rank** — (1) Weekly use: Relay's pipeline picks models weekly (workflow 2: "which models do image-to-image and 4K, cheapest first"); Maya compares weekly — yes. (2) Wrapper vs leverage: not a wrapper — two-table constraint ranking no endpoint returns. (3) Transcendence proof: cross-source join (`models.supported_parameters` × `endpoints` pricing) in local SQLite. (4) Sibling kill: endpoints cheapest — single-row subset of this output space with overlapping model-slug input. (5) Buildability: hand-code. (6) Long-description validity: references `models endpoints` (absorbed #3) and `cost-estimate` (survives) — valid.

**regenerate** — (1) Weekly use: Maya re-runs and tweaks winning prompts weekly; Relay re-runs failed cron generations with identical params — yes. (2) Wrapper vs leverage: not a wrapper — the request is rebuilt from stored ledger params (seed, quality, etc.) no generic client has access to. (3) Transcendence proof: local SQLite history + seed-determinism service pattern. (4) Sibling kill: history find — index lookup killed as non-transcendent; regenerate is the meaningful replay half of the pair. (5) Buildability: hand-code. (6) Long-description validity: references `generate` (survives) — valid.

**models diff** — (1) Weekly use: Relay checks catalog changes when scheduling each week's batch; the catalog is explicitly dynamic (40+ models today) — yes, at sync cadence. (2) Wrapper vs leverage: not a wrapper — snapshot-vs-snapshot diff, no endpoint returns it. (3) Transcendence proof: local SQLite snapshot history (needs minor snapshot-table addition). (4) Sibling kill: models audit — drift-detection family killed for failing the weekly-ritual bar. (5) Buildability: hand-code. (6) Long-description validity: references `models list` (absorbed #2, survives) — valid.

**batch** — (1) Weekly use: Relay runs scheduled batches weekly (nightly assets, doc diagrams); Sam runs budgeted batches weekly — yes. (2) Wrapper vs leverage: not a wrapper — plan-estimate-execute orchestration with a hard budget gate from local pricing; no single endpoint does this. (3) Transcendence proof: local SQLite pricing estimate + ledger append, agent-shaped exit codes. (4) Sibling kill: plan — identical estimate pass folded into `--dry-run`. (5) Buildability: hand-code. (6) Long-description validity: references `generate` and `cost-estimate` (both survive) — valid.

**usage digest** — (1) Weekly use: Sam's Monday spend review; Relay's weekly cost report to its operator — yes. (2) Wrapper vs leverage: not a wrapper — two-window self-join trend the generic analytics command cannot emit. (3) Transcendence proof: cross-source local query (period-over-period ledger join) with agent-shaped output. (4) Sibling kill: usage top — redundant with framework analytics; digest keeps the cross-period trend top lacked. (5) Buildability: hand-code. (6) Long-description validity: references `cost-estimate` (survives) — valid.

| # | Feature | Command | Score | Persona | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|---------|--------------|--------------------------|------------------|
| 1 | Pre-spend cost estimate | `cost-estimate --model <slug> [--resolution 1K] [--quality high] [--n 4] [--json]` | 8/10 | Sam, Maya | hand-code | Joins synced endpoints pricing lines (unit=image\|megapixel) with the model's resolution tiers in local SQLite to compute a pre-spend USD estimate with no API call, so no generic client can produce it offline. | Use this command to estimate the cost of a future generation before spending credits. Do NOT use it to preview the request payload; use 'generate --dry-run' instead. |
| 2 | Capability+budget model ranker | `models rank [--image-to-image] [--resolution 4K] [--max-cost 0.10] [--limit 5] [--json]` | 10/10 | Relay, Maya | hand-code | Joins `image_models.supported_parameters` against per-provider endpoints pricing in local SQLite to rank (model, provider) combos cheapest-first under capability+budget constraints — a query no single endpoint and no generic client can answer. | Use this command to choose a model+provider combo from capability and budget constraints. Do NOT use it to inspect a specific model's providers; use 'models endpoints <model>' instead. Do NOT use it to estimate cost for a model you already picked; use 'cost-estimate' instead. |
| 3 | Deterministic re-generation | `regenerate <generation-id> [--tweak "<prompt edit>"] [--output <file>] [--dry-run] [--json]` | 8/10 | Maya, Relay | hand-code | Reads the exact stored parameter set (model, seed, resolution, quality, output_format, references) from the local generation ledger and rebuilds a deterministic POST /images request — replay power only a local history store provides. | Use this command to re-run a past generation with its exact stored parameters. Do NOT use it for a brand-new prompt; use 'generate' instead. |
| 4 | Catalog change diff | `models diff [--since 7d] [--json]` | 7/10 | Relay, Maya | hand-code | Compares the current catalog snapshot against the stored previous snapshot in local SQLite to emit newly added, retired, and price-changed models — impossible without a local snapshot history. | Use this command to see what changed in the model catalog between syncs. Do NOT use it to browse the current catalog; use 'models list' instead. |
| 5 | Budget-gated batch run | `batch --spec <csv> [--budget <usd>] [--dry-run] [--json]` | 10/10 | Relay, Sam | hand-code | Estimates every CSV row from local pricing lines before the first API call, gates execution on a hard budget, then runs each generation and appends real cost to the ledger — a plan-estimate-execute loop no single endpoint offers. | Use this command to plan and run a budgeted batch of generations from a CSV. Do NOT use it for a single ad-hoc image; use 'generate' instead. Do NOT use it to estimate one model's cost interactively; use 'cost-estimate' instead. |
| 6 | Weekly spend digest | `usage digest [--since 7d] [--json] [--agent]` | 7/10 | Sam, Relay | hand-code | Self-joins the local generation ledger across two time windows to compute spend/volume trends the generic analytics command cannot emit, output as an agent-shaped report. | Use this command for a period-over-period spend and volume summary. Do NOT use it to estimate a future generation's cost; use 'cost-estimate' instead. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| usage top | Generic framework `analytics --type generations --group-by model` emits the same grouped spend table from the synced generations resource, and provider attribution already lives on generation records, so no cross-source join survives. | usage digest |
| credits | Thin wrapper over GET /key + GET /credits with no local join or computation beyond --json, and the planned `doctor` command already covers key/reachability checks. | cost-estimate |
| models audit | Periodic safety-net rather than a weekly ritual; retired-model drift is caught by `models diff` plus generate-time model validation, so it fails the weekly-use bar. | models diff |
| reference | A base64 conversion shell one-liner with no API involvement, and `generate --reference` already accepts local data URLs directly. | generate |
| plan | `batch --dry-run` performs the same estimate-then-print pass over a CSV spec, so a separate command duplicates behavior with no new output shape. | batch |
| endpoints cheapest | Per-model cheapest-provider lookup is one row of `models rank`'s output space and its model-slug input overlaps `models endpoints`, so it fails wrapper-vs-leverage. | models rank |
| history find | Plain index lookup on the ledger with no join; `search --type generations` and history listing already locate past runs, so it is not transcendent. | regenerate |

---

**Summary:** Ran the three-pass novel-features brainstorm for `openrouter-image-pp-cli`. Read the brief, the absorb-scoring rubric, and the novel-features subagent playbook from disk. Produced the contract-compliant markdown above: 3 personas (Relay/Maya/Sam) grounded in the brief's Users + Top Workflows; 13 pre-cut candidates (sources a/b/c/e; reprint reconciliation omitted — first print, no Codebase Intelligence section); adversarial cut to 6 survivors (all hand-code, 7–10/10) and 7 killed candidates with one-sentence reasons and surviving siblings. No files written; no issues encountered.