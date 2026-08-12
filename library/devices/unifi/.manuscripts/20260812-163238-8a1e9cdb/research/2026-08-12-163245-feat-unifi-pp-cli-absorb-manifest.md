# UniFi CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List/get sites | unifi-cli sites list | (generated endpoint) sites list/get | Offline mirror, --select |
| 2 | List/get devices, adopt/unadopt actions | unifi-cli devices; mcp-unifi | (generated endpoint) devices list/get/actions | Offline mirror, drift-aware |
| 3 | Device statistics | unifi-cli devices stats | (generated endpoint) devices statistics latest | Local history via sync snapshots |
| 4 | Pending (unadopted) devices | integration API /v1/pending-devices | (generated endpoint) pending-devices list | n/a |
| 5 | Port actions (PoE cycle etc.) | mcp-unifi switch port tools | (generated endpoint) devices interfaces ports actions | n/a |
| 6 | List/get clients | unifi-cli clients | (generated endpoint) clients list/get | FTS search by name/MAC |
| 7 | Client actions (block/reconnect) | mcp-unifi | (generated endpoint) clients actions | dry-run safe by default |
| 8 | Networks CRUD + references | unifi-cli networks | (generated endpoint) networks list/get/create/update/delete/references | n/a |
| 9 | Firewall zones CRUD | unifi-cli firewall zones | (generated endpoint) firewall zones list/get | n/a |
| 10 | Firewall policies CRUD + ordering | unifi-cli firewall policies-ordering | (generated endpoint) firewall policies list/get/ordering | rule-predict transcends this |
| 11 | ACL rules CRUD + ordering | integration API | (generated endpoint) acl-rules list/get/ordering | n/a |
| 12 | Device tags | integration API | (generated endpoint) device-tags list | n/a |
| 13 | WiFi broadcasts CRUD | unifi-cli wifi | (generated endpoint) wifi broadcasts list/get/create/update/delete | n/a |
| 14 | DNS policies CRUD | unifi-cli dns | (generated endpoint) dns policies list/get/create/update/delete | n/a |
| 15 | Hotspot vouchers CRUD | integration API | (generated endpoint) hotspot vouchers list/get/create/delete | guest report transcends this |
| 16 | RADIUS profiles | integration API | (generated endpoint) radius profiles list | n/a |
| 17 | Switching: LAGs, MC-LAG domains, switch stacks | integration API | (generated endpoint) switching lags/mc-lag-domains/switch-stacks | port-audit transcends this |
| 18 | Traffic-matching lists | integration API | (generated endpoint) traffic-matching-lists list/get | rule-predict consumes this |
| 19 | VPN servers, site-to-site tunnels | unifi-cli vpn tunnels-list/servers-list | (generated endpoint) vpn servers/site-to-site-tunnels | n/a |
| 20 | WANs | integration API | (generated endpoint) wans list | n/a |
| 21 | DPI applications/categories | integration API | (generated endpoint) dpi applications/categories | n/a |
| 22 | Countries reference list | integration API | (generated endpoint) countries list | n/a |
| 23 | Info/version endpoint | integration API | (generated endpoint) info | Used by doctor |
| 24 | Raw request escape hatch | unifi-cli raw GET/POST | <api>-pp-cli raw (behavior in generated client) | (generator-standard; verify present) |
| 25 | JSON/table/select output | unifi-cli --format, --fields | <api>-pp-cli --json/--select/--csv | Framework-standard, no extra work |
| 26 | Dry-run on mutating commands | unifi-cli --dry-run; mcp-unifi dry_run | <api>-pp-cli <mutating cmd> --dry-run | Framework-standard |
| 27 | TLS-skip-verify for self-signed local gateway | unifi-cli --insecure; mcp-unifi (implicit) | (behavior in unifi-pp-cli auto TLS handling) auto-skip for private/loopback/link-local + UNIFI_INSECURE_SKIP_VERIFY override | No manual flag needed for the common case — generator gap, patched this run |
| 28 | Local SQLite mirror + full-text search | none of the competitors | (generated endpoint) sync/search | Sole differentiator among UniFi tools |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Device topology tree | topology | hand-code | Requires joining devices + uplink/port data locally into a tree; no single endpoint returns hierarchy | Use this to see the physical device tree (gateway → switches → APs) built from local mirror data. Do NOT use for live per-device stats; use 'devices stats' instead. |
| 2 | Client connection history | client history | hand-code | Requires diffing successive local snapshots of the clients table over time; the API only exposes current state | Use for a specific client's connection history over synced snapshots. Returns not-found (exit 3) for a MAC never seen locally — that is correct behavior, not a bug. |
| 3 | Config drift detection | drift | hand-code | Requires comparing two local snapshots (networks/firewall/wifi/dns) taken at different sync times; API has no versioning/audit trail | Use to see what changed in site config since the last sync snapshot. Returns [] when nothing changed — that is a valid, non-error result. |
| 4 | Newcomer / new-device detection | newcomer | hand-code | Requires local snapshot diff of the devices+clients tables; API has no "since" filter | Use to list devices/clients first seen since a given sync. Do NOT use for live network scanning; this is local-mirror-only. |
| 5 | Port audit across switch stacks | port-audit | hand-code | Requires joining device port config with the local topology and PoE/link data across all switches in the site; no endpoint aggregates this | Use to review port utilization/PoE draw across every switch on a site in one table. |
| 6 | Guest network report | guest report | hand-code | Requires joining hotspot vouchers + guest WiFi broadcasts + guest client sessions from local store; no combined endpoint exists | Use to summarize guest network usage (active vouchers, connected guest clients) from local data. |
| 7 | Firewall rule prediction | rule-predict | hand-code | Requires locally replaying firewall zone/policy/ACL/traffic-matching-list evaluation order against a hypothetical packet; the API has no "trace" or "simulate" endpoint | Use to predict which firewall policy would match a given source/dest/port before making a live change. Do NOT use this to guarantee live gateway behavior — it is a local simulation of the synced ruleset. |
