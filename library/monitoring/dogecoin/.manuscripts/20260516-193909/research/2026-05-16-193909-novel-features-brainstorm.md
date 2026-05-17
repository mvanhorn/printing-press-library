# Novel Features Brainstorm — dogecoin-pp-cli

## Customer model

**Persona 1 — Jorge (homelab node runner / n8n automator)**
Runs a Dogecoin Core 1.10.0 full node on VLAN 20 in Docker. Uses n8n shell nodes to collect daily mining stats and push summaries to Obsidian. Checks peer count on a 15-minute cron; wants exit codes that n8n can branch on without parsing output. Monitors hashrate trends on an XEMD dashboard widget. Companion CLI is miningcore-pp-cli.

**Persona 2 — The MCP Agent (Claude via Homelab MCP)**
Queries live chain state conversationally. Needs clean JSON output and compound answers in a single tool call.

**Persona 3 — Pool Operator (adjacent)**
Paired with miningcore-pp-cli. Wants difficulty trend vs. pool hashrate side-by-side.

**Persona 4 — Solo Miner**
Polls for block finds. Uses `blocks found --since 24h` in a shell script. Wants exit 0 = found, exit 2 = none.

## Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|---------|
| 1 | Compound mining stats | `mining stats` | 9/10 | Joins getmininginfo + getnetworkhashps + getblockchaininfo; emits flat JSON | Brief top workflow #1; n8n daily stat collection |
| 2 | Peer health gate | `peers health` | 9/10 | Counts getpeerinfo connections; exit 0 ≥ threshold, exit 3 below | Brief top workflow #2; n8n exit-code branch |
| 3 | Hashrate drop alert | `mining alert` | 8/10 | getnetworkhashps + SQLite delta; exit 3 if drop > threshold | Brief explicit alert requirement |
| 4 | Hashrate + difficulty history | `mining history` | 8/10 | Query mining_snapshots SQLite; --since flag; XEMD source | Brief top workflow #4 |
| 5 | Block event log | `blocks log` | 7/10 | block_events SQLite table query by --since window | MCP agent "what happened today" |
| 6 | Blocks found detector | `blocks found` | 7/10 | block_events + coinbase address match; exit 0/2 | Brief top workflow #3 |
| 7 | Peer version breakdown | `peers breakdown` | 6/10 | getpeerinfo array grouped by subver, inbound/outbound | dogecoin-node-monitor pattern |
| 8 | Version + node health | `node health` | 6/10 | getnetworkinfo version check + verificationprogress; exit 1 if unhealthy | Brief explicit requirement |
| 9 | Mempool compound status | `mempool status` | 6/10 | getmempoolinfo + estimatefee(1) + estimatefee(6) | Brief top workflow mempool |
| 10 | Difficulty trend | `mining trend` | 5/10 | mining_snapshots % change over --window | Pool operator persona |

## Killed candidates
| Feature | Kill Reason |
|---------|------------|
| mining compare (local vs network hashrate) | generate=false always on monitoring node; value is zero |
| wallet history | 0.0 DOGE balance; no value to trend |
| blockchain fresh | Thin single-field wrapper; node health covers it |
| mining stats --n8n | Reimplementation smell; --json flag already covers it |
| Obsidian note push | External service not in spec |
| node summary | Duplicate of mining stats + node health |
