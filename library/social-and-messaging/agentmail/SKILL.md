---
name: pp-agentmail
description: "AgentMail operations with local memory, safe sends, and fleet-wide insight. Trigger phrases: `check my AgentMail inboxes`, `search AgentMail messages`, `review a draft before sending`, `find unresolved AgentMail follow-ups`, `audit scheduled AgentMail sends`, `use AgentMail`, `run AgentMail`."
author: "Som Samantray"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - agentmail-pp-cli
    install:
      - kind: go
        bins: [agentmail-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/agentmail/cmd/agentmail-pp-cli
---

# AgentMail — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `agentmail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install agentmail --cli-only
   ```
2. Verify: `agentmail-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/agentmail/cmd/agentmail-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The CLI covers AgentMail's inbox, message, thread, draft, webhook, domain, list, metric, key, pod, and organization surfaces. It adds local triage queues, pre-send risk checks, conversation rollups, schedule audits, delivery reconciliation, and fleet health so agents can reason across time and resources instead of replaying isolated API calls.

## When to Use This CLI

Use AgentMail when an agent needs to provision inboxes, read or send email, manage conversations, prepare reviewed or scheduled drafts, or operate multiple tenants. Prefer the local operational commands when the decision depends on history across messages, threads, drafts, and fleet resources. Use the hosted AgentMail MCP or SDK directly when you need a resident event stream or application-embedded async control.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI as a general-purpose human email client or interactive inbox UI.
- Do not use it to send real mail without an explicit recipient review and idempotency key.
- Do not use local reports before syncing the relevant resources or treat an empty mirror as proof that the remote API has no data.
- Do not use this CLI when a long-lived WebSocket event consumer must remain embedded inside another process; use the AgentMail SDK or hosted MCP integration.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local operational memory
- **`triage queue`** — Rank unresolved inbound conversations across inboxes with age, direction, labels, and pending drafts.

  _Choose this when an agent needs an actionable unresolved-mail queue instead of raw paginated messages._

  ```bash
  agentmail-pp-cli triage queue --db /tmp/agentmail.db --since 7d --json --agent
  ```
- **`thread rollup`** — Render compact conversation handoff context with participants, counts, latest direction, age, labels, and extracted reply content.

  _Choose this when an agent or human needs conversation context without repeated thread and message fetches._

  ```bash
  agentmail-pp-cli thread rollup thread_demo --db /tmp/agentmail.db --json --agent --select thread_id,latest_direction,message_count,pending_draft
  ```

### Safe automation
- **`send check`** — Review a draft for deterministic recipient, attachment, schedule, duplicate, and idempotency risks before sending.

  _Choose this before releasing a draft when a safe, auditable send decision matters more than simply calling send._

  ```bash
  agentmail-pp-cli send check draft_demo --db /tmp/agentmail.db --json --agent
  ```
- **`schedule audit`** — Find scheduled drafts that are overdue, orphaned, duplicated, or missing review state.

  _Choose this before a scheduled send window when stale or duplicate drafts need deterministic review._

  ```bash
  agentmail-pp-cli schedule audit --db /tmp/agentmail.db --due-within 24h --json --agent
  ```
- **`delivery reconcile`** — Reconcile outbound messages with status, thread placement, timestamps, and later inbound activity.

  _Choose this after a send batch when an agent must identify stale, failed, or unthreaded outcomes._

  ```bash
  agentmail-pp-cli delivery reconcile --db /tmp/agentmail.db --since 7d --json --agent
  ```

### Fleet operations
- **`fleet health`** — Report inbox, domain, webhook, list, metrics, API-key, pod, and organization readiness findings.

  _Choose this for a preflight fleet review before agents depend on multiple inboxes or tenants._

  ```bash
  agentmail-pp-cli fleet health --db /tmp/agentmail.db --json --agent
  ```

## Command Reference

**agent** — Manage agent

- `agentmail-pp-cli agent sign-up` — Create a new agent organization with an inbox and API key. This endpoint is for signing up for the first time.
- `agentmail-pp-cli agent verify` — Verify an agent organization using the 6-digit OTP sent to the human's email during sign-up.

**api-keys** — Manage api keys

- `agentmail-pp-cli api-keys create` — **CLI:** ```bash agentmail api-keys create --name 'My Key' ```
- `agentmail-pp-cli api-keys create-public-key` — Register a public P-256 JWK using an existing AgentMail bearer API key with `api_key_create`.
- `agentmail-pp-cli api-keys delete` — **CLI:** ```bash agentmail api-keys delete --api-key-id <api_key_id> ```
- `agentmail-pp-cli api-keys list` — **CLI:** ```bash agentmail api-keys list ```
- `agentmail-pp-cli api-keys list-public-keys` — List only public-key credentials visible to the bearer caller's scope.
- `agentmail-pp-cli api-keys revoke-all-agent-id-sign-in-keys` — Invalidate every current public-key credential in the caller's organization by advancing its AgentID key generation.
- `agentmail-pp-cli api-keys revoke-public-key` — Permanently revoke one public-key credential. This hard-deletes the credential; repeating the request returns not found.
- `agentmail-pp-cli api-keys update-public-key-name` — Rename the credential. All security-relevant fields are immutable. Requires `api_key_update`.

**domains** — Manage domains

- `agentmail-pp-cli domains create` — **CLI:** ```bash agentmail domains create --domain example.com ```
- `agentmail-pp-cli domains delete` — **CLI:** ```bash agentmail domains delete --domain-id <domain_id> ```
- `agentmail-pp-cli domains get` — **CLI:** ```bash agentmail domains get --domain-id <domain_id> ```
- `agentmail-pp-cli domains list` — **CLI:** ```bash agentmail domains list ```
- `agentmail-pp-cli domains update` — **CLI:** ```bash agentmail domains update --domain-id <domain_id> ```

**drafts** — Manage drafts

- `agentmail-pp-cli drafts get` — **CLI:** ```bash agentmail drafts get --draft-id <draft_id> ```
- `agentmail-pp-cli drafts list` — **CLI:** ```bash agentmail drafts list ```

**inboxes** — Manage inboxes

- `agentmail-pp-cli inboxes create` — **CLI:** ```bash agentmail inboxes create --display-name 'My Agent' --username myagent --domain agentmail.to ```
- `agentmail-pp-cli inboxes delete` — **CLI:** ```bash agentmail inboxes delete --inbox-id <inbox_id> ```
- `agentmail-pp-cli inboxes get` — **CLI:** ```bash agentmail inboxes get --inbox-id <inbox_id> ```
- `agentmail-pp-cli inboxes list` — **CLI:** ```bash agentmail inboxes list ```
- `agentmail-pp-cli inboxes update` — **CLI:** ```bash agentmail inboxes update --inbox-id <inbox_id> --display-name 'Updated Name' ```

**lists** — Manage lists

- `agentmail-pp-cli lists create` — **CLI:** ```bash agentmail lists create --direction <direction> --type <type> --entry user@example.com ```
- `agentmail-pp-cli lists delete` — **CLI:** ```bash agentmail lists delete --direction <direction> --type <type> --entry <entry> ```
- `agentmail-pp-cli lists get` — **CLI:** ```bash agentmail lists get --direction <direction> --type <type> --entry <entry> ```
- `agentmail-pp-cli lists list` — **CLI:** ```bash agentmail lists list --direction <direction> --type <type> ```

**metrics** — Manage metrics

- `agentmail-pp-cli metrics query-events` — Counts of email events (sent, delivered, bounced, etc.) over time for the organization.
- `agentmail-pp-cli metrics query-usage` — Cumulative usage series for the organization.

**organizations** — Manage organizations

- `agentmail-pp-cli organizations` — Returns the organization for the authenticated API key (usage limits, counts, and billing metadata).

**pods** — Manage pods

- `agentmail-pp-cli pods create` — **CLI:** ```bash agentmail pods create --client-id my-pod ```
- `agentmail-pp-cli pods delete` — **CLI:** ```bash agentmail pods delete --pod-id <pod_id> ```
- `agentmail-pp-cli pods get` — **CLI:** ```bash agentmail pods get --pod-id <pod_id> ```
- `agentmail-pp-cli pods list` — **CLI:** ```bash agentmail pods list ```

**reference-auth** — Manage reference auth

- `agentmail-pp-cli reference-auth` — Returns the identity and scope of the authenticated credential.

**threads** — Manage threads

- `agentmail-pp-cli threads delete` — Permanently deletes a thread and all of its messages.
- `agentmail-pp-cli threads get` — **CLI:** ```bash agentmail threads get --thread-id <thread_id> ```
- `agentmail-pp-cli threads list` — Lists threads, most recent first. Pass `senders`, `recipients`, or `subject` to filter by substring.
- `agentmail-pp-cli threads search` — Full-text search across threads in the organization, ranked by relevance.
- `agentmail-pp-cli threads update` — Updates thread labels. Cannot add or remove system labels (sent, received, bounced, etc.).

**webhooks** — Manage webhooks

- `agentmail-pp-cli webhooks create` — **CLI:** ```bash agentmail webhooks create --url https://example.com/webhook --event-type message.received ```
- `agentmail-pp-cli webhooks delete` — **CLI:** ```bash agentmail webhooks delete --webhook-id <webhook_id> ```
- `agentmail-pp-cli webhooks get` — **CLI:** ```bash agentmail webhooks get --webhook-id <webhook_id> ```
- `agentmail-pp-cli webhooks list` — **CLI:** ```bash agentmail webhooks list ```
- `agentmail-pp-cli webhooks update` — Update inbox or pod subscriptions


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
agentmail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find overdue inbound work

```bash
agentmail-pp-cli triage queue --db /tmp/agentmail.db --since 7d --json --agent
```

Produce an action-ranked queue from synchronized inbox, thread, message, label, and draft state.

### Narrow a large search result

```bash
agentmail-pp-cli inboxes messages search inb_demo --query "invoice overdue" --agent --select messages.message_id,messages.subject,messages.from
```

Keep only high-value fields when a relevance-ranked message response is large.

### Review a draft before sending

```bash
agentmail-pp-cli send check draft_demo --db /tmp/agentmail.db --json --agent
```

Expose deterministic recipient, schedule, duplicate, and idempotency risks before an irreversible send.

### Audit scheduled sends

```bash
agentmail-pp-cli schedule audit --db /tmp/agentmail.db --due-within 24h --json
```

Find overdue, orphaned, duplicated, or unreviewed scheduled drafts.

### Reconcile recent delivery

```bash
agentmail-pp-cli delivery reconcile --db /tmp/agentmail.db --since 7d --json --agent
```

Correlate outbound outcomes with later inbound activity and thread placement.

## Auth Setup

Set AGENTMAIL_API_KEY to a bearer token from AgentMail. Configured credentials are never printed; newly created API-key and signup secrets are returned only by the upstream create response and are not persisted in the local mirror. Use drafts, --dry-run, and Idempotency-Key for controlled writes; verify the human OTP during first-time agent signup.

Run `agentmail-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  agentmail-pp-cli api-keys list --agent
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

Parse `.results` for data and `.meta.source` to know whether the response is `live`, `local`, or `dry-run`. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `AGENTMAIL_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `AGENTMAIL_CONFIG_DIR`, `AGENTMAIL_DATA_DIR`, `AGENTMAIL_STATE_DIR`, `AGENTMAIL_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `AGENTMAIL_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `agentmail-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "agentmail": {
        "command": "agentmail-pp-mcp",
        "env": {
          "AGENTMAIL_HOME": "/srv/agentmail"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `AGENTMAIL_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `AGENTMAIL_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
agentmail-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "agentmail-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `agentmail-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `agentmail-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `agentmail-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
agentmail-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
agentmail-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
agentmail-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
agentmail-pp-cli playbook amend \
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

`agentmail-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `AGENTMAIL_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
agentmail-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
agentmail-pp-cli feedback --stdin < notes.txt
agentmail-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `AGENTMAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AGENTMAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
agentmail-pp-cli profile save briefing --json
agentmail-pp-cli --profile briefing api-keys list
agentmail-pp-cli profile list --json
agentmail-pp-cli profile show briefing
agentmail-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `agentmail-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/agentmail/cmd/agentmail-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add agentmail-pp-mcp -- agentmail-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which agentmail-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   agentmail-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `agentmail-pp-cli <command> --help`.
