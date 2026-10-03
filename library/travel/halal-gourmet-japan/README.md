# Halal Gourmet Japan CLI

**Compare explicit food and prayer evidence across Japan with source dates and honest unknowns.**

Search restaurants, mosques and prayer spaces by destination. Inspect full conditions, then compare saved evidence, match explicit requirements, identify missing facts, pair food and prayer stops, and track factual changes. Certification labels remain separate from HGJ verification and other conditions.

## Install

The recommended path installs both the `halal-gourmet-japan-pp-cli` binary and the `pp-halal-gourmet-japan` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install halal-gourmet-japan
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install halal-gourmet-japan --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install halal-gourmet-japan --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install halal-gourmet-japan --agent claude-code
npx -y @mvanhorn/printing-press-library install halal-gourmet-japan --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/cmd/halal-gourmet-japan-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/halal-gourmet-japan-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install halal-gourmet-japan --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-halal-gourmet-japan --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-halal-gourmet-japan --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install halal-gourmet-japan --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/halal-gourmet-japan-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/cmd/halal-gourmet-japan-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "halal-gourmet-japan": {
      "command": "halal-gourmet-japan-pp-mcp"
    }
  }
}
```

</details>

## Authentication

The scoped public pages require no account or API key. Requests use ordinary HTTPS and never book, post or alter an account.

## Quick Start

```bash
# Check installation without network or credentials.
halal-gourmet-japan-pp-cli doctor --dry-run

# Discover bounded restaurant cards; some conditions are hidden on cards.
halal-gourmet-japan-pp-cli restaurants search --prefecture Tokyo --query ramen --limit 5 --agent

# Save full restaurant facts and source verification.
halal-gourmet-japan-pp-cli restaurants get 300739 --agent

# Save full prayer-space facts and access unknowns.
halal-gourmet-japan-pp-cli prayer get 838884 --agent

# Evaluate each condition from saved full-detail evidence.
halal-gourmet-japan-pp-cli plan match --restaurants 300739 --require certified,meat,prayer --agent

```

## Cookbook

```bash
# Discover source-reported certification labels; filters are discovery semantics.
halal-gourmet-japan-pp-cli restaurants search --prefecture Kyoto --genre Ramen --feature certified --limit 5 --agent

# Prayer spaces with source-reported Wudu facilities; inspect the returned IDs.
halal-gourmet-japan-pp-cli prayer search --prefecture Tokyo --place-type spaces --prayer-feature wudu --limit 5 --agent

# Compare only saved detail observations, with useful clarification questions.
halal-gourmet-japan-pp-cli plan compare --restaurants 300739 --prayer 838884 --agent
halal-gourmet-japan-pp-cli plan gaps --restaurants 300739 --prayer 838884 --agent
halal-gourmet-japan-pp-cli plan pair --restaurants 300739 --prayer 838884 --max-km 5 --agent

# Save another successful inspection to establish the second change baseline.
halal-gourmet-japan-pp-cli restaurants get 300739 --data-source live --agent
halal-gourmet-japan-pp-cli plan changes --restaurants 300739 --agent
```

## Evidence and boundaries

`reported` means an explicit source condition. `not_reported` means full-detail evidence is absent; `not_visible_on_card` means a card cannot establish it; `inapplicable` belongs to another entity family. An explicit source JSON-LD false value is preserved as `reported_negative`. These states never collapse Muslim-friendly, halal meat, no pork, vegetarian options, certification and verification into one assurance.

Food keys: `certified`, `owner`, `noAlcoholicDrinks`, `porkFree`, `meat`, `seasoning`, `tableware`, `meal`, `vegetarian`, `prayer`. Prayer keys: `wudu`, `wifi`, `hotWater`, `qibla`.

Detail output keeps canonical IDs/URLs, source names, Japanese names when supplied, observation timestamps, explicit labels, verification months, published hours, source coordinates and prayer access text. Price ranges retain their source currency text and are directory ranges rather than dated quotes. Weekly hours and prose hours are reported facts; no opening-now promise is calculated.

This supports nationwide prefecture discovery, not a complete inventory mirror. It does not provide ingredient/allergy certainty, religious judgments, guaranteed cross-contamination control, certification validity, prayer times, routes, current access, date-specific availability, bookings, reviews, check-ins or account writes. Prayer notes can contain restrictions such as customer access, security desks, doors or airport airside areas; absence of such notes never grants access.

## Agent Usage

```bash
halal-gourmet-japan-pp-cli restaurants search --prefecture Tokyo --query ramen --limit 5 --agent --select results.id,results.name,results.source_url
halal-gourmet-japan-pp-cli restaurants get 300739 --agent --select id,name,certification,verification
```

`--json` emits a plain result object; `--agent` adds provenance in `meta.source` with a `results` envelope. Search already has its own `results` list and retains one envelope. Dotted `--select` paths project fields before automatic wrapping. Errors/warnings stay on stderr; API errors also carry structured JSON when machine output is requested. `--dry-run` does no network or cache I/O. Formats include `--json`, `--compact`, `--select`, `--csv`, `--plain` and `--quiet`.

## Local state

Successful `get` reads save two factual observations per source kind/ID, capped at 1,000 places. Failed fetches/parses and search cards never become history baselines. Default auto/live inspections refresh the source and propagate failures; `--data-source local` explicitly reads a saved observation. Search has no local equivalent. All `plan` commands read local snapshots and reject `--data-source live`.

Use the same `--db PATH` for corresponding inspections and planning, or relocate all CLI data with `--home DIR` / `HALAL_GOURMET_JAPAN_HOME`. `--no-cache` suppresses inspection history writes; `--max-age` controls stale-observation hints, with a 24-hour default and 0 disabling the hint. The generated learning loop is separate from factual history; use `--no-learn` for deterministic/private prompts. Nothing is a bulk source sync.

## Health Check

```bash
halal-gourmet-japan-pp-cli doctor --json
halal-gourmet-japan-pp-cli agent-context --pretty
```

## Troubleshooting

| Symptom | Action |
|---|---|
| Missing full-detail snapshot | Run the corresponding get command with the same `--db` or home |
| `baseline_missing` | Save a second successful live inspection before `plan changes` |
| Hidden/missing condition | Inspect the full record; clarify absent facts with the venue |
| Source HTTP/format error | Inspect the canonical page; an error is distinct from no results |
| Throttle, exit 7 | Wait and retry the bounded read |
| Cache cap | Use `--no-cache` for an uncached inspection or another cache path |

Exit codes: 0 successful result, 2 usage, 3 missing record, 5 upstream/parsing, 7 rate limit, 10 configuration/cache. A missing planning snapshot is an explicit exit-0 planning gap, not a successful source inspection.

### API-specific
- **No saved full-detail snapshot for a selected ID** — Run the corresponding restaurants get ID or prayer get ID --data-source live before local planning.
- **Condition is not reported or hidden on a card** — Inspect the full detail; ask the venue to clarify missing facts before relying on them.
- **Source access failed or response format changed** — Read the explicit source error and use the canonical page; empty results never replace a failed request.
- **cache_visibility_unavailable for saved evidence** — Finish and close active database writers, then retry. Saved get, all plans and MCP SQL conservatively reject a non-empty WAL (even checkpointed but untruncated) or changing path/database identity with CLI exit10. Readers and snapshot writers resolve symlinks to one canonical database; hard-linked databases are rejected where the host exposes link-count metadata. Use a database with one hard link and keep files in place until writers close. Live inspection with --no-cache remains available. Facts retain exact observed_at timestamps. Saved reads use private temporary immutable images copied from verified pinned file descriptors, bounded at128 MiB and removed after each read. A larger cache fails explicitly; use a smaller cache. Cache paths and resolved alias targets containing percent, question-mark or hash bytes are rejected before SQLite opening; choose a plain filesystem name. The temporary-directory path and its resolved target must also use plain filesystem names. Use an ordinary TMPDIR path. Snapshot writes retain normal SQLite transaction/WAL semantics; external same-user file replacement during writing is unsupported, and detected retargeting returns an explicit error.
- **Invalid global --data-source value** — Choose auto, live or local as documented. This global framework validation currently exits1; domain input errors exit2.
- **Previewing source and cache work** — Pass --dry-run with the same required ID, scoped query/prefecture or selected IDs and requirements. Pure leaf validation runs before the action receipt and before source/config/cache work. Standard framework learning may still record command activity; --no-learn disables it.

## Source and implementation

The public search and full-detail contracts were observed with native Browser Use and verified over normal HTTPS on 2026-10-03. The parser supports streamed server-rendered listing fragments and canonical JSON-LD detail records. Requests are bounded by timeout, response bytes and result/selection caps; throttles surface typed errors. Output stores factual projections, excluding signed media links, sessions, contributor profiles and full restaurant descriptions.

Built with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press). Local source adapters and planning code live in `internal/hgj` and the preserved CLI extension files; see `AGENTS.md` for maintenance invariants. Public per-CLI release versions are assigned by library automation after merge.

## Recipes

### Narrow restaurant discovery

```bash
halal-gourmet-japan-pp-cli restaurants search --prefecture Tokyo --query ramen --limit 5 --agent --select results.id,results.name,results.source_url
```

Cards discover candidates; missing card icons are not a requirement verdict.

### Inspect food and prayer evidence

```bash
halal-gourmet-japan-pp-cli plan compare --restaurants 300739 --prayer 838884 --agent
```

Load successful detail snapshots before comparing saved facts.

### Clarify missing evidence

```bash
halal-gourmet-japan-pp-cli plan gaps --restaurants 300739 --prayer 838884 --agent
```

List unreported certifier, validity, applicable hours/access and coordinates.

### Compare proximity

```bash
halal-gourmet-japan-pp-cli plan pair --restaurants 300739 --prayer 838884 --max-km 5 --agent
```

Straight-line distance is not a route, walking time or access guarantee.

### Recheck saved observations

```bash
halal-gourmet-japan-pp-cli plan changes --restaurants 300739 --agent
```

Run restaurants get --data-source live again before comparing two successful observations.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Evidence planning
- **`plan compare`** — Align the source-reported food and prayer facts of selected saved places.

  _Compare full facts before choosing from a shortlist._

  ```bash
  halal-gourmet-japan-pp-cli plan compare --restaurants 300739,949742 --prayer 838884 --agent
  ```
- **`plan match`** — See which requested source labels are reported and which need confirmation.

  _Test every explicit requirement without issuing a halal verdict._

  ```bash
  halal-gourmet-japan-pp-cli plan match --restaurants 300739 --require certified,meat,prayer --agent
  ```
- **`plan gaps`** — List missing certifier, certificate validity, hours and access evidence.

  _Prepare factual questions for selected food and prayer stops._

  ```bash
  halal-gourmet-japan-pp-cli plan gaps --restaurants 300739 --prayer 838884 --agent
  ```
- **`plan pair`** — Pair selected restaurants and prayer places by straight-line distance.

  _Assess proximity while preserving access and hours unknowns._

  ```bash
  halal-gourmet-japan-pp-cli plan pair --restaurants 300739 --prayer 838884 --max-km 5 --agent
  ```

### Saved observations
- **`plan changes`** — See factual changes between the latest two successful observations.

  _Recheck a shortlist before visiting and inspect changed evidence._

  ```bash
  halal-gourmet-japan-pp-cli plan changes --restaurants 300739 --prayer 838884 --agent
  ```
