# Dogecoin Core CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | getmininginfo (hashrate, difficulty, blocks) | bitcoin-cli / dogecoin-cli | `mining info --json` | --compact high-gravity fields, --json for scripts |
| 2 | getnetworkhashps | bitcoin-cli | `mining networkhashps --json` | --blocks to average, readable units (TH/s PH/s) |
| 3 | getblockchaininfo | bitcoin-cli | `blockchain info --json` | sync %, bestblockhash |
| 4 | getblockcount | bitcoin-cli | `blockchain count` | plain number, scriptable |
| 5 | getblock / getblockhash | bitcoin-cli | `blockchain get <hash>`, `blockchain hash <height>` | human summary + full JSON |
| 6 | getnetworkinfo (version, peers, subversion) | dogecoin-node-monitor | `network info --json` | surfaces version warning inline |
| 7 | getpeerinfo (per-peer detail) | bitcoin-cli | `network peers --json` | per-peer table, ban filter |
| 8 | getwalletinfo (balance, unconfirmed, immature) | bitcoin-cli | `wallet info --json` | shows immature mining rewards |
| 9 | listtransactions | bitcoin-cli | `wallet transactions --count N` | --json, filtering |
| 10 | getmempoolinfo (size, bytes) | bitcoin-cli | `mempool info --json` | fee summary alongside size |
| 11 | estimatefee (per-KB fee) | bitcoin-cli | `mempool fees <blocks>` | human DOGE/KB display |
| 12 | node uptime | dogecoin-node-monitor | `node uptime` | human duration + seconds |
| 13 | Prometheus scraping (node-monitor) | dogecoin-node-monitor | `sync` → SQLite | offline, queryable, no Prometheus overhead |
| 14 | Version tracking | getnetworkinfo.version | `doctor`, `network info` | threshold check, obsolete warning |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Compound mining stats | `mining stats` | Single call returns hashrate + difficulty + network hashrate + block height with --compact/--json — no existing tool combines these for script consumption |
| 2 | Typed peer health check | `peers health` | Exit 0=healthy, exit 3=low peers — enables n8n shell node branching; no existing tool has typed exit codes |
| 3 | Block window detector | `blocks found --since 7d` | Checks if blocks were mined in a time window; exit 0=found, exit 2=none — n8n alerting with no manual scripting |
| 4 | Historical hashrate trending | `hashrate history --since 30d` | SQLite-backed time-series query; no existing Dogecoin tool stores and queries historical hashrate |
| 5 | Difficulty trend alert | `difficulty trend --days 7` | Detects difficulty spike/drop from SQLite history; exit 0=stable, exit 5=spike — proactive mining strategy alerts |
| 6 | Mempool status compound | `mempool status` | getmempoolinfo + estimatefee combined with fee tier labels (low/medium/high DOGE/KB) |
| 7 | Node health snapshot | `doctor` | Version obsolescence check (< 1.14.0 threshold), auth validation, peer count, sync progress, uptime — all in one command |
| 8 | XEMD dashboard export | `stats export --format xemd` | Compact JSON payload shaped for XEMD widget consumption; no other tool formats for this target |
| 9 | MCP native state query | MCP surface (Cobra tree mirror) | Claude can query mining state, peer health, wallet balance natively via Homelab MCP without n8n |

## Final Transcendence Table (post-cut, all ≥5/10)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|---------|
| 1 | Compound mining stats | `mining stats` | 9/10 | Joins getmininginfo + getnetworkhashps + getblockchaininfo into single JSON with --compact | Brief top workflow #1; n8n daily stats |
| 2 | Peer health gate | `peers health` | 9/10 | Counts getpeerinfo; exit 0 healthy, exit 3 below --min-peers threshold | Brief top workflow #2; n8n exit-code branching |
| 3 | Hashrate drop alert | `mining alert` | 8/10 | getnetworkhashps vs. last SQLite snapshot; exit 3 if drop > --threshold % | Brief explicit alert requirement; SQLite join |
| 4 | Hashrate + difficulty history | `hashrate history` | 8/10 | Query mining_snapshots SQLite; --since flag (30d/7d); JSON array for XEMD | Brief top workflow #4; data layer |
| 5 | Block event log | `blocks log` | 7/10 | block_events SQLite table query by --since window; timestamp + hash | MCP agent; daily note use case |
| 6 | Blocks found detector | `blocks found` | 7/10 | block_events + coinbase filter; exit 0 = found in window, exit 2 = none | Brief top workflow #3; solo miner |
| 7 | Peer version breakdown | `peers breakdown` | 6/10 | getpeerinfo grouped by subver, inbound/outbound counts | dogecoin-node-monitor; pool operator |
| 8 | Version + node health | `node health` | 6/10 | getnetworkinfo version < 1140000 check + verificationprogress; exit 1 unhealthy | Brief explicit requirement |
| 9 | Mempool compound status | `mempool status` | 6/10 | getmempoolinfo + estimatefee(1) + estimatefee(6); fee tier labels | Brief mempool workflow; MCP single-call |
| 10 | Difficulty trend | `mining trend` | 5/10 | mining_snapshots % change over --window; exit 5 on spike | Pool operator persona; SQLite compute |
