---
name: pp-foxnews
description: "Fox News headlines from Google Publisher RSS — pick a section (latest, world, politics, sports, ...) or default to latest. Trigger phrases: `fox news headlines`, `what's on fox news`, `fox politics headlines`, `fox sports rss`, `use foxnews`, `run foxnews`."
author: "John Fiedler"
license: "Apache-2.0"
argument-hint: "headlines [--section latest] [--limit N] | sections | doctor | agent-context"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - foxnews-pp-cli
---

# Fox News — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `foxnews-pp-cli` binary. **Verify the CLI is installed before invoking any command.**

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install foxnews --cli-only
   ```
2. Verify: `foxnews-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If `--version` reports "command not found", fix `$PATH` before running skill commands.

## When to Use This CLI

Use for **read-only Fox News headline lists** from official Google Publisher RSS feeds. One bounded JSON call per section — no API key, no HTML parsing.

- Default section **`latest`** = all-topic headline mix
- Use **`--section`** for world, politics, science, health, sports, travel, tech, opinion, or video

## When Not to Use This CLI

Do not use for posting, commenting, paywalled article bodies, live TV listings, or non-Fox data sources. This CLI only reads the RSS feeds listed below.

## Not the Drudge Report CLI

This is **not** `drudgereport-pp-cli`. These commands **do not exist** here:

- `which`, `sync`, `splash`, `breaking`, `tail`, `tenure`, `sources`, `on-date`, `bent`, `story`, `digest`

For Fox News, use:

- **`sections`** — list valid `--section` ids (default feed is `latest`)
- **`headlines`** — fetch headlines for one section
- **`agent-context`** — runtime JSON catalog of commands, sections, and defaults
- **`doctor`** — RSS connectivity check

## RSS sections

| Section | CLI `--section` | Feed URL |
|---------|-----------------|----------|
| Latest (default) | `latest` | https://moxie.foxnews.com/google-publisher/latest.xml |
| World | `world` | https://moxie.foxnews.com/google-publisher/world.xml |
| Politics | `politics` | https://moxie.foxnews.com/google-publisher/politics.xml |
| Science | `science` | https://moxie.foxnews.com/google-publisher/science.xml |
| Health | `health` | https://moxie.foxnews.com/google-publisher/health.xml |
| Sports | `sports` | https://moxie.foxnews.com/google-publisher/sports.xml |
| Travel | `travel` | https://moxie.foxnews.com/google-publisher/travel.xml |
| Tech | `tech` | https://moxie.foxnews.com/google-publisher/tech.xml |
| Opinion | `opinion` | https://moxie.foxnews.com/google-publisher/opinion.xml |
| Video | `video` | https://moxie.foxnews.com/google-publisher/videos.xml |

Run `foxnews-pp-cli sections --json` for the machine-readable list.

## Commands

### `headlines`

Fetch headlines for a section (default `latest`).

```bash
foxnews-pp-cli headlines --json
foxnews-pp-cli headlines --section politics --limit 10 --json --select title,link,published
foxnews-pp-cli headlines --section sports --agent
```

### `sections`

List valid `--section` values.

```bash
foxnews-pp-cli sections --json
```

### `agent-context`

Emit structured JSON for agents (commands, sections, `default_section: latest`, and explicit `not_available` list).

```bash
foxnews-pp-cli agent-context --pretty
```

### `doctor`

Probe the latest feed (connectivity smoke test).

```bash
foxnews-pp-cli doctor --json
```

## Recipes

### Top stories right now (all sections)

```bash
foxnews-pp-cli headlines --limit 15 --agent
```

Parse `.results` for headline rows; use `.meta.section` and `.meta.feed_url` for provenance.

### Politics briefing

```bash
foxnews-pp-cli headlines --section politics --limit 10 --agent --select title,link
```

### Discover sections at runtime

```bash
foxnews-pp-cli agent-context | jq '.default_section, .sections[].id'
```

## Auth Setup

No authentication required.

Run `foxnews-pp-cli doctor` to verify RSS connectivity.

## Output defaults

Matches other Printing Press CLIs (e.g. `drudgereport`, `hackernews`):

- **Piped stdout** (agent/bash tools): JSON automatically — no `--json` required
- **Interactive terminal**: plain table unless you pass `--json` or `--agent`

## Agent Mode

Add `--agent` to any command. Expands to: **`--json --compact --no-input --no-color --yes`**.

- **Pipeable** — JSON on stdout, errors on stderr
- **Compact** — keeps `title`, `link`, `published`, `section` on headlines (drops `categories`, `description`, `guid` unless `--select` overrides)
- **Filterable** — `--select` wins over `--compact` for explicit field lists
- **Previewable** — `--dry-run` validates flags without fetching
- **Read-only** — never mutates remote state

### Response envelope

Machine JSON wraps data like Drudge store-backed reads:

```json
{
  "meta": {"source": "live", "feed_url": "...", "section": "latest", "count": 10},
  "results": [ {"title": "...", "link": "...", "published": "...", "section": "latest"} ]
}
```

Parse **`.results`** for rows and **`.meta.source`** / **`.meta.feed_url`** for provenance. `sections` uses `"source": "catalog"`; `doctor` returns a status object in `results`.

```bash
foxnews-pp-cli headlines --section latest --limit 10 --agent
```

## Install skill only

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-foxnews -g -y
```
