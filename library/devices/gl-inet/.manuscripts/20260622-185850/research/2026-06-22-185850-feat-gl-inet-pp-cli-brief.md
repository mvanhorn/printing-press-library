# GL.iNet Router (Local API + OpenWrt/LuCI) CLI Brief

## API Identity
- **Domain:** On-device LAN control of GL.iNet travel routers running GL firmware 4.x (GL-MT3000 "Beryl", GL-AXT1800 "Slate AX", GL-MT2500 "Brume 2", etc.). These are **OpenWrt-based** (21.02 lineage) with GL's own UI on top.
- **Two surfaces (confirmed against the live Beryl AX), on the router LAN IP `192.168.8.1`):**
  1. **GL JSON-RPC** — `POST /rpc` (HTTP+HTTPS confirmed working), JSON-RPC 2.0 `call` envelope. Friendly travel/status/control. Auth: GL `challenge`+`login` → `sid` (params[0]). **challenge returns `hash-method: sha256` on this firmware — confirm final-hash digest (sha256 vs md5) before coding.**
  2. **SSH + `uci`** — port 22 confirmed open; Windows OpenSSH client present. The config-snapshot engine backbone: `uci export`/`show`/`import`/`set`/`commit`, `sysupgrade -b` tarballs, version/model detection, regdomain changes, power-user passthrough. **This replaces the ubus-HTTP plan: `/ubus` is NOT exposed — nginx 302-redirects POSTs to `/ubus` back to root on this firmware.**
  - LuCI web (`/cgi-bin/luci/`, present, 403/needs auth): not used — SSH supersedes it for tarball backup and config.
- **Users:** Privacy-minded travelers / power users running a personal travel router between hotel/cafe/cellular networks. Want scriptable, cloud-free control; distrust GoodCloud.
- **Data profile:** Live device state (clients, WAN/internet, WiFi radios, VPN, modem) + configuration spread across ~43 GL RPC modules AND the OpenWrt UCI tree (`/etc/config/*`).

## VERSION- & MODEL-AWARENESS (core principle — user-mandated, applied at every step)
The CLI MUST detect and adapt to model + versions before acting, and never assume:
- **Test device:** GL-MT3000 "Beryl AX" (dual-band WiFi 6 AX3000). **Cross-model differences are real** — other GL models differ in radios (single-band, 6GHz, cellular), available RPC modules, and firmware. Detect model and gate model-specific features; never hardcode for one model.
- **Detect first, every session:** model + GL firmware version (`system.get_info`), OpenWrt release (`/etc/openwrt_release` / ubus `system.board`), LuCI presence/version (`opkg list-installed | grep luci`), reachable surfaces (GL `/rpc` confirmed; `/ubus` NOT exposed by nginx on this firmware — use SSH for uci), and SSH availability.
- **Capability gating:** resolve module-name hyphenation (`wg-client` vs `wg_client`), available modules, LuCI backup path (`/admin/system/flash/*` vs `/flashops/*`), and `/ubus` availability *from the live device*, not hardcoded constants. Probe + cache per device.
- **Snapshot provenance:** every config snapshot is stamped with `{model, gl_firmware, openwrt_release, luci_version, taken_at}`. Restore checks the stamp and **warns on firmware/LuCI drift, refuses (without `--force`) on model mismatch** — mirroring sysupgrade's same-model+firmware restriction.
- **Graceful degradation:** if `/ubus` is absent (LuCI not installed), fall back to the GL RPC per-module `get_config`/`set_config` snapshot path and say so. If a module is missing on this firmware, report it, don't crash.
- **`doctor` / `compat` command** surfaces all detected versions, reachable surfaces, ACLs, and per-feature availability so the user always knows what this firmware supports.

## Reachability Risk
- **Low** — it's the user's own LAN device; no bot protection, not internet-reachable. Reachable when router is on the LAN and IP + admin password are known. **Live testing requires the user's router IP + admin password.**
- **Firmware drift is the real risk**, mitigated by the version-awareness principle above. 4.x only (3.x is a different REST API — out of scope, detect and refuse).

## Auth (ground truth)
**GL `/rpc` (session_handshake) — CONFIRMED on fw 4.8.1:** `challenge {username:"root"}` → `{alg,salt,"hash-method",nonce}` (nonce ~1000ms TTL). `cipher = crypt(password,"$<alg>$<salt>$")` (alg 1=md5,5=sha256/5000,6=sha512/5000). **`hash = <hash-method>_hex("root:"+cipher+":"+nonce)` where hash-method is sha256 on 4.8.1 (md5 on older firmware) — read it from the challenge, do NOT hardcode.** `login {username:"root",hash}` → `{sid}`. Calls: `{"method":"call","params":[sid,"<module>","<fn>",{args?}]}`. sid in params[0]. Keepalive ~30s, re-login on auth error.

**ubus `/ubus`:** null session = 32 zeros. `session.login {username:"root",password,timeout?}` → `result:[0,{ubus_rpc_session, timeout(300s sliding), acls,...}]`. **Result is `[status_int, payload]`; status 0=OK, 6=perm-denied (bad password returns `[6]`).** Token in params[0]. Sliding 300s timeout; any call renews; `session.destroy` to logout.

**LuCI:** form login → `sysauth_http` cookie + CSRF `token`. Used only for the tarball backup/restore endpoints.

Both `/rpc` and `/ubus` typically share the same `root`/admin password but issue distinct tokens. **All auth + JSON-RPC envelopes are hand-built in Phase 3** — not stock generator auth/REST shapes.

## Top Workflows
1. **Travel config profiles** — capture a known-good "home/standard" config as a named snapshot; on the road adjust for a venue; revert cleanly. (User's #1 ask. No tool does this.)
2. **Venue onboarding** — repeater/WISP scan + join a hotel/cafe SSID; captive-portal macro (MAC clone, drop VPN/AdGuard DNS, restore after).
   - **WiFi region/regdomain diagnosis + instant fix (headline travel feature):** abroad, repeater can't see a venue's network because it's on a channel the current regulatory domain forbids (e.g. US regdomain hides 2.4GHz ch12/13 used in the EU). Diagnose (scan all APs, compare each AP's channel to the channels allowed by the current country; if the target is on a disallowed channel, name the country that would permit it) and fix in one command (`wifi region set IT` → `uci set wireless.radioX.country=IT; commit; wifi reload`). Region is model/radio-specific → uses model+capability detection.
3. **VPN control** — start/stop WireGuard/OpenVPN client, toggle kill-switch, verify egress IP changed.
4. **Status at a glance** — clients online, WAN/internet state, WiFi radios, modem/tethering, firmware-update-available.
5. **Deep config / power-user** — direct `uci get/set`, raw `ubus call` / GL `rpc call` passthrough, opkg packages, WAN-mode switching.

## Table Stakes (absorbed across all sources)
- python-glinet (dynamic full-RPC library — the one to beat), gli4py (clean known-good 4.x endpoint map), spusuf/glinet_api-hass (richest entity/toggle inventory), GL-iNet_utils (SSH/UCI audit + backup ideas).
- Full challenge-response login + persisted session + keepalive + auto re-auth (GL **and** ubus).
- system info/status/load, reboot, timezone, security policy.
- clients list/status/block/rename, DHCP leases (luci-rpc `getDHCPLeases`), ARP.
- wifi get/set per-radio (enable/SSID/channel/key), txpower; iwinfo scan/assoclist.
- repeater scan/connect/disconnect/saved-APs; netmode get/set.
- wg-client + ovpn-client list/start/stop/status; wg-server, ovpn-server; vpn-policy killswitch/domain/mac/vlan.
- tailscale/zerotier/tor where firmware exposes them.
- firewall zones/rules/port-forward/DMZ; macclone get/set.
- tethering + modem (cellular) status/connect; SMS; upgrade check/local/online (keep_config).
- ddns, dns/custom_dns, lan/DHCP + static binds, ipv6; plugins (opkg); diag ping/traceroute; logread; AdGuardHome.
- **UCI engine (ubus):** configs/get/set/add/delete/changes/commit/revert/apply(rollback); service/network reload.

## Data Layer (SQLite)
- **Primary entities:** `devices` (model+firmware+luci provenance), `clients` (ip/mac/name/tx/rx/online), `wifi_radios`, `vpn_tunnels` (wg/ovpn config+status), `saved_aps` (SSID), `firewall_rules`/`port_forwards`, `modem_status`, `status_snapshots` (system.get_status over time), **`config_snapshots`** (named profile = full UCI tree JSON + GL module configs + provenance stamp), **`snapshot_changes`** (parsed UCI change-tuples for diff).
- **Sync cursor:** on-demand poll (`system.get_status` + `clients.get_list` + `uci get` per config). No server-side change feed.
- **FTS/search:** clients (name/mac/ip), saved APs (SSID), snapshots (name/tag/provenance), and config sections/options (search for an option across the whole tree).

## Source Priority
- Single source (router local API + its OpenWrt/LuCI underbelly — same device). No combo CLI.

## User Vision (verbatim intent + additions)
Travel router. As the user travels they adjust settings for new places/networks, then want to revert to normal settings. Wants: **save normal config, restore standard config, report current-config summary** — plus diff "what did this venue change vs my standard," multiple named profiles, one-command revert. Explicitly asked to **include the LuCI/OpenWrt layer** (more complexity, novel-feature opportunities) and to **be mindful of GL firmware AND LuCI version at every step**. Device is a **Beryl AX (GL-MT3000)**; cross-model differences must be handled. Added use case: **abroad (e.g. Italy), repeater mode fails to find the venue WiFi because the channel isn't allowed under the US regulatory domain; the fix is changing the region, which is multi-step — diagnose and fix it instantly via the CLI.**

## Product Thesis
- **Name (working):** `gl-inet` CLI.
- **Why it should exist:** GL.iNet's UI has no named/versioned config profiles, and its only backup is an SSH/LuCI tarball restore-locked to the same model+firmware. There is no maintained firmware-4 CLI and no MCP server anywhere in this ecosystem. A local-first CLI that snapshots config through the UCI engine, diffs it at option granularity, reverts in one command, runs a captive-portal/VPN travel macro, and is rigorously version-aware — with offline search, a local SQLite store, agent-native output, and an MCP server — owns a wide-open space and solves the exact travel pain.

## Build Priorities
1. **Foundation:** version/capability probe; GL `/rpc` `call` transport + challenge auth; ubus `/ubus` `session.login` transport; persisted sessions + keepalive; SQLite store.
2. **Config snapshot engine (transcendence #1):** `snapshot save/list/show/apply/revert/diff/summary` over the UCI tree (ubus path; GL-module fallback), provenance-stamped, version-checked restore.
3. **Absorb (status + control):** clients, wifi, repeater/netmode, wg/ovpn start/stop/status, vpn-policy, firewall, macclone, tethering/modem, system, upgrade.
4. **Travel macros (transcendence):** `wifi region` diagnose+fix (the Italy/regdomain repeater fix), `venue connect` (scan→region-check→join→captive-portal macro), `vpn toggle` (start+killswitch+egress verify), `wan mode` switch.
5. **Power-user passthrough (transcendence, LuCI-enabled):** `uci get/set`, raw `ubus call` + GL `rpc call`, opkg, `doctor`/`compat`.
6. **Agent-native + MCP:** `--json/--select/--compact`, typed exit codes, SQL/search/sync, MCP server.
