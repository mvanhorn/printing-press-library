# VirusTotal CLI - Transcendence Features Test Guide

This document demonstrates the 5 priority transcendence features added to the VirusTotal CLI.

## Prerequisites

```bash
export VIRUSTOTAL_API_KEY="your-api-key-here"
```

## Feature 1: Local SQLite Intelligence Store

The CLI now maintains a local SQLite cache at `~/.virustotal/cache.db` that stores all fetched IOCs for offline search and analysis.

### Test Store Creation

```bash
# The store is created automatically on first use
./virustotal-pp-cli files get <some-hash>

# View store location and stats
ls -lh ~/.virustotal/cache.db
sqlite3 ~/.virustotal/cache.db "SELECT COUNT(*) FROM files"
```

### Schema

The store includes tables for:
- `files` - File reports with indexed hashes, detection stats
- `domains` - Domain reports with reputation, categories
- `ip_addresses` - IP reports with ASN, location
- `urls` - URL scan results
- `relationships` - Graph edges for pivot traversal
- `iocs_fts` - FTS5 full-text search index

## Feature 2: Pivot Workflows (Graph Traversal)

Automatically traverse file → domain → IP → file relationships.

### Basic Pivot

```bash
# File to contacted domains
./virustotal-pp-cli pivot file <sha256> --through domains

# File to domains to IPs (2 hops)
./virustotal-pp-cli pivot file <sha256> --through domains --to ips --depth 2

# IP to communicating files
./virustotal-pp-cli pivot ip 1.2.3.4 --through files
```

### Output Formats

```bash
# Table format (default)
./virustotal-pp-cli pivot file <hash> --through domains

# JSON for programmatic use
./virustotal-pp-cli pivot file <hash> --through domains --format json

# Mermaid diagram
./virustotal-pp-cli pivot file <hash> --through domains --format mermaid > graph.mermaid
```

### Multi-hop Traversal

```bash
# 3-hop traversal: file → domains → IPs → related files
./virustotal-pp-cli pivot file <hash> --through domains --to files --depth 3
```

Results are cached in the SQLite store for offline re-querying.

## Feature 3: Batch IOC Enrichment Pipeline

Process multiple IOCs in parallel and generate structured reports.

### Create Test IOC File

```bash
cat > iocs.txt <<EOF
44d88612fea8a8f36de82e1278abb02f
275a021bbfb6489e54d471899f7db9d1663fc695ec2fe2a2c4538aabf651fd0f
google.com
8.8.8.8
EOF
```

### Enrich from File

```bash
# Basic enrichment
./virustotal-pp-cli enrich --input iocs.txt

# JSON output
./virustotal-pp-cli enrich --input iocs.txt --output report.json --json

# High concurrency
./virustotal-pp-cli enrich --input iocs.txt --concurrency 10
```

### Enrich from stdin

```bash
cat iocs.txt | ./virustotal-pp-cli enrich --type auto
```

### Report Contents

The enrichment report includes:
- Total IOCs processed
- Enriched vs cached vs failed counts
- Malicious vs clean breakdown
- Top high-confidence threats (detection ratio >= 10)
- Per-type and per-status statistics
- Full IOC data with detection ratios

## Feature 4: Agent-Native Output Mode (--llm flag)

Restructures JSON responses into human-readable format optimized for LLM consumption.

### File Reports

```bash
# Standard JSON output
./virustotal-pp-cli files get <hash> --json

# LLM-friendly output
./virustotal-pp-cli files get <hash> --llm
```

LLM output includes:
```
Detection: 45/70 engines flagged as malicious or suspicious
  - Malicious: 45
  - Suspicious: 0
  - Harmless: 20
  - Undetected: 5

Consensus verdict: Trojan.Generic (reported by 12 engines)

Reputation: -89 (highly malicious)

First seen: 2024-03-15 14:22:11 UTC
Last seen: 2024-05-10 10:30:00 UTC

SHA256: 275a021bbfb6489e54d471899f7db9d1663fc695ec2fe2a2c4538aabf651fd0f
MD5: 44d88612fea8a8f36de82e1278abb02f
Size: 1.2 MB
Type: Win32 EXE

Behavioral indicators:
  - Creates registry keys in HKLM\Software\Microsoft\Windows\CurrentVersion\Run
  - Network connections to known C2 IP 192.0.2.1
  - File drops in %TEMP%

Contacted domains: 5
  - evil.com
  - malware-c2.net
  ... and 3 more

Tags: trojan, malware, ransomware
```

### Domain Reports

```bash
./virustotal-pp-cli domains <domain> --llm
```

### IP Reports

```bash
./virustotal-pp-cli ip-addresses <ip> --llm
```

## Feature 5: Detection Diff Command

Compare two file reports to identify detection differences.

### Basic Diff

```bash
./virustotal-pp-cli diff <hash1> <hash2>
```

Output:
```
File Comparison
===============

Hash 1: 275a021b...51fd0f
Hash 2: 44d88612...bb02f

Metadata:
  Size:            1.2 MB vs 856 KB
  Type:            Win32 EXE vs Win32 DLL
  Times Submitted: 150 vs 89

Detection:
  Malicious:       45/70 vs 32/68
  Only in first:   13 engines
  Only in second:  5 engines
  Both detected:   32 engines

Behavior:
  Common indicators: 8
  Differences: 5
```

### Detailed Diff

```bash
./virustotal-pp-cli diff <hash1> <hash2> --detailed
```

Shows full engine lists:
```
Detection:
  Only in first:   13 engines
    - Kaspersky
    - Bitdefender
    - ESET-NOD32
    ...

  Only in second:  5 engines
    - ClamAV
    - Sophos
    ...

  Both detected:   32 engines
    - Microsoft
    - Symantec
    - McAfee
    ...

Behavior:
  Common indicators: 8
    - Creates registry keys
    - Network connections
    ...

  Differences: 5
    - Registry persistence (only in first)
    - File encryption (only in second)
    ...
```

### JSON Diff

```bash
./virustotal-pp-cli diff <hash1> <hash2> --json
```

Returns structured JSON for programmatic comparison.

## Combined Workflows

### Threat Hunting Workflow

```bash
# 1. Enrich a list of suspicious hashes
./virustotal-pp-cli enrich --input suspicious.txt --output enriched.json --json

# 2. Pivot from a malicious sample
./virustotal-pp-cli pivot file <malicious-hash> --through domains --depth 3 --format json > graph.json

# 3. Compare variants
./virustotal-pp-cli diff <variant1> <variant2> --detailed

# 4. Query cached data offline
sqlite3 ~/.virustotal/cache.db "SELECT sha256, detection_ratio FROM files WHERE malicious_votes > 20"
```

### Agent Integration

```bash
# Agent-friendly flags
./virustotal-pp-cli enrich --input iocs.txt --agent --llm

# Combines:
# --json (JSON output)
# --compact (minimal fields)
# --no-input (no prompts)
# --no-color (plain text)
# --yes (auto-confirm)
# --llm (LLM-optimized format)
```

## Cache Management

### View Cache Stats

```bash
sqlite3 ~/.virustotal/cache.db <<EOF
SELECT 'Files', COUNT(*) FROM files
UNION ALL
SELECT 'Domains', COUNT(*) FROM domains
UNION ALL
SELECT 'IPs', COUNT(*) FROM ip_addresses
UNION ALL
SELECT 'Relationships', COUNT(*) FROM relationships;
EOF
```

### Search Cached IOCs

```bash
# Full-text search
sqlite3 ~/.virustotal/cache.db "SELECT ioc_type, ioc_id FROM iocs_fts WHERE iocs_fts MATCH 'trojan' LIMIT 10"

# Query by detection ratio
sqlite3 ~/.virustotal/cache.db "SELECT sha256, detection_ratio, malicious_votes FROM files WHERE malicious_votes >= 30 ORDER BY malicious_votes DESC"
```

### Clear Cache

```bash
rm ~/.virustotal/cache.db
```

## Performance Notes

- **Caching**: All API responses are cached. Subsequent queries are instant.
- **Batch Enrichment**: Uses worker pool (default 5 workers, configurable via --concurrency)
- **Pivot Traversal**: BFS algorithm with visited tracking to avoid cycles
- **Rate Limiting**: Respects --rate-limit flag and VT API quotas

## Error Handling

All commands handle:
- API authentication errors
- Rate limiting (with exponential backoff)
- Network failures
- Invalid IOC formats
- Missing API keys

## Integration with Existing Features

The transcendence features integrate seamlessly with existing commands:

```bash
# Use --data-source local to query cache only (no API calls)
./virustotal-pp-cli files get <hash> --data-source local

# Combine with sync command
./virustotal-pp-cli sync files
./virustotal-pp-cli enrich --input hashes.txt  # Uses synced data
```

## Architecture

- **vtstore package**: SQLite schema for IOC intelligence
- **pivot.go**: Graph traversal with BFS
- **enrich.go**: Parallel batch enrichment
- **diff.go**: Detection comparison engine
- **llm_format.go**: Human-readable formatters
- Integration with existing client, config, and CLI infrastructure

All features follow the printing-press CLI patterns:
- Cobra commands
- rootFlags integration
- Agent-friendly defaults
- JSON/CSV/table output modes
- Proper error handling and exit codes
