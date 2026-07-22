---
name: pp-isitagentready
description: "The terminal scanner for AI-agent readiness: every check the web tool runs, plus copy-paste fixes, CI gating, scan history, and a local store the web UI has no answer for. Trigger phrases: `scan my site for agent readiness`, `is my site agent-ready`, `what's my agent readiness level`, `how do I make my site ready for AI agents`, `fix my site for AI agents`, `check llms.txt and MCP discovery on this site`, `use isitagentready`, `run isitagentready`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - isitagentready-pp-cli
    install:
      - kind: go
        bins: [isitagentready-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/cmd/isitagentready-pp-cli
---

# Is It Agent Ready — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `isitagentready-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install isitagentready --cli-only
   ```
2. Verify: `isitagentready-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/cmd/isitagentready-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

isitagentready.com gives you a one-shot score in a browser tab with no memory. This CLI turns it into a repeatable loop: check any site, get the prioritized fixes with advice, gate it in CI, diff it over time, compare it to competitors, and batch-scan a whole portfolio, with every scan stored locally so history and open-advice tell you exactly what changed and what is still unfixed.

## When to Use This CLI

Reach for this when you need to check or improve how ready a website is for AI agents: scanning a site for robots.txt, llms.txt, MCP, OAuth, and commerce-protocol readiness, getting copy-paste fixes to raise its level, gating a deploy on readiness, tracking a site's score over time, or comparing several sites. It is ideal inside a coding agent that will then apply the fixes it surfaces.

## Anti-triggers

Do not use this CLI for:
- Generating an llms.txt or robots.txt file from scratch (this CLI scans and advises; it does not author those files for you).
- Tracking whether ChatGPT or Perplexity cite your brand (that is AI-visibility tracking, a different product).
- General SEO keyword-ranking or backlink audits.
- Fetching a page's content as markdown for an agent to read (that is the markdown-negotiation feature on the target site, not this scanner).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Ship it without regressing
- **`gate`** — Fail a build when a site drops below a target readiness level or when a previously-passing check regresses.

  _Reach for this in CI when you want a deterministic exit code, not a brittle grep of the JSON._

  ```bash
  isitagentready-pp-cli gate https://isitagentready.com --min-level 3
  ```
- **`open-advice`** — List every still-failing check across all your scanned sites with its fix prompt, so you see exactly what is left to do.

  _Reach for this to answer 'what fixes are still open across all my sites' in one command._

  ```bash
  isitagentready-pp-cli open-advice --agent
  ```

### Track readiness over time
- **`history`** — Show a site's readiness level across every past scan and flag which checks flipped pass/fail between scans.

  _Reach for this to confirm a fix actually landed and to catch silent regressions._

  ```bash
  isitagentready-pp-cli history https://example.com --agent
  ```
- **`diff`** — Diff two scans of a site (default: the latest two) into a per-check regressed/fixed/unchanged table plus the level delta.

  _Reach for this to see precisely what changed between two points in time, not just the new score._

  ```bash
  isitagentready-pp-cli diff https://example.com
  ```

### Across many sites
- **`compare`** — Scan several sites and print a check-by-check matrix of which agent standards each one implements, plus each site's level.

  _Reach for this to see exactly which standards a competitor implemented that you have not._

  ```bash
  isitagentready-pp-cli compare https://example.com https://isitagentready.com
  ```
- **`batch`** — Scan a list of URLs from a file or stdin, persist each, and print a leaderboard ranked by level or failing-check count.

  _Reach for this to triage a whole web estate worst-first instead of scanning sites one browser tab at a time._

  ```bash
  isitagentready-pp-cli batch urls.txt --rank failing --csv
  ```

## Command Reference

**scan** — Scan a website for AI-agent readiness

- `isitagentready-pp-cli scan` — Scan a URL; returns readiness level (0-5), per-check results across 5 categories


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
isitagentready-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Get the copy-paste fixes for a site

```bash
isitagentready-pp-cli advice https://example.com --copy
```

Prints every next-level fix prompt as one pasteable block, ready to drop into a coding agent.

### See what is still unfixed across all your sites

```bash
isitagentready-pp-cli open-advice --agent
```

Cross-site backlog of every check still failing, with its fix prompt, from your local scan history.

### Narrow a verbose scan to just the checks you care about

```bash
isitagentready-pp-cli report https://example.com --agent --only-failing --select checks.discovery.mcpServerCard.status,checks.discovery.mcpServerCard.message
```

A full scan is large and deeply nested; --select pulls just the dotted fields you want so an agent does not parse tens of KB.

### Gate a deploy on readiness

```bash
isitagentready-pp-cli gate https://example.com --min-level 3 --no-regress
```

Exits non-zero if the site is below level 3 or any previously-passing check regressed; safe for CI.

### Compare against a competitor

```bash
isitagentready-pp-cli compare https://example.com https://isitagentready.com
```

Prints a per-standard matrix of which agent-readiness checks each site implements.

## Auth Setup

No API key or login required. The scan endpoint is public and read-only, so every command works out of the box.

Run `isitagentready-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  isitagentready-pp-cli scan --url https://example --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `ISITAGENTREADY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ISITAGENTREADY_CONFIG_DIR`, `ISITAGENTREADY_DATA_DIR`, `ISITAGENTREADY_STATE_DIR`, `ISITAGENTREADY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ISITAGENTREADY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `isitagentready-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "isitagentready": {
        "command": "isitagentready-pp-mcp",
        "env": {
          "ISITAGENTREADY_HOME": "/srv/isitagentready"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ISITAGENTREADY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ISITAGENTREADY_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
isitagentready-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
isitagentready-pp-cli feedback --stdin < notes.txt
isitagentready-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ISITAGENTREADY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ISITAGENTREADY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
isitagentready-pp-cli profile save briefing --json
isitagentready-pp-cli --profile briefing scan --url https://example
isitagentready-pp-cli profile list --json
isitagentready-pp-cli profile show briefing
isitagentready-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `isitagentready-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/cmd/isitagentready-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add isitagentready-pp-mcp -- isitagentready-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which isitagentready-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   isitagentready-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `isitagentready-pp-cli <command> --help`.
