# UniFi CLI Brief (reprint, cloud-first)

## API Identity
- Domain: Ubiquiti's official API family, documented at developer.ui.com with machine-readable OpenAPI for every product (llms.txt indexes them).
- Users: UniFi owners and small MSPs who operate a console from anywhere, without a VPN or an open port, and want agents to do it safely.
- Homepage: https://unifi.ui.com (console UI), https://developer.ui.com (docs). Evidence: llms.txt, spec `servers`.
- Data profile (official operation counts, from the downloaded specs):
  - Site Manager v1.0.0, 14 ops at https://api.ui.com: hosts, sites, devices, isp-metrics, sd-wan-configs, plus the connector (`/v1/connector/consoles/{id}/*path`, 5 verbs).
  - Network v11.0.97, 73 ops, up from 44 paths in the prior print (v10.5.67). New: device adopt/remove, device tags, network references, more.
  - Protect v7.3.74, about 75 ops: cameras, PTZ, RTSPS, snapshot, talkback, lights, sensors, sirens, chimes, relays, speakers, bridges, alarm hubs, arm profiles, liveviews, viewers, fobs, users, files, POS, NVR, and two WebSocket subscribe endpoints.
  - Mobility v1.0.0, 8 ops. InnerSpace v1.3.23, 6 ops. Carrier Fabric v1.0.0, 11 ops.
- Not in official specs: the classic controller API (`/proxy/network/api/s/{site}/...`, `/proxy/network/v2/api/...`) that the console's own web UI uses. Community tools rely on it for health, port forwards, legacy firewall, DPI stats. UniFi Access has no developer.ui.com spec.

## Connector facts (from the official Site Manager spec)
- Path maps to the console's local `http://127.0.0.1/proxy/[path]`. Example: `GET /v1/connector/consoles/{id}/network/integration/v1/sites`, and `.../protect/integration/v1/meta/info`.
- The spec's own `path` parameter example contradicts this by showing `/proxy/network/integration/v1/sites`. Resolve live. A community tool had a 404 bug from this family of URL mistakes.
- Requires console firmware >= 5.0.3. Non-organization keys reach only the key owner's consoles; organization keys reach any console in the org.
- Limits: 100 requests/min per console, 25 second timeout per request (408), 10 MB max response.
- Errors use `{code, httpStatusCode, message, traceId}`; success passes upstream through unchanged. Non-connector Site Manager: 10,000 req/min on v1, 100 req/min on EA; 429 carries Retry-After.
- Auth: `X-API-KEY`. Key created at unifi.ui.com, Settings > API Keys, shown once.
- OPEN: Getting Started says "The API key is currently read-only" and write access needs a manual update when available, yet the connector documents POST, PUT, PATCH and DELETE. Whether a cloud key can write through the connector must be settled live, read-safely. Everything about "operating" a console remotely rests on this.

## Reachability Risk
- Low for reads: official and documented. Unverified until a key exists.
- Medium for cloud writes: see OPEN above.
- Local path still needs TLS handling for self-signed gateway certs and the /proxy/network prefix (prior patches).
- Connector is unusable on firmware below 5.0.3.
- Protect WebSocket subscribe endpoints probably cannot traverse a 25 second REST proxy; treat as local-only until proven.

## Top Workflows
1. Check fleet and console health from anywhere: hosts, devices, firmware, ISP metrics.
2. Change network config remotely with confidence: firewall, WiFi, VLANs, previewing first and diffing after.
3. Answer "what changed and what joined" across history: drift, newcomers.
4. Act on a device or client: restart, power-cycle port, block client, guest authorize, vouchers.
5. Protect from anywhere: list cameras, pull a snapshot, PTZ, arm or disarm.
6. Drive all of it from an agent on any device via remote MCP, with no VPN.

## Table Stakes
- sirkirby/unifi-mcp (979 stars, Python, active): Network, Protect, Access MCP servers plus a Graph API.
- enuno/unifi-mcp-server (333 stars, Python, active): 343 local tools, 71 cloud; Site Manager, connector, Mobility, InnerSpace, Carrier Fabric. Every mutation needs confirm=True with dry_run; read-only and dry-run server modes; risk-tier roles; hash-chained audit log; HTTP transport with bearer token.
- pete-builds/mcp-unifi (29 stars, Python, active): 57 Network, 11 Protect, 18 Access tools; dry-run previews, JSONL audit with secret scrubbing, rollback composites, multi-controller.
- jason-mspkickstart/mcp-unifi (1 star, TypeScript): runs on Cloudflare Workers through the connector; fleet_health, firmware_report, diff_config; allowlisted consoles, MAX_BATCH, partial-failure fan-out, size-guarded responses, comma-separated MCP_TOKEN rotation. Closest to the requested shape, but apply_config refuses to write.
- lucasilverentand/unifi-cli: 67 commands, raw escape hatch, schema introspection.
- Elastic Kibana connector: 20 actions, confirming the connector as a production path.
- Prior unifi-pp-cli: local-only, 73 MCP tools, topology, drift, newcomer, port-audit, guest report, rule-predict.
- Nobody combines a real CLI, a local mirror with history, cloud plus local transports, and a compact remote MCP.

## Data Layer
- Primary entities: hosts, cloud sites, cloud devices, isp_metrics, sd_wan_configs, plus every prior Network entity scoped by console, plus Protect cameras, sensors, NVRs and arm profiles.
- Critical modeling change: the mirror key must include console id. The prior print assumed one gateway; a cloud fleet makes siteId alone ambiguous.
- Sync cursor: full resync per resource with local timestamps. Budget against 100 req/min per console.
- FTS: device and client names, models, MACs, IPs, across consoles.

## User Vision
- "let's make a printing press cli for the unifi.ui.com to operate my console? ideally we make a remote MCP out of it using the APIs."
- "everything!" Read as: full coverage of what the APIs and console allow.
- Logged in to unifi.ui.com in Chrome, available for read-only capture.
- API key to be provided before live testing.

## Product Thesis
- Name: unifi-pp-cli
- Why: operate a whole UniFi estate from anywhere with one binary. Cloud connector and local gateway are interchangeable transports for the same commands; the mirror remembers what the network looked like yesterday; the remote MCP stays compact and will not change your network without a confirmed preview.

## Build Priorities
1. Dual transport: same Network and Protect commands via `--console` over the connector or direct to a local gateway. Keep prior TLS and base-URL behavior.
2. Full official surface across six products as a single spec, with a compact MCP surface (read plus confirm-gated execute).
3. Console-scoped mirror, carrying forward drift, newcomer, topology, port-audit, guest report, rule-predict.
4. Connector-aware limits: per-console 100/min throttle, 25 second timeout, 10 MB guard, no replay of ambiguous writes.
5. Remote MCP auth and deployment story.
6. Classic controller coverage only if sniffing shows it reaches what the integration API cannot.

## Sources
- https://developer.ui.com/llms.txt and each product's openapi.json (saved under research/docs)
- https://developer.ui.com/site-manager/v1.0.0/gettingstarted
- https://www.elastic.co/docs/reference/kibana/connectors-kibana/unifi-action-type
- https://glama.ai/mcp/servers/jason-mspkickstart/mcp-unifi
- https://github.com/enuno/unifi-mcp-server, https://github.com/sirkirby/unifi-mcp, https://github.com/pete-builds/mcp-unifi
