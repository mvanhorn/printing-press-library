---
name: pp-nasa-images
description: "Every NASA Image and Video Library endpoint, plus a local SQLite mirror, FTS search, resumable bulk download,... Trigger phrases: `NASA images`, `NASA image library`, `images.nasa.gov`, `NASA Apollo photos`, `Mars rover images`, `NASA video captions`, `download NASA images`, `use nasa-images`, `run nasa-images`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nasa-images-pp-cli
---

# NASA Image and Video Library — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nasa-images-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install nasa-images --cli-only
   ```
2. Verify: `nasa-images-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/nasa-images/cmd/nasa-images-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when an agent or workflow needs deterministic access to NASA's public-domain media archive — image search, asset rendition picking, caption extraction, bulk album archiving — without parsing prose. The local SQLite mirror makes repeat queries free and unlocks chronological/topic-timeline analyses the upstream API doesn't support. The MCP surface gives Claude agents typed tools for every endpoint plus the novel composed commands (assets best, captions fetch text, citation, etc.).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`download album`** — Bulk-download every asset in a curated NASA album with byte-range resume — re-runs pick up exactly where the last failed transfer left off.

  _When you need 200 NASA images for a deck or archive, this is the one command that finishes — flaky wifi, sleep-resume, mid-run cancellation all become non-events._

  ```bash
  nasa-images-pp-cli download album Apollo-at-50 --variant orig --resume --out ./apollo
  ```
- **`recent`** — FTS5 search over the synced local mirror (title, description, description_508, keywords, album), with chronological sort the upstream API doesn't expose.

  _When breaking news hits, you can pull the most recent NASA images on a topic in milliseconds — without paging through decades of archival results._

  ```bash
  nasa-images-pp-cli recent --q "perseverance" --sort date-desc --limit 20 --json
  ```
- **`center profile`** — Local aggregation over the synced cache: counts by media_type, year-bucket histogram, top keywords, and top photographers for one of the 11 NASA centers.

  _Journalists answer 'which center publishes this kind of image' before they search — saving search time on stories where a center is the obvious source._

  ```bash
  nasa-images-pp-cli center profile JPL --json
  ```
- **`unused-in`** — Lists nasa_ids in a curated album that haven't been downloaded yet locally — anti-join against the downloads ledger.

  _Plan next week's slides without picking the same image twice — without keeping a manual list of nasa_ids you've already used._

  ```bash
  nasa-images-pp-cli unused-in Apollo-at-50 --json
  ```
- **`timeline`** — Local GROUP BY strftime('%Y-%m', date_created) over FTS-matched rows; prints month-bucket counts showing when a topic got coverage.

  _Spot publishing patterns — quiet months, new-release bursts — that surface story angles a keyword search hides._

  ```bash
  nasa-images-pp-cli timeline --q "perseverance" --bucket month --json
  ```

### API indirections we follow for you
- **`captions fetch`** — Returns the captions file CONTENT (.srt or .vtt), not just the URL — with an optional `--format text` mode that strips cue numbers and timecodes for readable transcripts.

  _Agents pulling NASA video captions for analysis, search, or quote-finding skip an extra HTTP call and a parse step._

  ```bash
  nasa-images-pp-cli captions fetch jsc2022m000123 --format text --agent
  ```
- **`metadata fetch`** — Follows the /metadata/{id} indirection, fetches the metadata.json sidecar, flattens AVAIL:* and EXIF fields, and drops internal-path leak fields (SourceFile, File:Directory, AVAIL:Owner).

  _Journalists and educators get the bylined photo credit (photographer + center + date) without scrubbing noise fields by hand._

  ```bash
  nasa-images-pp-cli metadata fetch PIA24439 --json --select AVAIL:Title,AVAIL:Photographer,AVAIL:DateCreated
  ```

### Agent-native plumbing
- **`assets best`** — Parses an asset's rendition manifest, classifies each file by variant (orig/large/medium/small/thumb), applies a caller preference order with optional byte ceiling, and prints exactly one URL.

  _Claude agents in MCP can ask for 'the best version of nasa_id X under 5 MB' and get exactly one URL — no token spend on parsing filenames._

  ```bash
  nasa-images-pp-cli assets best as11-40-5874 --prefer orig,large,medium --max-bytes 5000000
  ```

### Domain-specific shortcuts
- **`citation`** — Generates a ready-to-paste citation string (APA / MLA / Chicago) from cached metadata (photographer, date, center, nasa_id, URL).

  _Drop a NASA image in a piece and paste the citation underneath — no formatting by hand._

  ```bash
  nasa-images-pp-cli citation PIA24439 --style apa
  ```

## Command Reference

**albums** — Retrieve the contents of a curated NASA album (e.g. Apollo-at-50, Mars-Perseverance)

- `nasa-images-pp-cli albums <album_name>` — Return paginated asset listing for a curated album. Album names are case-sensitive (Apollo-at-50 works, apollo-at-50...

**assets** — Retrieve the rendition manifest for an asset (every file URL: original/large/medium/small/thumb for images; mp4 variants for video; mp3/m4a for audio)

- `nasa-images-pp-cli assets <nasa_id>` — Return the list of every file URL for an asset

**captions** — Retrieve the location URL of a video asset's captions file (.srt or .vtt)

- `nasa-images-pp-cli captions <nasa_id>` — Return the location URL of the captions file. Video assets only — 400 for images, 404 for video without captions....

**media** — Search the NASA Image and Video Library catalog by free text and filters

- `nasa-images-pp-cli media` — Search the catalog by free text or filters. At least one query parameter is required.

**metadata** — Retrieve the location URL of an asset's metadata.json sidecar (AVAIL editorial + ExifTool fields)

- `nasa-images-pp-cli metadata <nasa_id>` — Return the location URL of the metadata.json sidecar for an asset. Use `metadata fetch` (novel command) to follow...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nasa-images-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `nasa-images-pp-cli download album <album_name>` — Bulk-download every asset in a curated album with byte-range resume — re-runs pick up exactly where the last...
- `nasa-images-pp-cli captions fetch <nasa_id>` — Fetch a video asset's caption file content (.srt or .vtt), not just its URL. With --format text, strips cue numbers...
- `nasa-images-pp-cli metadata fetch <nasa_id>` — Fetch an asset's full metadata sidecar content, flattened (AVAIL:* and EXIF fields). Drops leak fields (SourceFile,...
- `nasa-images-pp-cli assets best <nasa_id>` — Deterministic best-variant picker. Parses an asset's rendition manifest, applies a preference order, and prints...
- `nasa-images-pp-cli recent` — FTS5 full-text search over the local synced mirror, sorted chronologically (date_created descending) — the...
- `nasa-images-pp-cli center profile <center_code>` — Local aggregation per NASA center (media_type counts, year-bucket histogram, top keywords, top photographers) over...
- `nasa-images-pp-cli unused-in <album_name>` — List nasa_ids in a curated album that haven't been downloaded locally yet (anti-join against the downloads ledger)
- `nasa-images-pp-cli timeline` — Topic timeline histogram. Local GROUP BY year-month over FTS-matched rows; shows when a topic got coverage
- `nasa-images-pp-cli citation <nasa_id>` — Generate a ready-to-paste citation string (APA, MLA, or Chicago) from cached or fetched metadata

## Recipes


### Find a recent Mars image and pick the best variant

```bash
nasa-images-pp-cli media --q "mars rover" --year-start 2024 --media-type image --page-size 5 --json --select 'collection.items.data.nasa_id,collection.items.data.title' | jq -r '.collection.items[0].data[0].nasa_id' | xargs -I {} nasa-images-pp-cli assets best {} --prefer orig,large
```

Two-step pattern: search returns nasa_ids, assets best resolves the chosen one to a single URL.

### Archive a full curated album

```bash
nasa-images-pp-cli download album Webb-Telescope-First-Images --variant large --resume --out ./webb-firsts
```

Walks every page of the album, fetches the asset manifest per item, then byte-range-resumes each variant download.

### Pull a video transcript

```bash
nasa-images-pp-cli captions fetch jsc2022m000123 --format text > webb-deployment.txt
```

Caption indirection is two GETs; --format text strips SRT cue numbers for clean transcripts.

### Build an offline FTS search index

```bash
nasa-images-pp-cli mirror search --q "apollo" --year-start 1968 --year-end 1975 && nasa-images-pp-cli recent --q "bootprint" --sort date-desc --json --select 'nasa_id,title,date_created'
```

mirror search populates the SQLite mirror; recent uses FTS5 and adds chronological sort NASA's upstream search doesn't expose.

### Generate an APA citation

```bash
nasa-images-pp-cli citation PIA24439 --style apa
```

Pulls the metadata sidecar (cached or live) and formats a ready-to-paste citation string.

## Auth Setup

No authentication required.

Run `nasa-images-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nasa-images-pp-cli albums mock-value --agent --select id,name,status
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
nasa-images-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nasa-images-pp-cli feedback --stdin < notes.txt
nasa-images-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.nasa-images-pp-cli/feedback.jsonl`. They are never POSTed unless `NASA_IMAGES_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NASA_IMAGES_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
nasa-images-pp-cli profile save briefing --json
nasa-images-pp-cli --profile briefing albums mock-value
nasa-images-pp-cli profile list --json
nasa-images-pp-cli profile show briefing
nasa-images-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `nasa-images-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add nasa-images-pp-mcp -- nasa-images-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nasa-images-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nasa-images-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nasa-images-pp-cli <command> --help`.
