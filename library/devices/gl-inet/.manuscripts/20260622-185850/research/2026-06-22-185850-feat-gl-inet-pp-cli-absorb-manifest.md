# GL.iNet CLI — Absorb Manifest

Sources mined: **python-glinet** (dynamic full-RPC library — the one to beat), **gli4py** (clean known-good fw-4 endpoint map), **spusuf/glinet_api-hass** (richest entity/toggle inventory), **GL-iNet_utils** (SSH/UCI audit + backup ideas), plus live validation against the GL-MT3000 (fw 4.8.1). No maintained fw-4 CLI and no MCP server exist anywhere — wide-open space.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|------------|--------------------|-------------|--------|
| 1 | Challenge/login + persisted session | python-glinet/gli4py | challenge→md5crypt→**sha256(hash-method)**→sid; keepalive; auto re-auth | version-aware hash, persisted, no IPython | |
| 2 | System info (model/fw/openwrt/caps) | gli4py | `system.get_info` → typed + capability maps | feeds doctor + feature gating | |
| 3 | System status (cpu/mem/clients/services) | spusuf | `system.get_status` | --json/--select, SQLite | |
| 4 | Reboot | gli4py | `system.reboot` | --dry-run, typed exit | |
| 5 | Timezone get/set | python-glinet | `system.get/set_timezone_config` | | |
| 6 | Security policy / https redirect | docs | `system.get/set_security_policy` | | |
| 7 | Clients list (ip/mac/name/tx/rx/online) | all | `clients.get_list` → SQLite + FTS | offline search, history | |
| 8 | Client status totals | spusuf | `clients.get_status` | | |
| 9 | Block client | python-glinet | `clients.block_client` | --dry-run | |
| 10 | Rename client | python-glinet | `clients.set_info` | | |
| 11 | DHCP leases | gli4py/luci-rpc | leases via rpc/uci | | |
| 12 | ARP table | python-glinet | network arp | | |
| 13 | WiFi status (radios/channels) | gli4py | `wifi.get_status` | | |
| 14 | WiFi get/set config (ssid/chan/key/enable) | gli4py | `wifi.get/set_config` per radio | --dry-run | |
| 15 | WiFi txpower | docs | `wifi.set_txpower` | | |
| 16 | iwinfo scan | GL-iNet_utils | SSH `iwinfo scan` | feeds region diagnose | |
| 17 | Repeater scan | all | `repeater.scan` | | |
| 18 | Repeater connect (join SSID) | python-glinet | `repeater.connect` | | |
| 19 | Repeater disconnect/forget/status/config | python-glinet | `repeater.*` | | |
| 20 | Saved-AP list / remove | python-glinet | `repeater.get_saved_ap_list` | | |
| 21 | Netmode get/set | spusuf | `netmode.get/set_mode` | | |
| 22 | WireGuard client list | python-glinet | `wg-client.get_all_config_list` (hyphenated, confirmed) | SQLite | |
| 23 | WireGuard client start/stop/status | gli4py | `wg-client.start/stop/get_status` | --dry-run, typed exit | |
| 24 | OpenVPN client list/start/stop/status | python-glinet | `ovpn-client.*` (hyphenated, confirmed) | | |
| 25 | WireGuard server status/peers | python-glinet | `wg-server.*` | | (cap-gated) |
| 26 | OpenVPN server | python-glinet | `ovpn-server.*` | | (cap-gated) |
| 27 | VPN policy / killswitch | spusuf | vpn-policy (method drifted on 4.8.1) | runtime-probed | (stub — method name not found on 4.8.1; probe+wire at runtime, honest message if absent) |
| 28 | Tailscale get/set/status | spusuf | `tailscale.*` | | (cap-gated) |
| 29 | Firewall zones/rules | docs | `firewall.*` | | |
| 30 | Port-forward list/add/remove | spusuf | `firewall.get_port_forward_list` (confirmed) | --dry-run | |
| 31 | DMZ | docs | `firewall.get/set_dmz` | | |
| 32 | MAC clone get/set | python-glinet | macclone (method drifted on 4.8.1) | runtime-probed | (stub — method name not found on 4.8.1; resolve via runtime probe / uci) |
| 33 | LAN/DHCP config + static binds | python-glinet | `lan.*` / uci | | |
| 34 | DDNS get/set/status | spusuf | `ddns.*` | | |
| 35 | DNS / custom DNS | docs | `dns.*` | | |
| 36 | IPv6 | spusuf | `ipv6.get/set` | | |
| 37 | Tethering status/connect/disconnect | python-glinet | `tethering.*` (confirmed) | | |
| 38 | Modem (cellular) status/connect/SMS | python-glinet | `modem.*` | | (cap-gated; Beryl AX = USB only) |
| 39 | Firmware check + upgrade (keep_config) | docs | `upgrade.check_firmware_local/online`, `upgrade_local` (confirmed) | --dry-run | |
| 40 | opkg / plugins | GL-iNet_utils | `plugins.*` / SSH opkg | | |
| 41 | Diag ping/traceroute | gli4py | `diag.ping/traceroute` | | |
| 42 | Logread / export logs | spusuf | `logread` / SSH | | |
| 43 | AdGuardHome toggle/config | spusuf | `adguardhome.*` | | (cap-gated) |
| 44 | UCI get/set/show/export/commit/revert | GL-iNet_utils | SSH `uci` | the config-engine backbone | |
| 45 | Service/network reload | OpenWrt | SSH `service`/`/etc/init.d` reload | applied after commit | |

## Transcendence (only possible with our approach) — from the novel-features subagent
| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Named config profile capture | `snapshot save <name>` | 10 | SSH `uci export` + GL module configs → `config_snapshots` w/ `{model,fw,openwrt,luci,taken_at}` provenance; GL UI has no named/versioned profiles |
| 2 | Version+model-gated restore | `snapshot apply <name>` / `revert` | 10 | Replays UCI; checks provenance — warns on fw/luci drift, refuses on model mismatch w/o --force (mirrors sysupgrade lock) |
| 3 | Option-level config diff | `snapshot diff <a> [<b>]` | 10 | Parses UCI change-tuples into `snapshot_changes`; pure local SQLite diff at option granularity |
| 4 | Current-config summary | `config summary` | 9 | Live `uci show` + GL status modules → structured per-subsystem report |
| 5 | WiFi region diagnose + fix | `wifi region diagnose` / `set <CC>` | 10 | iwinfo scan vs static country→allowed-channels table; names permitting country; sets regdomain+commit+reload (the Italy fix) |
| 6 | Venue onboarding macro | `venue connect <ssid>` | 10 | Orchestrates scan→region-check→join→captive-portal prep (macclone, DNS drop)→restore |
| 7 | VPN toggle + egress verify | `vpn toggle <tunnel>` | 10 | Start + killswitch + confirm public egress IP changed; typed exit on leak |
| 8 | Doctor / compat probe | `doctor` | 10 | Detects model/fw/openwrt/luci, reachable surfaces, per-feature availability (version-awareness mandate) |
| 9 | Raw passthrough escape hatch | `rpc call` / `ubus call` / `uci get\|set` | 9 | Thin authed transport over GL `/rpc` + SSH `uci`; reach any of 43 modules / any UCI option |
| 10 | Config option search | `config find <term>` | 7 | FTS over snapshot sections/options — find an option anywhere across the tree + 43 modules |
| 11 | WAN source switch | `wan mode <ethernet\|repeater\|tethering>` | 6 | Set WAN source + verify reconnect + internet state |
| 12 | Connectivity troubleshooter | `troubleshoot` (`--fix`) | 10 | Decision tree across netmode + WAN/internet + repeater source + cable + tethering + DNS + killswitch + region; names the most likely cause and the exact fix command; `--fix` auto-applies safe remedies. User's killer idea ("router on but no source network — usually not connected in repeater mode") |
| 13 | Source-uplink quality + improvements | `uplink` | 9 | Joins `iwinfo info`/`scan`/`assoclist` (SSH) + `repeater.get_status`: RSSI, band, link rate/PHY mode, channel congestion, latency (ping) → ranked improvement suggestions (reposition, switch to 5GHz band, venue AP is WiFi-4-capped, try cellular). Local metrics only — no external speedtest (that branch cut as unverifiable). User idea ("source network old/weak/slow + ideas to improve") |

## Stub disclosure (per Phase 1.5 rule)
- **#27 vpn-policy / killswitch** and **#32 macclone**: the older method names return "Method not found" on fw 4.8.1. They ship as runtime-probed: the CLI tries known method names, and if absent on the detected firmware, it reports honestly (and macclone falls back to the UCI path). Not silently broken — version-aware by design.
- **Capability-gated (#25,26,28,38,43):** wg-server/ovpn-server/tailscale/modem/adguardhome render only when `get_info` capability maps say the feature exists on this model/firmware; otherwise the command reports "not available on this model/firmware."
