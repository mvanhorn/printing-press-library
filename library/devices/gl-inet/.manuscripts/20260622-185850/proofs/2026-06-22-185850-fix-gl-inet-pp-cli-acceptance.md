# Acceptance Report: gl-inet (GL.iNet Beryl AX)

- **Level:** Quick Check (read-only + dry-run), device-safe by design.
- **Why not full live dogfood:** mutating commands (wifi region set, wan mode, venue connect, vpn toggle, snapshot apply) change a real, in-use travel router. Per the skill's "unapproved real-world side effects" rule, those were validated by dry-run only; reads were validated live against the actual GL-MT3000 (fw 4.8.1).
- **Tests:** 16/16 passed.

## Live read-only validation (against the real router)
- `doctor --json` → reachable; model mt3000, fw 4.8.1, OpenWrt 21.02, LuCI git-20.074, country US, RPC reachable, SSH connected, capability map. PASS
- `clients --json` → real client list (.clients extracted). PASS
- `system info --json --select board_info.model,firmware_version,country_code` → --select works. PASS
- `wifi --json` → radios mt798112 (ch48) / mt798111. PASS
- `snapshot save home` → 76468 bytes captured + provenance (model/fw/openwrt/luci/country). PASS
- `snapshot list` / `snapshot show home` → metadata + notes. PASS
- `snapshot diff home` → vs live = 0 changes (correct). PASS
- `wifi region show` → per-radio mt798111=IT, mt798112=US. PASS
- `wifi region diagnose` → **found live neighbor AP "Tenda_E40808" on ch13, flagged disallowed under US, suggested IT** (the headline Italy use case, proven live). PASS
- `vpn` (list) → real tunnels AzireVPN, Mullvad. PASS
- `rpc call netmode get_mode` → {mode:router}. PASS
- `rpc uci get wireless.mt798111.country` → IT. PASS
- `config find country` → live + snapshot matches. PASS
- `config summary` → per-subsystem digest (40 clients, network/wifi/vpn). PASS
- `troubleshoot` → decision tree + SSH egress check (35ms), concluded connectivity OK. PASS
- `uplink` → correctly reports not-in-repeater (router mode). PASS

## Mutation commands (dry-run validated, not executed against live config)
- `snapshot apply home --dry-run` → in_sync, 0 changes.
- `wifi region set US --dry-run` → uci set + commit + wifi reload preview.
- `vpn toggle Mullvad --dry-run` → resolves tunnel, shows wg-client.start {group_id:2797}.
- `wan mode repeater --dry-run` → netmode.set_mode {mode:repeater}.
- `venue connect "Tenda_E40808" --dry-run` → scan→connect→verify sequence.

## Gate: PASS
Headline features (snapshot engine, region diagnose/fix, doctor, troubleshoot) all live-validated. Bug found+fixed during validation: vpn toggle dry-run short-circuited its own tunnel-resolution read (fixed so reads run for real, only the mutation is previewed).
