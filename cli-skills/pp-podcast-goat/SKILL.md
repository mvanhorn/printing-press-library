---
name: pp-podcast-goat
description: "Pull long-form podcast transcripts as speaker-labeled markdown — cookie-first across the four major paid... Trigger phrases: `pull this podcast transcript`, `get the transcript from this URL`, `summarize this Dwarkesh episode`, `what did Senra say about`, `bundle these episodes for me`, `grep my podcast cache`, `use podcast-goat`, `run podcast-goat`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - podcast-goat-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/podcast-goat/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Podcast GOAT — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `podcast-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install podcast-goat --cli-only
   ```
2. Verify: `podcast-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/cmd/podcast-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for podcast-goat when an agent needs to read or quote from a long-form podcast and you already subscribe to the show. The CLI walks your member cookies first, falls back to free YouTube/Substack/RSS sources, and only spends money on spoken.md or audio transcription when nothing free works. The MCP surface makes it the canonical podcast adapter for Claude Code agents — every command is an MCP tool, every read is annotated `mcp:read-only`.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

### Anti-triggers

Skip podcast-goat for:

- **Audio downloads.** This CLI never writes audio to disk. Use `yt-dlp` or your podcast app directly.
- **Paywall bypass.** Cookie-tier adapters replay your own logged-in session for shows you already subscribe to. Do not route a request through here to read content the user has not paid for.
- **LLM summarization or analysis.** This CLI ships canonical markdown to your agent. The summarization happens in the agent, not here.
- **Real-time live transcription.** Audio-pipeline (`whisperapi`) is a deferred v0.2 path; for live captions reach for a streaming Deepgram client instead.
- **Editing or annotating transcripts in place.** Output is read-only; persist your annotations elsewhere.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-source corpus that compounds
- **`magic`** — Bundle top-N cached transcripts about a topic into one markdown file an agent can summarize in a single call.

  _Replaces the chip-supply-chain.fly.dev copy-paste workflow with one command. Reach for this when the agent needs cross-episode synthesis._

  ```bash
  podcast-goat-pp-cli magic 'AI chip supply chain' --out ~/chip-supply-chain.md
  ```
- **`episode get --explain`** — Dry-run shows which source tier will fire and why earlier tiers were skipped, with projected cost before any paid call.

  _Lets agents preview cost and source attribution before committing. Reach for this before any paid run._

  ```bash
  podcast-goat-pp-cli episode get https://www.hubermanlab.com/episode/example --explain
  ```
- **`episode quote`** — FTS5 phrase search returns the matched segment plus N surrounding segments preserving the canonical speaker shape and deeplink timestamp.

  _Grep your podcast memory in 5 seconds. Reach for this when you remember a half-citation and need the exact line back._

  ```bash
  podcast-goat-pp-cli episode quote 'pricing power' -C 3 --json
  ```
- **`source compare`** — For an episode resolvable on multiple sources, fetch all available adapters and diff segment count, token count, distinct speakers, label confidence.

  _Reveals when free sources are good enough vs when paid is needed. Reach for this before recommending an upstream source._

  ```bash
  podcast-goat-pp-cli source compare https://www.acquired.fm/episodes/vanguard --json
  ```
- **`speakers list`** — Aggregate speaker names across the cached corpus with episode counts, optionally filtered by show.

  _Answers 'what do I have on Senra/Buffett/Karpathy'. Reach for this when building a synthesis prompt._

  ```bash
  podcast-goat-pp-cli speakers list --show acquired --json
  ```

### Multilingual reach
- **`episode get --bilingual`** — yt-dlp dual-language auto-subs, greedy nearest-neighbor alignment, emits one markdown file with paired Chinese + auto-translated English per turn.

  _Makes Mandarin-only podcasts (e.g., Xiaojun) usable for English-reading agents in one step._

  ```bash
  podcast-goat-pp-cli episode get 'https://www.youtube.com/watch?v=EXAMPLE' --bilingual zh-Hans,en
  ```

### Agent-native plumbing
- **`auth services`** — One-row-per-service table of cookie age, expiry, last-fetch result, with remediation hint when stale.

  _Cookies decay silently. Reach for this before a batch run to confirm member access still works._

  ```bash
  podcast-goat-pp-cli auth services --json
  ```
- **`budget show --by-show`** — Pivot spend.jsonl joined to episodes by URL; group by show, provider, month to attribute cost.

  _Shows which subscriptions are paying off and which shows still cost money. Reach for this monthly._

  ```bash
  podcast-goat-pp-cli budget show --by-show --since 30 --json
  ```

## Command Reference

**episode** — Pull, search, and inspect podcast episode transcripts

- `podcast-goat-pp-cli episode get` — Fetch one transcript by URL via the cookie -> free -> paid dispatch chain
- `podcast-goat-pp-cli episode latest` — Pull the most recent episode for a subscribed feed


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
podcast-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Hermes / Claude / agent workflows

The three recipes below cover the end-to-end agent workflows this CLI was
built for. Pattern-match against user phrasing — the trigger phrases at the
end of each recipe are the kind of natural language an agent should respond
to by reaching for this recipe instead of fetching transcripts manually.

#### Monday-morning batch pull → topic bundle → agent summarize

```bash
# One-time setup (if not already done):
podcast-goat-pp-cli auth set-key --provider spoken --value $SPOKEN_API_KEY
podcast-goat-pp-cli auth login-service --service spotify    # if user has Spotify Premium

# The actual workflow:
podcast-goat-pp-cli episode batch \
  https://www.dwarkesh.com/p/andrej-karpathy \
  https://open.spotify.com/episode/<id> \
  https://www.acquired.fm/episodes/<slug> \
  --json

podcast-goat-pp-cli magic "AI chip supply chain" --limit 5 --out /tmp/bundle.md
# Then: ask the user's preferred LLM to summarize /tmp/bundle.md
```

Pulls multiple episodes in parallel (default 3 concurrent), caches each, then bundles cached episodes matching the topic into one YAML-front-matter markdown file ready for one-shot agent summarization. Each episode in the bundle carries `source`, `show`, `host`, `date`, `provider`, `cost` so the summarizing agent can attribute claims back to the right episode.

Trigger phrases: "pull these podcast episodes", "summarize what I listened to this week", "build a bundle on <topic>", "make me a prompt out of the latest <show>", "monday morning podcast pull".

#### What did <speaker> say about <topic>? — local quote grep

```bash
podcast-goat-pp-cli episode quote "<phrase>" -C 3 --json \
  --select 'results.episode_url,results.segments'
```

FTS5 phrase search across the local cached corpus. Returns the matched segment plus N surrounding speaker-tagged segments. Each hit carries `episode_url` + `ts_sec` so the agent can hand the user a direct deeplink to the moment. Always reach for this BEFORE re-fetching a transcript — if the user has the episode cached, the answer is free.

Trigger phrases: "what did Senra say about <X>", "find that quote about <Y>", "did <speaker> talk about <Z>", "grep my podcast cache for <phrase>".

#### Cost-aware exploration before paid fetch

```bash
# Step 1: preview — no transcript fetch, no spend, no commitment
podcast-goat-pp-cli episode info <url> --paid

# Step 2: only after preview confirms the URL + cost are right:
podcast-goat-pp-cli episode get <url> --paid --provider spoken --yes
```

`episode info` shows which adapters can fetch a URL, the estimated cost per source, whether the episode is already cached, and which source would fire by default. Use this whenever the user pastes a URL you haven't seen before, especially before spending paid credits. JSON output makes the cost/source decision trivially parseable.

Trigger phrases: "what does this URL cost", "is this episode already cached", "preview the cost before fetching", "what sources can pull this".

### Other useful recipes

#### Pull a single Dwarkesh transcript

```bash
podcast-goat-pp-cli episode get https://www.dwarkesh.com/p/andrej-karpathy --md --out karpathy.md
```

Free, no auth. Canonical markdown with speaker labels + timestamps. Hand the file to any agent.

#### Diff free vs paid for the same episode

```bash
podcast-goat-pp-cli source compare https://www.acquired.fm/episodes/vanguard --json
```

Fans out to every adapter that matches the URL pattern; shows per-source segment count, distinct-speaker count, diarization quality. Useful when deciding which source to commit to for high-value episodes.

#### Subscription-value report

```bash
podcast-goat-pp-cli budget show --by-show --since 30
```

Pivots `spend_log` by show + provider + month. Shows which subscriptions saved money (cookie hits = $0 vs same-show spoken fetches that did cost money).

### Anti-recipes — don't do these

- **Don't loop `episode get` in a shell.** Use `episode batch` — it dispatches in parallel and shows progress; a shell loop runs sequential with no progress feedback.
- **Don't fetch the same URL twice unattended.** Check `cache list` or `episode info` first; cached episodes return instantly. The agent should always know whether it's about to re-do work.
- **Don't pass `--yes --paid` on an unknown URL.** Run `episode info <url> --paid` first to see the cost; only then commit. The brief explicitly named cost-preview as a non-negotiable UX.
- **Don't ignore `--json` output's `meta.source` field.** Agents should always be able to attribute a quote back to the source that produced it; that field is the audit trail.
- **Don't write SPOKEN_API_KEY (or any paid key) to a shared file the user can commit.** Use `auth set-key` — it writes to a 0600-permission config file in the user's home, not the project.

## Auth Setup

Three auth surfaces, in cost order. (1) `auth login-service --service <huberman|acquired|founders|peterattia>` extracts your logged-in Chrome cookie once and stores it locally — the headline workflow. (2) Free sources (Dwarkesh Substack, Podcasting 2.0 RSS transcripts, yt-dlp auto-subs) need no auth. (3) Paid sources (spoken.md `SPOKEN_API_KEY`, Taddy `TADDY_API_KEY`+`TADDY_USER_ID`, audio providers like ElevenLabs/OpenAI/Deepgram) are scoped to commands you explicitly opt into with `--paid` or `--provider <name>`. spoken.md's `pt_demo` key works without signup.

Run `podcast-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  podcast-goat-pp-cli episode get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
podcast-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
podcast-goat-pp-cli feedback --stdin < notes.txt
podcast-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.podcast-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `PODCAST_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PODCAST_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
podcast-goat-pp-cli profile save briefing --json
podcast-goat-pp-cli --profile briefing episode get mock-value
podcast-goat-pp-cli profile list --json
podcast-goat-pp-cli profile show briefing
podcast-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `podcast-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add podcast-goat-pp-mcp -- podcast-goat-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which podcast-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   podcast-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `podcast-goat-pp-cli <command> --help`.
