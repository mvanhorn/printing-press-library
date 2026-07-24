---
name: pp-lichess
description: "Printing Press CLI for Lichess. Welcome to the reference for the Lichess API!"
author: "avanderheyde"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - lichess-pp-cli
    install:
      - kind: go
        bins: [lichess-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lichess/cmd/lichess-pp-cli
---

# Lichess — Printing Press CLI

## Safety scope

Use this CLI only for profile lookup, bounded completed-game review, puzzle
practice, and explicit named-player challenges. It has no Board/Bot move,
live-game analysis, local engine, messaging, or mass puzzle-enumeration
commands. Do not use it for assistance during an ongoing game.

## Prerequisites: Install the CLI

This skill drives the `lichess-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install lichess --cli-only
   ```
2. Verify: `lichess-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lichess/cmd/lichess-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

# Introduction
Welcome to the reference for the Lichess API! Lichess is free/libre,
open-source chess server powered by volunteers and donations.
- Get help in the [Lichess Discord channel](https://discord.gg/lichess)
- API demo app with OAuth2 login and gameplay: [source](https://github.com/lichess-org/api-demo) / [demo](https://lichess-org.github.io/api-demo/)
- API UI app with OAuth2 login and endpoint forms: [source](https://github.com/lichess-org/api-ui) / [website](https://lichess.org/api/ui)
- [Contribute to this documentation on Github](https://github.com/lichess-org/api)
- Check out [Lichess widgets to embed in your website](https://lichess.org/developers)
- [Download all Lichess rated games](https://database.lichess.org/)
- [Download all Lichess puzzles with themes, ratings and votes](https://database.lichess.org/#puzzles)
- [Download all evaluated positions](https://database.lichess.org/#evals)

## Endpoint
All requests go to `https://lichess.org` (unless otherwise specified).

## Clients
- [Python general API](https://github.com/lichess-org/berserk)
- [MicroPython general API](https://github.com/mkomon/uberserk)
- [Python general API - async](https://pypi.org/project/async-lichess-sdk)
- [Python Lichess Bot](https://github.com/lichess-bot-devs/lichess-bot)
- [Python Board API for Certabo](https://github.com/haklein/certabo-lichess)
- [Java general API](https://github.com/tors42/chariot)
- [JavaScript & TypeScript general API](https://github.com/devjiwonchoi/equine)
- [Rust general API](https://github.com/obazin/litchee)
- [LichessNET - C# API Wrapper](https://github.com/Rabergsel/LichessNET)
- [.NET general API](https://github.com/Dblike/LichessSharp)

## Rate limiting
All requests are rate limited using various strategies,
to ensure the API remains responsive for everyone.
Only make one request at a time.
If you receive an HTTP response with a [429 status](https://en.wikipedia.org/wiki/List_of_HTTP_status_codes#429),
you have exceded one of the rate limits.
In most cases, waiting one minute before retrying will be sufficient, but some limits may require longer.
Reduce your request frequency before retrying.

## Streaming with ND-JSON
Some API endpoints stream their responses as [Newline Delimited JSON a.k.a. **nd-json**](https://github.com/ndjson/ndjson-spec), with one JSON object per line.

Here's a [JavaScript utility function](https://gist.github.com/ornicar/a097406810939cf7be1df8ea30e94f3e) to help reading NDJSON streamed responses.

## Authentication
### Which authentication method is right for me?
[Read about the Lichess API authentication methods and code examples](https://github.com/lichess-org/api/blob/master/example/README.md)

### Personal Access Token
Personal API access tokens allow you to quickly interact with Lichess API without going through an OAuth flow.
- [Generate a personal access token](https://lichess.org/account/oauth/token)
- `curl https://lichess.org/api/account -H "Authorization: Bearer {token}"`
- [NodeJS example](https://github.com/lichess-org/api/tree/master/example/oauth-personal-token)

### Token Security
- Keep your tokens secret. Do not share them in public repositories or public forums.
- Your tokens can be used to make your account perform arbitrary actions (within the limits of the tokens' scope). You remain responsible for all activities on your account.
- Do not hardcode tokens in your application's code. Use environment variables or a secure storage and ensure they are not shipped/exposed to users. Be especially careful that they are not included in frontend bundles or apps that are shipped to users.
- If you suspect a token has been compromised, revoke it immediately.

To see your active tokens or revoke them, see [your Personal API access tokens](https://lichess.org/account/oauth/token).

### Authorization Code Flow with PKCE
The authorization code flow with PKCE allows your users to **login with Lichess**.
Lichess supports unregistered and public clients (no client authentication, choose any unique client id).
The only accepted code challenge method is `S256`.
Access tokens are long-lived (expect one year), unless they are revoked.
Refresh tokens are not supported.

See the [documentation for the OAuth endpoints](#tag/OAuth) or
the [PKCE RFC](https://datatracker.ietf.org/doc/html/rfc7636#section-4) for a precise protocol description.

- [Demo app](https://lichess-org.github.io/api-demo/)
- [Minimal client-side example](https://github.com/lichess-org/api/tree/master/example/oauth-app)
- [Flask/Python example](https://github.com/lakinwecker/lichess-oauth-flask)
- [Java example](https://github.com/tors42/lichess-oauth-pkce-app)
- [NodeJS Passport strategy to login with Lichess OAuth2](https://www.npmjs.com/package/passport-lichess)

#### Real life examples
- [PyChess](https://github.com/gbtami/pychess-variants) ([source code](https://github.com/gbtami/pychess-variants))
- [Lichess4545](https://www.lichess4545.com/) ([source code](https://github.com/cyanfish/heltour))
- [English Chess Federation](https://ecf.octoknight.com/)
- [Rotherham Online Chess](https://rotherhamonlinechess.azurewebsites.net/tournaments)

### Token format
Access tokens and authorization codes match `^[A-Za-z0-9_]+$`.
The length of tokens can be increased without notice. Make sure your application can handle at least 512 characters.
By convention tokens have a recognizable prefix, but do not rely on this.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`challenge ten`** — Create an explicit 10+0 Lichess challenge for a named player only after --send is supplied.
- **`loss-patterns`** — Mechanically group official completed-game judgments by opening, performance, and phase.
- **`training-brief`** — Combine official puzzle-theme performance with local loss-pattern evidence while preserving the absence of a causal mapping.

## Recipes

### 

```bash
lichess-pp-cli training-brief --dry-run --json
```

## Command Reference

**account** — Read and write account information and preferences.
<https://lichess.org/account/preferences/game-display>

- `lichess-pp-cli account` — Public information about the logged in user.

**challenge** — Create only an explicit named-player 10+0 challenge. It never accepts, plays, or analyzes a live game.

- `lichess-pp-cli challenge ten <username> --send` — Sends one 10+0 challenge only after explicit confirmation. Omit `--send` to inspect the request plan.

For bounded retrospective review, use `lichess-pp-cli loss-patterns <username> --max 30`. It only reads completed games and Lichess-provided judgments.

**player** — Manage player

- `lichess-pp-cli player` — Provides autocompletion options for an incomplete username.

**puzzle** — Fetch and solve [puzzles](https://lichess.org/training), view your puzzle history and dashboard.

Our collection of puzzles is in the public domain, you can [download it here](https://database.lichess.org/#puzzles).
For a list of our [puzzle themes](https://lichess.org/training/themes) with their description, check out the [theme translation file](https://github.com/ornicar/lila/blob/master/translation/source/puzzleTheme.xml).

- `lichess-pp-cli puzzle dashboard` — Download your [puzzle dashboard](https://lichess.org/training/dashboard/30/dashboard) as JSON.
- `lichess-pp-cli puzzle next --angle <theme>` — Fetch exactly one official puzzle for the selected training theme.
Use `lichess-pp-cli training-brief` for one theme-guided practice follow-up, then run `lichess-pp-cli puzzle next --angle <theme>`. The angle is required and the command fetches exactly one puzzle; it has no paging, batching, or enumeration mode.

**user** — Access registered users on Lichess.
<https://lichess.org/player>

- Each user blog exposes an atom (RSS) feed, like <https://lichess.org/@/thibault/blog.atom>
- User blogs mashup feed: https://lichess.org/blog/community.atom
- User blogs mashup feed for a language: https://lichess.org/blog/community/fr.atom

- `lichess-pp-cli user <username>` — Read public data of a user.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
lichess-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `lichess-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then provide it for the current shell:

```bash
export LICHESS_OAUTH2="YOUR_TOKEN_HERE"
```

Or set `LICHESS_OAUTH2` as an environment variable.

Run `lichess-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  lichess-pp-cli account --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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

- Use `--home <dir>` for one invocation, or set `LICHESS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `LICHESS_CONFIG_DIR`, `LICHESS_DATA_DIR`, `LICHESS_STATE_DIR`, `LICHESS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `LICHESS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `lichess-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "lichess": {
        "command": "lichess-pp-mcp",
        "env": {
          "LICHESS_HOME": "/srv/lichess"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `LICHESS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `LICHESS_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
lichess-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "lichess-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `lichess-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `lichess-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
lichess-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
lichess-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
lichess-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
lichess-pp-cli playbook amend \
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

`lichess-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `LICHESS_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
lichess-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
lichess-pp-cli feedback --stdin < notes.txt
lichess-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `LICHESS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LICHESS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
lichess-pp-cli profile save briefing --json
lichess-pp-cli --profile briefing account
lichess-pp-cli profile list --json
lichess-pp-cli profile show briefing
lichess-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `lichess-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lichess/cmd/lichess-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add lichess-pp-mcp -- lichess-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which lichess-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   lichess-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `lichess-pp-cli <command> --help`.
