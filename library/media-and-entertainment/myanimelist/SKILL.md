---
name: pp-myanimelist
description: "The entire public MyAnimeList in one binary, plus offline analytics the site itself cannot answer. Trigger phrases: `look up an anime on MyAnimeList`, `what anime airs tonight`, `watch order for this series`, `is this anime worth starting`, `search MyAnimeList for a manga`, `track my anime progress locally`."
author: "Som Samantray"
license: "Apache-2.0"
argument-hint: "anime show 52991 | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - myanimelist-pp-cli
    install:
      - kind: go
        bins: [myanimelist-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/cmd/myanimelist-pp-cli
---

# MyAnimeList — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `myanimelist-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install myanimelist --cli-only
   ```
2. Verify: `myanimelist-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/cmd/myanimelist-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search, inspect, and rank anime and manga from the terminal with no API key, no OAuth, and no browser. Everything you look at is cached in SQLite so it stays searchable offline, and local commands like `drift`, `week`, and `franchise gap` answer questions MyAnimeList has no page for: how a score moved, what airs tonight in your timezone, and which franchise entries you skipped.

## When to Use This CLI

Use this CLI when an agent or a person needs MyAnimeList data without registering an application or holding a token: title lookups, seasonal and ranking exploration, cast and staff research, and analytics over score and status distributions. It is also the right tool for local watch tracking when a user does not want their list tied to an account. It is well suited to scripted seasonal planning, backlog triage, and franchise audits because every read is cached and every command emits JSON.

## Anti-triggers

Do not use this CLI for:
- Do not use it to stream, download, or watch anime; it returns metadata and streaming links only.
- Do not use it to modify a MyAnimeList account: there is no authenticated write path, by design.
- Do not use it as a general anime database across sites; it reads MyAnimeList alone and does not merge AniList, Kitsu, or Shikimori data.
- Do not use it for bulk crawling of the whole catalog; sitemap enumeration is explicit and rate-limited.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Reception analytics the site hides
- **`anime divisive`** — See whether a title is universally loved or bitterly split, instead of reading one average score.

  _Use it before committing to a long series when the average score hides a love-it-or-hate-it split._

  ```bash
  myanimelist-pp-cli anime divisive 5114 --json
  ```
- **`anime drop-risk`** — Judge how likely a title is to be abandoned before you commit dozens of episodes to it.

  _Use it to triage a plan-to-watch backlog by follow-through rather than by average score._

  ```bash
  myanimelist-pp-cli anime drop-risk 21 --json
  ```
- **`anime consistency`** — See whether a show improves, holds, or falls apart across its episodes using per-episode poll data.

  _Use it to decide whether to keep watching mid-season rather than finishing a dud._

  ```bash
  myanimelist-pp-cli anime consistency 52991 --json
  ```

### Local state that compounds
- **`drift`** — See how a title's score, member count, favorites, and rank have moved since you last looked.

  _Reach for it when a question depends on change over time rather than the current number._

  ```bash
  myanimelist-pp-cli drift --since 30d --json
  ```
- **`week`** — Get a timezone-correct seven-day grid of just the shows you track, with same-hour collisions flagged.

  _Use it to plan what to watch tonight without hand-converting Japan time for every show._

  ```bash
  myanimelist-pp-cli week --json
  ```

### Franchise intelligence
- **`adaptation`** — Learn whether an anime covered its source manga and how much source material is left.

  _Use it when a user asks where to pick up the manga after the anime._

  ```bash
  myanimelist-pp-cli adaptation 52991 --json
  ```
- **`franchise gap`** — Find the sequel, movie, or side story you skipped in every franchise you have already started.

  _Use it to catch missing entries before starting a new season of a franchise._

  ```bash
  myanimelist-pp-cli franchise gap --json
  ```

## Command Reference

**anime** — Browse, search, and inspect anime titles

- `myanimelist-pp-cli anime characters` — Characters and their Japanese/English voice actors for an anime
- `myanimelist-pp-cli anime episodes` — Episode table with titles, air dates, poll averages, and reply counts
- `myanimelist-pp-cli anime forum` — Discussion boards attached to an anime title
- `myanimelist-pp-cli anime get` — Fetch one anime title's full detail page by MyAnimeList id
- `myanimelist-pp-cli anime list` — Search anime with the site's full advanced filters (query, format, score, airing status, rating, year range, genre
- `myanimelist-pp-cli anime moreinfo` — Extended credits, external links, and full theme-song credits
- `myanimelist-pp-cli anime pictures` — Image gallery (posters, key visuals, screenshots) for an anime
- `myanimelist-pp-cli anime recommendations` — Community recommendations for an anime, with recommendation counts
- `myanimelist-pp-cli anime reviews` — Community reviews with scores, helpful counts, and spoiler flags
- `myanimelist-pp-cli anime stats` — Score distribution (1-10 with vote counts) and status distribution for an anime
- `` — Streaming episodes and promos for an anime (the site's own video JSON, no auth) with a real pager
- `myanimelist-pp-cli anime videos` — Promotional videos and trailers for an anime

**character** — Characters

- `myanimelist-pp-cli character <id>` — A character's profile with appearances and voice actors

**feed** — RSS feeds published by MyAnimeList

- `` — Featured-article RSS feed (XML)
- `` — News RSS feed (XML)
- `` — A member's public activity feed (needs ?u=; without it the endpoint 404s)

**forum** — Public forum boards and topics

- `myanimelist-pp-cli forum boards` — All forum boards with their current topic counts
- `myanimelist-pp-cli forum topic` — One forum topic and its posts
- `myanimelist-pp-cli forum topics` — Topics in one forum board

**genre** — Browse by genre, theme, or demographic

- `myanimelist-pp-cli genre anime` — Anime in a genre, theme, or demographic
- `myanimelist-pp-cli genre manga` — Manga in a genre, theme, or demographic

**instant** — Instant cross-entity search

- `myanimelist-pp-cli instant` — Instant search across anime, manga, characters, and people (the site's own suggest API, JSON)

**magazine** — Manga serialization magazines

- `myanimelist-pp-cli magazine <id>` — A magazine's page with every serialized title

**manga** — Browse, search, and inspect manga, novels, and one-shots

- `myanimelist-pp-cli manga characters` — Characters credited in a manga
- `myanimelist-pp-cli manga get` — Fetch one manga title's full detail page by MyAnimeList id
- `myanimelist-pp-cli manga list` — Search manga with the site's advanced filters
- `myanimelist-pp-cli manga moreinfo` — Extended credits and external links for a manga
- `myanimelist-pp-cli manga pictures` — Image gallery for a manga
- `myanimelist-pp-cli manga recommendations` — Community recommendations for a manga
- `myanimelist-pp-cli manga reviews` — Community reviews for a manga
- `myanimelist-pp-cli manga stats` — Score distribution and status distribution for a manga

**member** — Public MyAnimeList user profiles and lists

- `myanimelist-pp-cli member anime-list` — A public anime list as JSON rows (status, score, progress, tags); no credential required
- `myanimelist-pp-cli member get` — A public member profile: statistics, favorites, and recent activity (no credential required)
- `myanimelist-pp-cli member manga-list` — A public manga list as JSON rows; no credential required
- `myanimelist-pp-cli member statistics` — A member's public aggregate statistics

**news** — Anime and manga news

- `myanimelist-pp-cli news get` — A single news article, body included
- `myanimelist-pp-cli news list` — Latest news headlines with pagination

**person** — Voice actors and staff

- `myanimelist-pp-cli person <id>` — A person's page: animeography, voice roles, and staff credits

**ranking** — Top anime and manga rankings

- `myanimelist-pp-cli ranking anime` — Top anime by type: all, airing, upcoming, tv, movie, ova, ona, special, bypopularity, favorite
- `myanimelist-pp-cli ranking manga` — Top manga by type: all, manga, novels, lightnovels, oneshots, doujin, manhwa, manhua, bypopularity, favorite

**season** — Seasonal anime charts

- `myanimelist-pp-cli season archive` — Index of every past season
- `myanimelist-pp-cli season current` — The season airing right now
- `myanimelist-pp-cli season later` — Upcoming seasons announced but not yet airing (the `upcoming` path 404s; `later` is the real route)
- `myanimelist-pp-cli season list` — Full seasonal chart for a year and season, grouped by format
- `myanimelist-pp-cli season schedule` — The weekly airing schedule, optionally shifted to a timezone the site understands

**sitemap** — The robots-advertised sitemap, for explicit id enumeration

- `` — Sitemap index listing every catalog shard (XML)
- `` — One sitemap shard, e.g. anime-001, manga-001, character-001, people-001, news-001 (XML)

**studio** — Animation studios and licensors

- `myanimelist-pp-cli studio <id>` — A studio's or licensor's page with its full production list


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `MYANIMELIST_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

Real command paths are discoverable at runtime:
- `myanimelist-pp-cli --help` for the 15 resource groups and the hand-written commands
- `myanimelist-pp-cli <resource> --help` for that group's endpoint commands
- `myanimelist-pp-cli which "<capability>" --json` to map a capability to a command
- `myanimelist-pp-cli agent-context` for the full machine-readable command tree

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
myanimelist-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Narrow a title record for an agent

```bash
myanimelist-pp-cli anime get 52991 --agent --select id,title,score,episodes,status,genres,studios
```

Title pages are large; --select keeps only the fields an agent needs.

### Trim nested instant-search results

```bash
myanimelist-pp-cli instant --query frieren --entity-type anime --agent --select categories.items.name,categories.items.payload.score
```

The suggest response nests categories and payloads; dotted --select flattens the useful fields.

### Is this show split or universally loved?

```bash
myanimelist-pp-cli anime divisive 5114
```

Turns the 1-10 vote distribution into a polarization reading.

### Audit a franchise you started

```bash
myanimelist-pp-cli franchise gap
```

Lists entries missing from franchises already in your local library.

### Plan the week in your own timezone

```bash
myanimelist-pp-cli week
```

Shows only your tracked shows, with same-hour collisions flagged.

## Auth Setup

No credentials, ever. The CLI reads MyAnimeList's public pages, its own suggest endpoint, its RSS feeds, and its advertised sitemap over plain HTTP. There is no API key to set, no OAuth flow to complete, and no session cookie to keep alive. Requests are polite by default: a descriptive User-Agent, one request per second, and a 24-hour cache so repeat reads cost nothing.

Run `myanimelist-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  myanimelist-pp-cli anime show 52991 --agent --select id,title,score,genres
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear

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

- Use `--home <dir>` for one invocation, or set `MYANIMELIST_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `MYANIMELIST_CONFIG_DIR`, `MYANIMELIST_DATA_DIR`, `MYANIMELIST_STATE_DIR`, `MYANIMELIST_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `MYANIMELIST_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `data.db` (the local SQLite mirror and your `track` library). `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files. This CLI needs no credentials, so there is no credential store.
- Run `myanimelist-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "myanimelist": {
        "command": "myanimelist-pp-mcp",
        "env": {
          "MYANIMELIST_HOME": "/srv/myanimelist"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `MYANIMELIST_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `MYANIMELIST_HOME`, or `doctor` will not find your database and library under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
myanimelist-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

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
      "next_action": ["<trial command>", "myanimelist-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `myanimelist-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `myanimelist-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `myanimelist-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
myanimelist-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
myanimelist-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
myanimelist-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
myanimelist-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
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

`myanimelist-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `MYANIMELIST_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```



```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `MYANIMELIST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MYANIMELIST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
myanimelist-pp-cli profile save briefing --json
myanimelist-pp-cli --profile briefing anime list
myanimelist-pp-cli profile list --json
myanimelist-pp-cli profile show briefing
myanimelist-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `myanimelist-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/cmd/myanimelist-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add myanimelist-pp-mcp -- myanimelist-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which myanimelist-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   myanimelist-pp-cli anime show 52991 --agent
   ```
4. If ambiguous, drill into subcommand help: `myanimelist-pp-cli anime --help`.
