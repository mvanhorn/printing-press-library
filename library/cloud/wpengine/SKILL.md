---
name: pp-wpengine
description: "Every WP Engine Hosting Platform API operation, plus a local fleet mirror that answers cross-account audit questions the portal cannot. Trigger phrases: `purge the WP Engine cache`, `backup my WP Engine install before deploy`, `which certs expire this month on WP Engine`, `which installs are on old PHP`, `audit my WP Engine fleet`, `use wpengine`, `run wpengine`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - wpengine-pp-cli
    install:
      - kind: go
        bins: [wpengine-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/wpengine/cmd/wpengine-pp-cli
---

# WP Engine — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `wpengine-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install wpengine --cli-only
   ```
2. Verify: `wpengine-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/wpengine/cmd/wpengine-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Manage sites, installs, domains, SSL, backups, and cache across every WP Engine account from one terminal. Sync the whole fleet into local SQLite, then run audits no API call can answer: cert expiry ('audit certs'), backup staleness ('audit backups'), PHP version drift ('audit versions'), and domain health ('audit domains'). 'audit usage' projects month-end overages from live plan limits, and 'guard' turns backup-then-deploy-then-purge into one CI-safe command.

## When to Use This CLI

Use this CLI for WP Engine hosting platform operations: inventorying accounts, sites, and installs; gating deploys on completed backups; purging caches; managing domains, TXT verification, and SSL certificates; and auditing the fleet for cert expiry, backup staleness, version drift, and usage overages. It shines for agencies managing many installs across multiple accounts, where every portal answer is one-install-at-a-time.

## Anti-triggers

Do not use this CLI for:
- Managing WordPress content (posts, pages, plugins, themes) — use WP-CLI over SSH or the WordPress REST API; this CLI manages the hosting platform, not WordPress itself
- WP Engine Smart Search AI / content MCP features — that is a separate WP Engine product with its own API
- Billing invoices, support tickets, or portal-only account settings — not exposed by the Hosting Platform API
- Downloading a database dump directly — request a backup and create an archive instead (see the installs backups and installs archives commands)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Fleet audits from local state
- **`audit certs`** — See every SSL certificate across your whole fleet sorted by days to expiry, before a client site goes insecure.

  _Reach for this when asked about certificate expiry anywhere in the fleet instead of walking installs one API call at a time._

  ```bash
  wpengine-pp-cli audit certs --expiring 30d --agent
  ```
- **`audit backups`** — Find production installs whose latest completed backup is older than a threshold you set.

  _Answers 'which prod installs are unprotected right now' in one call instead of N backup-list calls._

  ```bash
  wpengine-pp-cli audit backups --stale 7d --env production --agent
  ```
- **`audit versions`** — Fleet-wide PHP and WordPress version distribution with outlier filters, plus per-site environment drift detection.

  _Use it when asked which installs run outdated PHP/WordPress or whether staging matches production._

  ```bash
  wpengine-pp-cli audit versions --php-below 8.2 --agent
  ```
- **`audit domains`** — One list of everything blocked across launches: unverified domains, domains without cert coverage, dangling redirects.

  _Use it to find every domain still blocking a launch instead of re-checking installs one by one._

  ```bash
  wpengine-pp-cli audit domains --agent
  ```
- **`whois`** — Resolve any domain name to the install, site, and account that serve it, plus cert and redirect status.

  _Use it first when a support ticket names a domain and you need to know which install and account own it._

  ```bash
  wpengine-pp-cli whois clientsite.com --agent
  ```

### Deploy plumbing
- **`guard`** — Gate a deploy: create a checkpoint backup, wait until it completes, then optionally purge cache, with CI-friendly exit codes.

  _Use it before any deploy that needs a verified restore point; it blocks until the backup actually completes._

  ```bash
  wpengine-pp-cli guard my-install --purge cdn
  ```

### Live API projections
- **`audit usage`** — Project month-end billable visits and storage against account limits and flag accounts trending over.

  _Use it to warn about overages before they land on an invoice, not after._

  ```bash
  wpengine-pp-cli audit usage --horizon 30d --agent
  ```

## Command Reference

**accounts** — Manage accounts

- `wpengine-pp-cli accounts get` — Returns a single Account
- `wpengine-pp-cli accounts list` — Use this to list your WP Engine accounts.

**install-copy** — Manage install copy

- `wpengine-pp-cli install-copy` — Copy the full file system and database from one WordPress installation to another

**installs** — Manage installs

- `wpengine-pp-cli installs create` — Create a new WordPress installation
- `wpengine-pp-cli installs delete` — This will delete the install, The delete is permanent and there is no confirmation prompt.
- `wpengine-pp-cli installs get` — Returns a single Install
- `wpengine-pp-cli installs list` — List your WordPress installations
- `wpengine-pp-cli installs update` — Update a WordPress installation

**site-reports** — Manage site reports

- `wpengine-pp-cli site-reports create-schedule` — Creates a scheduled report for a site.
- `wpengine-pp-cli site-reports delete-schedule` — Deletes a report schedule by its ID.
- `wpengine-pp-cli site-reports get` — Returns a list of generated site reports for a particular site.
- `wpengine-pp-cli site-reports get-schedules` — Reports can be scheduled to run monthly, weekly, or every two weeks.
- `wpengine-pp-cli site-reports get-sections` — Returns the available sections for a site report
- `wpengine-pp-cli site-reports list-templates` — Returns a list of report templates available for Site Reports.
- `wpengine-pp-cli site-reports update-schedule` — Updates an existing report schedule.

**sites** — Manage sites

- `wpengine-pp-cli sites create` — Create a new site
- `wpengine-pp-cli sites delete` — This will delete the site and any installs associated with this site.
- `wpengine-pp-cli sites get` — Returns a single site
- `wpengine-pp-cli sites list` — List your sites
- `wpengine-pp-cli sites update` — Change a site name

**ssh-keys** — Manage ssh keys

- `wpengine-pp-cli ssh-keys create` — Use this to add a new SSH key to WP Engine.
- `wpengine-pp-cli ssh-keys delete` — This will delete the SSH key.
- `wpengine-pp-cli ssh-keys list` — Use this to list the SSH keys that you've added to WP Engine.

**status** — Manage status

- `wpengine-pp-cli status` — This endpoint will report the system status and any outages that might be occurring.

**swagger** — Manage swagger

- `wpengine-pp-cli swagger` — This will output the current swagger specification

**user** — Manage user

- `wpengine-pp-cli user` — Returns the currently authenticated user


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
wpengine-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match in plain mode — under `--json`/`--agent` a no-match exits 0 with an empty `matches` array. Fall back to `--help` or use a narrower query.

## Recipes

### Monday fleet audit

```bash
wpengine-pp-cli sync --full && wpengine-pp-cli audit certs --expiring 30d && wpengine-pp-cli audit backups --stale 7d --env production
```

Refresh the mirror, then surface expiring certs and unprotected production installs in one pass.

### Gate a deploy on a verified backup

```bash
wpengine-pp-cli guard my-install --purge cdn
```

Creates a checkpoint backup, blocks until it reaches completed, then purges the CDN cache — exit code tells CI whether to proceed.

### Support-ticket triage by domain

```bash
wpengine-pp-cli whois clientsite.com --agent --select install,account,environment,cert_status
```

One local join resolves a domain to the install, account, environment, and cert status that serve it.

### Who is trending over their plan

```bash
wpengine-pp-cli audit usage --horizon 30d --agent
```

Projects month-end billable visits and bandwidth against plan limits from live month-to-date usage — no sync needed.

### Fleet PHP distribution in one query

```bash
wpengine-pp-cli analytics --type installs --group-by php_version
```

The whole fleet is local SQLite — group any synced resource by any field without an API call.

## Auth Setup

WP Engine uses HTTP Basic auth with an API User ID and Password generated at my.wpengine.com/api_access (enable API access on your account first). Set WP_ENGINE_API_USERNAME and WP_ENGINE_API_PASSWORD in your environment — the same variables the community SDKs use. Rate limits are undocumented server-side; the client retries 429s with backoff automatically.

Run `wpengine-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  wpengine-pp-cli accounts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
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

- Use `--home <dir>` for one invocation, or set `WPENGINE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WPENGINE_CONFIG_DIR`, `WPENGINE_DATA_DIR`, `WPENGINE_STATE_DIR`, `WPENGINE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WPENGINE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `wpengine-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "wpengine": {
        "command": "wpengine-pp-mcp",
        "env": {
          "WPENGINE_HOME": "/srv/wpengine"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WPENGINE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WPENGINE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
wpengine-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "wpengine-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `wpengine-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `wpengine-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `wpengine-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
wpengine-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
wpengine-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
wpengine-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
wpengine-pp-cli playbook amend \
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

`wpengine-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `WPENGINE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
wpengine-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
wpengine-pp-cli feedback --stdin < notes.txt
wpengine-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WPENGINE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WPENGINE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
wpengine-pp-cli profile save briefing --json
wpengine-pp-cli --profile briefing accounts list
wpengine-pp-cli profile list --json
wpengine-pp-cli profile show briefing
wpengine-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `wpengine-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/wpengine/cmd/wpengine-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add wpengine-pp-mcp -- wpengine-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which wpengine-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   wpengine-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `wpengine-pp-cli <command> --help`.

## Works with pp-woocommerce (same machine)

The commerce layer of WP Engine-hosted stores is owned by the sibling CLI `woocommerce-pp-cli` (in PATH). Split the work by layer:

- Store commerce (orders, products, refunds, revenue, stock): use `woocommerce-pp-cli` — this CLI has no visibility into store data.
- Hosting for a store: `wpengine-pp-cli whois <store-domain>` resolves which install/account serves it.
- After Woo catalog/price changes on a hosted store: `wpengine-pp-cli installs purge-cache create <install_id> --type page` so the storefront stops serving stale pages.
- Before bulk Woo mutations: `wpengine-pp-cli guard <install>` gates on a completed checkpoint backup.

Full layer map + inter-CLI choreographies: `~/docs/runbooks/pp-web-stack.md`.
