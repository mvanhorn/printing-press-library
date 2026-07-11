---
name: pp-quranku
description: "Every Al-Qur'an feature the app has, plus the Indonesian Tarjamah Tafsiriyah offline, a local database, offline search, a khatam plan, and a hifz tracker no other Qur'an tool has. Trigger phrases: `read surah`, `tarjamah tafsiriyah`, `cari ayat`, `verse of the day`, `khatam plan`, `use quranku`, `run quranku`."
author: "erikgunawans"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - quranku-pp-cli
    install:
      - kind: go
        bins: [quranku-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/education/quranku/cmd/quranku-pp-cli
---

# QuranKu — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `quranku-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install quranku --cli-only
   ```
2. Verify: `quranku-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/education/quranku/cmd/quranku-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

QuranKu carries Ustadz Muhammad Thalib's Tarjamah Tafsiriyah translation alongside Quran.com's metadata and audio, all in a local SQLite store. Read any surah, look up any verse, search offline with `find`, follow a `plan` to finish the Qur'an, and track memorization with `hifz`.

## When to Use This CLI

Use QuranKu when an agent or user needs Qur'an text with the Indonesian Tarjamah Tafsiriyah, offline verse search, precise ayah lookups, or personal reading/memorization tracking. It is the only tool that carries this specific translation with a local database.

## Anti-triggers

Do not use this CLI for:
- Do not use for hadith, fiqh rulings, or non-Qur'an Islamic texts.
- Do not use for prayer times or qibla direction.
- Do not use to modify or publish Qur'an content upstream — it is read-only plus local personal state.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`find`** — Search the Qur'an offline by word or meaning across Arabic and the Indonesian Tafsiriyah.

  _Reach for this when you need to locate verses by meaning without a live search API._

  ```bash
  quranku-pp-cli find "kasih sayang" --json
  ```
- **`verse`** — Get a single verse (e.g. 2:255) with Arabic and the Tafsiriyah translation joined together.

  _Use for a precise ayah lookup instead of fetching a whole surah._

  ```bash
  quranku-pp-cli verse 2:255 --json
  ```
- **`daily`** — A stable, date-seeded verse of the day with its Tafsiriyah translation.

  _Use for a reproducible daily reflection verse._

  ```bash
  quranku-pp-cli daily --json
  ```
- **`random`** — A random verse with its Tafsiriyah translation for reflection.

  _Use for a spontaneous verse for study or reflection._

  ```bash
  quranku-pp-cli random --json
  ```

### Personal practice tracking
- **`plan`** — Build and follow a plan to finish the Qur'an in N days, tracking progress locally.

  _Use to pace and track a full Qur'an reading over time._

  ```bash
  quranku-pp-cli plan start --days 30
  ```
- **`hifz`** — Mark and review Qur'an memorization progress per surah or verse.

  _Use to track what you have memorized across sessions._

  ```bash
  quranku-pp-cli hifz mark 1 --json
  ```
- **`bookmark`** — Save verses with personal notes for later, offline.

  _Use to keep a personal, portable set of saved verses._

  ```bash
  quranku-pp-cli bookmark add 2:255 --note "Ayat Kursi"
  ```

## Command Reference

**audio** — Per-surah reciter audio manifest (quranapi)

- `quranku-pp-cli audio <surah>` — Get the reciter->audio-URL map for a surah

**chapters** — Chapter metadata from Quran.com

- `quranku-pp-cli chapters get` — Get background/introduction text for a chapter
- `quranku-pp-cli chapters list` — List chapters with localized metadata

**juz** — Juz (para) structure from Quran.com

- `quranku-pp-cli juz` — List all 30 juz with their verse mappings

**recitation** — Chapter recitation audio from Quran.com

- `quranku-pp-cli recitation <reciter> <chapter>` — Get the audio file URL for a chapter by reciter

**surah** — Surahs with the Indonesian Tarjamah Tafsiriyah translation (primary source)

- `quranku-pp-cli surah get` — Get a surah with all verses and the Tarjamah Tafsiriyah translation
- `quranku-pp-cli surah list` — List all 114 surahs (Tafsiriyah source)

**tafsirs** — Available tafsir resources on Quran.com

- `quranku-pp-cli tafsirs` — List available tafsirs (other than Tafsiriyah)

**uthmani** — Uthmani-script Arabic text from Quran.com

- `quranku-pp-cli uthmani` — Get clean Uthmani-script Arabic for a chapter

**verses** — Qur'an verses (Arabic) from Quran.com

- `quranku-pp-cli verses <chapter>` — List verses of a chapter with Arabic text

**websearch** — Live online full-text search across the whole Qur'an (Quran.com)

- `quranku-pp-cli websearch` — Search the Qur'an by word or meaning


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
quranku-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Read a surah with translation

```bash
quranku-pp-cli surah get 36 --json --select data.verses.translations.terjemahTafsiriyah
```

Fetch Yasin and narrow the payload to just the Tafsiriyah translation lines.

### Offline meaning search

```bash
quranku-pp-cli find "sabar" --json
```

Find verses mentioning patience across the local Arabic + Tafsiriyah index.

### Start a 30-day khatam

```bash
quranku-pp-cli plan start --days 30
```

Create a reading plan that finishes the Qur'an in 30 days and tracks progress.

## Auth Setup

No authentication required.

Run `quranku-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  quranku-pp-cli chapters list --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `QURANKU_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `QURANKU_CONFIG_DIR`, `QURANKU_DATA_DIR`, `QURANKU_STATE_DIR`, `QURANKU_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `QURANKU_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `quranku-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "quranku": {
        "command": "quranku-pp-mcp",
        "env": {
          "QURANKU_HOME": "/srv/quranku"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `QURANKU_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `QURANKU_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
quranku-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
quranku-pp-cli feedback --stdin < notes.txt
quranku-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `QURANKU_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `QURANKU_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
quranku-pp-cli profile save briefing --json
quranku-pp-cli --profile briefing chapters list
quranku-pp-cli profile list --json
quranku-pp-cli profile show briefing
quranku-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `quranku-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/education/quranku/cmd/quranku-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add quranku-pp-mcp -- quranku-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which quranku-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   quranku-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `quranku-pp-cli <command> --help`.
