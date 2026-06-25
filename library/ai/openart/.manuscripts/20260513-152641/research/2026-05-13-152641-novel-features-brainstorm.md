# OpenArt CLI — Novel-Features Brainstorm (subagent output)

> Full audit-trail output from the Phase 1.5 Step 1.5c.5 novel-features subagent. Saved per the SKILL contract for retro/dogfood debugging.

## Customer model

**Persona 1: Matt the Seedance-curious indie creator**
- Today (without this CLI): Opens openart.ai in Chrome, navigates to /suite/create-video/byte-plus-seedance-2, types prompt into the form, submits, leaves the tab open for ~10 minutes, comes back, downloads the MP4 manually, repeats for the next variation.
- Weekly ritual: Burns ~2,000-5,000 credits per week prototyping Seedance video ideas - usually wants 2-4 variations of the same prompt to compare.
- Frustration: The 10-minute wait kills momentum. Forgets to come back to the tab. No notification when it's done. Can't kick off 4 prompts in parallel from the terminal while writing other code.

**Persona 2: The credit-conscious Infinite-plan power user**
- Today (without this CLI): Sees credit balance in the OpenArt UI corner but has no idea where the credits went. Pricing is opaque (Seedance 2 = 800 credits/720p/10s, Kling 2.6 = 100, Grok = 150) and varies wildly by model + duration + resolution.
- Weekly ritual: Periodically logs in, sees a smaller balance, shrugs. Has 43,206 credits today and wants them to last.
- Frustration: Can't answer "what did I spend last week?" or "which model is eating my credits?" or "if I run this Seedance batch of 4 will it cost me 3,200 or 6,400 credits?" The UI's credit log is paginated and not searchable.

**Persona 3: The "I had a great prompt last month" hunter**
- Today (without this CLI): Knows they generated a perfect Seedance dragon clip three weeks ago. Scrolls the OpenArt media grid hunting for it. Can vaguely remember "had the word 'molten' in the prompt." Eventually gives up and re-prompts from scratch, burning more credits.
- Weekly ritual: Re-uses 20-30% of past prompts as starting points for new generations.
- Frustration: OpenArt's media library has no full-text search over prompts. No way to filter by model + has-audio + duration. The prompt was the most valuable artifact and it's effectively lost.

**Persona 4: The cross-model A/B tester**
- Today (without this CLI): Wants to see "this prompt on Seedance 2 vs Kling 2.6 vs Grok Imagine" to decide which model to commit a long shoot to. Has to navigate to three separate /suite routes, copy-paste prompt into each form, submit each, wait, manually correlate the outputs.
- Weekly ritual: Whenever a new model drops or before a real project, runs comparison passes.
- Frustration: No way to fan out one prompt to N models in one command and get back a side-by-side report with cost + duration + resource URLs.

## Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Cost preview before submit | `openart cost estimate --model seedance2 --duration 10 --count 4` | 8/10 | Reads local `models` table (credits_per_unit_default x duration x count); falls back to `/suite/api/topaz/estimate`. | Persona 2; User Vision: "credit-aware". Browser-sniff confirms `topaz/estimate`. |
| 2 | Credit burn breakdown | `openart credits burn --since 7d --by model` | 9/10 | Pure local SQL over `credits_ledger` synced from `/suite/api/credits/logs`, grouped by model/tool/day. UI has no aggregation. | Persona 2; browser-sniff confirms ledger shape. |
| 3 | Prompt FTS recovery search | `openart prompts find "molten dragon" --model seedance2 --has-audio --since 30d` | 8/10 | FTS5 over `media_fts(prompt, label, model, project_name)` joined to metadata. OpenArt-specific filter set. | Persona 3 weekly ritual; UI has no prompt search. |
| 4 | Replay a past generation, optionally on a different model | `openart prompts replay <resourceId> [--model NEW] [--bump duration=10]` | 7/10 | Looks up `media.input` JSON, remaps params across model schemas via `models` table, re-issues submit. | Personas 3 + 4. No competitor does cross-model remap. |
| 5 | Cross-model fan-out compare | `openart compare --prompt "..." --models seedance2,kling2-6,grok-imagine --duration 5` | 8/10 | Submits N parallel forms, polls all, joins results with `models.credits_per_unit_default` for a markdown report. | Persona 4 ritual. NOI named in brief. |
| 6 | Credit runway forecast | `openart credits forecast` | 7/10 | Joins rolling 4-week burn from `credits_ledger` against current `free_credit_balance`. | Persona 2; UI shows neither rate nor projection. |
| 7 | Cheapest-model lookup for a job shape | `openart models cheapest --type video --duration 10 --resolution 720p` | 7/10 | Pure local query against `models` table by capability, ranked by `credits_per_unit_default`. | Personas 2/4; brief lists 4-5 video models with 5x cost spread. |
| 8 | Library stats blob | `openart media stats` | 6/10 | One SQL query: counts per type/model/period + top-spending prompts + rolling totals. | Personas 2/3; pure local aggregation. |
| 9 | Prompt spend leaderboard | `openart prompts top --since 30d --by spend` | 6/10 | Joins `media.prompt` x `credits_ledger.amount` via resource id, groups by prompt-hash, ranks by total cost. | Personas 2/3; OpenArt UI never aggregates per-prompt across jobs. |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `openart video gen --notify` | Notification is a 5-line shell-out, not a feature. Fold into absorbed `--wait` UX. | absorbed `gen --wait` |
| `openart batch run prompts.txt` | A generic batch runner is a shell loop over absorbed `gen`. | survivor 5 (`compare`) — has the cross-model angle that justifies a top-level slot. |
| `openart watch <resourceId>` | Functionally identical to absorbed `gen --wait` once you have the resourceId. | absorbed `gen --wait` |
| `openart character use <name>` | Thin wrapper over submit + table lookup. The Cobra-walked auto MCP tool already serves agents. | absorbed `forms submit` |
| `openart download sync --since 7d` | `find | xargs download` doesn't earn its own line. | survivor 3 (`prompts find`) + absorbed `download` |
| `openart job kill --stale 30m` | OpenArt's API doesn't expose cancel; would only mutate local store. Low value. | (none) |
| `openart prompts diff A B` | Investigative one-time use, not weekly. | survivor 5 (`compare`) |
