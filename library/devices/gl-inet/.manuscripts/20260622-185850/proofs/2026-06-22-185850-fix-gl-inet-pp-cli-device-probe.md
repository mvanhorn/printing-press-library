# Live Device Probe — Confirmed Facts (GL-MT3000 Beryl AX)

Probed directly via curl from the user's LAN machine to `192.168.8.1`. Device identifiers (MAC, serial) deliberately omitted — not recorded in any artifact.

## Reachable surfaces
- GL JSON-RPC `POST /rpc` — **working** over both HTTP and HTTPS (self-signed cert).
- ubus `/ubus` — **NOT exposed**: nginx 302-redirects POSTs to `/ubus` → root. OpenWrt ubus-over-HTTP is not proxied on this firmware.
- LuCI `/cgi-bin/luci/` — present (403, needs auth). Not used.
- SSH port 22 — **open**; Windows OpenSSH client present. → config-engine transport.
- HTTPS 443 — open.

## Auth algorithm (CONFIRMED, firmware 4.8.1)
1. `challenge {username:"root"}` → `{salt, "hash-method":"sha256", alg:1, nonce}`.
2. `cipher = openssl passwd -1 -salt <salt> <password>` (alg 1 = md5-crypt; full `$1$salt$hash` string).
3. `loginHash = sha256_hex("root:" + cipher + ":" + nonce)`  ← **hash-method driven; sha256 on this firmware, md5 on older.**
4. `login {username:"root", hash:loginHash}` → `{sid}`. sid in `params[0]` of every `call`.

**Generation rule:** read `hash-method` from the challenge and select the final digest (sha256|md5); read `alg` and select the crypt variant (1/5/6). Never hardcode.

## Device facts (from system.get_info)
- model: `mt3000` / `GL.iNet GL-MT3000` (Beryl AX)
- firmware_version: **4.8.1**, firmware_type: release8, firmware_date: 2025-08-19
- openwrt_version: **OpenWrt 21.02-SNAPSHOT**, kernel 5.4.211
- architecture: ARMv8, cpu_num: 2
- **country_code: US** (current WiFi regulatory domain)
- hardware_feature: radio `mt798111 mt798112` (dual), wan eth0, lan eth1, usb, fan; no bluetooth/screen/gps/built-in-modem
- software_feature: ipv6 ✓, adguard ✓, vpn ✓, tor ✓, nas ✓, sms_forward ✓; mlo ✗, ids_ips ✗, secondwan ✗, cellular_upgrade ✗, repeater_eap ✗

## Implications
- `hardware_feature`/`software_feature` maps = the capability-detection surface for `doctor` + feature gating.
- `country_code` = live regdomain for the region-diagnosis feature.
- Config-snapshot engine uses SSH + `uci` (ubus-HTTP unavailable). Confirm SSH/uci during build/test.
