---
name: pp-podcastindex
description: "The only PodcastIndex tool that is agent-native and works offline: search, resolve Trigger phrases: `search podcasts for`, `find episodes with`, `resolve this show to its episodes`, `what podcasts mention`, `podcast cadence for`, `use podcastindex`, `run podcastindex`."
author: "adbonnet"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - podcastindex-pp-cli
    install:
      - kind: go
        bins: [podcastindex-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcastindex/cmd/podcastindex-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/podcastindex/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# PodcastIndex — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `podcastindex-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install podcastindex --cli-only
   ```
2. Verify: `podcastindex-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcastindex/cmd/podcastindex-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every existing PodcastIndex tool is a language SDK that mirrors the endpoints and returns raw JSON in-process. This CLI adds a local SQLite mirror, full-text search over synced shows and episodes, agent-native output, and compound commands no wrapper has: tgrep searches inside transcripts, cadence and dead-watch reason over publish history, guest-graph joins people across feeds.

## When to Use This CLI

Use this CLI when an agent or user needs to discover podcasts, resolve a show to its episodes, or analyse podcast metadata, publish cadence, guests, value4value splits, or transcript content from the PodcastIndex database. It is the right tool when offline reuse, full-text search, or cross-feed analysis matters.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to download or play audio files; it returns enclosure URLs, not media.
- Do not use it to add or manage feeds in PodcastIndex; the add/* endpoints need publisher auth and are out of scope.
- Do not use it for Apple Podcasts-only metadata beyond the iTunes lookup passthrough.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Mine data wrappers ignore
- **`tgrep`** — Search inside actual episode transcripts, not just titles and descriptions.

  _Reach for this when the user wants what was *said* in episodes, not show metadata._

  ```bash
  podcastindex-pp-cli tgrep "jepa" --cat Technology --agent
  ```

## Command Reference

**categories** — Categories used by the Podcast Index

- `podcastindex-pp-cli categories` — Return all the possible categories supported by the index. Example: https://api.podcastindex.org/api/1.

**episodes** — Find details about one or more episodes of a podcast or podcasts.

- `podcastindex-pp-cli episodes byfeedid` — This call returns all the episodes we know about for this feed from the PodcastIndex ID.
- `podcastindex-pp-cli episodes byfeedurl` — This call returns all the episodes we know about for this feed from the feed URL.
- `podcastindex-pp-cli episodes byguid` — Get all the metadata for a single episode by passing its guid and the feed id or URL.
- `podcastindex-pp-cli episodes byid` — Get all the metadata for a single episode by passing its id. Example: https://api.podcastindex.org/api/1.
- `podcastindex-pp-cli episodes byitunesid` — This call returns all the episodes we know about for this feed from the iTunes ID.
- `podcastindex-pp-cli episodes bypodcastguid` — This call returns all the episodes we know about for this feed from the [Podcast GUID](https://github.
- `podcastindex-pp-cli episodes live` — Get all episodes that have been found in the [podcast:liveitem](https://github.
- `podcastindex-pp-cli episodes random` — This call returns a random batch of episodes, in no specific order. Examples: - https://api.podcastindex.org/api/1.

**find** — Manage find

- `podcastindex-pp-cli find search` — Replaces the Apple search API but returns data from the Podcast Index database.
- `podcastindex-pp-cli find search-byperson` — This call returns all of the episodes where the specified person is mentioned.
- `podcastindex-pp-cli find search-byterm` — This call returns all of the feeds that match the search terms in the `title`, `author` or `owner` of the feed.
- `podcastindex-pp-cli find search-bytitle` — This call returns all of the feeds where the `title` of the feed matches the search term (ignores case).
- `podcastindex-pp-cli find search-music-byterm` — This call returns all of the feeds that match the search terms in the `title`

**lookup** — Manage lookup

- `podcastindex-pp-cli lookup` — Replaces the Apple podcast lookup API but returns data from the Podcast Index database.

**podcasts** — Find details about a Podcast and its feed.

- `podcastindex-pp-cli podcasts batch-byguid` — This call returns everything we know about the feed from the feed's GUID provided in JSON array in the body of the
- `podcastindex-pp-cli podcasts byfeedid` — This call returns everything we know about the feed from the PodcastIndex ID Examples: - https://api.podcastindex.
- `podcastindex-pp-cli podcasts byfeedurl` — This call returns everything we know about the feed from the feed URL Examples: - https://api.podcastindex.org/api/1.
- `podcastindex-pp-cli podcasts byguid` — This call returns everything we know about the feed from the feed's GUID.
- `podcastindex-pp-cli podcasts byitunesid` — This call returns everything we know about the feed from the iTunes ID Example: https://api.podcastindex.org/api/1.
- `podcastindex-pp-cli podcasts bymedium` — This call returns all feeds marked with the specified [medium](https://github.
- `podcastindex-pp-cli podcasts bytag` — This call returns all feeds that support the specified [podcast namespace](https://github.
- `podcastindex-pp-cli podcasts dead` — This call returns all feeds that have been marked dead (`dead` == 1) Hourly statistics can also be access at https
- `podcastindex-pp-cli podcasts trending` — This call returns the podcasts/feeds that in the index that are trending. Example: https://api.podcastindex.org/api/1.

**recent** — Find recent additions to the index

- `podcastindex-pp-cli recent data` — This call returns every new feed and episode added to the index over the past 24 hours in reverse chronological order.
- `podcastindex-pp-cli recent episodes` — This call returns the most recent `max` number of episodes globally across the whole index
- `podcastindex-pp-cli recent feeds` — This call returns the most recent `max` feeds, in reverse chronological order. Examples: - https://api.podcastindex.
- `podcastindex-pp-cli recent newfeeds` — This call returns every new feed added to the index over the past 24 hours in reverse chronological order.
- `podcastindex-pp-cli recent newvaluefeeds` — This call returns feeds that have added a `value` tag in reverse chronological order. Example: https://api.podcastindex.
- `podcastindex-pp-cli recent soundbites` — This call returns the most recent `max` soundbites that the index has discovered.

**stats** — Statistics for items in the Podcast Index

- `podcastindex-pp-cli stats` — Return the most recent index statistics. Hourly statistics can also be access at https://stats.podcastindex.

**value** — The podcast's "Value for Value" information

- `podcastindex-pp-cli value batch-byepisodeguid` — This call returns the information for supporting the podcast episode via one of the 'Value for Value' methods from a
- `podcastindex-pp-cli value byepisodeguid` — This call returns the information for supporting the podcast episode via one of the 'Value for Value' methods from
- `podcastindex-pp-cli value byfeedid` — This call returns the information for supporting the podcast via one of the 'Value for Value' methods from the
- `podcastindex-pp-cli value byfeedurl` — This call returns the information for supporting the podcast via one of the 'Value for Value' methods from feed URL.
- `podcastindex-pp-cli value bypodcastguid` — This call returns the information for supporting the podcast via one of the 'Value for Value' methods from podcast GUID.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
podcastindex-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Resolve a show in one step

```bash
podcastindex-pp-cli find search-byterm --q "no agenda" --max 1 --json --select feeds.id,feeds.title
```

Get the feedId for a show by name, ready to pass to episodes byfeedid.

### Catch up on a show

```bash
podcastindex-pp-cli episodes byfeedid --id 920666 --newest --max 10 --json --select items.title,items.datePublished,items.enclosureUrl
```

List a show's newest episodes with just the fields an agent needs.

### Search inside transcripts

```bash
podcastindex-pp-cli tgrep "interest rates" --feed 920666 --agent
```

Full-text search the actual transcript files of a show, not just metadata.

## Auth Setup

PodcastIndex signs every request with four headers: X-Auth-Key, X-Auth-Date (unix now), Authorization (sha1 of key+secret+date), and User-Agent. Set PODCASTINDEX_KEY and PODCASTINDEX_SECRET and the CLI computes the signature for you on each call.

Run `podcastindex-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  podcastindex-pp-cli categories --agent --select id,name,status
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
podcastindex-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
podcastindex-pp-cli feedback --stdin < notes.txt
podcastindex-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/podcastindex-pp-cli/feedback.jsonl`. They are never POSTed unless `PODCASTINDEX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PODCASTINDEX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
podcastindex-pp-cli profile save briefing --json
podcastindex-pp-cli --profile briefing categories
podcastindex-pp-cli profile list --json
podcastindex-pp-cli profile show briefing
podcastindex-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `podcastindex-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcastindex/cmd/podcastindex-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add podcastindex-pp-mcp -- podcastindex-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which podcastindex-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   podcastindex-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `podcastindex-pp-cli <command> --help`.
