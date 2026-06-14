# PodcastIndex CLI

**The only PodcastIndex tool that is agent-native and works offline: search, resolve, and analyse any podcast from one signed binary with a local mirror you can FTS and SQL.**

Every existing PodcastIndex tool is a language SDK that mirrors the endpoints and returns raw JSON in-process. This CLI adds a local SQLite mirror, full-text search over synced shows and episodes, agent-native output, and compound commands no wrapper has: tgrep searches inside transcripts, cadence and dead-watch reason over publish history, guest-graph joins people across feeds.

Learn more at [PodcastIndex](https://podcastindex.org/).

Created by [@adbonnet](https://github.com/adbonnet).

## Install

The recommended path installs both the `podcastindex-pp-cli` binary and the `pp-podcastindex` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install podcastindex
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install podcastindex --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install podcastindex --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install podcastindex --agent claude-code
npx -y @mvanhorn/printing-press-library install podcastindex --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcastindex/cmd/podcastindex-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/podcastindex-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install podcastindex --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-podcastindex --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-podcastindex --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install podcastindex --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/podcastindex-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PODCASTINDEX_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcastindex/cmd/podcastindex-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "podcastindex": {
      "command": "podcastindex-pp-mcp",
      "env": {
        "PODCASTINDEX_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

PodcastIndex signs every request with four headers: X-Auth-Key, X-Auth-Date (unix now), Authorization (sha1 of key+secret+date), and User-Agent. Set PODCASTINDEX_KEY and PODCASTINDEX_SECRET and the CLI computes the signature for you on each call.

## Quick Start

```bash
# verify the signer and credentials without hitting the API
podcastindex-pp-cli doctor --dry-run

# search podcasts by term
podcastindex-pp-cli find search-byterm --q "batman university" --max 5

# find episodes featuring a person
podcastindex-pp-cli find search-byperson --q "adam curry" --max 5

# resolve a show to its recent episodes
podcastindex-pp-cli episodes byfeedid --id 75075 --newest --max 10

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Mine data wrappers ignore
- **`tgrep`** — Search inside actual episode transcripts, not just titles and descriptions.

  _Reach for this when the user wants what was *said* in episodes, not show metadata._

  ```bash
  podcastindex-pp-cli tgrep "jepa" --cat Technology --agent
  ```

## Recipes

### Resolve a show in one step

```bash
podcastindex-pp-cli find search-byterm --q "no agenda" --max 1 --json --select feeds.id,feeds.title
```

Get the feedId for a show by name, ready to pass to episodes byfeedid.

### Find a person's (or company's) appearances

This is the primary path for "where did **X** appear". PodcastIndex has **no
episode-content or guest search** — `find search-byterm` matches feed metadata,
`find search-byperson` matches `<podcast:person>` tags (and fuzzy-matches the
first name, so it is noisy and misses untagged guests). So you cannot reach an
untagged appearance from a name alone: find the **show** first (a web search),
then resolve it here.

```bash
# A guest on a known show — resolves the show, filters its episodes by name,
# emits the enclosure URLs ready to transcribe.
podcastindex-pp-cli workflow find-appearances --match "Arthur Mensch" --show "Big Technology" \
  --json --select feedTitle,title,datePublished,enclosureUrl

# Company + founder across a candidate show
podcastindex-pp-cli workflow find-appearances --match "Implicity" --match "Arnaud Rosier" \
  --show "Med in Tech"

# Scan an explicit feed id directly (skip the show lookup)
podcastindex-pp-cli workflow find-appearances --match "Rosier" --feed 4712435
```

`--match` is repeatable (OR-matched against each episode's title and
description); `--show` and `--feed` are repeatable. Add `--byperson` to also fold
in tag-based hits (still filtered through `--match`). Prefer this over
`find search-byperson` whenever you know the show.

### Catch up on a show

```bash
podcastindex-pp-cli episodes byfeedid --id 920666 --newest --max 10 --json --select items.title,items.datePublished,items.enclosureUrl
```

List a show's newest episodes with just the fields an agent needs.

### Search inside transcripts

```bash
podcastindex-pp-cli tgrep "interest rates" --cat Business --agent
```

Full-text search the actual transcript files, not just show metadata.

## Usage

Run `podcastindex-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

Categories used by the Podcast Index

- **`podcastindex-pp-cli categories`** - Return all the possible categories supported by the index.

Example: https://api.podcastindex.org/api/1.0/categories/list?pretty

### episodes

Find details about one or more episodes of a podcast or podcasts.

- **`podcastindex-pp-cli episodes byfeedid`** - This call returns all the episodes we know about for this feed from the PodcastIndex ID.
Episodes are in reverse chronological order.

When using the `enclosure` parameter, only the episode matching the URL is returned.

Examples:

  - https://api.podcastindex.org/api/1.0/episodes/byfeedid?id=75075&pretty
  - https://api.podcastindex.org/api/1.0/episodes/byfeedid?id=41504,920666&pretty
  - https://api.podcastindex.org/api/1.0/episodes/byfeedid?id=75075&newest&pretty
  - https://api.podcastindex.org/api/1.0/episodes/byfeedid?id=41504,920666&newest&pretty
  - Includes `persons`: https://api.podcastindex.org/api/1.0/episodes/byfeedid?id=169991&pretty
  - Includes `value`: https://api.podcastindex.org/api/1.0/episodes/byfeedid?id=4058673&pretty
  - Using `enclosure`: https://api.podcastindex.org/api/1.0/episodes/byfeedid?id=41504&enclosure=https://op3.dev/e/mp3s.nashownotes.com/NA-1551-2023-04-30-Final.mp3&pretty
- **`podcastindex-pp-cli episodes byfeedurl`** - This call returns all the episodes we know about for this feed from the feed URL.
Episodes are in reverse chronological order.

Examples:

  - https://api.podcastindex.org/api/1.0/episodes/byfeedurl?url=https://feeds.theincomparable.com/batmanuniversity&pretty
  - Includes `persons`: https://api.podcastindex.org/api/1.0/episodes/byfeedurl?url=https://engineered.network/pragmatic/feed/index.xml&pretty
  - Includes `value`: https://api.podcastindex.org/api/1.0/episodes/byfeedurl?url=https://closing-the-loop.github.io/feed.xml&pretty
- **`podcastindex-pp-cli episodes byguid`** - Get all the metadata for a single episode by passing its guid and the feed id or URL.

The `feedid`, `feedurl`, or `podcastguid` is required.

Examples: 

  - Search using Podcast Index feed ID: https://api.podcastindex.org/api/1.0/episodes/byguid?guid=PC2084&feedid=920666&pretty
  - Search using feed URL: https://api.podcastindex.org/api/1.0/episodes/byguid?guid=PC2084&feedurl=http://mp3s.nashownotes.com/pc20rss.xml&pretty
- **`podcastindex-pp-cli episodes byid`** - Get all the metadata for a single episode by passing its id.

Example: https://api.podcastindex.org/api/1.0/episodes/byid?id=16795090&pretty
- **`podcastindex-pp-cli episodes byitunesid`** - This call returns all the episodes we know about for this feed from the iTunes ID.
Episodes are in reverse chronological order.

When using the `enclosure` parameter, only the episode matching the URL is returned.

Examples:

  - https://api.podcastindex.org/api/1.0/episodes/byitunesid?id=1441923632&pretty
  - Using `enclosure`: https://api.podcastindex.org/api/1.0/episodes/byitunesid?id=269169796&enclosure=https://op3.dev/e/mp3s.nashownotes.com/NA-1551-2023-04-30-Final.mp3&pretty
- **`podcastindex-pp-cli episodes bypodcastguid`** - This call returns all the episodes we know about for this feed from the [Podcast GUID](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#guid).
Episodes are in reverse chronological order.

Example: https://api.podcastindex.org/api/1.0/episodes/bypodcastguid?guid=856cd618-7f34-57ea-9b84-3600f1f65e7f&pretty
- **`podcastindex-pp-cli episodes live`** - Get all episodes that have been found in the [podcast:liveitem](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#live-item) from the feeds.

Examples: 

  - https://api.podcastindex.org/api/1.0/episodes/live?pretty
- **`podcastindex-pp-cli episodes random`** - This call returns a random batch of episodes, in no specific order.

Examples:

  - https://api.podcastindex.org/api/1.0/episodes/random?notcat=News,Religion&lang=en,es&pretty
  - https://api.podcastindex.org/api/1.0/episodes/random?max=2&pretty

### find

Manage find

- **`podcastindex-pp-cli find search`** - Replaces the Apple search API but returns data from the Podcast Index database.

Note: No API key needed for this endpoint.

Example:

  - Apple: https://itunes.apple.com/search?media=podcast&entity=podcast&term=batman
  - PodcastIndex: https://api.podcastindex.org/search?term=batman
- **`podcastindex-pp-cli find search-byperson`** - This call returns all of the episodes where the specified person is mentioned.

It searches the following fields:

  - Person tags
  - Episode title
  - Episode description
  - Feed owner
  - Feed author

Examples:

  - https://api.podcastindex.org/api/1.0/search/byperson?q=adam%20curry&pretty
  - https://api.podcastindex.org/api/1.0/search/byperson?q=Martin+Mouritzen&pretty
  - https://api.podcastindex.org/api/1.0/search/byperson?q=Klaus+Schwab&pretty
- **`podcastindex-pp-cli find search-byterm`** - This call returns all of the feeds that match the search terms in the `title`, `author` or `owner` of the feed.

Example: https://api.podcastindex.org/api/1.0/search/byterm?q=batman+university&pretty
- **`podcastindex-pp-cli find search-bytitle`** - This call returns all of the feeds where the `title` of the feed matches the search term (ignores case).

Example "everything everywhere daily" will match the podcast
[Everything Everywhere Daily](https://podcastindex.org/podcast/437685) by "everything everywhere" will not.

Example: https://api.podcastindex.org/api/1.0/search/bytitle?q=everything+everywhere+daily&pretty
- **`podcastindex-pp-cli find search-music-byterm`** - This call returns all of the feeds that match the search terms in the `title`, `author` or `owner` of the <feed></feed>
where the [medium](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#medium) is `music`.

Example: https://api.podcastindex.org/api/1.0/search/music/byterm?q=able+kirby&pretty

### lookup

Manage lookup

- **`podcastindex-pp-cli lookup`** - Replaces the Apple podcast lookup API but returns data from the Podcast Index database.

Note: No API key needed for this endpoint.

Example:

  - Apple: https://itunes.apple.com/lookup?media=podcast&entity=podcast&id=1636765656
  - PodcastIndex: https://api.podcastindex.org/lookup?entity=podcast&id=1636765656

### podcasts

Find details about a Podcast and its feed.

- **`podcastindex-pp-cli podcasts batch-byguid`** - This call returns everything we know about the feed from the feed's GUID provided in JSON array in the body of the request.

The GUID is a unique, global identifier for the podcast. See the namespace spec for
[guid](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#guid) for details.
- **`podcastindex-pp-cli podcasts byfeedid`** - This call returns everything we know about the feed from the PodcastIndex ID

Examples:

  - https://api.podcastindex.org/api/1.0/podcasts/byfeedid?id=75075&pretty
  - Includes `value` and `funding`: https://api.podcastindex.org/api/1.0/podcasts/byfeedid?id=169991&pretty
- **`podcastindex-pp-cli podcasts byfeedurl`** - This call returns everything we know about the feed from the feed URL

Examples:

  - https://api.podcastindex.org/api/1.0/podcasts/byfeedurl?url=https://feeds.theincomparable.com/batmanuniversity&pretty
  - Includes `value` and `funding`: https://api.podcastindex.org/api/1.0/podcasts/byfeedurl?url=https://engineered.network/pragmatic/feed/index.xml&pretty
- **`podcastindex-pp-cli podcasts byguid`** - This call returns everything we know about the feed from the feed's GUID.

The GUID is a unique, global identifier for the podcast. See the namespace spec for
[guid](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#guid) for details.

Examples:

  - https://api.podcastindex.org/api/1.0/podcasts/byguid?guid=9b024349-ccf0-5f69-a609-6b82873eab3c&pretty
  - Includes `value` and `funding`: https://api.podcastindex.org/api/1.0/podcasts/byguid?guid=9b024349-ccf0-5f69-a609-6b82873eab3c&pretty
- **`podcastindex-pp-cli podcasts byitunesid`** - This call returns everything we know about the feed from the iTunes ID

Example: https://api.podcastindex.org/api/1.0/podcasts/byitunesid?id=1441923632&pretty
- **`podcastindex-pp-cli podcasts bymedium`** - This call returns all feeds marked with the specified
[medium](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#medium) tag value.

Example: https://api.podcastindex.org/api/1.0/podcasts/bymedium?medium=music&pretty
- **`podcastindex-pp-cli podcasts bytag`** - This call returns all feeds that support the specified
[podcast namespace](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md) tag.

The only supported tags are:
  - `podcast:value` using the `podcast-value` parameter
  - `podcast:valueTimeSplit` using the `podcast-valueTimeSplit` parameter

Only the `podcast-value` or `podcast-valueTimeSplit` parameter should be used. If multiple are specified, the
first parameter is used and the others are ignored.

When called without a `start_at` value, the top 500 feeds sorted by popularity are returned in descending order.

When called with a `start_at` value, the feeds are returned sorted by the `feedId` starting with the specified value
up to the max number of feeds to return. The `nextStartAt` specifies the value to pass to the next `start_at`.
Repeat this sequence until no items are returned.

Examples:
  - https://api.podcastindex.org/api/1.0/podcasts/bytag?podcast-value&max=200&pretty
  - https://api.podcastindex.org/api/1.0/podcasts/bytag?podcast-value&max=200&start_at=1&pretty
  - https://api.podcastindex.org/api/1.0/podcasts/bytag?podcast-valueTimeSplit&pretty
- **`podcastindex-pp-cli podcasts dead`** - This call returns all feeds that have been marked dead (`dead` == 1)

Hourly statistics can also be access at https://public.podcastindex.org/podcastindex_dead_feeds.csv
For details, see [Dead Feeds](#get-/static/public/podcastindex_dead_feeds.csv).

Example: https://api.podcastindex.org/api/1.0/podcasts/dead?pretty
- **`podcastindex-pp-cli podcasts trending`** - This call returns the podcasts/feeds that in the index that are trending.

Example: https://api.podcastindex.org/api/1.0/podcasts/trending?pretty

### recent

Find recent additions to the index

- **`podcastindex-pp-cli recent data`** - This call returns every new feed and episode added to the index over the past 24 hours in reverse chronological order.

This is similar to `/recent/feeds` but uses the date the feed was found by the index rather than the feed's
internal timestamp.

Similar data can also be accessed using object storage root url https://tracking.podcastindex.org/current
For details, see [Current](#get-/static/tracking/current).

Examples:

  - https://api.podcastindex.org/api/1.0/recent/data?pretty
  - https://api.podcastindex.org/api/1.0/recent/data?pretty&max=10
  - https://api.podcastindex.org/api/1.0/recent/data?pretty&max=10&since=1671164867
- **`podcastindex-pp-cli recent episodes`** - This call returns the most recent `max` number of episodes globally across the whole index,
in reverse chronological order.

Example: https://api.podcastindex.org/api/1.0/recent/episodes?max=7&pretty
- **`podcastindex-pp-cli recent feeds`** - This call returns the most recent `max` feeds, in reverse chronological order.

Examples:

  - https://api.podcastindex.org/api/1.0/recent/feeds?pretty
  - https://api.podcastindex.org/api/1.0/recent/feeds?max=20&cat=102,health&lang=de,ja&pretty
- **`podcastindex-pp-cli recent newfeeds`** - This call returns every new feed added to the index over the past 24 hours in reverse chronological order.

Examples:

  - https://api.podcastindex.org/api/1.0/recent/newfeeds?pretty
  - https://api.podcastindex.org/api/1.0/recent/newfeeds?pretty&since=1613805000
  - https://api.podcastindex.org/api/1.0/recent/newfeeds?feedid=2653471&pretty
  - https://api.podcastindex.org/api/1.0/recent/newfeeds?feedid=2653471&desc&pretty
- **`podcastindex-pp-cli recent newvaluefeeds`** - This call returns feeds that have added a `value` tag in reverse chronological order.

Example: https://api.podcastindex.org/api/1.0/recent/newvaluefeeds?pretty
- **`podcastindex-pp-cli recent soundbites`** - This call returns the most recent `max` soundbites that the index has discovered.

A soundbite consists of an enclosure url, a start time and a duration.
It is documented in the [podcast namespace](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#soundbite).

Example: https://api.podcastindex.org/api/1.0/recent/soundbites?pretty

### stats

Statistics for items in the Podcast Index

- **`podcastindex-pp-cli stats`** - Return the most recent index statistics.

Hourly statistics can also be access at https://stats.podcastindex.org/hourly_counts.json
For details, see [Stats Hourly Counts](#get-/static/stats/hourly_counts.json).

Daily statistics can also be access at https://stats.podcastindex.org/daily_counts.json.
For details, see [Stats Daily Counts](#get-/static/stats/daily_counts.json).

Example: https://api.podcastindex.org/api/1.0/stats/current?pretty

### value

The podcast's "Value for Value" information

- **`podcastindex-pp-cli value batch-byepisodeguid`** - This call returns the information for supporting the podcast episode via one of the "Value for Value" methods from
a JSON object containing one or more podcast GUID and one or more episode GUID for the podcast.

The JSON object key shall be the `podcastguid` from the `podcast:guid` tag in the feed.
This value is a unique, global identifier for the podcast. See the namespace spec for
[guid](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#guid) for details.

The value of the `podcastguid` shall be an array of `episodeguid` values.
This is the unique guid specified for the `<item>` in the feed but may not be globally unique.

Note: No API key needed for this endpoint.
- **`podcastindex-pp-cli value byepisodeguid`** - This call returns the information for supporting the podcast episode via one of the "Value for Value" methods from
podcast GUID and the episode GUID.

The `podcastguid` is the GUID from the `podcast:guid` tag in the feed. This value is a unique, global identifier
for the podcast. See the namespace spec for
[guid](https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md#guid) for details.

The `episodeguid` is the unique guid specified for the `<item>` in the feed but may not be globally unique.

Note: No API key needed for this endpoint.

Examples:

  - https://api.podcastindex.org/api/1.0/value/byepisodeguid?podcastguid=917393e3-1b1e-5cef-ace4-edaa54e1f810&episodeguid=PC20143&pretty
  - https://api.podcastindex.org/api/1.0/value/byepisodeguid?podcastguid=c73b1a23-1c28-5edb-94c3-10d1745d0877&episodeguid=bdea6759-a7b6-4c0d-9d1e-acca3133f4a9&pretty
- **`podcastindex-pp-cli value byfeedid`** - This call returns the information for supporting the podcast via one of the "Value for Value" methods from the
PodcastIndex ID.

Additionally, the value block data can be accessed using static JSON files (updated every 15 minutes).

  - Feeds: https://tracking.podcastindex.org/feedValueBlocks.json
  - Episodes: https://tracking.podcastindex.org/episodeValueBlocks.json

Note: No API key needed for this endpoint.

Examples:

  - https://api.podcastindex.org/api/1.0/value/byfeedid?id=920666&pretty
  - https://api.podcastindex.org/api/1.0/value/byfeedid?id=779873&pretty
- **`podcastindex-pp-cli value byfeedurl`** - This call returns the information for supporting the podcast via one of the "Value for Value" methods from feed URL.

Additionally, the value block data can be accessed using static JSON files (updated every 15 minutes).

  - Feeds: https://tracking.podcastindex.org/feedValueBlocks.json
  - Episodes: https://tracking.podcastindex.org/episodeValueBlocks.json

Note: No API key needed for this endpoint.

Examples:

  - https://api.podcastindex.org/api/1.0/value/byfeedurl?url=https://mp3s.nashownotes.com/pc20rss.xml&pretty
  - https://api.podcastindex.org/api/1.0/value/byfeedurl?url=https://lespoesiesdheloise.fr/@heloise/feed.xml&pretty
- **`podcastindex-pp-cli value bypodcastguid`** - This call returns the information for supporting the podcast via one of the "Value for Value" methods from podcast GUID.

Note: No API key needed for this endpoint.

Example: https://api.podcastindex.org/api/1.0/value/bypodcastguid?guid=917393e3-1b1e-5cef-ace4-edaa54e1f810&pretty

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
podcastindex-pp-cli categories

# JSON for scripting and agents
podcastindex-pp-cli categories --json

# Filter to specific fields
podcastindex-pp-cli categories --json --select id,name,status

# Dry run — show the request without sending
podcastindex-pp-cli categories --dry-run

# Agent mode — JSON + compact + no prompts in one flag
podcastindex-pp-cli categories --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
podcastindex-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/podcastindex-org-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PODCASTINDEX_KEY` | per_call | No | Set to your API credential. |
| `PODCASTINDEX_ORG_API_KEY` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `podcastindex-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `podcastindex-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PODCASTINDEX_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / authorization error** — ensure PODCASTINDEX_KEY and PODCASTINDEX_SECRET are exported; run doctor to check clock skew on X-Auth-Date
- **empty results on a known term** — drop --clean or add --fulltext; the API truncates descriptions unless --fulltext is set

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**podcast-index-api**](https://github.com/comster/podcast-index-api) — JavaScript (26 stars)
- [**python-podcastindex**](https://github.com/SarvagyaVaish/python-podcastindex) — Python (25 stars)
- [**PodcastIndexKit**](https://github.com/SparrowTek/PodcastIndexKit) — Swift (10 stars)
- [**PodcastIndex-SDK**](https://github.com/mr3y-the-programmer/PodcastIndex-SDK) — Kotlin (7 stars)
- [**podcast-index**](https://github.com/jasonyork/podcast-index) — Ruby (5 stars)
- [**PodcastIndexSharp**](https://github.com/brb3/PodcastIndexSharp) — C# (5 stars)
- [**podcast-index-client**](https://github.com/kilobit/podcast-index-client) — Go (2 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
