---
name: pp-gmail
description: "Mailbox cleanup that can prove itself — preview, confirm, undo, verify — from a binary that structurally cannot send email. Trigger phrases: `clean up my inbox`, `summarize my email`, `who emails me the most`, `unsubscribe me from these`, `what's eating my Gmail storage`, `use gmail-pp-cli`, `run gmail`."
author: "Derik Parkinson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gmail-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/gmail/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Gmail — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gmail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install gmail --cli-only
   ```
2. Verify: `gmail-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/gmail/cmd/gmail-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every read and cleanup surface from the Gmail tool landscape, multi-account, with a local SQLite store underneath: sender intelligence, all-category digests, bulk trash/label with preview-confirm-undo, RFC 8058 one-click unsubscribes with a compliance ledger. Send, drafts, settings, and permanent deletion are absent from the binary by construction — Trash is the ceiling.

## When to Use This CLI

Reach for this CLI when an agent or operator needs to understand or clean a Gmail mailbox safely: summaries across every category, sender and storage intelligence, bulk trash/label with preview and undo, and one-click unsubscribes with follow-up verification. It is the wrong tool for composing or sending anything — by design it cannot.

Do NOT use this CLI for:

- **Composing, sending, replying, forwarding, or drafting email** — no send or draft surface exists in the binary.
- **Permanently deleting messages or emptying Trash** — Trash is the ceiling; the gmail.modify scope cannot permanently delete.
- **Deleting labels** — `labels create` and `labels rename` are the only label writes; there is no `labels delete`, update, or patch.
- **Managing Gmail settings, filters, forwarding, or vacation responders** — settings endpoints do not exist in this binary.
- **Thread-level mutations** — threads are read-only; mutations operate on messages through the cleanup engine.
- **Sending mailto: unsubscribes** — `unsub run` executes RFC 8058 one-click HTTPS POSTs only; mailto-only senders are surfaced as a desk list, never acted on.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cleanup that compounds
- **`unsub verify`** — See which senders kept mailing you after a one-click unsubscribe, with an escalation query per violator.

  _After running unsubscribes, this is the only way to learn which ones actually stuck before escalating to block/filter decisions._

  ```bash
  gmail-pp-cli unsub verify --account ads --agent
  ```
- **`rules run`** — Named local recipes (query plus trash or label action) replayed through the preview-confirm-undo engine as one merged plan.

  _Standing hygiene (old promos to trash, receipts to their folder) becomes one command a scheduled job can run daily, always previewed before applying._

  ```bash
  gmail-pp-cli rules run --plan-only --agent
  ```
- **`sort suggest`** — Senders whose mail you already label consistently, with a generated plan to label the rest the same way.

  _Folder-sorting at scale without guessing: the plan only proposes what the operator's own labeling history already proves._

  ```bash
  gmail-pp-cli sort suggest --account personal --min-confidence 0.8 --agent
  ```

### Local state that answers instantly
- **`delta`** — Everything new since your last check: new messages per category and sender, never-seen senders, volume spikes.

  _The first question of any recurring mailbox check is what changed — this answers it without re-reporting what the operator already saw._

  ```bash
  gmail-pp-cli delta --account personal --agent
  ```
- **`storage report`** — Which senders, labels, years, and attachments own your storage, with ready-to-run cleanup queries per row.

  _Turns a vague quota number into a ranked hit-list whose rows paste straight into cleanup plan._

  ```bash
  gmail-pp-cli storage report --account ads --top 15 --agent
  ```

### Safety made visible
- **`trash report`** — What you trashed, grouped by applied plan, with days remaining before Gmail's 30-day purge makes undo impossible.

  _The last regret-check: surfaces batches whose undo window is closing so recovery happens while it still can._

  ```bash
  gmail-pp-cli trash report --closing-soon --agent
  ```
- **`score`** — Per-account hygiene metrics — unread share, promo share, subscription count, storage headroom — snapshotted over time.

  _Shows whether the cleanup campaign is actually winning — Promotions down 60% since baseline beats a feeling._

  ```bash
  gmail-pp-cli score --account ads --agent
  ```

## Command Reference

Raw modify/trash/delete subcommands do not exist. Every mailbox mutation flows through `cleanup plan` → `cleanup apply` (reversible via `undo`), `labels create`/`labels rename`, or `unsub plan` → `unsub run` — all gated by one-time plan tokens. One-click POSTs additionally require the unsubscribe URL host to share the sender's registrable domain: ESP-hosted (third-party) destinations are listed by `unsub plan` under `third_party_hosts` and are skipped by `unsub run` unless it is invoked with `--allow-third-party`.

**Mailbox engine**

- `gmail-pp-cli accounts` / `accounts auth` — List gauth profiles and token status; run the browser OAuth flow for one profile (or `--all`).
- `gmail-pp-cli sync` — Sync message metadata into the local store (full or historyId-incremental); never mutates the mailbox.
- `gmail-pp-cli digest` — Per-category mailbox summary plus an account rollup.
- `gmail-pp-cli senders` — Rank senders by volume, with size, unread rate, and unsubscribe capability.
- `gmail-pp-cli delta` / `storage report` / `trash report` / `score` / `sort suggest` — The local-intelligence reports described under Unique Capabilities.
- `gmail-pp-cli cleanup plan|apply|recover` — Preview-confirm-undo cleanup: `plan` freezes what would change (mailbox untouched), `apply` executes exactly that, `recover` finishes a crashed apply.
- `gmail-pp-cli undo --ledger <id>` — Reverse a ledgered apply delta-by-delta; changed ids are skipped as conflicts, never forced.
- `gmail-pp-cli rules add|list|rm|run` — Named local cleanup recipes replayed through the engine as one merged plan; `run` always stops at the plan.
- `gmail-pp-cli unsub audit|plan|run|verify` — One-click unsubscribe engine: classify, freeze, POST (RFC 8058, hardened), verify.
- `gmail-pp-cli search` / `analytics` / `tail` / `workflow archive|status` / `api` — Local FTS search and analytics, polling change stream, archive workflows, endpoint browsing.

**API reads (plus the two safe label writes)** — `<userId>` accepts `me`

- `gmail-pp-cli history <userId>` — Lists the history of all changes to the given mailbox.
- `gmail-pp-cli labels list|get` — Read labels.
- `gmail-pp-cli labels create` — Create a label, idempotently by name (an existing case-insensitive match is returned, not duplicated).
- `gmail-pp-cli labels rename` — Rename a label by id, ledgering the inverse for `undo --ledger <id>`. No `labels delete`, update, or patch exists.
- `gmail-pp-cli messages list|get` / `messages attachments get` — Read messages and attachments (reads only).
- `gmail-pp-cli threads list|get` — Read threads (thread mutations do not exist).
- `gmail-pp-cli users-profile <userId>` — Gets the current user's Gmail profile.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gmail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning mailbox delta

```bash
gmail-pp-cli delta --account ads --agent
```

What arrived since the last check, grouped by category and sender, without re-reporting what was already seen.

### Top senders, narrowed for an agent

```bash
gmail-pp-cli senders --account personal --top 25 --agent --select senders.email,senders.count,senders.unread_rate
```

Dotted --select keeps the payload to the three fields a triage decision needs.

### Unsubscribe audit before acting

```bash
gmail-pp-cli unsub audit --account ads --min-count 5 --agent
```

Ranks subscription senders by volume and classifies each as one-click (actionable) or mailto-only (desk list).

### Preview a year-old promotions purge

```bash
gmail-pp-cli cleanup plan --q "category:promotions older_than:1y" --action trash --account personal --agent
```

Counts and samples first; the printed plan token is required by cleanup apply, and every apply is undoable.

### Verify last week's unsubscribes stuck

```bash
gmail-pp-cli unsub verify --account ads --since 7d --agent
```

Joins the unsubscribe ledger to fresh arrivals — violators come back with an escalation query.

## Auth Setup

Installed-app OAuth with named multi-account profiles (per-profile token store, consented-account verification). The only scope ever requested is gmail.modify; send/draft/settings endpoints do not exist in this binary, and permanent deletion is impossible under this scope — Google enforces the Trash ceiling, not us.

Run `gmail-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gmail-pp-cli labels list me --agent --select id,name
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success (`labels create` already matches by name)

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

- Use `--home <dir>` for one invocation, or set `GMAIL_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `GMAIL_CONFIG_DIR`, `GMAIL_DATA_DIR`, `GMAIL_STATE_DIR`, `GMAIL_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `GMAIL_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `gmail-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "gmail": {
        "command": "gmail-pp-mcp",
        "env": {
          "GMAIL_HOME": "/srv/gmail"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `GMAIL_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `GMAIL_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
gmail-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "gmail-pp-cli learnings confirm 12"] }
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
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `gmail-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `gmail-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `gmail-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
gmail-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
gmail-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
gmail-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
gmail-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
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

`gmail-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `GMAIL_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
gmail-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gmail-pp-cli feedback --stdin < notes.txt
gmail-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `GMAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GMAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gmail-pp-cli profile save briefing --account personal --agent
gmail-pp-cli --profile briefing digest
gmail-pp-cli profile list --json
gmail-pp-cli profile show briefing
gmail-pp-cli profile delete briefing --yes
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

The mutation engine (`cleanup apply|recover`, `unsub run`, `undo`) reuses `3` for a partial run and `7` for a busy apply lock, and uses `4` for refusals (identity, plan-sha, token, drift). Each engine command's `--help` documents its typed exits.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `gmail-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gmail-pp-mcp -- gmail-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gmail-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gmail-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gmail-pp-cli <command> --help`.
