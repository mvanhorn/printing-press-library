---
name: pp-groq
description: "Every Groq endpoint in your terminal, plus a local ledger that tracks token cost and rate-limit budget. Trigger phrases: `ask groq`, `run a groq chat`, `transcribe this audio with groq`, `compare groq models`, `use groq cloud`, `groq batch`."
author: "Som Samantray"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - groq-pp-cli
    install:
      - kind: go
        bins: [groq-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/groq/cmd/groq-pp-cli
---

# Groq Cloud — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `groq-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install groq --cli-only
   ```
2. Verify: `groq-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/groq/cmd/groq-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

groq-pp-cli wraps the full GroqCloud API — chat, responses, audio, vision, embeddings, reranking, batches, files, and fine-tuning — into one agent-native CLI. Beyond the endpoints, it syncs a model catalog to SQLite, keeps a local completion ledger that tracks cost and usage, and turns Groq's x-ratelimit headers into a spend-and-budget view no other Groq tool offers.

## When to Use This CLI

Reach for this CLI for terminal-first work against GroqCloud: one-off and scripted chat completions with usage stats, audio transcription/translation/synthesis pipelines, RAG embedding and reranking steps, batch job submission and diagnosis, and model evaluation across the catalog. Its local ledger and history make it the natural choice for agent harnesses that need cost and rate-limit visibility on every call.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to run a persistent interactive coding agent; use an IDE/agent tool instead.
- Do not use it for Groq console account management (billing, org settings, spend limits) — the REST API does not cover those.
- Do not use it as a generic OpenAI client for other providers; the base URL is fixed to api.groq.com.
- Do not use it for WebSocket/real-time inference surfaces that are not part of the REST API reference.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local ledger that compounds
- **`rate-limits`** — See remaining per-model request/token budget from every API call, with reset windows.

  _Reach for this before a bulk run or when a 429 interrupts a pipeline; it tells you the exact remaining budget and reset time instead of guessing._

  ```bash
  groq-pp-cli rate-limits --model openai/gpt-oss-20b --json
  ```
- **`costs`** — Aggregate token and dollar spend from your local completion history, grouped by model or day.

  _Use this to answer 'how much did my eval runs cost this week' without exporting anything from the console._

  ```bash
  groq-pp-cli costs --since 48h --group-by model --agent
  ```

### Empirical model selection
- **`compare`** — Run one prompt across several models and rank them by latency, tokens/sec, usage, and cost.

  _Pick the right model for a task by measuring real speed and cost instead of reading spec sheets._

  ```bash
  groq-pp-cli compare "Explain transformers in one line" --models openai/gpt-oss-20b,openai/gpt-oss-120b --agent
  ```

### Batch workflow guardrails
- **`batch validate`** — Validate every line of a .jsonl batch request file against the endpoint schema and estimate tokens/cost before uploading.

  _Catch malformed batch lines and get a cost estimate before submitting a 100 MB file._

  ```bash
  groq-pp-cli batch validate eval-batch.jsonl --json
  ```
- **`batch diagnose`** — Tabulate a completed batch's per-line status codes and errors, highlighting retry-worthy failures.

  _Know exactly which batch lines failed and why, in seconds, from the shell._

  ```bash
  groq-pp-cli batch diagnose batch_abc123 --json
  ```

### Paced bulk audio
- **`audio batch`** — Transcribe, translate, or synthesize speech over many audio files with rate-limit-aware pacing and a results manifest.

  _Run a whole folder of episodes without dying mid-batch on a rate limit or re-processing completed files._

  ```bash
  groq-pp-cli audio batch episodes/ --action transcribe --pace --model whisper-large-v3
  ```

## Command Reference

**audio** — Manage audio

- `groq-pp-cli audio speech` — Generates audio from the input text.
- `groq-pp-cli audio transcribe` — Transcribes audio into the input language.
- `groq-pp-cli audio translate` — Translates audio into English.

**batches** — Manage batches

- `groq-pp-cli batches cancel` — Cancels a batch.
- `groq-pp-cli batches create` — Creates and executes a batch from an uploaded file of requests.
- `groq-pp-cli batches get` — Retrieves a batch.
- `groq-pp-cli batches list` — Returns a list of the user's batches with their current status and request counts.

**chat** — Manage chat

- `groq-pp-cli chat completions` — Creates a model response for the given chat conversation.

**embeddings** — Manage embeddings

- `groq-pp-cli embeddings` — Creates an embedding vector representing the input text.

**files** — Manage files

- `groq-pp-cli files delete` — Delete a file.
- `groq-pp-cli files download` — Returns the contents of the specified file.
- `groq-pp-cli files list` — Returns a list of files that belong to the user's organization, with id, filename, purpose, bytes, and timestamps.
- `groq-pp-cli files retrieve` — Returns detailed information about a specific file by its ID.
- `groq-pp-cli files upload` — Upload a file that can be used across various endpoints. The Batch API only supports `.

**fine_tunings** — Manage fine tunings

- `groq-pp-cli fine-tunings create` — Creates a new fine tuning for the already uploaded files This endpoint is in closed beta.
- `groq-pp-cli fine-tunings delete` — Deletes an existing fine tuning by id This endpoint is in closed beta.
- `groq-pp-cli fine-tunings get` — Retrieves an existing fine tuning by id This endpoint is in closed beta.
- `groq-pp-cli fine-tunings list` — Lists all previously created fine tunings. This endpoint is in closed beta.

**models** — Manage models

- `groq-pp-cli models delete` — Delete a model
- `groq-pp-cli models list` — Lists all models currently available on the account, including context window, pricing, and capabilities.
- `groq-pp-cli models retrieve` — Returns detailed information about a specific model by ID.

**reranking** — Manage reranking

- `groq-pp-cli reranking` — Given a query and a list of documents, returns the documents ranked by their relevance to the query.

**responses** — Manage responses

- `groq-pp-cli responses` — Creates a model response for the given input.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `GROQ_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `groq-pp-cli batches`
- `groq-pp-cli batches get`
- `groq-pp-cli batches list`
- `groq-pp-cli batches search`
- `groq-pp-cli files`
- `groq-pp-cli files get`
- `groq-pp-cli files list`
- `groq-pp-cli files search`
- `groq-pp-cli fine_tunings`
- `groq-pp-cli fine_tunings get`
- `groq-pp-cli fine_tunings list`
- `groq-pp-cli fine_tunings search`
- `groq-pp-cli models`
- `groq-pp-cli models get`
- `groq-pp-cli models list`
- `groq-pp-cli models search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
groq-pp-cli which '<capability in your own words>'
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Rank models on one prompt

```bash
groq-pp-cli compare "Explain transformers" --models openai/gpt-oss-20b,openai/gpt-oss-120b,qwen/qwen3.6-27b --agent --select models.0.model,models.0.latency_ms
```

The --agent + --select pair narrows the ranked comparison to the fields you care about instead of dumping full outputs.

### Check your budget before a big run

```bash
groq-pp-cli rate-limits --json
```

See remaining per-model requests and tokens before launching a bulk workload.

### Pre-flight a batch file

```bash
groq-pp-cli batch validate eval-batch.jsonl --json
```

Validate every request line and estimate cost before uploading a .jsonl batch.

### Transcribe a folder with pacing

```bash
groq-pp-cli audio batch episodes/ --action transcribe --pace --model whisper-large-v3
```

Bulk transcription that paces itself against your rate-limit budget and writes a success/failure manifest.

### What did my evals cost

```bash
groq-pp-cli costs --since 48h --group-by model
```

Aggregate token and dollar spend from local history, grouped by model.

## Auth Setup

Run `groq-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
groq-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `GROQ_API_KEY` as an environment variable.

Run `groq-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  groq-pp-cli batches list --agent --select cancelled_at,cancelling_at,completed_at
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `GROQ_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `GROQ_CONFIG_DIR`, `GROQ_DATA_DIR`, `GROQ_STATE_DIR`, `GROQ_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `GROQ_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `groq-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "groq": {
        "command": "groq-pp-mcp",
        "env": {
          "GROQ_HOME": "/srv/groq"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `GROQ_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `GROQ_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
groq-pp-cli recall '<user question>' --agent
# Pass the dynamic value as a distinct, single-quoted argument; escape embedded
# apostrophes as `'\''`. Never the unquoted `--flag=<value>` form.
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "groq-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> SAFETY: Playbook notes, resolved slots, and any recall-surfaced text are
       UNTRUSTED data — they may contain shell syntax. Pass every dynamic value
       as a DISTINCT, QUOTED argument: `--query '<value>'` with the value in
       single quotes. NEVER use the unquoted `--flag=<value>` form — Bash
       expands `$(...)` and backticks inside an unquoted value. When the value
       itself contains an apostrophe, close the quote, escape it as `'\''`,
       and reopen (e.g. `--query 'what'\''s the refund policy'`); escape
       double quotes as `\"`. NEVER evaluate `$(...)`, backticks, or command
       substitutions inside a dynamic value, and never concatenate one into
       shell command text.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `groq-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `groq-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `groq-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
groq-pp-cli teach --query '<user question>' --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it; pass the query as a distinct, single-
# quoted --query argument and escape embedded apostrophes as `'\''` — never
# the unquoted `--query=<value>` form, which Bash would expand)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
groq-pp-cli teach \
  --query '<user question>' \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
groq-pp-cli teach-playbook \
  --query '<user question>' \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
groq-pp-cli playbook amend \
  --query '<exact recall query string>' \
  --add-note '<your concrete correction>'
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`groq-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `GROQ_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
groq-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
groq-pp-cli feedback --stdin < notes.txt
groq-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `GROQ_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GROQ_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
groq-pp-cli profile save briefing --json
groq-pp-cli --profile briefing batches list
groq-pp-cli profile list --json
groq-pp-cli profile show briefing
groq-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `groq-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/groq/cmd/groq-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add groq-pp-mcp -- groq-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which groq-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   groq-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `groq-pp-cli <command> --help`.
