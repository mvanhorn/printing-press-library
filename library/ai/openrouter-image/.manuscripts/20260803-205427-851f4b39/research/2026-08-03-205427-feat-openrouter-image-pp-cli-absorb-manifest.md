# OpenRouter Image CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Generate image from prompt | Official spec POST /images | (generated endpoint) images generate | --json, --dry-run, offline history |
| 2 | List image models w/ capabilities | Official spec GET /images/models | (generated endpoint) images models list | local cache, FTS search |
| 3 | Per-endpoint pricing/capability records | Official spec GET /images/models/{author}/{slug}/endpoints | (generated endpoint) images models endpoints | provider price compare |
| 4 | Generation metadata + cost | Official spec GET /generation | (generated endpoint) generation get | typed cost fields |
| 5 | Generation content lookup | Official spec GET /generation/content | (generated endpoint) generation content | agent-readable |
| 6 | Generation feedback | Official spec POST /generation/feedback | (generated endpoint) generation feedback | scriptable |
| 7 | Model discovery via chat API filter | Official spec GET /models?output_modalities=image | (generated endpoint) models list | unified catalog |
| 8 | Agent-first structured output | motif-cli (npm, fal.ai agent-first image CLI) | (behavior in openrouter-image-pp-cli generate) --json --agent | dry-run, typed exit codes |
| 9 | Generation history | motif-cli (npm) | (behavior in openrouter-image-pp-cli history) | local SQLite, offline |
| 10 | Multi-model selection | imagnx (npm) | openrouter-image-pp-cli generate --model | 40+ models one key |
| 11 | Reference images (image-to-image) | openrouter-image-mcp (hamzatrq) | openrouter-image-pp-cli generate --reference | URL or base64 |
| 12 | Parallel/batch generation | openrouter-image-mcp (hamzatrq) | openrouter-image-pp-cli generate --n | 1-10 images |
| 13 | Model list + gallery UI | imagen-openrouter (yusufipk) | (behavior in openrouter-image-pp-cli history) | offline gallery index |
| 14 | Cost attribution per generation | existing openrouter library CLI (rvdlaar) | (behavior in openrouter-image-pp-cli usage) | local cost ledger |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Pre-spend cost estimate | cost-estimate --model <slug> [--resolution 1K] [--quality high] [--n 4] | hand-code | Joins synced endpoints pricing lines (unit=image\|megapixel) with the model's resolution tiers in local SQLite to compute a pre-spend USD estimate with no API call, so no generic client can produce it offline. | Use this command to estimate the cost of a future generation before spending credits. Do NOT use it to preview the request payload; use 'generate --dry-run' instead. |
| 2 | Capability+budget model ranker | models rank [--image-to-image] [--resolution 4K] [--max-cost 0.10] [--limit 5] | hand-code | Joins `image_models.supported_parameters` against per-provider endpoints pricing in local SQLite to rank (model, provider) combos cheapest-first under capability+budget constraints — a query no single endpoint and no generic client can answer. | Use this command to choose a model+provider combo from capability and budget constraints. Do NOT use it to inspect a specific model's providers; use 'models endpoints <model>' instead. Do NOT use it to estimate cost for a model you already picked; use 'cost-estimate' instead. |
| 3 | Deterministic re-generation | regenerate <generation-id> [--tweak "<prompt edit>"] [--output <file>] | hand-code | Reads the exact stored parameter set (model, seed, resolution, quality, output_format, references) from the local generation ledger and rebuilds a deterministic POST /images request — replay power only a local history store provides. | Use this command to re-run a past generation with its exact stored parameters. Do NOT use it for a brand-new prompt; use 'generate' instead. |
| 4 | Catalog change diff | models diff [--since 7d] | hand-code | Compares the current catalog snapshot against the stored previous snapshot in local SQLite to emit newly added, retired, and price-changed models — impossible without a local snapshot history. | Use this command to see what changed in the model catalog between syncs. Do NOT use it to browse the current catalog; use 'models list' instead. |
| 5 | Budget-gated batch run | batch --spec <csv> [--budget <usd>] [--dry-run] | hand-code | Estimates every CSV row from local pricing lines before the first API call, gates execution on a hard budget, then runs each generation and appends real cost to the ledger — a plan-estimate-execute loop no single endpoint offers. | Use this command to plan and run a budgeted batch of generations from a CSV. Do NOT use it for a single ad-hoc image; use 'generate' instead. Do NOT use it to estimate one model's cost interactively; use 'cost-estimate' instead. |
| 6 | Weekly spend digest | usage digest [--since 7d] | hand-code | Self-joins the local generation ledger across two time windows to compute spend/volume trends the generic analytics command cannot emit, output as an agent-shaped report. | Use this command for a period-over-period spend and volume summary. Do NOT use it to estimate a future generation's cost; use 'cost-estimate' instead. |

### Stubs
None. All 6 transcendence features ship fully.

## Scores (from brainstorm subagent)
- models rank: 10/10, batch: 10/10, cost-estimate: 8/10, regenerate: 8/10, models diff: 7/10, usage digest: 7/10
