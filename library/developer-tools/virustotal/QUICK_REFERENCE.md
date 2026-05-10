# VirusTotal CLI - Transcendence Features Quick Reference

## Setup

```bash
export VIRUSTOTAL_API_KEY="your-api-key"
./virustotal-pp-cli --version  # Should show 1.0.0
```

## 5 New Commands/Features

### 1. Local Cache (Automatic)

Every API call is cached at `~/.virustotal/cache.db`

```bash
# Check cache stats
sqlite3 ~/.virustotal/cache.db "SELECT COUNT(*) FROM files"

# Search cached IOCs
sqlite3 ~/.virustotal/cache.db \
  "SELECT sha256, detection_ratio FROM files WHERE malicious_votes > 20"
```

### 2. Pivot (Graph Traversal)

```bash
# Basic pivot
virustotal-pp-cli pivot file <hash> --through domains

# Multi-hop
virustotal-pp-cli pivot file <hash> --through domains --to ips --depth 2

# Mermaid diagram
virustotal-pp-cli pivot file <hash> --through domains --format mermaid > graph.mermaid
```

### 3. Enrich (Batch Processing)

Create `iocs.txt`:
```
44d88612fea8a8f36de82e1278abb02f
google.com
8.8.8.8
```

```bash
# Enrich from file
virustotal-pp-cli enrich --input iocs.txt

# JSON output
virustotal-pp-cli enrich --input iocs.txt --json > report.json

# From stdin
cat iocs.txt | virustotal-pp-cli enrich

# High concurrency
virustotal-pp-cli enrich --input iocs.txt --concurrency 10
```

### 4. LLM Output Mode

```bash
# Human-readable format optimized for LLMs
virustotal-pp-cli files get <hash> --llm

# Compare with JSON
virustotal-pp-cli files get <hash> --json
```

Output example:
```
Detection: 45/70 engines flagged as malicious
Consensus verdict: Trojan.Generic (12 engines)
Reputation: -89 (highly malicious)
First seen: 2024-03-15 14:22:11 UTC
...
```

### 5. Diff (Compare Files)

```bash
# Basic diff
virustotal-pp-cli diff <hash1> <hash2>

# Detailed (shows full engine lists)
virustotal-pp-cli diff <hash1> <hash2> --detailed

# JSON output
virustotal-pp-cli diff <hash1> <hash2> --json
```

## Combined Workflows

### Threat Hunting
```bash
# 1. Batch enrich suspicious IOCs
virustotal-pp-cli enrich --input suspects.txt --json > enriched.json

# 2. Pivot from confirmed malware
virustotal-pp-cli pivot file <malicious-hash> --through domains --depth 3

# 3. Compare variants
virustotal-pp-cli diff <variant1> <variant2> --detailed

# 4. Query offline
sqlite3 ~/.virustotal/cache.db \
  "SELECT sha256 FROM files WHERE malicious_votes > 30 ORDER BY malicious_votes DESC"
```

### Agent Mode
```bash
# All agent-friendly flags at once
virustotal-pp-cli enrich --input iocs.txt --agent --llm
```

Combines: `--json --compact --no-input --no-color --yes --llm`

## Global Flags

All commands support:
- `--json` - JSON output
- `--llm` - LLM-friendly format
- `--agent` - Agent defaults
- `--compact` - Minimal fields
- `--dry-run` - Preview request
- `--no-cache` - Skip cache
- `--rate-limit <n>` - Requests/sec limit
- `--timeout <duration>` - Request timeout

## Cache Schema

Tables:
- `files` - SHA256, MD5, SHA1, detection stats
- `domains` - Reputation, categories
- `ip_addresses` - ASN, geolocation
- `urls` - Scan results
- `relationships` - Graph edges (pivot)
- `iocs_fts` - Full-text search

## Troubleshooting

```bash
# Missing API key
export VIRUSTOTAL_API_KEY="your-key"

# Clear cache
rm ~/.virustotal/cache.db

# Check binary
./virustotal-pp-cli --version

# View help
./virustotal-pp-cli pivot --help
./virustotal-pp-cli enrich --help
./virustotal-pp-cli diff --help
```

## Performance

- Cache reads: Sub-millisecond
- Batch enrichment: Parallel workers (default 5, max 10)
- Pivot depth: BFS traversal, avoids cycles
- Rate limiting: Respects API quotas

## Output Formats

All commands support multiple formats:
- Table (default)
- JSON (`--json`)
- LLM-friendly (`--llm`)
- CSV (`--csv`)
- Plain text (`--plain`)
- Quiet (`--quiet`)

## Key Features

✓ **Local-first**: Cache all API responses
✓ **Offline**: Query cache without API calls
✓ **Parallel**: Batch enrichment with worker pools
✓ **Graph traversal**: Multi-hop pivot workflows
✓ **LLM-native**: Human-readable structured output
✓ **Comparison**: Detection diff between samples
✓ **Full-text search**: FTS5 across all cached IOCs

## Documentation

- `TEST_TRANSCENDENCE_FEATURES.md` - Detailed test guide
- `TRANSCENDENCE_IMPLEMENTATION.md` - Implementation notes
- `IMPLEMENTATION_SUMMARY.txt` - Architecture summary
- This file - Quick reference

## Location

```
/Users/cal/claude_projects/printing-press-custom-cli/virustotal-cli/
```

## Build From Source

```bash
go build -o virustotal-pp-cli ./cmd/virustotal-pp-cli
```

No dependencies needed beyond go.mod (pure Go, CGO-free).
