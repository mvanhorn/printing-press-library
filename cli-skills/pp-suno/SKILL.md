---
name: pp-suno
description: "Every Suno feature, plus a local SQLite library, offline FTS5 search, MCP-native agent surface, and a single-binary... Trigger phrases: `generate a song`, `make me a track`, `suno that`, `extend this song`, `find my synthwave clips`, `how many credits have I burned`, `use suno`, `run suno`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - suno-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/suno/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Suno — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `suno-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install suno --cli-only
   ```
2. Verify: `suno-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Suno workflow lands in a single Go binary: generate from a prompt, extend, concat, remaster, download with embedded lyrics and cover art. On top of that, every clip you generate syncs to a local SQLite store so you can list your library offline, run cross-table SQL queries, build vibe recipes that compound across sessions, and watch your credit burn by tag or persona. Built-in MCP server exposes the whole surface (stdio and HTTP) so agents reach Suno through one orchestration pair instead of 30 raw tools.

## When to Use This CLI

Reach for suno-pp-cli when you need agent-controlled music generation against your real Suno account: producing background music for video, building music-aware workflows, exploring prompt evolutions, analyzing your credit burn by tag, or running an MCP-served music tool that any LLM-driven host can call. Skip it when you only need one-off song generation - the suno.com web UI is faster for single tracks. Skip it when you need to access someone else's Suno library; the CLI authenticates as the logged-in user only.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`vibes`** — Save prompt + tag + persona + model bundles as named recipes and replay them with one-line topic substitution.

  _When an agent needs the same vibe applied to many topics, this beats re-pasting tags every call._

  ```bash
  suno vibes use synthwave-banger debugging-at-3am --json
  ```
- **`burn`** — Cross-table aggregation showing credits spent by tag, persona, model, or hour-of-day over a window.

  _Agents asking "how much did we spend on test runs today?" need this; existing wrappers cannot answer._

  ```bash
  suno burn --by tag --since 30d --json --select tag,credits,count
  ```
- **`persona leaderboard`** — Ranks the user's voice personas by which produced the most-liked, most-played, or most-extended clips.

  _Tells the user which voice has been working for them so they invest credits in winners, not duds._

  ```bash
  suno persona leaderboard --by likes --since 90d --json
  ```
- **`sessions`** — Groups recent generations into ~30-minute-gap sessions and reports per-session credit spend, persona usage, and tag drift.

  _Agents and humans alike can ask "what did I work on last Tuesday afternoon?" against their library._

  ```bash
  suno sessions --today --json
  ```

### Suno-specific dual-variant pattern
- **`generate create`** — Suno returns two clip variants per generation; this auto-ranks them on duration match, lyrics-word-count, and bitrate, downloads only the winner.

  _Half of Suno's credit spend goes to the variant you ignore; agents that pick mechanically reclaim it._

  ```bash
  suno generate create --gpt-description-prompt "30 second piano interlude" --pick best --target-duration 30 --json
  ```
- **`tree`** — Walks parent_id and direct_children recursively to render the ASCII tree of an extend/concat/cover/remaster ancestry.

  _Find the head of a remix chain, see every variant of a song concept in one view._

  ```bash
  suno tree <clip-id>
  ```
- **`generate evolve`** — Takes an existing clip's full parameter bundle and mutates one axis (tag, persona, model) for a focused re-roll.

  _Casey's tweak-and-reroll ritual becomes one command instead of three clicks._

  ```bash
  suno generate evolve 6b055eee-3b1c-4a74-9aa9-1f16c0818fba --mutate tags+1 --tags-add reverb
  ```
- **`generate create`** — Resubmits a prompt until a returned variant lands in the requested duration window or attempts run out.

  _Mechanically achieves what manual re-clicking does in the web app, with budget enforcement._

  ```bash
  suno generate create --gpt-description-prompt "upbeat 30s" --until-duration 30-45 --max-attempts 5 --max-spend 50
  ```

### Reachability mitigation
- **`doctor`** — Beyond standard health checks: fires a zero-credit lyrics-only generation to confirm the live generate path is reachable and not intercepted by CAPTCHA.

  _Agents running scheduled music tasks must distinguish 'auth expired' from 'Suno offline'; this is the only tool that does it._

  ```bash
  suno doctor --probe-generate --json
  ```
- **`budget`** — Sets a daily or monthly credit cap; generate refuses to submit when the projected spend would exceed the cap.

  _Prevents an agent in a runaway loop from burning the user's whole quota._

  ```bash
  suno budget set monthly 1500 && suno generate "..." --max-spend 50
  ```

### Agent-native plumbing
- **`ship`** — One-shot publishing bundle: MP3 with ID3+USLT+SYLT, MP4, cover PNG, LRC subtitle, JSON sidecar of metadata.

  _Content creator prepping a CapCut import needs every artifact at once; one command versus a download chain._

  ```bash
  suno ship 9baa5d3c-02fb-466d-80f9-a4edfc9f0a65 --to ./vid-2026-05-14/
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 27 API entries from 27 total network entries
- Protocols: rest_json (75% confidence)
- Generation hints: requires_protected_client
- Candidate command ideas: create_check — Derived from observed POST /api/c/check traffic.; create_feed — Derived from observed POST /api/feed/v3 traffic.; create_v2_web — Derived from observed POST /api/generate/v2-web/ traffic.; get_attribution — Derived from observed GET /api/clips/{uuid}/attribution traffic.; get_comments — Derived from observed GET /api/gen/{uuid}/comments traffic.; list_badge_count — Derived from observed GET /api/notification/v2/badge-count traffic.; list_direct_children_count — Derived from observed GET /api/clips/direct_children_count traffic.; list_forked_onboarding — Derived from observed GET /api/statsig/experiment/forked-onboarding traffic.
- Caveats: error_status_cluster: Endpoint cluster only observed error HTTP statuses.

## Command Reference

**billing** — Credits, plans, billing info

- `suno-pp-cli billing eligible_discounts` — List discounts the account is eligible for
- `suno-pp-cli billing get` — Account credits, plan tier, renewal date
- `suno-pp-cli billing usage_plan` — Plan comparison table
- `suno-pp-cli billing usage_plan_faq` — Plan FAQ

**clips** — User's generated songs (clips). Each clip is one song variation.

- `suno-pp-cli clips aligned_lyrics` — Word-aligned lyrics with timestamps (LRC-compatible)
- `suno-pp-cli clips attribution` — Get attribution info (who generated, when, lineage)
- `suno-pp-cli clips comments` — Comments on a clip
- `suno-pp-cli clips delete` — Move clips to trash
- `suno-pp-cli clips direct_children_count` — Count of direct child clips (extends/covers)
- `suno-pp-cli clips edit` — Edit clip metadata (title, tags, lyrics)
- `suno-pp-cli clips get` — Get a single clip by ID
- `suno-pp-cli clips list` — List clips in the user's library (paginated feed)
- `suno-pp-cli clips parent` — Get the parent clip (for extends/covers/remixes)
- `suno-pp-cli clips set_visibility` — Set clip visibility (public/private/unlisted)
- `suno-pp-cli clips similar` — Get similar clips by ID

**custom_model** — Custom model training (Suno Pro feature)

- `suno-pp-cli custom_model` — List pending custom-model training jobs

**generate** — Music generation (create new songs, extend, cover, remix)

- `suno-pp-cli generate concat` — Concatenate clip extensions into a single song
- `suno-pp-cli generate create` — Generate a new song from a description or custom lyrics
- `suno-pp-cli generate lyrics` — Generate lyrics from a prompt (free, no credits)
- `suno-pp-cli generate lyrics_status` — Poll lyrics generation status
- `suno-pp-cli generate video_status` — Poll video render status for a clip

**notification** — User notifications

- `suno-pp-cli notification badge_count` — Unread notification count
- `suno-pp-cli notification list` — List notifications

**persona** — Voice personas (saved voice characteristics)

- `suno-pp-cli persona get` — Get persona by ID with linked clips
- `suno-pp-cli persona list` — List user's personas

**project** — Workspaces (default workspace is auto-created)

- `suno-pp-cli project default` — Default workspace details
- `suno-pp-cli project me` — User's project memberships
- `suno-pp-cli project pinned_clips` — Pinned clips in default workspace

**user** — User profile and settings

- `suno-pp-cli user config` — User config (feature flags, plan tier, preferences)
- `suno-pp-cli user personalization` — Personalization settings
- `suno-pp-cli user personalization_memory` — Personalization memory entries


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
suno-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Generate a song and ship it ready for CapCut

```bash
suno generate "upbeat lo-fi for product demo" --pick best --wait --json --select id | xargs -I% suno ship % --to ./demo-bgm/
```

Generates, waits for completion, picks the winning variant, then bundles MP3+MP4+cover+LRC+JSON into a directory ready for video editor import.

### Save and reuse a vibe recipe

```bash
suno vibes save synthwave-banger --tags "synthwave, female vocal, melancholic" --mv chirp-v5 --persona-id <id> && suno vibes use synthwave-banger "another rainy night" --json
```

Persists the recipe to local SQLite; future invocations swap in any topic without remembering the tag list.

### Audit credit spend by tag for the last month

```bash
suno burn --by tag --since 30d --json --select tag,credits,count,first_at,last_at
```

Joins local credits_snapshots against generations; uses dotted-path select to keep the agent's context small.

### Find the lineage of a remix

```bash
suno tree 9baa5d3c-02fb-466d-80f9-a4edfc9f0a65 --json
```

Walks parent_id and direct_children to render the extend/concat/cover ancestry.

### Reroll until a clip fits a TikTok slot

```bash
suno generate "hyperpop bedroom-producer 30-45s" --until-duration 30-45 --max-attempts 5 --max-spend 50 --pick best --json
```

Loops generation until a variant lands in the duration window, capped at 50 credits, picking the winner each time.

## Auth Setup

Suno has no official API. Auth is Clerk: import your `__session` cookie from Chrome with `suno auth login --chrome` and the CLI sends `Authorization: Bearer <__session>` on every request. The CLI refreshes the JWT in the background by exchanging the longer-lived `__client` cookie against `clerk.suno.com`. If your browser session is signed in, the CLI is signed in.

Run `suno-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  suno-pp-cli clips list --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
suno-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
suno-pp-cli feedback --stdin < notes.txt
suno-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.suno-pp-cli/feedback.jsonl`. They are never POSTed unless `SUNO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUNO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
suno-pp-cli profile save briefing --json
suno-pp-cli --profile briefing clips list
suno-pp-cli profile list --json
suno-pp-cli profile show briefing
suno-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `suno-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add suno-pp-mcp -- suno-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which suno-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   suno-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `suno-pp-cli <command> --help`.
