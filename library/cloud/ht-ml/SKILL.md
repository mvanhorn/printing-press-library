---
name: pp-ht-ml
description: "The only ht-ml.app tool that remembers what you published, a local registry of every site, its once-only update_key, and full version history, plus one-command publish-with-assets. Trigger phrases: `publish this HTML`, `get me a public URL for this page`, `host this HTML file`, `update my ht-ml site`, `list my published sites`, `use ht-ml`, `run ht-ml`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ht-ml-pp-cli
    install:
      - kind: go
        bins: [ht-ml-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/cmd/ht-ml-pp-cli
---

# ht-ml.app — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ht-ml-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ht-ml --cli-only
   ```
2. Verify: `ht-ml-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/cmd/ht-ml-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

ht-ml.app is deliberately accountless: you POST one HTML document, get a public URL, and a write key shown exactly once. That makes publishing frictionless and management impossible. This CLI is the missing memory layer. Every publish is captured to a local SQLite store, so you can list and audit everything you've shipped, update a site by id without ever touching its key, auto-upload referenced assets in one pass, roll back to any prior version, and export a passphrase-sealed vault of your keys for disaster recovery.

## When to Use This CLI

Use this CLI whenever an agent needs to publish a single HTML document to a public URL fast (prototypes, diagrams, decks, status reports, code reviews) and then manage it over time. It is the right tool when you will publish more than once and need to find, update, version, or recover past sites, capabilities ht-ml.app's accountless API does not provide on its own.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to host multi-file apps, SPAs with build steps, or anything needing a server runtime; ht-ml.app serves one HTML document per site.
- Do not publish private or confidential content; everything is public, and a per-site password is only a shared-secret gate, not real security.
- Do not use it as a CDN for a large media library; assets must be referenced by the site's HTML to be uploadable.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Registry & recovery
- **`list`** — See every site you've ever published, with health flags for orphaned sites and broken assets.

  _Reach for this to recall or audit what you've shipped; it's the only way to enumerate sites the accountless API won't list._

  ```bash
  ht-ml-pp-cli list --agent --select url,title,status
  ```
- **`keys export`** — Reveal a single write key, or export/import a passphrase-sealed vault of all your update_keys for disaster recovery and a second machine.

  _Use after publishing or before switching machines; without a vault, a lost key orphans the site forever._

  ```bash
  ht-ml-pp-cli keys export --out ht-ml-keys.vault
  ```

### Asset reconciliation
- **`assets sync`** — Parse a site's HTML, find every referenced-but-missing image or video, and upload them all in one pass.

  _Pick this whenever a published page has broken images; it fixes all of them at once instead of one upload per file._

  ```bash
  ht-ml-pp-cli assets sync <site_id> --root ./public
  ```
- **`assets audit`** — Across all your sites, list the ones with publicly-visible broken or missing images.

  _Run during a review to catch broken client-facing pages before someone else does._

  ```bash
  ht-ml-pp-cli assets audit --missing-only --agent
  ```

### Versioning & living docs
- **`rollback`** — Revert a live site to any prior HTML version stored locally, with the update_key resolved for you.

  _Use when a republish shipped bad data; it restores the last-good HTML in one command._

  ```bash
  ht-ml-pp-cli rollback <site_id>
  ```
- **`republish`** — Publish a recurring document under a stable local alias: update in place if the alias exists, or create it once and bind it.

  _Pick this for scheduled or daily publishes so the public URL never churns._

  ```bash
  ht-ml-pp-cli republish --as status-report ./status.html
  ```

### Publish safety
- **`scan`** — Mechanically scan HTML for leaked secrets and PII before it becomes a public, permanent URL.

  _Run before any publish on a person's behalf; it returns a typed exit code so it can gate a pipeline._

  ```bash
  ht-ml-pp-cli scan ./page.html
  ```

## Command Reference

**sites** — Inspect ht-ml.app sites

- `ht-ml-pp-cli sites <site_id>` — Get a site's status and the assets its HTML references (no auth; public read)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ht-ml-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Publish a deck and get just the URL

```bash
ht-ml-pp-cli publish ./deck.html --agent --select url,site_id
```

Publish and return only the live URL and id, ready to hand to a downstream step.

### Inspect a site's referenced assets compactly

```bash
ht-ml-pp-cli sites <site_id> --agent --select status,assets.relative_path,assets.status
```

Use dotted --select to narrow the verbose site-plus-assets payload to just status and each asset's path and status.

### Audit every site for broken images

```bash
ht-ml-pp-cli assets audit --missing-only --agent
```

Cross-site join that lists publicly-visible missing assets the API cannot surface in one call.

### List your published sites by title

```bash
ht-ml-pp-cli list --sort title --agent --select site_id,title,url
```

The local registry is the only inventory of what you have shipped (the API has no list endpoint); sort by title to find a page fast.

### Recover keys on a second machine

```bash
ht-ml-pp-cli keys import ht-ml-keys.vault
```

Import a passphrase-sealed vault so update and rollback work from another machine.

## Auth Setup

ht-ml.app has no global API key. Creating and reading sites needs no credential at all. Each site mints a high-entropy update_key once at creation — the only write credential, with no recovery endpoint. This CLI captures that key into a local store at create time, so update, asset, and password commands resolve it automatically by site_id. Keep the store safe and back it up with `keys export`; losing an update_key with no backup orphans the site forever.

Run `ht-ml-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ht-ml-pp-cli sites mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — list and audit commands use the local SQLite store when available
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `HT_ML_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `HT_ML_CONFIG_DIR`, `HT_ML_DATA_DIR`, `HT_ML_STATE_DIR`, `HT_ML_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `HT_ML_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `ht-ml-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "ht-ml": {
        "command": "ht-ml-pp-mcp",
        "env": {
          "HT_ML_HOME": "/srv/ht-ml"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `HT_ML_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `HT_ML_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ht-ml-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ht-ml-pp-cli feedback --stdin < notes.txt
ht-ml-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `HT_ML_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HT_ML_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ht-ml-pp-cli profile save briefing --json
ht-ml-pp-cli --profile briefing sites mock-value
ht-ml-pp-cli profile list --json
ht-ml-pp-cli profile show briefing
ht-ml-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ht-ml-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/cmd/ht-ml-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ht-ml-pp-mcp -- ht-ml-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ht-ml-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ht-ml-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ht-ml-pp-cli <command> --help`.
