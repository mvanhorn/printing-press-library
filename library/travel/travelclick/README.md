# TravelClick CLI

**Search any TravelClick-powered hotel's own rates directly -- room types, rate plans, fee-inclusive pricing, and the cheapest night to book -- without opening a browser tab per property.**

TravelClick/iHotelier powers the direct-booking widget for thousands of independent and boutique hotels. This CLI calls the same JSON API the widget itself uses to search availability, scan a date range for the lowest rate, validate corporate/group codes, and compare several hotels side by side. Search and info only -- no reservation, no guest PII, no payment data ever touches it.

Learn more at [TravelClick](https://api.travelclick.com).

Created by [@enlewof](https://github.com/enlewof) (Allen Lew).

## Install

The recommended path installs both the `travelclick-pp-cli` binary and the `pp-travelclick` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install travelclick
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install travelclick --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install travelclick --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install travelclick --agent claude-code
npx -y @mvanhorn/printing-press-library install travelclick --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/travelclick/cmd/travelclick-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/travelclick-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install travelclick --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-travelclick --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-travelclick --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install travelclick --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/travelclick-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TRAVELCLICK_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/travelclick/cmd/travelclick-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "travelclick": {
      "command": "travelclick-pp-mcp",
      "env": {
        "TRAVELCLICK_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

TravelClick's widget mints a short-lived (~1 hour) OAuth2 client_credentials Bearer token client-side during page load. The exact token-mint endpoint was not isolated during discovery (it fires before any capture hook can attach), so this CLI cannot mint its own token yet -- capture one manually: open the hotel's booking page in Chrome, open DevTools' Network tab, filter for api.travelclick.com, click any request, and copy the Authorization header's value (everything after 'Bearer ') into TRAVELCLICK_TOKEN. Re-capture roughly every hour.

## Quick Start

```bash
# Health check that works without a token.
travelclick-pp-cli doctor --dry-run

# Look up Made Hotel NYC's profile, policies, and fees once TRAVELCLICK_TOKEN is set.
travelclick-pp-cli hotel 102306

# Search room types and rate plans for specific dates.
travelclick-pp-cli rates search 102306 --check-in 2026-09-15 --check-out 2026-09-18

# Scan two months for the cheapest night before committing to dates.
travelclick-pp-cli rates calendar 102306 --from 2026-09-01 --to 2026-10-31

# Save a memorable name for the hotel ID.
travelclick-pp-cli hotels alias add made-nyc 102306

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-hotel local queries
- **`rates compare`** — Check several boutique hotels for the same dates in one call, ranked by lowest fee-inclusive total.

  _Reach for this when the user names more than one property for the same trip instead of calling 'rates search' repeatedly._

  ```bash
  travelclick-pp-cli rates compare --hotels 102306,<id2>,<id3> --check-in 2026-09-15 --check-out 2026-09-18
  ```
- **`rates cheapest-night`** — Scan several hotels' calendars at once and return the single best hotel+date combination.

  _Reach for this when the user is flexible on both hotel and date, not just date._

  ```bash
  travelclick-pp-cli rates cheapest-night --hotels made-nyc,<alias2> --from 2026-09-01 --to 2026-10-31
  ```
- **`codes check-all`** — Test one corporate or group code against every saved hotel at once.

  _Reach for this when the user has a code but isn't sure which of their saved hotels honors it._

  ```bash
  travelclick-pp-cli codes check-all ACME2026 --type corporate --hotels made-nyc,<alias2>
  ```

### Local state that compounds
- **`hotels alias`** — Give a memorable name to a TravelClick hotel ID instead of memorizing a 6-digit number.

  _Reach for this before repeated lookups against the same property so later commands can take the alias instead of the raw ID._

  ```bash
  travelclick-pp-cli hotels alias add made-nyc 102306
  ```
- **`analytics price-drift`** — Track how a hotel's rates move over time from your own saved search history.

  _Reach for this after the user has run a few 'rates search --save' calls and wants to know if a rate went up or down._

  ```bash
  travelclick-pp-cli analytics price-drift --hotel 102306
  ```

## Recipes

### Cheapest room+rate for a stay

```bash
travelclick-pp-cli rates search 102306 --check-in 2026-09-15 --check-out 2026-09-18 --agent --select roomStays.roomTypes.roomTypeName,roomStays.roomTypes.averageRates.rate,roomStays.roomTypes.averageRates.rateExternalCode
```

Narrows the deeply-nested avail response to just room name, rate, and rate code per plan instead of the full policy/amenity/image payload.

### Find the cheapest night in a month

```bash
travelclick-pp-cli rates calendar 102306 --from 2026-09-01 --to 2026-09-30 --json
```

Returns the lowest rate for every day in the range so you can sort for the minimum.

### Check a corporate code before searching

```bash
travelclick-pp-cli codes validate-corporate 102306 ACME2026
```

Confirms whether a rate-access code is live for this hotel. **Validating a code does not
currently let you search rates for it** — see Known Gaps below.

### Compare three boutique hotels for the same weekend

```bash
travelclick-pp-cli rates compare --hotels 102306,<id2>,<id3> --check-in 2026-09-15 --check-out 2026-09-18
```

Fans out the search across hotels and ranks them by lowest fee-inclusive total.

## Usage

Run `travelclick-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TRAVELCLICK_CONFIG_DIR`, `TRAVELCLICK_DATA_DIR`, `TRAVELCLICK_STATE_DIR`, or `TRAVELCLICK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TRAVELCLICK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TRAVELCLICK_HOME=/srv/travelclick
travelclick-pp-cli doctor
```

Under `TRAVELCLICK_HOME=/srv/travelclick`, the four dirs resolve to `/srv/travelclick/config`, `/srv/travelclick/data`, `/srv/travelclick/state`, and `/srv/travelclick/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "travelclick": {
      "command": "travelclick-pp-mcp",
      "env": {
        "TRAVELCLICK_HOME": "/srv/travelclick"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TRAVELCLICK_DATA_DIR` overrides an explicit `--home` for that kind. Use `TRAVELCLICK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TRAVELCLICK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `travelclick-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### codes

Validate special rate codes (corporate/rate-access or group-attendee) against a hotel before applying them to a search.

- **`travelclick-pp-cli codes validate-corporate`** - Check whether a corporate / rate-access code is valid for this hotel. Returns 200 when valid; returns a structured error (surfaced by the CLI) when the code is unrecognized.
- **`travelclick-pp-cli codes validate-group`** - Check whether a group-attendee code is valid for this hotel. Returns 200 when valid; returns a structured error (surfaced by the CLI) when the code is unrecognized.

### hotel

Read-only hotel property information: address, policies, check-in/out times, and amenities. No PII, no reservation data.

- **`travelclick-pp-cli hotel <hotel_id>`** - Fetch a hotel's public profile: name, address, geolocation, check-in/out times, accepted cards, and textual policies (cancellation, pets, parking, resort/curation fees).

### rates

Search room availability and rate plans for specific dates, or scan a date range for the cheapest night.

- **`travelclick-pp-cli rates calendar`** - Scan a date range (up to ~60 days) for the lowest available rate per night -- use this to find the cheapest night to check in before running a full search.
- **`travelclick-pp-cli rates search`** - Search available rooms and rate plans for a hotel between check-in and check-out.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`travelclick-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`travelclick-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`travelclick-pp-cli learnings list`** - Inspect taught rows
- **`travelclick-pp-cli learnings forget <query>`** - Undo a teach
- **`travelclick-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`travelclick-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`travelclick-pp-cli teach-pattern`** - Install a query/resource template up front
- **`travelclick-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TRAVELCLICK_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `travelclick-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
travelclick-pp-cli rates search mock-value --check-in 2026-01-15 --check-out 2026-01-15

# JSON for scripting and agents
travelclick-pp-cli rates search mock-value --check-in 2026-01-15 --check-out 2026-01-15 --json
# Filter to specific fields
travelclick-pp-cli rates search mock-value --check-in 2026-01-15 --check-out 2026-01-15 --json --select timeSpan,roomTypes,allRoomTypes

# Dry run — show the request without sending
travelclick-pp-cli rates search mock-value --check-in 2026-01-15 --check-out 2026-01-15 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
travelclick-pp-cli rates search mock-value --check-in 2026-01-15 --check-out 2026-01-15 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
travelclick-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `travelclick-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/travelclick-pp-cli/config.toml`; `--home`, `TRAVELCLICK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TRAVELCLICK_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `travelclick-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `travelclick-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TRAVELCLICK_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 or a JWT validation error** — TRAVELCLICK_TOKEN has expired (~1 hour TTL) -- re-capture it from DevTools per the auth instructions above.
- **codes validate-corporate returns INVALID_CORP_ID** — The code isn't recognized by this specific hotel -- corporate/group codes are hotel-specific, not chain-wide. Try 'codes check-all' across your saved hotels instead.
- **rates calendar returns fewer days than requested** — TravelClick truncates a multi-room calendar scan to roughly 60 days -- split a longer range into two calls.

## Known Gaps

- **Auth token is captured manually, not minted automatically.** The booking widget mints its OAuth2 `client_credentials` Bearer token client-side during page load; the exact token-mint endpoint fires before any capture hook can attach and was not isolated during discovery. Until that's found, `TRAVELCLICK_TOKEN` must be re-captured from browser DevTools roughly every hour. See [Authentication](#authentication).
- **Travel-agency rate codes are not implemented.** Only `corporate` and `group` code types were confirmed live (`codes validate-corporate`, `codes validate-group`). The travel-agency path segment is unconfirmed -- a guessed value returned `UNSUPPORTED_CODE_TYPE` -- so no `codes validate-travel-agency` command was built rather than shipping a guess.
- **The successful (valid-code) response shape for `codes validate-*` is unconfirmed.** Every live test used a fake code, so only the 404/invalid response shape was observed. A 2xx is treated as "valid" generically; if the real success body carries useful fields (e.g. a discount percentage), they aren't surfaced yet. **Concrete consequence, confirmed 2026-08-20**: `rates search` has no `--corporate-code`, `--rate-plan-id`, or equivalent parameter at all — so even a successfully-validated code cannot currently be used to filter a rates search to that specific rate plan, regardless of what the success response turns out to contain. **Verified workaround**: the booking widget's own page accepts a `RatePlanId` query parameter that pre-selects a corporate rate directly — `https://reservations.travelclick.com/{hotel_id}?RatePlanId={id}` (redirects to `bookings.travelclick.com`). Confirmed working against a real hotel/RatePlanId pair (pre-selected the corporate rate, real price readable from the rendered page). This is a page-level UI parameter only, not a confirmed API query parameter — re-sniffing the `avail` call while this URL parameter is active would likely reveal the real parameter name and unblock wiring it into `rates search`. (Specific rate-plan IDs are sensitive — each one maps to a named corporation's negotiated price, so don't publish one alongside the company it belongs to.)
- **`hotels alias`, `codes validate-*`, and `analytics price-drift` have no live-sandboxed test coverage** in this project's own dogfood matrix, which runs each command in an isolated, freshly-sandboxed local store and skips mutating commands by default. All three were verified working correctly by hand instead (including `hotels alias` under 27 concurrent invocations with zero errors) -- see the shipcheck proof for the exact commands run.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `TRAVELCLICK_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://api.travelclick.com/ibe-shop/v1/hotel/102306/avail
- Capture coverage: 5 API entries from 5 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: bearer_token — headers: Authorization
- Candidate command ideas: create_multi_room — Derived from observed POST /ibe-shop/v1/hotel/{hotel_id}/basicavail/multi-room traffic.; get_TEST123 — Derived from observed GET /ibe-codes/v1/hotel/{hotel_id}/specialcodes/corporate/TEST123 traffic.; get_avail — Derived from observed GET /ibe-shop/v1/hotel/{hotel_id}/avail traffic.; get_info — Derived from observed GET /ibe-entity/v1/hotel/{hotel_id}/info traffic.

Warnings from discovery:
- error_status_cluster: Endpoint cluster only observed error HTTP statuses.
- error_status_cluster: Endpoint cluster only observed error HTTP statuses.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
