# Lichess CLI

## Safety scope

This CLI is intentionally limited to profile lookup, bounded completed-game
review, puzzle practice, and explicit named-player challenges. It does not
expose Board/Bot moves, live-game analysis, local engine analysis, messaging,
or mass puzzle enumeration. Never use it for assistance during an ongoing game.

# Introduction
Welcome to the reference for the Lichess API! Lichess is free/libre,
open-source chess server powered by volunteers and donations.
- Get help in the [Lichess Discord channel](https://discord.gg/lichess)
- API demo app with OAuth2 login and gameplay: [source](https://github.com/lichess-org/api-demo) / [demo](https://lichess-org.github.io/api-demo/)
- API UI app with OAuth2 login and endpoint forms: [source](https://github.com/lichess-org/api-ui) / [website](https://lichess.org/api/ui)
- [Contribute to this documentation on Github](https://github.com/lichess-org/api)
- Check out [Lichess widgets to embed in your website](https://lichess.org/developers)
- [Download all Lichess rated games](https://database.lichess.org/)
- [Download all Lichess puzzles with themes, ratings and votes](https://database.lichess.org/#puzzles)
- [Download all evaluated positions](https://database.lichess.org/#evals)

## Endpoint
All requests go to `https://lichess.org` (unless otherwise specified).

## Clients
- [Python general API](https://github.com/lichess-org/berserk)
- [MicroPython general API](https://github.com/mkomon/uberserk)
- [Python general API - async](https://pypi.org/project/async-lichess-sdk)
- [Python Lichess Bot](https://github.com/lichess-bot-devs/lichess-bot)
- [Python Board API for Certabo](https://github.com/haklein/certabo-lichess)
- [Java general API](https://github.com/tors42/chariot)
- [JavaScript & TypeScript general API](https://github.com/devjiwonchoi/equine)
- [Rust general API](https://github.com/obazin/litchee)
- [LichessNET - C# API Wrapper](https://github.com/Rabergsel/LichessNET)
- [.NET general API](https://github.com/Dblike/LichessSharp)

## Rate limiting
All requests are rate limited using various strategies,
to ensure the API remains responsive for everyone.
Only make one request at a time.
If you receive an HTTP response with a [429 status](https://en.wikipedia.org/wiki/List_of_HTTP_status_codes#429),
you have exceded one of the rate limits.
In most cases, waiting one minute before retrying will be sufficient, but some limits may require longer.
Reduce your request frequency before retrying.

## Streaming with ND-JSON
Some API endpoints stream their responses as [Newline Delimited JSON a.k.a. **nd-json**](https://github.com/ndjson/ndjson-spec), with one JSON object per line.

Here's a [JavaScript utility function](https://gist.github.com/ornicar/a097406810939cf7be1df8ea30e94f3e) to help reading NDJSON streamed responses.

## Authentication
### Which authentication method is right for me?
[Read about the Lichess API authentication methods and code examples](https://github.com/lichess-org/api/blob/master/example/README.md)

### Personal Access Token
Personal API access tokens allow you to quickly interact with Lichess API without going through an OAuth flow.
- [Generate a personal access token](https://lichess.org/account/oauth/token)
- `curl https://lichess.org/api/account -H "Authorization: Bearer {token}"`
- [NodeJS example](https://github.com/lichess-org/api/tree/master/example/oauth-personal-token)

### Token Security
- Keep your tokens secret. Do not share them in public repositories or public forums.
- Your tokens can be used to make your account perform arbitrary actions (within the limits of the tokens' scope). You remain responsible for all activities on your account.
- Do not hardcode tokens in your application's code. Use environment variables or a secure storage and ensure they are not shipped/exposed to users. Be especially careful that they are not included in frontend bundles or apps that are shipped to users.
- If you suspect a token has been compromised, revoke it immediately.

To see your active tokens or revoke them, see [your Personal API access tokens](https://lichess.org/account/oauth/token).

### Authorization Code Flow with PKCE
The authorization code flow with PKCE allows your users to **login with Lichess**.
Lichess supports unregistered and public clients (no client authentication, choose any unique client id).
The only accepted code challenge method is `S256`.
Access tokens are long-lived (expect one year), unless they are revoked.
Refresh tokens are not supported.

See the [documentation for the OAuth endpoints](#tag/OAuth) or
the [PKCE RFC](https://datatracker.ietf.org/doc/html/rfc7636#section-4) for a precise protocol description.

- [Demo app](https://lichess-org.github.io/api-demo/)
- [Minimal client-side example](https://github.com/lichess-org/api/tree/master/example/oauth-app)
- [Flask/Python example](https://github.com/lakinwecker/lichess-oauth-flask)
- [Java example](https://github.com/tors42/lichess-oauth-pkce-app)
- [NodeJS Passport strategy to login with Lichess OAuth2](https://www.npmjs.com/package/passport-lichess)

#### Real life examples
- [PyChess](https://github.com/gbtami/pychess-variants) ([source code](https://github.com/gbtami/pychess-variants))
- [Lichess4545](https://www.lichess4545.com/) ([source code](https://github.com/cyanfish/heltour))
- [English Chess Federation](https://ecf.octoknight.com/)
- [Rotherham Online Chess](https://rotherhamonlinechess.azurewebsites.net/tournaments)

### Token format
Access tokens and authorization codes match `^[A-Za-z0-9_]+$`.
The length of tokens can be increased without notice. Make sure your application can handle at least 512 characters.
By convention tokens have a recognizable prefix, but do not rely on this.

Learn more at [Lichess](https://lichess.org/api).

Created by [@Avanderheyde](https://github.com/Avanderheyde) (avanderheyde).

## Install

The recommended path installs both the `lichess-pp-cli` binary and the `pp-lichess` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install lichess
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install lichess --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install lichess --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install lichess --agent claude-code
npx -y @mvanhorn/printing-press-library install lichess --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lichess/cmd/lichess-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lichess-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install lichess --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-lichess --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-lichess --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install lichess --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lichess-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `LICHESS_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lichess/cmd/lichess-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "lichess": {
      "command": "lichess-pp-mcp",
      "env": {
        "LICHESS_OAUTH2": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
lichess-pp-cli user lichess --json

lichess-pp-cli challenge ten a-named-player --dry-run --json

lichess-pp-cli loss-patterns a-named-player --dry-run --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`challenge ten`** — Create an explicit 10+0 Lichess challenge for a named player only after --send is supplied.
- **`loss-patterns`** — Mechanically group official completed-game judgments by opening, performance, and phase.
- **`training-brief`** — Combine official puzzle-theme performance with local loss-pattern evidence while preserving the absence of a causal mapping.

## Recipes

### 

```bash
lichess-pp-cli training-brief --dry-run --json
```

## Usage

Run `lichess-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `LICHESS_CONFIG_DIR`, `LICHESS_DATA_DIR`, `LICHESS_STATE_DIR`, or `LICHESS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `LICHESS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export LICHESS_HOME=/srv/lichess
lichess-pp-cli doctor
```

Under `LICHESS_HOME=/srv/lichess`, the four dirs resolve to `/srv/lichess/config`, `/srv/lichess/data`, `/srv/lichess/state`, and `/srv/lichess/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "lichess": {
      "command": "lichess-pp-mcp",
      "env": {
        "LICHESS_HOME": "/srv/lichess"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `LICHESS_DATA_DIR` overrides an explicit `--home` for that kind. Use `LICHESS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `LICHESS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `lichess-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Read and write account information and preferences.
<https://lichess.org/account/preferences/game-display>

- **`lichess-pp-cli account`** - Public information about the logged in user.

### challenge

Create only an explicit named-player 10+0 challenge. The CLI never accepts,
plays, or analyzes a live game.

- **`lichess-pp-cli challenge ten <username> --send`** - Sends one 10+0 challenge only after explicit confirmation. Omit `--send` to inspect the request plan.

For bounded retrospective review, use **`lichess-pp-cli loss-patterns <username> --max 30`**. It only reads completed games and Lichess-provided judgments.

### player

Manage player

- **`lichess-pp-cli player`** - Provides autocompletion options for an incomplete username.

### puzzle

Fetch and solve [puzzles](https://lichess.org/training), view your puzzle history and dashboard.

Our collection of puzzles is in the public domain, you can [download it here](https://database.lichess.org/#puzzles).
For a list of our [puzzle themes](https://lichess.org/training/themes) with their description, check out the [theme translation file](https://github.com/ornicar/lila/blob/master/translation/source/puzzleTheme.xml).

- **`lichess-pp-cli puzzle dashboard`** - Download your [puzzle dashboard](https://lichess.org/training/dashboard/30/dashboard) as JSON.
Also includes all puzzle themes played, with aggregated results.
Allows re-creating the [improvement/strengths](https://lichess.org/training/dashboard/30/improvementAreas) interfaces.
Use **`lichess-pp-cli training-brief`** for one theme-guided practice follow-up. It does not expose standalone puzzle enumeration.

### user

Access registered users on Lichess.
<https://lichess.org/player>

- Each user blog exposes an atom (RSS) feed, like <https://lichess.org/@/thibault/blog.atom>
- User blogs mashup feed: https://lichess.org/blog/community.atom
- User blogs mashup feed for a language: https://lichess.org/blog/community/fr.atom

- **`lichess-pp-cli user <username>`** - Read public data of a user.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`lichess-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`lichess-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`lichess-pp-cli learnings list`** - Inspect taught rows
- **`lichess-pp-cli learnings forget <query>`** - Undo a teach
- **`lichess-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`lichess-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`lichess-pp-cli teach-pattern`** - Install a query/resource template up front
- **`lichess-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `LICHESS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `lichess-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
lichess-pp-cli account

# JSON for scripting and agents
lichess-pp-cli account --json

# Filter to specific fields
lichess-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
lichess-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
lichess-pp-cli account --agent
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
lichess-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `lichess-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/lichess-org-reference-pp-cli/config.toml`; `--home`, `LICHESS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `LICHESS_OAUTH2` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `lichess-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `lichess-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $LICHESS_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tamnd/lichess-cli**](https://github.com/tamnd/lichess-cli) — Go
- [**karayaman/lichess-mcp**](https://github.com/karayaman/lichess-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
