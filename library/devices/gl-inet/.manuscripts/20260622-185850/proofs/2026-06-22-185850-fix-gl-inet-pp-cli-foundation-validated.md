# Foundation Validated Against Live Beryl AX (fw 4.8.1)

Both transports built and tested against the real GL-MT3000.

## GL JSON-RPC (internal/client/glrpc.go) — WORKS
- challenge → crypt(alg) → `<hash-method>_hex(user:cipher:nonce)` → login → sid; session cached per-host (4 min), re-login on auth error.
- Validated calls returned real data: `system.get_info`, `clients.get_list` (→ `{clients:[...]}`), `wifi.get_status` (→ `{res:[{name,channel,state}]}`), `repeater.get_config`.
- Module hyphenation confirmed: `wg-client`, `ovpn-client` (underscore forms 404).

## SSH + UCI (internal/glssh/glssh.go) — WORKS
- Go crypto/ssh password auth to dropbear (no sshpass). `Run`, `UCIExport`, `UCIShow`.
- `/etc/glversion` → 4.8.1; openwrt_release → 21.02-SNAPSHOT, target mediatek/mt7981, arch aarch64_cortex-a53; LuCI → git-20.074.84698.
- `uci export` → 76KB, 3674 lines, 62 packages (full config tree = snapshot backbone).
- Per-radio regdomain: `wireless.mt798111.country` (2.4G), `wireless.mt798112.country` (5G). Device currently 2.4G=IT, 5G=US.

## Validated response shapes
- system.get_info: board_info{model,openwrt_version,kernel_version,hostname,architecture}, firmware_version, model, country_code, hardware_feature{radio,wan,lan,...}, software_feature{vpn,adguard,tor,...}.
- clients.get_list: {clients:[{ip,mac,name,iface,blocked,last_rx[],last_tx[],...}]}.
- wifi.get_status: {res:[{name,channel,state}]}. netmode.get_mode: {mode}. tethering.get_status: {status,devices}. firewall.get_port_forward_list: {res:[]}. upgrade.check_firmware_local: {sha256,status}.

## Deps added
- github.com/GehirnInc/crypt (md5/sha256/sha512-crypt, pure Go)
- golang.org/x/crypto/ssh
