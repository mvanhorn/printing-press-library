# QuranKu CLI

**Every Al-Qur'an feature the app has, plus the Indonesian Tarjamah Tafsiriyah offline, a local database, offline search, a khatam plan, and a hifz tracker no other Qur'an tool has.**

QuranKu carries Ustadz Muhammad Thalib's Tarjamah Tafsiriyah translation alongside Quran.com's metadata and audio, all in a local SQLite store. Read any surah, look up any verse, search offline with `find`, follow a `plan` to finish the Qur'an, and track memorization with `hifz`.

## Install

The recommended path installs both the `quranku-pp-cli` binary and the `pp-quranku` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install quranku
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install quranku --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install quranku --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install quranku --agent claude-code
npx -y @mvanhorn/printing-press-library install quranku --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/education/quranku/cmd/quranku-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/quranku-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install quranku --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-quranku --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-quranku --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install quranku --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/quranku-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/education/quranku/cmd/quranku-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "quranku": {
      "command": "quranku-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the sources are reachable before syncing.
quranku-pp-cli doctor --dry-run

# Pull all 114 surahs (Arabic + Tafsiriyah) into the local store once.
quranku-pp-cli sync

# Read Al-Fatihah with the Tafsiriyah translation.
quranku-pp-cli surah get 1 --json

# Search offline across Arabic and the Tafsiriyah.
quranku-pp-cli find "kasih sayang" --json

```

## Unique Features

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

## Usage

Run `quranku-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `QURANKU_CONFIG_DIR`, `QURANKU_DATA_DIR`, `QURANKU_STATE_DIR`, or `QURANKU_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `QURANKU_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export QURANKU_HOME=/srv/quranku
quranku-pp-cli doctor
```

Under `QURANKU_HOME=/srv/quranku`, the four dirs resolve to `/srv/quranku/config`, `/srv/quranku/data`, `/srv/quranku/state`, and `/srv/quranku/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `QURANKU_DATA_DIR` overrides an explicit `--home` for that kind. Use `QURANKU_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `QURANKU_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `quranku-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### audio

Per-surah reciter audio manifest (quranapi)

- **`quranku-pp-cli audio <surah>`** - Get the reciter->audio-URL map for a surah

### chapters

Chapter metadata from Quran.com

- **`quranku-pp-cli chapters get`** - Get background/introduction text for a chapter
- **`quranku-pp-cli chapters list`** - List chapters with localized metadata

### juz

Juz (para) structure from Quran.com

- **`quranku-pp-cli juz`** - List all 30 juz with their verse mappings

### recitation

Chapter recitation audio from Quran.com

- **`quranku-pp-cli recitation <reciter> <chapter>`** - Get the audio file URL for a chapter by reciter

### surah

Surahs with the Indonesian Tarjamah Tafsiriyah translation (primary source)

- **`quranku-pp-cli surah get`** - Get a surah with all verses and the Tarjamah Tafsiriyah translation
- **`quranku-pp-cli surah list`** - List all 114 surahs (Tafsiriyah source)

### tafsirs

Available tafsir resources on Quran.com

- **`quranku-pp-cli tafsirs`** - List available tafsirs (other than Tafsiriyah)

### uthmani

Uthmani-script Arabic text from Quran.com

- **`quranku-pp-cli uthmani`** - Get clean Uthmani-script Arabic for a chapter

### verses

Qur'an verses (Arabic) from Quran.com

- **`quranku-pp-cli verses <chapter>`** - List verses of a chapter with Arabic text

### websearch

Live online full-text search across the whole Qur'an (Quran.com)

- **`quranku-pp-cli websearch`** - Search the Qur'an by word or meaning


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
quranku-pp-cli chapters list

# JSON for scripting and agents
quranku-pp-cli chapters list --json

# Filter to specific fields
quranku-pp-cli chapters list --json --select id,name,status

# Dry run — show the request without sending
quranku-pp-cli chapters list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
quranku-pp-cli chapters list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
quranku-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `quranku-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/quranku-pp-cli/config.toml`; `--home`, `QURANKU_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **find returns nothing** — Run `quranku-pp-cli sync` first to populate the local store.
- **surah get is slow or fails offline** — After `sync`, reads come from the local store; check the Tafsiriyah source with `doctor` if a fresh fetch fails.
