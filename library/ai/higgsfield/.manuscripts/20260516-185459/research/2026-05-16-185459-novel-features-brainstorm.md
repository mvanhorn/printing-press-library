# Novel Features Brainstorm — higgsfield-pp-cli

Full subagent output, retained for retro/dogfood debugging. Survivors flow into the absorb manifest's transcendence table; the Customer model and Killed candidates do NOT, but are preserved here.

## Customer model

**Persona 1: Kai (creator-executive, Scam Patrol + SupraOS)**

*Today (without this CLI):* Kai runs the official `higgsfield` CLI in one terminal tab, has the Higgsfield web UI open in two browser tabs (one for Soul ID gallery, one for credit balance), and keeps a Notion page with hand-written model parameter cheatsheets. To generate Riggs cut-ins he runs `higgsfield generate create dop_cinematic --soul-id <some-id>` one prompt at a time, waiting 3-5 minutes per job, then re-runs `higgsfield generate list` to see results. He cannot answer: "how much did I spend on Veo 3.1 last week?", "which Soul ID did I use for Riggs in episode 4?", "what was that prompt that worked really well two weeks ago?".

*Weekly ritual:* Produce one Scam Patrol episode (12-20 DOP cinematic Riggs cut-ins from a trained Soul ID) and 2-3 SupraOS social videos (Seedance/Veo b-roll). Each video is a fanout: he writes 4-6 prompts and wants to see them across 3-4 models to pick the best take.

*Frustration:* Submitting the same prompt to 4 models is 4 terminal commands, 4 job IDs to track, 4 `wait` calls. No way to search his Soul library by name from the command line. No way to see "this Veo 3.1 fanout will cost 240 credits before I submit."

**Persona 2: Hanh (Scam Patrol co-producer, agent-assisted)**

*Today (without this CLI):* Hanh works via a Claude Code agent that shells out to the official `higgsfield` CLI. The agent has no memory of prior generations — every "find the Riggs Soul ID" request re-pages through `higgsfield soul-id list` and parses JSON. The agent can't answer "did we already generate a cut-in for this scene?" without listing all generations and grep'ing prompts.

*Weekly ritual:* Drives the Soul ID library — adds new Soul IDs for new characters, retires unused ones, picks Soul IDs to reuse for episode continuity. Reviews credit ledger weekly to forecast next month's spend.

*Frustration:* Soul IDs have non-descriptive names. The agent has no FTS over prompts or Soul ID names. Every weekly credit review is a re-paginate of `account transactions` because there's no local ledger.

**Persona 3: SupraOS social-video agent (autonomous, batch-driven)**

*Today (without this CLI):* The supra-social-build skill runs Higgsfield via shell-out for b-roll. It submits jobs serially because there's no batch primitive, and it has no credit guard — a 6-clip Veo 3.1 fanout can silently consume 360+ credits before the agent notices it overspent.

*Weekly ritual:* Builds 1-2 SupraOS videos per week. Each needs 4-8 b-roll clips. Picks model per shot (Seedance for motion, Veo 3.1 for hero shots). Wants reproducible cost forecasts so the operator (Kai) can approve spend before submission.

*Frustration:* Submitting 8 jobs is 8 CLI calls plus 8 polls. No "submit this prompt to these 3 models in parallel and tell me the cost first" primitive. No way to compare the same prompt rendered by different models side-by-side from the local store after the fact.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Kill/keep |
|---|------|---------|-------------|---------|--------|-----------|
| C1 | Batch fanout | `fanout --prompt "..." --models veo-3.1,seedance-2,kling-3` | Submit one prompt to N models in parallel, returns N job_ids, persists fanout_id linking them | Kai, SupraOS agent | (a)(b)(e) | KEEP |
| C2 | Fanout wait + compare | `fanout wait <fanout_id>` / `fanout compare <fanout_id>` | Polls all jobs in a fanout, returns side-by-side result table | Kai, SupraOS agent | (c)(e) | KEEP |
| C3 | Soul library search | `soul-id search "riggs"` | FTS5 over soul_ids.name + linked-prompt history | Hanh, Kai | (a)(c)(e) | KEEP |
| C4 | Soul usage report | `soul-id usage <soul_id>` | Joins generations × soul_ids: every generation that used this Soul ID | Hanh | (c) | KEEP |
| C5 | Credit guard for batches | `fanout ... --max-cost 200` | Pre-flight cost estimate per model, refuses if sum exceeds cap | Kai, SupraOS agent | (a)(b)(e) | KEEP |
| C6 | Prompt history search | `search "cinematic riggs"` | FTS5 over generations.prompt + model + soul_id name | Kai, Hanh | (c)(e) | KEEP |
| C7 | Cost summary report | `account spend --since 7d --group-by model` | Local SQL aggregation over transactions table | Hanh | (c) | KEEP |
| C8 | Stale Soul IDs | `soul-id stale --days 60` | Soul IDs not referenced by any generation in N days | Hanh | (c) | KEEP (pre-cut), KILLED in Pass 3 |
| C9 | Compare two generations | `generate compare <a> <b>` | Pretty diff of two arbitrary generations | Kai | (c) | KILLED in Pass 3 |
| C10 | Rerun last prompt | `generate rerun <id> [--model X]` | Reads prompt from store, resubmits | Kai | (b)(c) | KILLED in Pass 3 |
| C11 | Workflow recipes | `workflow scam-patrol-cutin ...` | Project-shaped bundles | Kai | (a)(b)(e) | KILLED in Pass 3 |
| C12 | Best-of-fanout picker | `fanout pick <fanout_id>` | Auto-selects best output | Kai | (b) | KILL pre-cut (LLM dependency) |
| C13 | Prompt template library | `prompts templates list/save/use` | User prompt templates | Kai | (b) | KILL pre-cut (scope creep) |
| C14 | Veo-vs-Seedance benchmark | `benchmark veo seedance --prompt "..."` | Wall-clock + cost compare | Kai, agent | (b)(e) | KILL pre-cut (merged into C1+C2) |
| C15 | Auto-pick cheapest model | `generate auto --type video --prompt "..."` | Auto-selects cheapest model | SupraOS agent | (b) | KILL pre-cut (fake autopilot) |
| C16 | Generations export | `generate export --since 7d --format csv` | Local SQL export to CSV/JSONL | Hanh, Kai | (c) | KEEP |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Batch model fanout | `fanout --prompt "..." --models veo-3.1,seedance-2,kling-3` | 9/10 | hand-code | Loops over `--models`, calls real per-model submit endpoint, persists `fanout_id` linking returned `request_id`s | Brief Top Workflow #2; User Vision names "batch fanout" |
| 2 | Fanout wait + compare | `fanout wait <fanout_id>` / `fanout compare <fanout_id>` | 8/10 | hand-code | Polls each linked job, renders side-by-side table from local `generations` joined on `fanout_id` | Brief Data Layer + Product Thesis |
| 3 | Soul library search | `soul-id search "riggs"` | 9/10 | hand-code | FTS5 on `soul_ids.name` + join into past prompts; ranks by last-used | User Vision explicit; Hikhakk MCP source confirms no equivalent |
| 4 | Credit guard for batches | `fanout --max-cost 200` | 8/10 | hand-code | Calls real `generate cost` per model, sums, refuses if over cap, prints breakdown | User Vision explicit; Brief Top Workflow #5 |
| 5 | Prompt history search | `search "cinematic riggs"` | 8/10 | spec-emits | Framework `search` reads local FTS5 index over `generations.prompt`, `model`, `soul_id name`, `transaction memo` | Brief Data Layer; framework default |
| 6 | Soul usage report | `soul-id usage <soul_id>` | 7/10 | hand-code | Local SQL: SELECT generations WHERE soul_id_used = ? ORDER BY created_at DESC, sums cost, lists result_urls | Brief Data Layer; Hanh persona |
| 7 | Account spend report | `account spend --since 7d --group-by model` | 7/10 | hand-code | Local SQL aggregation over `transactions` grouped by linked `model` | Brief Data Layer; Hanh ritual |
| 8 | Generations export | `generate export --since 7d --format csv` | 6/10 | hand-code | Local SQL SELECT joined with models/soul_ids, emits CSV/JSONL | Brief Data Layer; agent-native principle |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| C8 Stale Soul IDs | Weekly use marginal — Hanh prunes monthly at best; framework `pp_stale` covers if Soul IDs have `updated_at` | C3 Soul library search |
| C9 Compare two generations | Two-row diff is a soft case of fanout compare; persona compares fanouts (C2) more often than arbitrary historical jobs | C2 Fanout wait + compare |
| C10 Rerun last prompt | Thin wrapper over store read + real submit; generic agent can do `generate get <id>` then copy prompt | C1 Batch model fanout |
| C11 Workflow recipes | Hardcodes project names (Scam Patrol, SupraOS) — violates "don't hardcode API/site names in reusable artifacts"; generic version reduces to one flag on `generate create` | C1 Batch model fanout |
| C12 Best-of-fanout picker | Requires visual quality judgment (LLM dependency, not mechanically verifiable) | C2 Fanout wait + compare |
| C13 Prompt template library | Scope creep — becomes prompt manager mini-app; shell aliases cover it | C5 Prompt history search |
| C14 Veo-vs-Seedance benchmark | Subsumed by C1 fanout + C2 compare with `--models veo,seedance` | C1 Batch model fanout |
| C15 Auto-pick cheapest model | Fake autopilot — cheap ≠ good; needs quality signal the CLI can't compute | C4 Credit guard for batches |
