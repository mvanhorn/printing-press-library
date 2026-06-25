---
name: pp-apple-docs
description: "Every Apple framework, indexed locally, with deprecation analysis and an MCP server no other Apple-docs tool has. Trigger phrases: `look up an Apple API`, `what's deprecated in iOS`, `find a SwiftUI symbol`, `port this from UIKit to SwiftUI`, `what changed at WWDC`, `use apple-docs`, `run apple-docs-pp-cli`."
author: "Joseph Alvin Castillo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - apple-docs-pp-cli
    install:
      - kind: go
        bins: [apple-docs-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/cmd/apple-docs-pp-cli
---

# Apple Developer Documentation — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `apple-docs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install apple-docs --cli-only
   ```
2. Verify: `apple-docs-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/cmd/apple-docs-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when you need to read Apple's docs from a terminal, when an agent is grounding Swift code generation against current Apple APIs, when planning a platform migration (iPad → visionOS, AppKit → SwiftUI), or when authoring a 'what's deprecated in iOS N' guide. The local SQLite store unlocks queries — cross-framework grep, snapshot diff, deprecation cliff — that no online docs viewer offers.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to author Apple ID or App Store Connect API calls — use a separate App Store Connect CLI for those.
- Do not use this CLI as a Swift package manager — it reads docs, not source.
- Do not use this CLI to fetch full WWDC video transcripts — it indexes WWDC references from doc pages but does not host video content.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`doc get`** — Project a 50KB+ DocC JSON page down to just the fields an agent actually needs — abstract, signature, platforms, or all three — saving context tokens on every lookup.

  _Use this when an agent is grounding code generation against an Apple symbol and only needs the abstract + signature + platform floor, not the full discussion + see-also tree._

  ```bash
  apple-docs-pp-cli doc get 'swiftui/view/onappear(perform:)' --shape min --agent
  ```
- **`bundle`** — Bundle a symbol's Markdown plus its depth-1 See-Also pages into a single token-budgeted blob, ready to paste into an agent prompt.

  _Use when an agent needs a self-contained context blob about a symbol plus its closest relatives, without doing N round-trips and N JSON parses._

  ```bash
  apple-docs-pp-cli bundle 'swiftui/view/onappear(perform:)' --depth 1 --max-tokens 4000
  ```

### Local state that compounds
- **`port-to`** — For a symbol unavailable on a target platform, walk the See-Also / Replacement-Of graph until landing on an alternative that IS available there and is not itself deprecated.

  _Use when porting code between Apple platforms (iPad → visionOS, AppKit → SwiftUI, deprecated → current) and you need the migration target, not just a similar-named API._

  ```bash
  apple-docs-pp-cli port-to visionOS uikit/uitableview --agent
  ```
- **`snapshot diff`** — Diff two stored framework index snapshots and classify each delta as added, removed, deprecated, or likely-renamed (path-stem similarity).

  _Use after every WWDC or dot-release to surface added/removed/deprecated symbols at the framework level._

  ```bash
  apple-docs-pp-cli snapshot diff swiftui --from 2025-06-09 --to 2026-05-28 --agent
  ```
- **`deprecation-cliff`** — List every Apple API deprecated in a given platform version, grouped by framework and symbol kind, with the replacement-hint column joined from references.

  _Use when planning a migration sprint or writing a 'what's deprecated this year' guide; the only way to get the full list in one shot._

  ```bash
  apple-docs-pp-cli deprecation-cliff --os iOS --version 18 --framework swiftui --agent
  ```
- **`conformance`** — Walk a framework's `relationshipsSections` to enumerate concrete conformers of a protocol and the protocol's ancestors.

  _Use when writing protocol-driven code (custom View, ObservableObject, Layout) and you need every concrete type that conforms._

  ```bash
  apple-docs-pp-cli conformance View --framework swiftui --agent
  ```
- **`grep`** — Regex over every synced framework's symbols with filters on kind, target platform, and deprecation status.

  _Use when you remember the shape of a symbol name but not where it lives, or when auditing API patterns across the whole Apple SDK._

  ```bash
  apple-docs-pp-cli grep onAppear --framework swiftui --json
  ```

### Service-specific patterns
- **`wwdc symbols`** — For a WWDC session ID, list every symbol whose doc page cites that session.

  _Use after watching a WWDC session to enumerate every API it touched, or to find which session officially introduced an API you're working with._

  ```bash
  apple-docs-pp-cli wwdc symbols wwdc2024-10169 --agent
  ```

## Command Reference

**doc get** — Fetch any documentation page (framework root, symbol, method, article) by path

- `apple-docs-pp-cli doc get <path>` — Fetch a documentation page by path and return it in the shape your tool actually wants (raw DocC JSON, projected via `--shape abstract|signature|platforms|min`, or rendered as Markdown via `--markdown`). The path is the lowercase-slashed identifier under /documentation/, e.g. `swiftui/view`, `swiftui/view/onappear(perform:)`, `foundation/url/init(string:)`.

**index** — Fetch the full structured index of a framework (every symbol, in tree form)

- `apple-docs-pp-cli index <framework>` — Fetch a framework's full hierarchical index. Useful for offline FTS sync. Indexes are large (500KB–2MB).

**technologies** — List every Apple framework and technology (Swift, SwiftUI, UIKit, etc.)

- `apple-docs-pp-cli technologies` — Fetch the master technologies index — every Apple framework, grouped by topic.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `APPLE_DOCS_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `apple-docs-pp-cli technologies`
- `apple-docs-pp-cli technologies get`
- `apple-docs-pp-cli technologies list`
- `apple-docs-pp-cli technologies search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
apple-docs-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find a symbol's iOS introduction version

```bash
apple-docs-pp-cli doc get 'swiftui/view/onappear(perform:)' --shape platforms --agent --select platforms[].introducedAt
```

Returns just the platform-availability map — under 500 bytes vs 9KB for the full page.

### List every concrete SwiftUI type that conforms to View

```bash
apple-docs-pp-cli conformance View --framework swiftui --json --select symbol,kind
```

Walks the local relationshipsSections graph; impossible from any single Apple endpoint.

### What changed in SwiftUI at this year's WWDC?

```bash
apple-docs-pp-cli snapshot diff swiftui --from 2025-06-09 --to 2026-05-28 --classify --json
```

Classifies every delta as added / removed / deprecated / likely-renamed in one call.

### Find the visionOS replacement for a UIKit symbol

```bash
apple-docs-pp-cli port-to visionOS uikit/uitableview --agent
```

Walks See-Also and Replacement-Of edges until landing on a symbol available on visionOS.

### Bundle a symbol's docs as agent context

```bash
apple-docs-pp-cli bundle 'swiftui/view/onappear(perform:)' --depth 1 --max-tokens 4000
```

Markdown render of the symbol + every depth-1 See-Also page, truncated to a token budget.

## Auth Setup

No auth required. The DocC JSON endpoints under https://developer.apple.com/tutorials/data/ are public and served by Apple's CDN.

Run `apple-docs-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  apple-docs-pp-cli doc get swiftui/view --agent --select abstract,declaration,platforms
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
apple-docs-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
apple-docs-pp-cli feedback --stdin < notes.txt
apple-docs-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/apple-docs-pp-cli/feedback.jsonl`. They are never POSTed unless `APPLE_DOCS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `APPLE_DOCS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
apple-docs-pp-cli profile save briefing --json
apple-docs-pp-cli --profile briefing doc get swiftui/view
apple-docs-pp-cli profile list --json
apple-docs-pp-cli profile show briefing
apple-docs-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `apple-docs-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/cmd/apple-docs-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add apple-docs-pp-mcp -- apple-docs-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which apple-docs-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   apple-docs-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `apple-docs-pp-cli <command> --help`.
