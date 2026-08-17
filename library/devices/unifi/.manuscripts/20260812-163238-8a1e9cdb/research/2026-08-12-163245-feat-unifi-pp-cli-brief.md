# UniFi Network API CLI Brief

## API Identity
- Domain: Self-hosted UniFi OS gateway's local Network integration API (`https://<gateway>/proxy/network/api-docs/integration.json`, v10.5.67). NOT the cloud UniFi Site Manager API — this is the on-box REST API authenticated with a local API key (Settings → Control Plane → Integrations).
- Users: Homelab/prosumer network operators running a UDM/UCG/UDR-class gateway who want to script device audits, firewall/ACL review, client history, and network-change detection without opening the web UI.
- Data profile: Sites, devices (APs/switches/gateways), clients, networks/VLANs, firewall zones+policies, ACL rules, WiFi broadcasts, DNS policies, hotspot vouchers, VPN servers/tunnels, WAN config, switching (LAGs, MC-LAG, switch stacks), RADIUS profiles, DPI apps/categories, traffic-matching lists, device statistics.

## Reachability Risk
- None — confirmed live against the real gateway (10.0.0.1) with a valid local API key. HTTP 200, spec fetched successfully (239KB, 44 paths).
- Known transport gap: gateway uses a self-signed cert. Every existing community tool (unifi-cli, mcp-unifi) ships a manual `--insecure`/`STUB`-style opt-out; our generator has none at all today — must patch (see Build Priorities).

## Top Workflows
1. Audit device health/status across a site (devices list + statistics/latest) before manually opening the controller UI.
2. Review and edit firewall zones/policies and ACL rules from the terminal, including reordering.
3. Look up a client (by MAC or name) and see connection history/status.
4. Detect configuration drift or newly-joined devices/clients since last sync — nothing in the raw API surfaces this; it requires a local snapshot diff.
5. Provision/manage guest WiFi + hotspot vouchers without the mobile app.

## Table Stakes (from competing tools)
- `lucasilverentand/unifi-cli` (67 auto-gen commands, Bun/TS): per-resource CRUD, `--format table/jsonl/json`, `--fields` column filter, `raw GET/POST` escape hatch, `--dry-run`, `--insecure` flag, `schema <op>` introspection, bundled MCP server with resources+prompts.
- `pete-builds/mcp-unifi` (Python, 57 Network tools + Protect/Access): dry-run previews on every destructive tool, JSONL audit log with secret scrubbing, composite provisioning tools with rollback on partial failure, multi-controller/multi-site support via a `controller` param.
- `@owine/unifi-network-mcp`, `claytono/go-unifi-mcp`, `gordcurrie/unifi-mcp`: straightforward MCP wrappers over the Network API, no CLI-first design, no offline store.
- `node-unifi`/`unifi-client` (npm): legacy Controller-API wrappers (not the integration API this spec targets) — signal that the *old* protected/undocumented controller API still has a large user base but is a different auth/surface entirely; out of scope here.
- None of the above ship a local SQLite mirror, offline search, or history/diff-over-time — that's the gap our data layer fills.

## Data Layer
- Primary entities: sites, devices, clients, networks, firewall_zones, firewall_policies, acl_rules, wifi_broadcasts, dns_policies, hotspot_vouchers, vpn_servers, vpn_tunnels, wans, switching (lags, mc-lag-domains, switch-stacks), radius_profiles, dpi_applications, dpi_categories, traffic_matching_lists, device_statistics.
- Sync cursor: no API-side updated-at filtering; full resync per resource, timestamped locally so drift/newcomer detection has a prior snapshot to diff against.
- FTS/search: device/client name, model, MAC, IP — high-value for "find that one AP" lookups.

## Product Thesis
- Name: unifi-pp-cli
- Why it should exist: every existing UniFi Network API tool is either a thin MCP wrapper (no offline store, no history) or a from-scratch CRUD CLI with no drift/audit intelligence. A gateway is a device that silently changes state (DHCP leases churn, devices rejoin, firmware auto-updates change ports) — the killer feature is a CLI that remembers what the network looked like yesterday and can tell you what changed, which nothing else in this space does.

## Build Priorities
1. TLS-skip-verify support (config.go + client.go) — auto-skip verification for private/loopback/link-local hosts (RFC1918 10.0.0.1 gateway target), override via `UNIFI_INSECURE_SKIP_VERIFY`. No existing generator support; every community tool hand-rolls this because self-signed local-gateway certs are universal in this space. Document under `.printing-press-patches/`.
2. Full 44-endpoint absorbed surface (sites, devices, clients, networks, firewall zones/policies + ordering, ACL rules + ordering, WiFi broadcasts, DNS policies, hotspot vouchers, VPN servers/tunnels, WANs, switching, RADIUS, DPI, traffic-matching-lists, device statistics).
3. Transcendence: topology (device tree), client history, drift (config drift detection via local snapshot diff), newcomer (new device/client detection), port-audit, guest report, rule-predict (match traffic against firewall rules — avoid "firewall explain", collides with generator's own `sites firewall` naming check).
