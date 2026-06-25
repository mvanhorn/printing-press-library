---
name: pp-mixlayer
description: "Privacy-first Mixlayer CLI for OpenAI-compatible chat, local PII redaction and vaulting, reasoning-aware model ladders, live model catalog refresh, and ledger-backed cost proofs."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - mixlayer-pp-cli
    install:
      - kind: go
        bins: [mixlayer-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/mixlayer/cmd/mixlayer-pp-cli
---

# Mixlayer — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `mixlayer-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install mixlayer --cli-only
   ```
2. Verify: `mixlayer-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/mixlayer/cmd/mixlayer-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use this skill for Mixlayer chat completions, privacy-safe frontier prompts, model-rung comparisons, reasoning ledger search, and local cost analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`shield ingest`** — Split large CSV/text inputs into deterministic tranches, redact each tranche through one shared local vault, and reassemble a masked corpus with a manifest.
- **`shield ask`** — Redact local data, run an outbound PII tripwire, send only the masked payload to a frontier model, rehydrate the answer locally, and save a privacy receipt.
- **`shield scan`** — Detect high-risk PII locally with CI-friendly exit codes and no upstream model call.
- **`shield redact`** — Write a masked copy of a file and populate the local token-to-value vault with stable pseudonyms.
- **`shield restructure`** — Drop columns, bucket numeric values, and coarsen dates to reduce re-identification risk before prompting.
- **`vault`** — List, rehydrate, rotate, or purge the local reversible pseudonym vault without sending values upstream.
- **`shield audit`** — Inspect outbound payload hashes, byte counts, model IDs, masked entity counts, and residual leak counts for shielded sends.
- **`ladder`** — Run one prompt across selected Mixlayer model rungs, optionally comparing reasoning_content, latency, token usage, and cost.
- **`escalate`** — Start on a cheap rung and climb only when the prior answer appears insufficiently confident.
- **`council`** — Fan out to member model rungs and ask a judge model to synthesize answers and flag disagreements.
- **`ask`** — Ask Mixlayer, optionally request reasoning_content, and save prompt, answer, model, seed, token usage, cost, and raw response locally.
- **`replay`** — Re-run a saved prompt with its original model and seed to check determinism or drift.
- **`grep`** — Search saved prompts, answers, and reasoning traces through the local FTS ledger.
- **`models query`** — Query a local model cache with DSL terms like ctx>=128k tools reasoning, seed it with documented and console-visible model IDs, and optionally refresh it from live /models.
- **`sql`** — Run SELECT-only SQLite queries over the local ledger, vault, audit, ladder, and model cache tables.
- **`savings`** — Roll up saved ledger cost against static GPT or Claude frontier baselines.
- **`compare`** — Run a prompt through a cheap rung and frontier rung, report the cost delta, and attach a local-history quality note.

## Command Reference

**chat** — OpenAI-compatible chat completions

- `mixlayer-pp-cli chat` — Create a chat completion for the given model and messages

**models** — Browse the model catalog

- `mixlayer-pp-cli models get` — Retrieve a model by ID
- `mixlayer-pp-cli models list` — List all available models
- `mixlayer-pp-cli models query` — Query the cached model catalog with a small DSL

**privacy firewall**

- `mixlayer-pp-cli shield scan <file> --max-risk 0 --json`
- `mixlayer-pp-cli shield redact <file> --diff -o masked.txt`
- `mixlayer-pp-cli shield ingest <bigfile> -o masked.txt --manifest tranches.json`
- `mixlayer-pp-cli shield ask "<question>" --data <file> --json`
- `mixlayer-pp-cli shield audit --json`
- `mixlayer-pp-cli vault list --json`
- `mixlayer-pp-cli vault rehydrate <text-or-file>`

**reasoning and cost**

- `mixlayer-pp-cli ask "<question>" --show-thinking --json`
- `mixlayer-pp-cli replay <run-id> --json`
- `mixlayer-pp-cli grep "<pattern>" --json`
- `mixlayer-pp-cli ladder "<question>" --reasoning --json`
- `mixlayer-pp-cli escalate "<question>" --confidence 0.85 --json`
- `mixlayer-pp-cli council "<question>" --json`
- `mixlayer-pp-cli sql "select model, count(*) from runs group by model" --json`
- `mixlayer-pp-cli savings --vs gpt-frontier --json`
- `mixlayer-pp-cli compare "<question>" --json`


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
mixlayer-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `mixlayer-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
mixlayer-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `MIXLAYER_API_KEY` as an environment variable.

Run `mixlayer-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  mixlayer-pp-cli models list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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

- Use `--home <dir>` for one invocation, or set `MIXLAYER_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `MIXLAYER_CONFIG_DIR`, `MIXLAYER_DATA_DIR`, `MIXLAYER_STATE_DIR`, `MIXLAYER_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `MIXLAYER_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `mixlayer-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "mixlayer": {
        "command": "mixlayer-pp-mcp",
        "env": {
          "MIXLAYER_HOME": "/srv/mixlayer"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `MIXLAYER_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `MIXLAYER_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
mixlayer-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
mixlayer-pp-cli feedback --stdin < notes.txt
mixlayer-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `MIXLAYER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MIXLAYER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
mixlayer-pp-cli profile save briefing --json
mixlayer-pp-cli --profile briefing models list
mixlayer-pp-cli profile list --json
mixlayer-pp-cli profile show briefing
mixlayer-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `mixlayer-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/mixlayer/cmd/mixlayer-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add mixlayer-pp-mcp -- mixlayer-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which mixlayer-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   mixlayer-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `mixlayer-pp-cli <command> --help`.
