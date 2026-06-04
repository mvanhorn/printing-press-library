# AnkiWeb CLI

**The only terminal-native way to search, rank, and download AnkiWeb shared decks — and read your cloud-synced decks — with an offline catalog no other Anki tool has.**

Every existing Anki CLI wraps the desktop AnkiConnect add-on; none touches ankiweb.net. AnkiWeb CLI talks directly to the website's service layer, decodes its protobuf responses, and keeps a local SQLite catalog so you can rank decks by approval rate, filter by audio coverage, compare candidates side by side, and watch for new decks since your last sync.

Learn more at [AnkiWeb](https://ankiweb.net).

Created by [@paulb](https://github.com/paulb) (Paul Bockewitz).

## Install

The recommended path installs both the `ankiweb-pp-cli` binary and the `pp-ankiweb` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ankiweb
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ankiweb --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ankiweb --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ankiweb --agent claude-code
npx -y @mvanhorn/printing-press-library install ankiweb --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/education/ankiweb/cmd/ankiweb-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ankiweb-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ankiweb --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-ankiweb --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install ankiweb --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
ankiweb-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ankiweb-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ankiweb": {
      "command": "ankiweb-pp-mcp"
    }
  }
}
```

</details>

## Authentication

AnkiWeb uses a session cookie, not an API key. Run `auth login --chrome` to import your logged-in ankiweb.net session, or set ANKIWEB_COOKIES. Public shared-deck search and info need no login.

The editor commands (`notetypes`, `notes add`) run on **ankiuser.net**, which AnkiWeb authenticates with a **separate** session cookie from ankiweb.net. Set it via `ANKIUSER_COOKIES` (or the config `ankiuser_cookies` key): open `https://ankiuser.net` while logged in and copy that domain's `ankiweb` cookie. The ankiweb.net cookie does not work for these commands (the editor returns HTTP 404).

## Quick Start

```bash
# Search the public shared-deck catalog — no login needed.
ankiweb-pp-cli shared search spanish

# Rank results by approval rate to find the best deck.
ankiweb-pp-cli shared rank spanish --min-votes 20

# See full detail and reviews for one deck.
ankiweb-pp-cli shared info 241428882

# Import your ankiweb.net session for your own decks.
ankiweb-pp-cli auth login --chrome

# List your cloud-synced decks and card counts.
ankiweb-pp-cli decks list

```

## Known Gaps

- **`sync` command is a catalog-only sync** — `ankiweb-pp-cli sync` populates the local SQLite catalog with shared-deck data. Your personal card data (`cards search`, `decks list`) comes directly from AnkiWeb's live API on each call and is not cached locally. This is a structural limitation of AnkiWeb's API, which does not expose a bulk-export endpoint for personal card data.
- **`drift` command reports unsupported** — The `drift` command tracks download-count changes on decks you've published, but AnkiWeb's available endpoints do not expose per-owner download history. The command returns a clear `supported: false` response rather than fabricated data.
- **AnkiWeb rate limits unauthenticated shared-deck searches** — Repeated rapid searches against `/svc/shared/list-decks` trigger HTTP 429 with a "please log in" message. Run `ankiweb-pp-cli auth login --chrome` to use an authenticated session, or add a `--rate-limit 0.5` flag to slow requests.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Catalog intelligence the website can't do
- **`shared rank`** — Rank shared decks by approval rate (upvotes vs downvotes) with a minimum-vote floor, instead of the raw vote counts the website shows.

  _Pick the highest-quality deck for a topic in one command instead of eyeballing vote counts deck by deck._

  ```bash
  ankiweb-pp-cli shared rank spanish --min-votes 20 --agent
  ```
- **`shared search`** — Filter shared decks by whether they include audio or images, critical for language learners.

  _Surface only media-rich decks when audio matters (language study) without opening each deck._

  ```bash
  ankiweb-pp-cli shared search japanese --has-audio --agent
  ```
- **`compare`** — Compare multiple shared decks in one table: approval rate, note count, audio/image coverage, and freshness.

  _Decide between near-duplicate decks at a glance instead of flipping between tabs._

  ```bash
  ankiweb-pp-cli compare 241428882 815543631 --agent
  ```
- **`shared fresh`** — Rank or filter shared decks by last-modified date to surface actively maintained decks.

  _Avoid stale abandoned decks by finding the ones updated recently._

  ```bash
  ankiweb-pp-cli shared fresh anatomy --since 2024-01-01 --agent
  ```

### Local state that compounds
- **`watch`** — Show shared decks that are new or changed for a search term since your last sync.

  _Re-run a weekly topic search and see only what's new instead of re-scanning the whole list._

  ```bash
  ankiweb-pp-cli watch spanish --since-last-sync --agent
  ```
- **`drift`** — Track download-count changes on the decks you've published, between syncs.

  _See whether your published decks are gaining traction over time without manual note-taking._

  ```bash
  ankiweb-pp-cli drift --agent
  ```
- **`brief`** — One digest for a topic: top decks by approval rate, audio coverage, the freshest deck, and how many are new since last sync.

  _Get a complete read on a topic's deck landscape in one call instead of running four commands._

  ```bash
  ankiweb-pp-cli brief spanish --agent
  ```

## Recipes

### Find the best audio-rich Spanish deck

```bash
ankiweb-pp-cli shared rank spanish --has-audio --min-votes 20 --agent --select decks.title,decks.approval,decks.notes
```

Ranks audio-bearing Spanish decks by approval rate and narrows the JSON to just title, approval, and note count.

### Compare three anatomy decks

```bash
ankiweb-pp-cli compare 241428882 815543631 1713698257 --agent
```

One table of approval rate, notes, audio/image coverage, and freshness across the three deck ids.

### What anatomy decks are new this week

```bash
ankiweb-pp-cli watch anatomy --since-last-sync --agent
```

Diffs the current catalog against your last sync and lists only new or changed decks.

### Topic briefing

```bash
ankiweb-pp-cli brief japanese --agent
```

Top decks by approval, audio coverage percentage, the freshest deck, and new-since-sync count in one digest.

## Usage

Run `ankiweb-pp-cli --help` for the full command reference and flag list.

## Commands

### decks

Your cloud-synced decks and study stats (requires AnkiWeb login)

- **`ankiweb-pp-cli decks`** - List your synced decks with card counts and study stats (protobuf response; 200 with session cookie, 403 without)

### shared

Browse, search, and download public shared decks (no login required)

- **`ankiweb-pp-cli shared download`** - Download a shared deck .apkg. NOTE: requires a signed ?t= token minted client-side (op=sdd); without it the endpoint returns 400/503.
- **`ankiweb-pp-cli shared info`** - Full detail + reviews for one shared deck (protobuf response)
- **`ankiweb-pp-cli shared search`** - Search the shared-deck catalog by keyword (protobuf response)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ankiweb-pp-cli decks

# JSON for scripting and agents
ankiweb-pp-cli decks --json

# Filter to specific fields
ankiweb-pp-cli decks --json --select id,name,status

# Dry run — show the request without sending
ankiweb-pp-cli decks --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ankiweb-pp-cli decks --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
ankiweb-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ankiweb-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ANKIWEB_COOKIES` | per_call | Yes | ankiweb.net session cookie (`ankiweb=…`). Used by all commands except the editor. |
| `ANKIUSER_COOKIES` | per_call | For editor | ankiuser.net session cookie, required by `notetypes` and `notes add`. Distinct from `ANKIWEB_COOKIES`. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ankiweb-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ANKIWEB_COOKIES`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **decks list returns 403 / not authenticated** — Run `ankiweb-pp-cli auth login --chrome` to import your ankiweb.net session cookie, or set ANKIWEB_COOKIES.
- **notetypes / notes add say "no ankiuser.net session cookie configured"** — These run on ankiuser.net, which needs a separate cookie. Open `https://ankiuser.net` while logged in, copy that domain's `ankiweb` cookie, and set `ANKIUSER_COOKIES='ankiweb=…'`.
- **shared download fails with a token error** — Deck download requires a signed token AnkiWeb mints in-browser; this is a known limitation — download the deck from ankiweb.net directly for now.
- **search returns nothing** — An empty search term returns no results; provide a keyword, e.g. `shared search spanish`.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://ankiweb.net/shared/decks
- Capture coverage: 5 API entries from 6 total network entries
- Reachability: standard_http (85% confidence)
- Protocols: protobuf (95% confidence), sveltekit_spa (90% confidence)
- Auth signals: cookie — cookies: has_auth
- Generation hints: protobuf_responses_require_handwritten_decoders, cookie_auth_validated, download_requires_signed_token
- Candidate command ideas: search — GET /svc/shared/list-decks?search= returns repeated deck protobuf {id,title,upvotes,downvotes,modified,notes,audio,images}; info — GET /svc/shared/item-info?sharedId= returns full deck detail + reviews protobuf; download — GET /svc/shared/download-deck/{id}?t= requires client-minted signed token (op=sdd); token generation unresolved; list — POST /svc/decks/deck-list-info returns user's synced decks; cookie-gated

Warnings from discovery:
- : All API responses are protobuf, not JSON; generated JSON decoding will not work without hand-written wire-format readers.
- : Shared-deck download needs a signed ?t= token minted by AnkiWeb client JS; not reproducible from captured traffic alone.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
