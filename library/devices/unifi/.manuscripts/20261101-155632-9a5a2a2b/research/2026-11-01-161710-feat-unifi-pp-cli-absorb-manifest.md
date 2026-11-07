# UniFi absorb manifest (reprint)

Source tools: sirkirby/unifi-mcp, enuno/unifi-mcp-server, pete-builds/mcp-unifi, jason-mspkickstart/mcp-unifi, lucasilverentand/unifi-cli, Elastic UniFi connector, prior unifi-pp-cli.

## Absorb Manifest

### Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Site Manager hosts, sites, devices, ISP metrics, SD-WAN (14 ops incl. connector) | official spec, enuno | (generated endpoint) site-manager resources | Mirror, history |
| 2 | Connector passthrough, raw escape hatch | official, jason raw_request | unifi-pp-cli raw | Console-allowlist-safe, confirm-gated writes |
| 3 | Network integration, 73 ops: devices, clients, networks, firewall zones and policies, ACL, WiFi, DNS, vouchers, VPN, WAN, switching, DPI, TMLs, tags, actions | official, enuno, pete-builds | (generated endpoint) sites resources | One command set over cloud or local |
| 4 | Protect, about 75 ops | official, enuno, sirkirby | (generated endpoint) protect resources | Same transports; WebSocket subscribe local-only until proven |
| 5 | Mobility, InnerSpace, Carrier Fabric | official, enuno | (generated endpoint) product resources | Typed commands |
| 6 | Controller reads: alarms, events, rogue APs, spectrum, DPI stats, reports, system log, traffic flows, neighbors, BGP/OSPF, settings | enuno source, UI bundles | (behavior in unifi-pp-cli raw) typed commands promoted for paths proven live | Only ships typed where proven |
| 7 | Controller writes: port forwards, legacy firewall, WLAN conf, port profiles, user groups, RADIUS, dynamic DNS, settings, backups, speedtest | enuno source, UI bundles | (behavior in unifi-pp-cli raw) typed commands promoted for paths proven live | Confirm-gated |
| 8 | Local gateway transport, self-signed TLS | prior patch | (behavior in unifi-pp-cli doctor) reports resolved transport | Auto-detected |
| 9 | Multi-console targeting | pete-builds, jason | (behavior in unifi-pp-cli sync) console flag scopes every command | Console-keyed mirror |
| 10 | Fleet health, inventory, firmware report | jason | unifi-pp-cli fleet | Partial-failure fan-out |
| 11 | Dry-run, confirm gate, read-only mode | enuno, pete-builds | (behavior in unifi-pp-cli raw) framework plus MCP execute | Preview by default on MCP |
| 12 | JSONL audit with secret scrubbing | pete-builds, enuno | unifi-pp-cli audit | Queryable |
| 13 | Compact MCP (read + confirm-gated execute), HTTP with bearer | Press, enuno | unifi-pp-mcp | Small tool surface |
| 14 | SQLite mirror, FTS | prior | unifi-pp-cli sync, unifi-pp-cli search | Cross-console |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Path and capability probe | reach | 10 | hand-code | One verdict per console across surfaces and both transports, including write status | Use this command to find out what works on a console right now, including whether writes are possible. Do NOT use it to assess a specific proposed change; use 'impact' instead. |
| 2 | Impact and lockout check | impact | 9 | hand-code | Local join across Network and Protect to the control path | Use this command to see what a proposed action would take down and whether it endangers the control path. Do NOT use it to evaluate one packet against firewall policy; use 'rule-predict' instead. Do NOT use it to make the change; use 'change apply' instead. |
| 3 | Journaled change, verify, undo, firewall trial (value requires writes) | change apply, change undo | 9 | hand-code | Before-image, read-back and journal; trial uses the gateway's ONE_TIME_ONLY schedule | Use this command to make a change with a recorded before-image, read-back verification and undo. Do NOT use it to preview consequences; use 'impact' instead. |
| 4 | What happened since I looked | digest | 8 | hand-code | Timeline across mirror snapshots, ISP metrics and journal | Use this command for a combined timeline across all categories. Do NOT use it for field-level config differences; use 'drift' instead. Do NOT use it for the full list of new clients; use 'newcomer' instead. |
| 5 | Config drift | drift | 9 | hand-code | Console-keyed snapshots with journal attribution | Use this command for field-level config changes between snapshots. Do NOT use it for a cross-category timeline; use 'digest' instead. |
| 6 | Newcomer detection | newcomer | 8 | hand-code | Local first-seen; API has no such field | Use this command to list clients and devices first seen in a window. Do NOT use it for config changes or a general timeline; use 'digest' instead. |
| 7 | Firewall rule prediction | rule-predict | 7 | hand-code | No simulate endpoint exists; low-confidence, returns indeterminate | Use this command to evaluate one hypothetical packet against the synced rules. Do NOT use it to assess what a device, port or WiFi action would affect; use 'impact' instead. |
| 8 | Device topology tree | topology | 7 | hand-code | Tree computed from parent links | Use this command to see the current device layout. Do NOT use it to work out what a change would affect; use 'impact' instead. |

### Dropped from prior print
- port-audit (6/10): no wattage in v11 Port overview, so PoE headroom cannot be computed.
- guest report (5/10): no recurring persona; vouchers and guest auth remain as endpoints.

### Out of scope
- UniFi Access and Talk: no official spec. Private cloudaccess and sso session surfaces: not replayable with a key.
