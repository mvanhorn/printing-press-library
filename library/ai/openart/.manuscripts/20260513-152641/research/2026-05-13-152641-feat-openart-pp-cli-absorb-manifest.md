# OpenArt CLI - Absorb Manifest

## Absorbed (match or beat what every comparable tool offers)

OpenArt has zero direct competitors (no SDK, no CLI, no MCP). These rows absorb the *pattern-equivalent* features from Replicate / Fal.ai / Higgsfield CLIs and MCPs, applied to OpenArt's REST surface.

| # | Feature | Best Source (pattern-equivalent) | Our Implementation | Added Value |
|---|---------|---------------------------------|--------------------|-------------|
| 1 | Submit a generation | Replicate `predictions create`, fal `queue.submit`, Higgsfield MCP `generate_video` | `openart forms submit <capability-id>` (auto-generated) | Plus high-UX wrapper `openart video gen` (see #6) |
| 2 | Poll a generation | Replicate `predictions get`, fal `queue.status` | `openart media get <resourceId>` (auto-generated) | Local-store snapshot on every poll for offline replay |
| 3 | List my generations | Replicate `predictions list` | `openart media list` (auto-generated, cursor-paginated) | Synced into local SQLite for FTS + filtering |
| 4 | List models | Replicate `model list`, Higgsfield `list_models` | `openart models list` | Curated cross-vendor catalog (Seedance, Kling, Veo, Wan, Grok, Hedra, ...) with cost per resolution/duration |
| 5 | Show model details | Replicate `model show` | `openart models show <slug>` | Surface OpenArt's per-model feature flags (Reference, Start/End, Audio) and credit cost |
| 6 | Submit + wait synchronously | replicate npm `run()`, fal-client `subscribe()` | `openart video gen --prompt ... --wait` | Defaults to async; `--wait` polls in-process, `--notify` rings the terminal bell on completion |
| 7 | Async fire-and-forget submit | fal `queue.submit` | `openart video gen --prompt ... --no-wait` (default) | Returns historyId+resourceIds immediately; user can `openart media get` later |
| 8 | Upload reference image | fal `storage.upload`, Replicate file upload | `openart upload <path>` then `openart upload list` | Auto-handles `/upload/sign` then `/upload/persist` |
| 9 | Get account credit balance | Table-stakes for credit-based services | `openart credits balance` (also surfaced in `doctor`) | Sourced from `/suite/api/user/my-info.free_credit_balance` |
| 10 | Credit ledger / spend history | Table-stakes for credit-based services | `openart credits log` (auto-generated) | Synced to local store; transcendence #2 aggregates over it |
| 11 | Project / workspace listing | Table-stakes for multi-tenant | `openart project list`, `openart workspace list` | Auto-generated; resolves default project for other commands |
| 12 | Folder management | OpenArt UI feature | `openart folder list` | Auto-generated; passable as `--folder` filter to other commands |
| 13 | Templates list | OpenArt UI feature | `openart templates list` (auto-generated) | Local cache; agent-friendly recall |
| 14 | Auto-polish a prompt | OpenArt UI Auto-Polish toggle | `openart prompt enhance "..."` (auto-generated) | Pipeable: `echo "..." \| openart prompt enhance` |
| 15 | Image-to-prompt reverse | OpenArt UI feature | `openart prompt from-image <url>` (auto-generated) | OpenArt-unique; agents can use to iterate on existing media |
| 16 | Download a generation | Standard pattern | `openart download <resourceId> --output <file>` | Streams CDN URL to disk; progress bar |
| 17 | MCP tool surface for agents | Replicate MCP, Fal MCP, Higgsfield MCP | `openart mcp` (auto-generated from Cobra tree) | Every CLI command becomes an MCP tool; `--read-only` annotations on lookups |
| 18 | Health check | Standard CLI pattern | `openart doctor` | Verifies cookie auth + API reachability + balance |
| 19 | Search local generations (FTS) | Standard CLI pattern | `openart search "<query>"` | Generic FTS5 over media table |
| 20 | SQL over local data | Standard CLI pattern | `openart sql "..."` | Read-only SELECT; full schema available |
| 21 | Cancel a generation | Replicate `predictions cancel` | (NOT exposed by OpenArt's API; documented in README "Known Gaps") | Pre-flight cost preview (transcendence #1) is the workaround |

## Transcendence (only possible because we have local SQLite + cross-model awareness + agent-shaped output)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Cost preview before submit | `openart cost estimate --model byte-plus-seedance-2 --duration 10 --count 4` | 8/10 | Reads local `models` table (credits_per_unit_default x duration x count); falls back to POST `/suite/api/topaz/estimate` for capabilities that expose it. | User Vision: "credit-aware". Browser-sniff confirmed `topaz/estimate` endpoint. Persona 2 frustration: "if I run this batch will it cost 3,200 or 6,400?" |
| 2 | Credit burn breakdown | `openart credits burn --since 7d --by model` | 9/10 | Pure local SQL over `credits_ledger` synced from `/suite/api/credits/logs`, grouped by model/tool/day/project. The OpenArt UI shows entries serially with no aggregation. | Browser-sniff confirmed ledger has `businessDetails[].subBusinessType` + `unitCredits`. Brief: "credit accounting that the OpenArt UI itself doesn't expose well." |
| 3 | Prompt FTS recovery search | `openart prompts find "molten dragon" --model seedance2 --has-audio --since 30d` | 8/10 | FTS5 over `media_fts(prompt, label, model, project_name)` joined to `media.metadata.has_audio`/`duration`/`createdAt`. OpenArt-specific filter set differentiates from generic absorbed `search`. | Brief Data Layer defines `media_fts`. Persona 3 weekly ritual: "Re-uses 20-30% of past prompts." OpenArt UI has no prompt search. |
| 4 | Replay a past generation, optionally on a different model | `openart prompts replay <resourceId> [--model NEW] [--bump duration=10]` | 7/10 | Looks up `media.input` JSON in local store, remaps params across model schemas using `models` table, re-issues `POST /suite/api/forms/creations/<capability>`. Cross-model remap is the local lift. | Browser-sniff confirmed `media.input` is preserved on resources. Personas 3 + 4. No competitor (Replicate, Fal, Higgsfield) does cross-model parameter remap because they're single-pool. |
| 5 | Cross-model fan-out compare | `openart compare --prompt "..." --models byte-plus-seedance-2,kling2-6,grok-imagine --duration 5` | 8/10 | Submits N parallel `POST /suite/api/forms/creations/<capability>` calls, polls all `/suite/api/resources/<id>` to completion, joins each result with `models.credits_per_unit_default` for a markdown report. | Persona 4 weekly ritual. Brief lists "cross-model comparison" as NOI. Browser-sniff confirmed the same submit envelope works across model slugs. |
| 6 | Credit runway forecast | `openart credits forecast` | 7/10 | Joins rolling 4-week burn from local `credits_ledger` against current `free_credit_balance` from `/suite/api/user/my-info`. Outputs "At 4,300 credits/wk you have ~10 weeks of runway." | Persona 2 frustration; brief target user has 43,206 credits on Infinite plan. UI shows neither rate nor projection. |
| 7 | Cheapest-model lookup | `openart models cheapest --type video --duration 10 --resolution 720p` | 7/10 | Pure local query against `models` table filtered by capability, ranked by `credits_per_unit_default`. | Brief lists video models with 5x credit-cost spread (Grok 150 vs Seedance 2 800). Personas 2 + 4. UI buries cost behind per-model tooltips. |
| 8 | Library stats blob | `openart media stats` | 6/10 | One SQL query yields counts per type/model/period + top-spending prompts + rolling totals. No TUI, no flags beyond `--since`. | Personas 2/3 want a "where am I" snapshot. Pure local aggregation. |
| 9 | Prompt spend leaderboard | `openart prompts top --since 30d --by spend` | 6/10 | Joins `media.prompt` x `credits_ledger.amount` via resource id, groups by prompt-hash, ranks by total cost. | Personas 2/3. OpenArt UI surfaces cost per-job, never per-prompt-across-jobs. |
