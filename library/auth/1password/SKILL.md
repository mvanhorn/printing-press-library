---
name: pp-1password
description: "Printing Press CLI for 1Password. Agent-safe command layer over the official 1Password CLI and SDK service-account workflows."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - 1password-pp-cli
    install:
      - kind: go
        bins: [1password-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/auth/1password/cmd/1password-pp-cli
---

# 1Password — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `1password-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install 1password --cli-only
   ```
2. Verify: `1password-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/auth/1password/cmd/1password-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use 1Password from agents without turning every task into a secret reveal. The CLI resolves fuzzy requests to exact op:// references, audits metadata, checks policy, and only calls op read, op inject, or op run after an explicit plan.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI is intended for secret-reference planning, metadata audits, policy checks, and tightly gated local `op read` / `op inject` workflows.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Secrets
- **`secrets resolve`** — Resolve fuzzy agent requests to exact op:// vault/item/field references without printing values.

  _Prevents accidental broad reads and gives later commands an exact reference to use._

  ```bash
  1password-pp-cli secrets resolve --query "token" --json
  ```
- **`secrets read`** — Read only an exact op:// reference, with policy checks and an explicit --reveal gate before values are printed.

  _Turns value access into a narrow, auditable action instead of a fuzzy search._

  ```bash
  1password-pp-cli secrets read op://Engineering/GitHub/token --dry-run --json
  ```
- **`secrets explain`** — Explain why a particular item and field was selected for a task without revealing the field value.

  ```bash
  1password-pp-cli secrets explain --query "token" --json
  ```
- **`secrets preflight`** — Check whether a planned task or command appears to require secret values, documents, cards, or write permissions.

  ```bash
  1password-pp-cli secrets preflight --task "deploy using op run --env-file .env" --json
  ```

### Environment
- **`env plan`** — Parse .env, config, or shell files for missing variables and map them to safe op:// references.

  ```bash
  1password-pp-cli env plan API_TOKEN= --json
  ```
- **`env inject`** — Wrap op inject with a redacted plan first and require --write before producing an output file.

  ```bash
  1password-pp-cli env inject --in-file README.md --out-file injected.env --json
  ```

### Items
- **`items classify`** — Find secure notes that look like API credentials, logins, SSH keys, cards, or documents.

  ```bash
  1password-pp-cli items classify --json
  ```
- **`items duplicates`** — Detect duplicate titles, URLs, usernames, and likely copied credentials across vaults without printing secret values.

  ```bash
  1password-pp-cli items duplicates --json
  ```
- **`items ownership`** — Flag shared or service credentials missing owner, purpose, rotation, or environment tags.

  ```bash
  1password-pp-cli items ownership --json
  ```

### Cards
- **`cards audit`** — Find cards stored as notes or logins, missing owner/purpose tags, or CVV-like fields without printing card values.

  ```bash
  1password-pp-cli cards audit --json
  ```
- **`cards resolve`** — Return card item and field references without printing card numbers, expiry values, or CVVs.

  ```bash
  1password-pp-cli cards resolve --query "card" --json
  ```

### Documents
- **`documents inventory`** — List document metadata and exact references without downloading document contents.

  ```bash
  1password-pp-cli documents inventory --json
  ```
- **`documents audit`** — Flag sensitive filenames, oversized docs, private-key/cert-like documents, and documents in shared vaults.

  ```bash
  1password-pp-cli documents audit --json
  ```

### Sharing
- **`share preflight`** — Before sharing an item, show recipient, item category, included fields, expiry, and risk.

  ```bash
  1password-pp-cli share preflight --ref op://Engineering/GitHub/token --recipient recipient --expires-in 1d --json
  ```
- **`share audit`** — Report whether existing/shareable item link inspection is supported by op or the SDK and document unsupported status clearly.

  ```bash
  1password-pp-cli share audit --json
  ```

### Policy
- **`policy check`** — Enforce rules such as never reading credit-card values, exact refs for production, and required owner tags.

  ```bash
  1password-pp-cli policy check --ref op://Production/API/token --require-exact --json
  ```

### Access
- **`access scope`** — Summarize what the current service account or op session can access by vault, category, and count without values.

  ```bash
  1password-pp-cli access scope --json
  ```
- **`rate-limit status`** — Wrap op service-account ratelimit so agents can avoid burning quota.

  ```bash
  1password-pp-cli rate-limit status --json
  ```

### Agent
- **`agent grant-plan`** — Suggest the minimum service-account vault permissions needed for a task.

  ```bash
  1password-pp-cli agent grant-plan --task "read staging deploy token" --json
  ```

### Runtime
- **`run plan`** — Inspect an op run command or env files and show which secret references will resolve before executing.

  ```bash
  1password-pp-cli run plan --command "npm test" --json
  ```

### Audit
- **`audit stale`** — Flag items that appear old, untagged, duplicated, or probably unused from metadata.

  ```bash
  1password-pp-cli audit stale --days 180 --json
  ```
- **`audit misplaced`** — Find API keys, cards, documents, or SSH material saved in the wrong 1Password category.

  ```bash
  1password-pp-cli audit misplaced --json
  ```

## Recipes

### Resolve a secret request without revealing the value

```bash
1password-pp-cli secrets resolve --query "token" --json
```

Returns exact op:// references and candidate metadata with values redacted so a later step can require an exact reference.

### Plan environment injection before writing output

```bash
1password-pp-cli env inject --in-file README.md --out-file injected.env --json
```

Shows the references op inject would resolve and keeps will_write false unless --write is explicitly supplied.

## Command Reference

**op** — Inspect the local 1Password CLI authentication surface

- `1password-pp-cli op` — Show whether op is installed and authenticated without reading secret values


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
1password-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Install the official 1Password CLI (`op`) and authenticate it before using this wrapper.

For automation, set a 1Password service-account token:

```bash
export OP_SERVICE_ACCOUNT_TOKEN="<service-account-token>"
```

For local desktop workflows, sign in with `op` using the normal 1Password CLI or desktop-app integration. This CLI warns when `OP_CONNECT_HOST` or `OP_CONNECT_TOKEN` is set because those Connect variables take precedence over service-account auth in `op`.

Run `1password-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  1password-pp-cli op --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
1password-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
1password-pp-cli feedback --stdin < notes.txt
1password-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/1password-pp-cli/feedback.jsonl`. They are never POSTed unless `API_1PASSWORD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `API_1PASSWORD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
1password-pp-cli profile save briefing --json
1password-pp-cli --profile briefing op
1password-pp-cli profile list --json
1password-pp-cli profile show briefing
1password-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `1password-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/auth/1password/cmd/1password-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add 1password-pp-mcp -- 1password-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which 1password-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   1password-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `1password-pp-cli <command> --help`.
