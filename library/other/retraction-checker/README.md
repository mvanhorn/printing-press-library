# Retraction Checker CLI

**Check whether a paper is retracted, why, and what the current research says now — keyless, over Crossref and OpenAlex.**

Retraction Checker turns Crossref's embedded Retraction Watch data into a one-shot verdict: is this DOI or PMID retracted, when, why, and where is the notice. It batch-scans reading lists and .bib files, finds citation-ranked superseding research via OpenAlex, and watches a topic or library for newly-announced retractions. Fully keyless.

Learn more at [Retraction Checker](https://api.crossref.org/swagger-ui/index.html).

Created by [@laci141](https://github.com/laci141) (laci141).

## Install

The recommended path installs both the `retraction-checker-pp-cli` binary and the `pp-retraction-checker` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install retraction-checker
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install retraction-checker --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install retraction-checker --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install retraction-checker --agent claude-code
npx -y @mvanhorn/printing-press-library install retraction-checker --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/retraction-checker/cmd/retraction-checker-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/retraction-checker-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install retraction-checker --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-retraction-checker --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-retraction-checker --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install retraction-checker --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/retraction-checker-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/retraction-checker/cmd/retraction-checker-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "retraction-checker": {
      "command": "retraction-checker-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the CLI and Crossref reachability before checking papers
retraction-checker-pp-cli doctor --dry-run

# Check a single DOI for retraction status
retraction-checker-pp-cli check 10.1016/j.micpro.2020.103768 --json

# Flag retracted entries across a bibliography
retraction-checker-pp-cli scan refs.bib --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Retraction intelligence
- **`check`** — Tell whether a paper (by DOI or PMID) has been retracted, when, why, and where the notice is.

  _Agents citing a paper should verify it is not retracted before relying on it._

  ```bash
  retraction-checker-pp-cli check 10.1016/j.micpro.2020.103768 --json
  ```
- **`scan`** — Batch-check a reading list or .bib file and flag every retracted entry.

  _Catches retracted citations across a whole manuscript or literature review at once._

  ```bash
  retraction-checker-pp-cli scan refs.bib --json
  ```
- **`superseded`** — For a retracted or older paper, find related more-recent research on the same topic, ranked by citation count.

  _When a paper is retracted, the agent still needs the current best evidence on the topic._

  ```bash
  retraction-checker-pp-cli superseded 10.1016/j.micpro.2020.103768 --json
  ```

### Local state that compounds
- **`watch`** — Monitor a topic or reading list for newly-announced retractions since the last run.

  _Surfaces new retractions in a field or personal library without re-reading everything._

  ```bash
  retraction-checker-pp-cli watch "machine learning" --json
  ```

## Recipes


### Check a DOI

```bash
retraction-checker-pp-cli check 10.1016/j.micpro.2020.103768 --json
```

Returns retraction status, date, reason source, and notice reference for one paper.

### Audit a bibliography

```bash
retraction-checker-pp-cli scan reading-list.txt --agent --select doi,retracted,reason
```

Scans one DOI/PMID per line and returns only the key retraction fields for each entry.

### Find superseding work

```bash
retraction-checker-pp-cli superseded 10.1016/j.micpro.2020.103768 --json
```

Lists more-recent related papers ranked by citations, published after the retracted paper.

### Throttle batch runs to avoid OpenAlex load-shedding

```bash
retraction-checker-pp-cli scan reading-list.txt --rate-limit 0.15 --agent
```

`--rate-limit` caps outbound requests per second across both Crossref and
OpenAlex (`0`, the default, disables the limiter). The default is off because a
single `check` is one request and needs no pacing — but `scan` over a large
bibliography and `superseded` both fan out into many back-to-back calls, and
**OpenAlex rate-limits anonymous traffic**: under load its public API sheds with
`HTTP 503 "temporarily rate-limited due to heavy load"` rather than queuing you.
When that happens a batch run degrades instead of completing.

Pass `--rate-limit 0.15` (~9 requests/minute) for batched `scan`/`superseded`
runs to stay safely under the typical shared anonymous limit and let the whole
batch finish. Raise it if you have headroom; lower it if you still see 503s.
Pairing it with `--mailto you@example.com` (the Crossref/OpenAlex polite pool)
further improves your limits.

### Watch a field

```bash
retraction-checker-pp-cli watch "crispr" --json
```

Baselines retraction notices for a topic and reports new ones on later runs.

## Usage

Run `retraction-checker-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `RETRACTION_CHECKER_CONFIG_DIR`, `RETRACTION_CHECKER_DATA_DIR`, `RETRACTION_CHECKER_STATE_DIR`, or `RETRACTION_CHECKER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `RETRACTION_CHECKER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export RETRACTION_CHECKER_HOME=/srv/retraction-checker
retraction-checker-pp-cli doctor
```

Under `RETRACTION_CHECKER_HOME=/srv/retraction-checker`, the four dirs resolve to `/srv/retraction-checker/config`, `/srv/retraction-checker/data`, `/srv/retraction-checker/state`, and `/srv/retraction-checker/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "retraction-checker": {
      "command": "retraction-checker-pp-mcp",
      "env": {
        "RETRACTION_CHECKER_HOME": "/srv/retraction-checker"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `RETRACTION_CHECKER_DATA_DIR` overrides an explicit `--home` for that kind. Use `RETRACTION_CHECKER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `RETRACTION_CHECKER_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `retraction-checker-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### works

Manage works

- **`retraction-checker-pp-cli works get`** - Get a single work by DOI
- **`retraction-checker-pp-cli works search`** - Search or filter scholarly works


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
retraction-checker-pp-cli works get mock-value

# JSON for scripting and agents
retraction-checker-pp-cli works get mock-value --json

# Filter to specific fields
retraction-checker-pp-cli works get mock-value --json --select id,name,status

# Dry run — show the request without sending
retraction-checker-pp-cli works get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
retraction-checker-pp-cli works get mock-value --agent
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
retraction-checker-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `retraction-checker-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/retraction-checker-via-pp-cli/config.toml`; `--home`, `RETRACTION_CHECKER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Rate-limited by Crossref** — Pass --mailto you@example.com to join the polite pool for better limits
- **OpenAlex 503 "temporarily rate-limited due to heavy load"** (seen on `superseded` and large `scan` runs) — OpenAlex sheds anonymous traffic under load. Re-run `--rate-limit 0.15` (~9 req/min) to pace the batch below the shared anonymous limit; add `--mailto you@example.com` for the polite pool. See the "Throttle batch runs" recipe above.
- **PMID not found** — PMIDs are resolved to DOIs first; some records have no DOI and cannot be checked via Crossref
