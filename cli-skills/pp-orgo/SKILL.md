---
name: pp-orgo
description: "The audit ledger Orgo doesn't otherwise have, plus every existing Orgo SDK feature in one Go binary. Trigger phrases: `orgo cli`, `what did the agent do on the orgo computer`, `audit my orgo computers`, `spin up an orgo desktop`, `screenshot the orgo computer`, `use orgo`, `run orgo`."
author: "NickVasilescu"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - orgo-pp-cli
---

# Orgo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `orgo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install orgo --cli-only
   ```
2. Verify: `orgo-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every screenshot, bash command, and click your agent runs through this CLI lands in a local SQLite store, so commands like `replay`, `audit`, `grep`, and `cost` see what no Orgo API call can. On top of that, the full computer-control surface — clone, resize, screenshot, click, type, bash — works offline-first with auto-JSON when piped, typed exit codes, and a `doctor` that flags stuck or suspended computers across every workspace in one call.

## When to Use This CLI

This CLI is the right choice for any agent task that involves Orgo cloud computers — provisioning, controlling, cleaning up, or auditing what an agent did on them. Reach for it when an agent needs to drive a desktop and you want a local audit trail, when you need fleet-wide visibility (idle, oversized, suspended), or when you need to reconstruct what happened during an incident. Use the orgo SDKs (Python or TS) only when you need in-process integration; the CLI is the right shape for shell, cron, and agent invocations.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Audit ledger only the CLI has
- **`replay`** — Generate a self-contained static HTML timeline of every screenshot, click, bash, and exec your agent ran on a computer.

  _Reach for this when you need to audit, share, or debug what an agent actually did on a desktop — no live API roundtrips, just a single HTML file you can email or attach to an issue._

  ```bash
  orgo replay desktop_abc --since 1h --out replay.html
  ```
- **`audit`** — Chronological table of every CLI-driven action against your computers in a time window, scoped by workspace, FTS-searchable.

  _Reach for this when a customer asks 'what did the agent do this week' or you need a regression bundle for an incident._

  ```bash
  orgo audit --workspace prod --since 7d --agent --select timestamp,computer,kind,summary
  ```
- **`grep`** — FTS5 search over historical bash commands, Python exec code, and click coordinates from the local actions store.

  _Reach for this when you need to find a specific command an agent ran but don't remember which computer or when._

  ```bash
  orgo grep 'pip install' --type bash --since 30d
  ```

### Fleet stewardship
- **`fleet`** — Cross-workspace health rollup: surfaces suspended (over-quota), errored, stuck-creating, and stuck-stopping computers, plus an API-key validity probe.

  _Reach for this for incident response, before pushing a fleet change, or as a daily cron — one call replaces walking every workspace by hand._

  ```bash
  orgo-pp-cli fleet --agent
  ```
- **`idle`** — Sorts running computers by hours-since-last-CLI-action, surfacing burns that could be stopped.

  _Reach for this on your weekly cost pass — every idle computer with auto-stop disabled is a known leak._

  ```bash
  orgo idle --threshold-hours 24
  ```
- **`oversized`** — Flags computers with CPU >= 4 cores or RAM >= 16 GB whose last CLI-recorded action is older than the threshold and whose auto-stop is disabled.

  _Reach for this when you suspect a workspace is overspending — it pinpoints the exact downsize candidates._

  ```bash
  orgo oversized --min-cores 4 --idle-days 7
  ```
- **`prune`** — Cross-workspace status-filtered batch delete with dry-run by default.

  _Reach for this for Friday cleanup or after a downgrade leaves a fleet of suspended computers behind._

  ```bash
  orgo prune --status suspended,error --older-than 7d --dry-run
  ```
- **`cost`** — Reconstructs per-computer running-hours from local action timestamps + observed status transitions, multiplies by per-tier rate, sums by workspace. --forecast projects month-end burn.

  _Reach for this for monthly invoicing, customer billing questions, or to forecast whether a workload will hit the plan ceiling._

  ```bash
  orgo cost --workspace prod --since 30d --forecast
  ```

### DOM-aware browser automation (`chrome`)

Drive Chrome inside the Orgo VM at the **DOM level** instead of pixel-based click/screenshot loops. Faster, more reliable, far fewer tokens for any web workflow.

A small Node bridge is shipped embedded in this binary and auto-deploys to `/tmp/orgo-chrome-bridge.js` inside the VM on first call. Subsequent calls reuse it. Bridge auto-redeploys when the CLI is upgraded and ships a new embedded version.

- **`chrome read-page <id> --filter interactive`** — Get the accessibility tree with element refs (`ref_N`). Cheap, fast, the right "seeing" tool for web pages.
- **`chrome find <id> --query "search bar"`** — Find elements by intent. Returns up to 20 matches with refs.
- **`chrome click <id> --ref ref_3`** — Click by ref (prefer over `--x`/`--y` coordinates). Resilient to layout shifts.
- **`chrome form-input <id> --ref ref_7 --value "..."`** — Set form field values directly. No focus-then-type dance.
- **`chrome evaluate <id> --expression "document.title"`** — JavaScript in the page context. Do not write `return` — just the expression.
- **`chrome screenshot <id> --out /tmp/page.png`** — Decoded PNG/JPEG straight to disk. Without `--out`, returns base64 inline.
- **`chrome console <id> --only-errors`** / **`chrome network <id> --url-pattern "/api/"`** — Buffered console + network for in-page debugging.

  _Reach for `chrome` whenever the task is "do something on a web page" — searching, form filling, scraping, scraping-then-acting. Use pixel-based `computers click mouse` / `computers screenshot get` only for native desktop apps or when the page has no useful DOM (canvases, maps, charts)._

  ```bash
  # Recipe: extract structured data from a page.
  orgo chrome navigate <id> --url https://news.ycombinator.com
  orgo chrome read-page <id> --filter interactive --max-chars 20000 --agent
  orgo chrome evaluate <id> --expression "[...document.querySelectorAll('.titleline a')].map(a => a.textContent).slice(0, 10)" --agent

  # Recipe: log in and grab post-login state.
  orgo chrome navigate <id> --url https://app.example.com/login
  orgo chrome find <id> --query "email" --agent
  orgo chrome form-input <id> --ref ref_2 --value "agent@example.com"
  orgo chrome form-input <id> --ref ref_3 --value "$EXAMPLE_PASSWORD"
  orgo chrome find <id> --query "sign in" --agent
  orgo chrome click <id> --ref ref_5
  orgo chrome page-text <id> --agent
  ```

  **VM-direct routing applies.** Add `--vm-from <id>` (or set `ORGO_VM_URL` + `ORGO_VM_TOKEN`) and chrome calls skip the central API the same way `bash`/`click`/`screenshot` do. Measured win for chrome on a sample machine: central ~1.23s avg vs VM-direct steady-state ~0.48s.

  **Full command set:** `navigate`, `tabs`, `new-tab`, `switch-tab`, `read-page`, `find`, `page-text`, `screenshot`, `click`, `type`, `form-input`, `scroll`, `evaluate`, `console`, `network`, `resize`. Every subcommand is auto-registered as an MCP tool (`chrome_navigate`, `chrome_read_page`, …) — one MCP server, both API and browser surfaces.

## Command Reference

**computers** — Provision and manage virtual computers

- `orgo-pp-cli computers create` — Creates a new virtual computer in a workspace. The computer starts automatically after creation.
- `orgo-pp-cli computers delete` — Permanently deletes a computer and all its data.
- `orgo-pp-cli computers get` — Returns computer details including current status.

**files** — Upload and download files

- `orgo-pp-cli files delete` — Permanently deletes a file from storage.
- `orgo-pp-cli files download` — Returns a signed download URL for a file. URLs expire in 1 hour.
- `orgo-pp-cli files export` — Exports a file from the computer's filesystem and returns a download URL.
- `orgo-pp-cli files list` — Lists all files in a workspace, optionally filtered by computer.
- `orgo-pp-cli files upload` — Uploads a file to a workspace. Maximum file size is 10MB.

**workspaces** — Organize computers into named workspaces

- `orgo-pp-cli workspaces create` — Creates a new workspace. Workspace names must be unique per user.
- `orgo-pp-cli workspaces delete` — Deletes a workspace and all its computers. This action cannot be undone.
- `orgo-pp-cli workspaces get` — Returns a workspace by ID, including its computers.
- `orgo-pp-cli workspaces list` — Returns all workspaces for the authenticated user.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
orgo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Recover stuck computers

```bash
orgo doctor --json | jq '.issues[] | select(.kind == "stuck") | .computer_id'
```

Lists every computer that's been stuck creating or stopping for too long, ready to pipe into `xargs orgo computers restart`.

### Weekly cost pass

```bash
orgo cost --since 7d --forecast --agent --select workspace,running_hours,projected_month_end
```

Per-workspace running-hours plus month-end projection; pairs `--agent` with `--select` to keep the response under a hundred bytes per row.

### Replay a debugging session

```bash
orgo replay agent-1 --since 30m --out /tmp/last-session.html
```

Single-file HTML timeline of every screenshot, bash, and exec the agent ran; attach to issues.

### Find the bash command that broke things

```bash
orgo grep 'rm -rf' --type bash --since 24h
```

FTS over the local actions store; finds the exact command across every computer.

### Friday fleet cleanup

```bash
orgo prune --status suspended,error --older-than 7d --dry-run
```

Cross-workspace dry-run pass; drop --dry-run after eyeballing the list.

## Auth Setup

Bearer auth via ORGO_API_KEY (sk_live_...). Get a key at https://www.orgo.ai/workspaces. The CLI reads ORGO_API_KEY on every invocation, so rotating keys requires no restart. orgo doctor probes the key against the live API and reports the source (env var vs config file) without printing the value.

Run `orgo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  orgo-pp-cli computers get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
orgo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
orgo-pp-cli feedback --stdin < notes.txt
orgo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.orgo-pp-cli/feedback.jsonl`. They are never POSTed unless `ORGO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ORGO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
orgo-pp-cli profile save briefing --json
orgo-pp-cli --profile briefing computers get mock-value
orgo-pp-cli profile list --json
orgo-pp-cli profile show briefing
orgo-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `orgo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add orgo-pp-mcp -- orgo-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which orgo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   orgo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `orgo-pp-cli <command> --help`.
