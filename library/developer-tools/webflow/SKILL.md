---
name: pp-webflow
description: "The whole Webflow Data API, plus a local database that lets you audit SEO, diff staging against live, and preview a publish before you trigger it. Trigger phrases: `audit the SEO on my Webflow site`, `what would change if I publish this Webflow site`, `which Webflow CMS items are not published yet`, `bulk update my Webflow collection items`, `check my Webflow redirects`, `list my Webflow pages`, `use webflow`, `run webflow`."
author: "Kerry Morrison"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - webflow-pp-cli
    install:
      - kind: go
        bins: [webflow-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/cmd/webflow-pp-cli
---

# Webflow — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `webflow-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install webflow --cli-only
   ```
2. Verify: `webflow-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/cmd/webflow-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Webflow's own CLI stops at sites, CMS, forms, and assets. Every page-level operation you actually need has an endpoint and no command: SEO metadata, page copy, redirects, robots.txt, custom code. This CLI covers all 117 Data API operations and adds a local SQLite mirror on top. The seven audit commands read that mirror rather than the API, so run `sync --full` once before `seo audit`, `drift`, `publish preview`, `overview`, `redirects audit`, `collections completeness`, or `items bulk-set`; without it they return empty.

## When to Use This CLI

Reach for this CLI when the task involves a Webflow site's structure or content rather than its visual design. It is the right tool for auditing SEO metadata across many pages, comparing staged CMS content against what is live, previewing or triggering a publish, bulk-editing collection items, managing redirects and robots.txt, and reading or rewriting page copy through the page DOM. It is also the right tool whenever the question spans more than one site. Note the split: the generated endpoint commands call the API directly, while the seven audit commands (`seo audit`, `drift`, `publish preview`, `overview`, `redirects audit`, `collections completeness`, `items bulk-set`) read the local mirror and need `sync --full` to have run first.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to change a site's visual design, styles, or layout. Those live in the Webflow Designer and are not exposed by the Data API.
- Do not use this CLI to build or deploy Webflow Cloud apps, DevLink component libraries, or Designer Extensions. The official `@webflow/webflow-cli` owns those workflows.
- Do not use this CLI to create or move elements on a page canvas. Only text content through the page DOM endpoints is reachable; structural editing requires the Designer app bridge.
- Do not use this CLI to crawl or audit the rendered public website. It reads Webflow's stored configuration, not the HTML a browser receives.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`seo audit`** — Find every page on a site with a missing, duplicated, or over-length SEO title or description before it costs you traffic.

  _Reach for this instead of paging the pages endpoint yourself when you need the site-wide metadata problems ranked, not the raw page objects._

  ```bash
  webflow-pp-cli seo audit 580e63e98c9a982ac9b8b741 --agent
  ```
- **`drift`** — See exactly which CMS items were edited in staging but never published, field by field.

  _Reach for this before any publish when you need to know what is actually about to go live rather than trusting the staging view._

  ```bash
  webflow-pp-cli drift 580e63fc8c9a982ac9b8b745 --agent
  ```
- **`overview`** — One row per site you have synced locally: pages, collections, unpublished items, SEO findings, days since last publish.

  _Reach for this when the question starts with 'across all my sites'; every other tool forces you to ask it one site at a time. Covers the sites present in the local mirror, so sync every site you care about first._

  ```bash
  webflow-pp-cli overview --agent --select sites.displayName,sites.unpublishedItems,sites.daysSincePublish
  ```

### Publish safety
- **`publish preview`** — See everything that would change if you published this site right now: pages edited since the last publish, draft pages, and unpublished CMS items, alongside the site's redirect-rule count.

  _Reach for this as the pre-flight check before calling the publish endpoint, especially in CI where a surprise publish is expensive to undo._

  ```bash
  webflow-pp-cli publish preview 580e63e98c9a982ac9b8b741 --agent
  ```
- **`redirects audit`** — Catch redirects that shadow a live page, point nowhere, loop, or duplicate another rule.

  _Reach for this during launch hygiene; the redirect endpoints let you read and write rules but never tell you which ones are wrong._

  ```bash
  webflow-pp-cli redirects audit 580e63e98c9a982ac9b8b741 --agent
  ```

### Bulk content work
- **`items bulk-set`** — Apply the same field value to many CMS items selected by a condition. Previews the change set by default; --apply writes it, paced so you never hit a rate limit mid-run.

  _Reach for this instead of looping the update endpoint yourself; it previews the change set first and survives a 60-request-per-minute ceiling._

  ```bash
  webflow-pp-cli items bulk-set 580e63fc8c9a982ac9b8b745 --match status=draft --set author=editorial --agent
  ```
- **`collections completeness`** — Per-field fill rate across a whole collection, so you can spot required fields nobody filled and schema fields nobody uses.

  _Reach for this before a content batch goes live when you need to know which fields are systematically empty rather than checking items one at a time._

  ```bash
  webflow-pp-cli collections completeness 580e63fc8c9a982ac9b8b745 --agent
  ```

## Command Reference

**asset-folders** — Manage asset folders

- `webflow-pp-cli asset-folders <asset_folder_id>` — Get details about a specific Asset Folder Required scope | `assets:read`

**assets** — Assets are files that are uploaded to your Webflow account.

- `webflow-pp-cli assets delete` — Delete an Asset Required Scope: `assets: write`
- `webflow-pp-cli assets get` — Get details about an asset Required scope | `assets:read`
- `webflow-pp-cli assets patch` — Update details of an Asset. Required scope | `assets:write`

**collections** — Collections are CMS collections of items.

- `webflow-pp-cli collections delete` — Delete a collection using its ID. Required scope | `cms:write`
- `webflow-pp-cli collections details` — Get the full details of a collection from its ID. Required scope | `cms:read`

**form-submissions** — Manage form submissions

- `webflow-pp-cli form-submissions delete` — Delete a form submission Required scope | `forms:write`
- `webflow-pp-cli form-submissions get` — Get information about a given form submissio. Required scope | `forms:read`
- `webflow-pp-cli form-submissions modify` — Update hidden fields on a form submission Required scope | `forms:write`

**forms** — Forms are forms that are created on your Webflow site.

- `webflow-pp-cli forms <form_id>` — Get information about a given form. Required scope | `forms:read`

**pages** — Pages are the pages in your Webflow site.

- `webflow-pp-cli pages get-metadata` — Get metadata information for a single page. Required scope | `pages:read`
- `webflow-pp-cli pages update-settings` — Update Page-level metadata, including SEO and Open Graph fields. Required scope | `pages:write`

**sites** — Sites are the sites in your Webflow workspace.

- `webflow-pp-cli sites delete` — Delete a site. <Warning title='Enterprise Only'>This endpoint requires an Enterprise workspace.
- `webflow-pp-cli sites get` — Get details of a site. Required scope | `sites:read`
- `webflow-pp-cli sites list` — List of all sites the provided access token is able to access. Required scope | `sites:read`
- `webflow-pp-cli sites update` — Update a site. <Warning title='Enterprise Only'>This endpoint requires an Enterprise workspace.

**token** — Manage token

- `webflow-pp-cli token authorized-by` — Information about the Authorized User Required Scope | `authorized_user:read`
- `webflow-pp-cli token introspect` — Information about the authorization token <Note>Access to this endpoint requires a bearer token from a [Data Client

**webhooks** — Webhooks are the webhooks in your Webflow site.

- `webflow-pp-cli webhooks get` — Get a specific Webhook instance Required scope: `sites:read`
- `webflow-pp-cli webhooks remove` — Remove a Webhook Required scope: `sites:read`

**workspaces** — Manage workspaces



### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
webflow-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Rank SEO problems across a site without a crawler

```bash
webflow-pp-cli seo audit 580e63e98c9a982ac9b8b741 --agent --select findings.pageSlug,findings.issue,findings.severity
```

Reads the local pages mirror and returns only the three fields an agent needs to build a fix list, instead of the full page objects.

### Check what a publish would actually change

```bash
webflow-pp-cli publish preview 580e63e98c9a982ac9b8b741 --json
```

Joins publish timestamps, page updates, unpublished item counts, and pending redirects into one pre-flight answer.

### Find CMS items edited in staging but never published

```bash
webflow-pp-cli drift 580e63fc8c9a982ac9b8b745 --json
```

Diffs the staged and live copies of every item in the collection field by field.

### Retag many items in one paced pass

```bash
webflow-pp-cli items bulk-set 580e63fc8c9a982ac9b8b745 --match category=news --set category=updates
```

Selects matching items from the local mirror and prints the change set. It previews by default; add --apply to execute against the API.

### Search every synced page, item, and submission at once

```bash
webflow-pp-cli search "pricing" --limit 20 --json
```

Full-text search across the local mirror, which no other Webflow tool offers.

## Auth Setup

Webflow accepts two token shapes on the same `Authorization: Bearer` header. A site token is generated per site under Site settings, Apps & Integrations, API access. A workspace token starts with `ws-` and covers every site in the workspace. Set either one as `WEBFLOW_API_TOKEN` in your environment; that is the only way to supply a static token, since this CLI has no set-token command. `login` runs the OAuth2 flow instead, and `auth status` shows what is currently loaded. Scopes matter more than the token shape: a token created without data scopes authenticates fine and then returns `403 OAuthForbidden: You are missing the following scopes` on the first real call. `doctor` tells you whether a token loaded and whether the API is reachable; it does not enumerate scopes. Use `token introspect` for that, and read the 403 body, which names the exact scope you are missing. One endpoint sits outside all of this: `workspaces audit-logs` needs an Enterprise workspace plus a workspace token carrying `workspace_activity:read`, a scope the OAuth2 flow cannot grant at all.

Run `webflow-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  webflow-pp-cli asset-folders mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `WEBFLOW_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WEBFLOW_CONFIG_DIR`, `WEBFLOW_DATA_DIR`, `WEBFLOW_STATE_DIR`, `WEBFLOW_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WEBFLOW_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `webflow-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "webflow": {
        "command": "webflow-pp-mcp",
        "env": {
          "WEBFLOW_HOME": "/srv/webflow"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WEBFLOW_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WEBFLOW_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
webflow-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "webflow-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `webflow-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `webflow-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `webflow-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
webflow-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
webflow-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
webflow-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
webflow-pp-cli playbook amend \
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

`webflow-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `WEBFLOW_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
webflow-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
webflow-pp-cli feedback --stdin < notes.txt
webflow-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WEBFLOW_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WEBFLOW_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
webflow-pp-cli profile save briefing --json
webflow-pp-cli --profile briefing asset-folders mock-value
webflow-pp-cli profile list --json
webflow-pp-cli profile show briefing
webflow-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `webflow-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/cmd/webflow-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add webflow-pp-mcp -- webflow-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which webflow-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   webflow-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `webflow-pp-cli <command> --help`.
