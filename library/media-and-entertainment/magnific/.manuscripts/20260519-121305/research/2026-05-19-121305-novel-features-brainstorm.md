## Customer model

**Persona A — Mara, indie creative director at a 4-person motion-design studio.**
- **Today (without this CLI):** Uses Magnific's web UI for upscaling client stills, then jumps to a separate browser tab for Kling i2v, then drags MP4s into a Dropbox folder she names by hand. Tracks credits in a spreadsheet.
- **Weekly ritual:** Friday delivery push — upscales 20-40 hero stills (creative upscaler), turns the 5 best into 5-second Kling 2.6 Pro shots, downloads, renames, uploads to client portal. Re-runs prompts she liked last month but can't find.
- **Frustration:** Loses the prompt that produced a winning frame two weeks ago. Can't compare "what would this look like in Seedream vs Mystic" without re-typing. Credit bill arrives and she can't tell which client ate them.

**Persona B — Dev, backend engineer wiring Magnific into a SaaS product's "generate hero image" feature.**
- **Today (without this CLI):** Curl + bash glue, or the TypeScript `Balneario-de-Cofrentes/freepik-cli` which requires Node and re-implements polling per command. Webhooks for production, manual polling in staging.
- **Weekly ritual:** Iterates on prompt templates for the product's three image categories; reruns the same prompt against new model releases (Flux 2 dropped → does it beat Mystic for our use case?); maintains an internal "approved model list" with cost/latency notes.
- **Frustration:** Async task polling boilerplate everywhere. No single command to say "run this prompt through N models and show me cost + latency." Has to write his own webhook receiver to test webhook payloads locally.

**Persona C — Sasha, automation/agent builder shipping a Claude Code workflow that generates marketing assets.**
- **Today (without this CLI):** Built an MCP wrapper around the 388-endpoint OpenAPI spec; the agent gets overwhelmed by the tool catalog and picks the wrong model. Re-downloads the same stock icon 4 times because there's no local cache.
- **Weekly ritual:** Tunes the agent's prompt to nudge it toward Mystic-for-photoreal and Flux-Klein-for-cheap-iteration; reviews failed runs and finds half are rate-limit retries the agent didn't handle.
- **Frustration:** Agent can't introspect "which model is cheapest for this aspect ratio at this quality" without trial and error. Wants the CLI surface to be agent-shaped, not 388 raw tools.

**Persona D — Tomas, stock-content power user / agency producer pulling Freepik icons and resources daily.**
- **Today (without this CLI):** Web UI search → click download → file lands in `~/Downloads` with a random ID. Re-downloads the same set monthly because he can't search his own library.
- **Weekly ritual:** Pulls 30-80 icons per project brief, organized by client; periodically searches the Freepik catalog for fresh "minimal line icon" sets and grabs the top 20.
- **Frustration:** No local index — can't grep "did we already download a rocket icon for ACME?" without opening every folder.

## Candidates (pre-cut)

(See subagent transcript — 16 candidates generated, 10 survived adversarial cut.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Persona served |
|---|---------|---------|-------|--------------|--------------|----------|----------------|
| 1 | Prompt history FTS | `history search "<query>" [--model] [--since]` | 8/10 | hand-code | FTS5 query over local `prompts` table populated on every generate/upscale/video dispatch; joins to `tasks` for cost + output URL | Brief Data Layer calls out `prompts` table FTS5 + Mara's "loses winning prompt" frustration; no API endpoint returns prompt-text history | Mara, Dev |
| 2 | Model bake-off | `compare "<prompt>" --models mystic,flux-2-pro,...` | 9/10 | hand-code | Fans real POST calls to each model's `/v1/ai/<model>` endpoint, polls each task_id, downloads outputs to a dated folder, writes a `manifest.json` with cost + latency per model | Brief lists 22+ text-to-image models and explicit "compare" as Phase 1.5 manifest candidate; competitor `freepik-cli` has no comparison surface | Dev, Mara |
| 3 | Unified task wait/watch | `task wait <task-id>` / `task watch <task-id>` | 8/10 | hand-code | Resolves task_id → model endpoint via local `tasks` row, polls real GET `/v1/ai/<model>/{task-id}` until terminal state; `watch` emits JSON-line status | Brief: every async POST returns task_id and a parallel GET; `freepik-mcp` hand-rolls this per route; unified surface is a transcendence claim | Dev, Sasha |
| 4 | Cost ledger + forecast | `cost ledger --group-by model --since 30d` / `cost forecast --model <m> --count <n>` | 7/10 | hand-code | Ledger: SQL aggregation over local `tasks.credit_cost` grouped by model/tag/day. Forecast: curated per-model credit cost × intended run count, compared to current `/v1/me/credits` balance | Mara's "credit bill arrives and I can't tell which client ate them"; pay-per-credit is Magnific's economic identity; no API endpoint returns this join | Mara, Dev |
| 5 | Local asset gallery | `gallery list [--tag] [--since]` / `gallery open <id>` (opt-in side-effect) | 6/10 | hand-code | Filters local `assets` table by tag/orientation/date/model; `open` honors `cliutil.IsVerifyEnv` and requires `--launch` flag per side-effect rule | Mara renames+files MP4s by hand; Tomas re-downloads icons monthly; brief calls out local asset FTS5 as data-layer goal | Mara, Tomas |
| 6 | Models registry with empirical stats | `models list --capability <cap> --sort cost` / `models stats <model>` | 8/10 | hand-code | Joins curated `models` table (`// pp:novel-static-reference`: capabilities, listed credit cost, family) with local `tasks` aggregation (your p50 latency, success rate, $ spent) | Brief: 80+ models is Magnific's defining trait; route-whitelist pattern in `freepik-mcp` exists because agents can't pick; Sasha's "wrong model" frustration | Sasha, Dev |
| 7 | Stale-task reconcile | `tasks stale --since 24h` / `tasks reconcile` | 7/10 | hand-code | Local SQL finds tasks in non-terminal state past threshold; reconcile re-polls each via real GET and updates row | Brief: async lifecycle drifts when polling crashes; standard local-data pattern in `internal/store`; Dev's polling-boilerplate frustration | Dev, Sasha |
| 8 | Prompt templates / replay | `prompt save <name>` / `prompt run <name> [--override k=v]` | 7/10 | hand-code | `save` writes to local `prompts` table with placeholder syntax; `run` substitutes vars and dispatches the real model endpoint via the typed client | Mara replays prompts from memory; brief Data Layer names `prompts` as primary entity; no API endpoint stores reusable prompt templates | Mara, Dev |
| 9 | Stock library local index | `stock library index` / `stock library search "<q>"` | 6/10 | hand-code | `index` walks downloaded icons/videos/resources into local FTS5 table; `search` queries local FTS before any API call | Tomas re-downloads same icons; brief: stock catalog is 250M+ assets, too large to pre-sync, but downloaded subset is small and FTS-able | Tomas |
| 10 | Agent context bundle | `context` | 6/10 | hand-code | Single read-only command (`mcp:read-only`) printing JSON: top 10 models used, current credit balance, last 10 prompts, recent output dirs — one fetch from local store + one `/v1/me/credits` call | `freepik-mcp` whitelists routes specifically to avoid 388-tool overload; Sasha's agent picks wrong models; agent-native surface is AGENTS.md mandate | Sasha |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Webhook receiver (`webhook serve`) | Scope creep — persistent background process, requires tunneling, undogfoodable | Unified task wait/watch (#3) |
| Pipeline runner (`pipeline run pipeline.yaml`) | Scope creep — fragile YAML schema, application not command | Model bake-off (#2) |
| AI vs real classifier batch | Thin wrapper around a single endpoint with a directory loop; no transcendence | Spec-emitted `analyze classify` + shell `xargs` |
| Credit-aware dry-run as standalone | Generator-surface polish (a flag), not a novel command | Cost forecast (#4) |
| Model search by capability standalone | Duplicates the filter flags on `models list` from #6 | Models registry (#6) |
| Rate-limit pacer (`pace status` / `--pace`) | Foundation/client behavior, not a user-facing feature | Generated client retry/backoff behavior |
