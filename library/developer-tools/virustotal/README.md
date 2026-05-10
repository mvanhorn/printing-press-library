# VirusTotal CLI

**The local-first malware intelligence tool that pivots through IOC relationships, enriches threats in parallel, and caches everything for offline correlation.**

virustotal-pp-cli wraps VirusTotal API v3 (files, URLs, domains, IPs, relationships, YARA) and layers a local SQLite intelligence store on top. That powers offline IOC search, multi-hop graph traversal, batch enrichment pipelines, detection diff analysis, and agent-optimized output formatting. Free tier supported, scriptable, works offline after initial sync.

Learn more at [VirusTotal](https://www.virustotal.com).

## Install

The recommended path installs both the `virustotal-pp-cli` binary and the `pp-virustotal` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install virustotal
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install virustotal --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/ca7ai/pp-virustotal/cmd/virustotal-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/ca7ai/pp-virustotal/releases). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

VirusTotal requires an API key for all operations.

1. Get your API key from [virustotal.com/gui/my-apikey](https://virustotal.com/gui/my-apikey)
   - Free tier: 4 requests/minute, 500/day
   - Premium: Up to 1000 requests/minute

2. Set the environment variable:
   ```bash
   export VIRUSTOTAL_API_KEY="your-key-here"
   ```

3. Add to `~/.zshrc` or `~/.bashrc` to persist:
   ```bash
   echo 'export VIRUSTOTAL_API_KEY="your-key-here"' >> ~/.zshrc
   ```

## Quick Start

```bash
# Verify setup and connectivity
virustotal-pp-cli doctor

# Look up a file hash
virustotal-pp-cli files get 44d88612fea8a8f36de82e1278abb02f

# Look up an IP address
virustotal-pp-cli ip-addresses 8.8.8.8

# Look up a domain
virustotal-pp-cli domains google.com

# Scan a URL
virustotal-pp-cli urls scan https://example.com

# Search VirusTotal (requires Premium)
virustotal-pp-cli virustotal-search "type:domain AND last_analysis_stats.malicious:>0"

# Pivot through IOC relationships: file → domains → IPs
virustotal-pp-cli pivot file 44d88612fea8a8f36de82e1278abb02f --through domains --to ips --depth 2 --json

# Batch enrich a list of IOCs
cat iocs.txt | virustotal-pp-cli enrich --input - --output report.json --workers 4

# Compare two file hashes
virustotal-pp-cli diff 44d88612fea8a8f36de82e1278abb02f 275a021bbfb6489e54d471899f7db9d1663fc695ec2fe2a2c4538aabf651fd0f
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local Intelligence Store
- **`sync`** — Populate local SQLite cache with IOC data for offline search and correlation.

  _After syncing, search and correlation commands work without API calls — critical when you're rate-limited or analyzing on an air-gapped system._

  ```bash
  virustotal-pp-cli sync files --hashes hashes.txt
  virustotal-pp-cli sync domains --list suspicious-domains.txt
  ```

- **Local search** — FTS5 full-text search over cached IOC metadata.

  _When you need to find "all files first seen after 2024-01-01 with detection ratio >20" without burning API quota._

  ```bash
  virustotal-pp-cli search "detection_ratio > 20 AND first_seen > 2024-01-01" --local
  ```

### Graph Traversal
- **`pivot`** — Multi-hop relationship traversal: file → contacted_domains → resolved_ips → communicating_files.

  _Agents investigating malware campaigns can automatically map infrastructure without manual clicking through the VT web UI._

  ```bash
  virustotal-pp-cli pivot file 44d88612fea8a8f36de82e1278abb02f --through domains --to ips --depth 3 --format mermaid
  ```

### Batch Operations
- **`enrich`** — Parallel IOC enrichment pipeline with auto-type detection (SHA256/MD5/SHA1/IP/domain).

  _Process 100+ IOCs from an incident response spreadsheet in one command; get a structured threat report with detection summaries._

  ```bash
  virustotal-pp-cli enrich --input iocs.txt --output report.json --workers 8 --rate-limit 4
  ```

### Analysis Tools
- **`diff`** — Compare two file reports side-by-side: detection engines, metadata, behavioral indicators.

  _When triaging "is this the same malware or a variant", concrete engine-level diff tells you what changed._

  ```bash
  virustotal-pp-cli diff <hash1> <hash2> --detailed
  ```

- **Agent-native output** — `--llm` flag reformats JSON into LLM-optimized text summaries.

  _Agents get "Detection: 45/70 engines, Consensus: Trojan.Generic" instead of parsing 70-element nested JSON._

  ```bash
  virustotal-pp-cli files get 44d88612fea8a8f36de82e1278abb02f --llm
  ```

## Usage

Run `virustotal-pp-cli --help` for the full command reference and flag list.

## Commands

The full command tree is discoverable via `virustotal-pp-cli --help`. Headline groups:

- **IOC Lookup** — `files get`, `urls get`, `domains get`, `ip-addresses get`
- **Scanning** — `files scan`, `urls scan`
- **Search** — `virustotal-search` (Premium only)
- **Graph Analysis** — `pivot`, `enrich`, `diff`
- **Local Store** — `sync`, `search --local`
- **Framework helpers** — `doctor`, `version`, `which`, `agent-context`, `auth`, `profile`, `workflow`

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
virustotal-pp-cli files get <hash>

# JSON for scripting and agents
virustotal-pp-cli files get <hash> --json

# Compact output (key fields only)
virustotal-pp-cli files get <hash> --compact

# Agent-optimized text summary
virustotal-pp-cli files get <hash> --llm

# Filter to specific fields
virustotal-pp-cli files get <hash> --json --select id,attributes.last_analysis_stats

# Dry run — show the request without sending
virustotal-pp-cli files get <hash> --dry-run

# Agent mode — JSON + compact + no prompts in one flag
virustotal-pp-cli files get <hash> --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-mostly** - scanning creates remote state; all other commands are read-only
- **Offline-friendly** - pivot/enrich/diff/search commands use local SQLite when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Rate-limit aware** - built-in backoff and retry with `--rate-limit` override

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Rate Limits

Free tier: 4 requests/minute, 500/day. CLI auto-throttles and retries on HTTP 429.

Override default rate limit:
```bash
virustotal-pp-cli files get <hash> --rate-limit 2.0  # 2 req/sec (Premium tier)
```

Set `VIRUSTOTAL_RATE_LIMIT` environment variable to apply globally:
```bash
export VIRUSTOTAL_RATE_LIMIT=4  # 4 req/min (free tier)
```

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add ca7ai/pp-virustotal/cli-skills/pp-virustotal -g
```

Then invoke `/pp-virustotal <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/ca7ai/pp-virustotal/cmd/virustotal-pp-mcp@latest
```

Then register it:

```bash
claude mcp add virustotal virustotal-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/ca7ai/pp-virustotal/releases).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/ca7ai/pp-virustotal/cmd/virustotal-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "virustotal": {
      "command": "virustotal-pp-mcp",
      "env": {
        "VIRUSTOTAL_API_KEY": "your-key-here"
      }
    }
  }
}
```

</details>

## Health Check

```bash
virustotal-pp-cli doctor
```

Verifies API key, configuration, and connectivity to VirusTotal.

## Configuration

Config file: `~/.config/virustotal-pp-cli/config.toml`

Local store: `~/.local/share/virustotal-pp-cli/data.db`

## Troubleshooting

**Authentication errors (HTTP 401)**
- Verify your API key is set: `echo $VIRUSTOTAL_API_KEY`
- Get a new key from [virustotal.com/gui/my-apikey](https://virustotal.com/gui/my-apikey)
- Check `virustotal-pp-cli doctor` output

**Rate limit errors (HTTP 429)**
- Free tier: 4 req/min, 500/day
- Wait 60 seconds between requests or use `--rate-limit 0.067` (4/min)
- Upgrade to Premium for higher limits

**Not found errors (HTTP 404)**
- File/URL/domain hasn't been scanned yet
- For files: submit via `virustotal-pp-cli files scan <path>`
- For URLs: submit via `virustotal-pp-cli urls scan <url>`

**Search returns empty (Premium only)**
- VirusTotal search requires a Premium API key
- Verify your key has search access: `virustotal-pp-cli doctor`

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**VirusTotal/vt-py**](https://github.com/VirusTotal/vt-py) — Official Python client (1.1k stars)
- [**@burtthecoder/mcp-virustotal**](https://www.npmjs.com/package/@burtthecoder/mcp-virustotal) — MCP server (npm)
- [**node-virustotal**](https://www.npmjs.com/package/node-virustotal) — JavaScript client (16k monthly downloads)
- [**VirusTotal-CLI**](https://github.com/VirusTotal/vt-cli) — Official Go CLI (Python, 9 stars)
- [**yassinech-99/virustotal_mcp**](https://github.com/yassinech-99/virustotal_mcp) — MCP server implementation
- [**barvhaim/virustotal-mcp-server**](https://github.com/barvhaim/virustotal-mcp-server) — Alternative MCP implementation

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
