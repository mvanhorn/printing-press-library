# Dogecoin Core CLI Brief

## API Identity
- Domain: Dogecoin Core full-node JSON-RPC (Bitcoin-derived protocol)
- Users: Solo miners, pool operators, homelab node runners, n8n automation workflows
- Data profile: Real-time chain state (blocks, difficulty, hashrate), wallet balances, mempool depth, peer health, historical trending
- Target node: http://10.10.20.7:8332/ — running in Docker on VLAN 20, Linux amd64

## Reachability Risk
- None — node is live, responding HTTP 200, Basic auth confirmed working
- All RPC methods probed successfully: getmininginfo, getblockchaininfo, getnetworkinfo, getwalletinfo, getmempoolinfo, getpeerinfo

## Live Node State (at research time)
- Version: 1.10.0 (Shibetoshi) — **OBSOLETE**: errors field says "Warning: This version is obsolete; upgrade required!"
- Block height: 6,209,302 (fully synced, verificationprogress: 0.9999999)
- Network hashrate: 2,859,678,419,607,174 H/s (~2.86 PH/s)
- Difficulty: 57,103,649.10
- Peers: 8 connections (networks not externally reachable — Docker)
- Mempool: 61 txs, 18,245 bytes
- Wallet: 0.0 DOGE balance (monitoring node, not actively mining solo)
- estimatesmartfee: not available (older protocol; estimatefee returns -1 = insufficient data)
- generate: false (CPU mining disabled)

## Version Obsolescence Handling
- Version 1100000 = Dogecoin Core 1.10.0
- Current latest: Dogecoin Core 1.14.x
- The `getmininginfo.errors` field surfaces the warning string directly
- The `getnetworkinfo.version` field carries the numeric version (1100000)
- doctor and health commands MUST check version and emit warning when version < 1140000
- Exit code: version warning is non-fatal (emit to stderr, still exit 0 from health)

## Top Workflows
1. **n8n daily mining stats** — collect hashrate + difficulty + block height → push to Obsidian note; alert if hashrate drops >20%
2. **Peer health alerting** — check peer count every 15min; exit 3 if < 8 peers → trigger n8n alert node
3. **Block finding detection** — poll getmininginfo at block intervals; exit 0 if found in window, exit 2 if none
4. **Historical hashrate trending** — sync to SQLite; query hashrate history for XEMD dashboard widget
5. **Claude/MCP mining queries** — agent queries live state via Homelab MCP without n8n shell nodes

## Table Stakes (from bitcoin-cli / dogecoin-node-monitor)
- getmininginfo: hashrate, difficulty, blocks
- getnetworkinfo: peer count, version, subversion
- getblockchaininfo: chain, height, bestblockhash, sync progress
- getwalletinfo: balance, unconfirmed, keypoolsize
- getpeerinfo: per-peer details, ban info
- getmempoolinfo: size, bytes
- estimatefee: fee estimates (older protocol only)
- getblockcount / getblockhash / getblock: block navigation

## Data Layer
- Primary entities: mining_snapshots, peer_snapshots, block_events, wallet_snapshots
- Sync cursor: block height (getblockcount) + timestamp
- FTS/search: not needed (time-series data, query by time window)
- Historical schema: timestamp, block_height, difficulty, hashrate_local, hashrate_network, peer_count, mempool_size, balance

## Product Thesis
- Name: dogecoin (binary: dogecoin-pp-cli)
- Why it should exist: bitcoin-cli is a generic raw JSON-RPC wrapper with no typed exit codes, no SQLite trending, no compact monitoring output, no n8n integration, and no MCP surface. This CLI provides typed exit codes for n8n branching, compact --json output for dashboard widgets, and historical SQLite trending that no existing Dogecoin tool offers.

## Build Priorities
1. **mining stats** — compound command (getmininginfo + getblockchaininfo + getnetworkhashps), exit 0, --compact/--json
2. **peers health** — peer count check, exit 0=healthy, exit 3=low (< MIN_PEERS)
3. **blocks found --since Nd** — block window check, exit 0=found, exit 2=none
4. **wallet balance** — getwalletinfo, --json
5. **sync** — poll all endpoints → SQLite snapshots
6. **hashrate history --since 30d** — SQLite query → trending data
7. **mempool status** — getmempoolinfo + estimatefee, fee tier display
8. **doctor** — version warning, auth check, node reachability; surface obsolete version
