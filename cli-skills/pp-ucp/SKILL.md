---
name: pp-ucp
description: "A Go CLI for Google's Universal Commerce Protocol — talk to UCP merchants over REST or MCP Trigger phrases: `use ucp`, `run ucp`, `check this UCP store`, `search UCP merchant`, `add to my UCP cart`, `prep a UCP checkout`."
author: "david"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ucp-pp-cli
    install:
      - kind: go
        bins: [ucp-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/ucp/cmd/ucp-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/ucp/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# UCP — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ucp-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install ucp --cli-only
   ```
2. Verify: `ucp-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/ucp/cmd/ucp-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

UCP-pp-cli is the terminal-grade tool the official Python and JS SDKs don't ship. It speaks both REST and MCP transports so it works against real merchants like checkout.coffeecircle.com (Shopify-hosted, MCP-only) and the bundled mock. Local SQLite holds carts, checkout sessions, and a merchant directory, enabling cross-merchant search, capability diffs, and price-drift watch loops nothing else offers today.

## Real-Merchant Compatibility

v0.1 supports two interaction patterns:

| Command | Real merchants (e.g. checkout.coffeecircle.com) | Bundled mock (`mock serve`) |
|---|---|---|
| `check <domain>` | ✅ Any merchant publishing `/.well-known/ucp` | ✅ |
| `search` / `cart` / `checkout prep` | ⚠️ Requires REST transport — most public merchants are MCP-only (deferred to v0.2) | ✅ Full end-to-end flow |

Known real merchants: `checkout.coffeecircle.com` (Shopify-hosted, MCP-only). v0.2 adds MCP transport client to unlock real-merchant transactions.

## When to Use This CLI

Use ucp-pp-cli when an agent needs to interact with UCP merchants directly: probe a manifest, build a multi-merchant cart, run a search across known UCP-supporting stores, or prep a checkout draft for an AP2 mandate. It is the natural local pair to an AP2 CLI for the search-and-buy half of an agentic shopping flow. Skip it when you only need ChatGPT-side checkout (use an ACP-flavored tool) or when you only need merchant-side validation (use awesomeucp/ucp-doctor).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-merchant discovery
- **`check`** — Fetch `/.well-known/ucp` for any domain and return a graded report covering schema validity, advertised transports, and capability coverage — works against any merchant publishing a UCP manifest, including MCP-only Shopify-hosted stores.

  _Lets an agent screen a domain for UCP viability without standing up an SDK or hand-rolling manifest parsing._

  ```bash
  ucp-pp-cli check checkout.coffeecircle.com --json
  ```

### Reachability mitigation
- **`mock serve`** — Spawn a UCP-compliant reference merchant locally (pure-Go, no external runtime) so `ucp check`, `search`, `cart`, and `checkout prep` flows work end-to-end without a third-party UCP merchant or extra language toolchains.

  _Lets an agent verify its UCP integration without coordinating with a live merchant or waiting for Google AI Mode approval._

  ```bash
  ucp-pp-cli mock serve --port 8080
  ```

## Command Reference

**checkout** — Operations on checkout

- `ucp-pp-cli checkout` — POST /checkout


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ucp-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Boot a UCP test environment from scratch

```bash
ucp-pp-cli mock serve --port 8080
```

Start the bundled pure-Go reference merchant; then run `check 127.0.0.1:8080` from another shell.

### End-to-end search-to-checkout against the mock merchant

```bash
ucp-pp-cli search "coffee" --merchant 127.0.0.1:8080 --json
```

Fan out a query, normalize results, and pipe the first product into `cart add` then `checkout prep`.

### Validate a real public UCP merchant manifest

```bash
ucp-pp-cli check checkout.coffeecircle.com --json
```

Probe a Shopify-hosted UCP merchant. Returns a graded report even though the merchant advertises MCP-only transport (REST-only commands won't transact against it in v0.1).

## Auth Setup

UCP has no global API key. Each merchant declares its own auth shape; the CLI identifies itself via the `UCP-Agent: profile="<url>"` header and signs requests with a per-agent ECDSA-P256 key. Run `ucp profile init` once to generate your profile. For account-linked flows, `ucp auth link <merchant>` walks an OAuth 2.0 flow per merchant (v1: print-mode — agent pastes the redirect URL back).

Run `ucp-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ucp-pp-cli checkout --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ucp-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ucp-pp-cli feedback --stdin < notes.txt
ucp-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ucp-pp-cli/feedback.jsonl`. They are never POSTed unless `UCP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `UCP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ucp-pp-cli profile save briefing --json
ucp-pp-cli --profile briefing checkout
ucp-pp-cli profile list --json
ucp-pp-cli profile show briefing
ucp-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ucp-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/ucp/cmd/ucp-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ucp-pp-mcp -- ucp-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ucp-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ucp-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ucp-pp-cli <command> --help`.
