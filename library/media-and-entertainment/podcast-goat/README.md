# Podcast GOAT CLI

**Pull long-form podcast transcripts as speaker-labeled markdown — cookie-first across the four major paid publishers, free everywhere else, paid only when nothing else works.**

Built for agentic users who already pay for Huberman, Acquired, Founders, and Peter Attia and want to feed those transcripts into Claude or Hermes without copy-pasting. Walks a cookie -> free -> paid dispatch chain across 10 sources, normalizes everything to the same `**Speaker** (MM:SS)` markdown shape, caches to a local FTS5 store, and ships an MCP wrapper so agents can drive the whole thing.

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `podcast-goat-pp-cli` binary and the `pp-podcast-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install podcast-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install podcast-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install podcast-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install podcast-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install podcast-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/cmd/podcast-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/podcast-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-podcast-goat --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-podcast-goat --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install podcast-goat --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
podcast-goat-pp-cli auth login-service
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/podcast-goat-current).
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
    "podcast-goat": {
      "command": "podcast-goat-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Three auth surfaces, in cost order. (1) `auth login-service --service <huberman|acquired|founders|peterattia>` extracts your logged-in Chrome cookie once and stores it locally — the headline workflow. (2) Free sources (Dwarkesh Substack, Podcasting 2.0 RSS transcripts, yt-dlp auto-subs) need no auth. (3) Paid sources (spoken.md `SPOKEN_API_KEY`, Taddy `TADDY_API_KEY`+`TADDY_USER_ID`, audio providers like ElevenLabs/OpenAI/Deepgram) are scoped to commands you explicitly opt into with `--paid` or `--provider <name>`. spoken.md's `pt_demo` key works without signup.

## Quick Start

```bash
# Confirm yt-dlp present, RSS reachability, cookie freshness
podcast-goat-pp-cli doctor

# Free path; canonical markdown straight to cache
podcast-goat-pp-cli episode get https://www.dwarkesh.com/p/andrej-karpathy

# One-time cookie capture from your logged-in Chrome
podcast-goat-pp-cli auth login-service --service huberman

# Cookie path; free because you subscribe
podcast-goat-pp-cli episode get https://www.hubermanlab.com/episode/<your-premium-slug>

# Bundle cached transcripts into one prompt-shaped file
podcast-goat-pp-cli magic 'AI chip supply chain' --out chips.md

```

## What works today vs. what's coming in v0.2

**Working in v0.1** — verified live 2026-05-17:

- **Spotify** — `auth login-service --service spotify` captures sp_dc once; adapter then auto-bootstraps a fresh Bearer via Spotify's TOTP-signed token endpoint on every fetch (uses the secret version in `internal/source/spotify/totp.go`; v0.2 will refresh at install time). `SPOTIFY_BEARER` env var works as a manual override. Live-verified against "10 Years of Acquired (with Michael Lewis)" — 5083 lines / 202KB.
- **Dwarkesh** — direct HTML scrape, no auth.
- **YouTube** — yt-dlp subprocess; auto-downloads yt-dlp on first call to `~/.config/podcast-goat/bin/` if not on PATH (~35MB one-time). Speaker diarization is single-speaker per channel (yt-dlp auto-subs limitation).
- **spoken.md** — universal paid path; takes any URL, returns named-speaker markdown. Needs `SPOKEN_API_KEY` (or `pt_demo` for evaluation only).
- **RSS Podcasting 2.0** — `<podcast:transcript>` tag MIME-routed; works for shows that advertise it (~1% of major podcasts).
- **Taddy** — GraphQL `getEpisodeTranscript`; needs `TADDY_API_KEY` + `TADDY_USER_ID`.

**Deferred to v0.2** — each one ships a clean typed `NotImplementedError` today with a remediation hint pointing at the workaround:

- **`huberman` / `acquired` / `founders` / `peterattia` HTML parsers** — cookie capture and authenticated GET both work (`auth login-service --service <name>` writes `~/.config/podcast-goat/cookies/cookies-<service>.json`, the adapter loads it and fires the authenticated request). The HTML-to-segment parser awaits first-time browser capture from a logged-in session to calibrate the per-publisher shape. Until then, most of these shows' free episodes are available via the **Spotify** path above.
- **`--bilingual zh-Hans,en` aligner** — the flag is wired but errors with a deferral message. v0.1 yt-dlp ships the English path; v0.2 adds Chinese + auto-translation.
- **`whisperapi` audio extraction** — provider switch (`--provider-name elevenlabs|openai|deepgram`) and key checks are live, but the yt-dlp audio extract → upload → diarize pipeline ships in v0.2. Use `--provider spoken` or `--provider taddy` for paid fallback today.
- **Chrome App-Bound v10 cookie strip** — Chrome 127+ App-Bound encryption requires a 32-byte host-prefix strip on the CDP path; lands in v0.2 alongside the cookie-tier HTML parsers that need it.
- **Persisted Spotify bearer cache** — the TOTP-bootstrapped bearer is cached in-memory for its ~1h TTL. Within one process (MCP server, scripted batch) the cache survives across fetches; across CLI invocations each `episode get` re-bootstraps (a few hundred ms). v0.2 adds on-disk persistence so even one-shot CLI calls hit a warm cache.

## Unique Features

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

## Usage

Run `podcast-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### episode

Pull, search, and inspect podcast episode transcripts

- **`podcast-goat-pp-cli episode get`** - Fetch one transcript by URL via the cookie -> free -> paid dispatch chain
- **`podcast-goat-pp-cli episode latest`** - Pull the most recent episode for a subscribed feed

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
podcast-goat-pp-cli episode get mock-value

# JSON for scripting and agents
podcast-goat-pp-cli episode get mock-value --json

# Filter to specific fields
podcast-goat-pp-cli episode get mock-value --json --select id,name,status

# Dry run — show the request without sending
podcast-goat-pp-cli episode get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
podcast-goat-pp-cli episode get mock-value --agent
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
podcast-goat-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/podcast-goat/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SPOKEN_API_KEY` | per_call | Yes | Set to your API credential. |
| `TADDY_API_KEY` | per_call | Yes | Set to your API credential. |
| `TADDY_USER_ID` | per_call | Yes | Set to your API credential. |
| `OPENAI_API_KEY` | per_call | Yes | Set to your API credential. |
| `DEEPGRAM_API_KEY` | per_call | Yes | Set to your API credential. |
| `ELEVENLABS_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `podcast-goat-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SPOKEN_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`episode get` returns nothing on a member URL** — Run `auth services` to confirm the service cookie isn't stale; re-run `auth login-service --service <name>` if so.
- **yt-dlp YouTube path fails with 'no subtitles'** — Some YouTube videos have no auto-subs. Use `--provider whisper --provider-name elevenlabs` to transcribe from audio (requires `ELEVENLABS_API_KEY`).
- **Paid fallback fires when you expected cookie hit** — Run `episode get <url> --explain` to see the dispatcher trace — usually a cookie expiry or a URL host that doesn't match a known publisher.
- **Bilingual alignment looks off** — yt-dlp auto-translate quality varies. Pass `--align greedy|exact` to switch alignment strategy; `exact` requires both tracks to have matching segment counts.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**podscript**](https://github.com/timf34/podscript) — Python
- [**summarize**](https://github.com/steipete/summarize) — Swift
- [**youwhisper-cli**](https://github.com/FlyingFathead/youwhisper-cli) — Python
- [**yt-transcript**](https://github.com/kiuckhuang/yt-transcript) — Python
- [**faster-whisper-skill**](https://github.com/theplasmak/faster-whisper) — Python
- [**yt-dlp**](https://github.com/yt-dlp/yt-dlp) — Python
- [**spoken.md (vendor)**](https://spoken.md/) — API
- [**Taddy podcast-api (vendor)**](https://taddy.org/developers/podcast-api/episode-transcripts) — API

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
