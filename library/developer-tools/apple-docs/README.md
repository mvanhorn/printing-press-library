# Apple Developer Documentation CLI

**Every Apple framework, indexed locally, with deprecation analysis and an MCP server no other Apple-docs tool has.**

apple-docs-pp-cli mirrors developer.apple.com's DocC JSON into a local SQLite store you can grep across every framework, diff between releases, and project down to just the fields an agent needs. Ships with offline FTS, a cross-platform 'port-to' walker, a deprecation-cliff report, and an MCP server you can plug into Claude Desktop.

Printed by [@jcastillo725](https://github.com/jcastillo725) (Joseph Alvin Castillo).

## Install

The recommended path installs both the `apple-docs-pp-cli` binary and the `pp-apple-docs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install apple-docs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install apple-docs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install apple-docs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install apple-docs --agent claude-code
npx -y @mvanhorn/printing-press-library install apple-docs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/cmd/apple-docs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apple-docs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-apple-docs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-apple-docs --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-apple-docs skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-apple-docs. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apple-docs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/cmd/apple-docs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "apple-docs": {
      "command": "apple-docs-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No auth required. The DocC JSON endpoints under https://developer.apple.com/tutorials/data/ are public and served by Apple's CDN.

## Quick Start

```bash
# Health check — verify network reachability without writing local state.
apple-docs-pp-cli doctor --dry-run

# Cache the master list of every Apple framework locally.
apple-docs-pp-cli sync --resources technologies --full

# Regex over a framework's index — finds every symbol whose title or path matches.
apple-docs-pp-cli grep onAppear --framework swiftui --json

# Token-lean projection: just the declaration fragments, not the full 50KB page.
apple-docs-pp-cli doc get 'swiftui/view/onappear(perform:)' --shape signature --agent

# Every SwiftUI API Apple deprecated in iOS 18.
apple-docs-pp-cli deprecation-cliff --os iOS --version 18 --framework swiftui --agent

```

## Unique Features

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

## Usage

Run `apple-docs-pp-cli --help` for the full command reference and flag list.

## Commands

### doc get

Fetch any documentation page (framework root, symbol, method, article) by path

- **`apple-docs-pp-cli doc get <path>`** - Fetch a documentation page by path and return it in the shape your tool actually wants (raw DocC JSON, projected via `--shape abstract|signature|platforms|min`, or rendered as Markdown via `--markdown`). The path is the lowercase-slashed identifier under /documentation/, e.g., `swiftui`, `swiftui/view`, or `swiftui/view/onappear(perform:)`.

### index

Fetch the full structured index of a framework (every symbol, in tree form)

- **`apple-docs-pp-cli index <framework>`** - Fetch a framework's full hierarchical index. Useful for offline FTS sync. Indexes are large (500KB–2MB).

### technologies

List every Apple framework and technology (Swift, SwiftUI, UIKit, etc.)

- **`apple-docs-pp-cli technologies`** - Fetch the master technologies index — every Apple framework, grouped by topic.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
apple-docs-pp-cli doc get swiftui/view

# JSON for scripting and agents
apple-docs-pp-cli doc get swiftui/view --json

# Filter to specific fields
apple-docs-pp-cli doc get swiftui/view --json --select abstract,declaration,platforms

# Dry run — show the request without sending
apple-docs-pp-cli doc get swiftui/view --dry-run

# Agent mode — JSON + compact + no prompts in one flag
apple-docs-pp-cli doc get swiftui/view --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `APPLE_DOCS_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `apple-docs-pp-cli technologies`
- `apple-docs-pp-cli technologies get`
- `apple-docs-pp-cli technologies list`
- `apple-docs-pp-cli technologies search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
apple-docs-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/apple-docs-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **404 from developer.apple.com for a framework slug** — Slugs are case-sensitive lowercase with no separators. 'photokit' works; 'photoskit' returns 404. Try 'apple-docs-pp-cli technologies list' to list valid slugs.
- **Cross-framework grep returns no results** — Pass --framework <slug> to focus the scan, or raise --max-scan-frameworks; grep fetches one index per framework.
- **Index sync is slow for SwiftUI/Foundation** — Framework indices are 500KB–2MB; pass '--resources index --max-pages 1' on first sync, then add frameworks one at a time.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**apple-docs-mcp**](https://github.com/kimsungwhee/apple-docs-mcp) — TypeScript (1300 stars)
- [**swift-docc**](https://github.com/swiftlang/swift-docc) — Swift (1100 stars)
- [**swift-docc-render**](https://github.com/swiftlang/swift-docc-render) — JavaScript (350 stars)
- [**sosumi-mcp**](https://github.com/nshipster/sosumi-mcp) — TypeScript (50 stars)
- [**appledeepdoc-mcp**](https://github.com/Ahrentlov/appledeepdoc-mcp) — Python (14 stars)
- [**apple-developer-toolkit**](https://github.com/Abdullah4AI/apple-developer-toolkit) — Go (8 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
