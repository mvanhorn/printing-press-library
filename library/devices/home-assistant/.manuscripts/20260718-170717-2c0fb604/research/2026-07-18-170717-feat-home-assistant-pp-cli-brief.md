# Home Assistant CLI Brief

## API Identity
- Domain: The open-source Home Assistant automation server, not Apple Home. It can control devices exposed through HomeKit Bridge and other integrations, but the target is a user's Home Assistant instance and its entity, registry, service, configuration, history, and event APIs.
- Users: (1) a renter or homeowner who asks an OpenClaw-style agent to run household routines without memorizing entity IDs; (2) a Home Assistant enthusiast who debugs automations and reorganizes entities weekly; (3) a household operator who wants safe bulk changes with a reviewable plan; (4) a self-hoster who monitors upgrades, backups, integrations, logs, and device health.
- Data profile: Current entity state is volatile; areas, floors, devices, entities, labels, categories, services, scenes, scripts, automations, traces, history, logbook entries, calendars, todo items, integrations, updates, and system health are relational and benefit from local snapshots and search.
- Auth: `Authorization: Bearer <long-lived-access-token>` for REST; WebSocket sends `{ "type": "auth", "access_token": "..." }`. Dominant incumbent variables are `HASS_SERVER`/`HASS_TOKEN`; the leading MCP uses `HOMEASSISTANT_URL`/`HOMEASSISTANT_TOKEN`.

## Reachability Risk
- Low for documented APIs, but the target normally lives on a user's LAN and requires a server URL plus a long-lived token. Official REST and WebSocket docs returned HTTP 200. No personal instance or token is available in this run, so live discovery and authenticated dogfood will be skipped.
- The official docs expose `/api/config`, `/api/components`, `/api/events`, `/api/services`, `/api/history/period`, `/api/logbook`, `/api/states`, `/api/error_log`, `/api/camera_proxy`, `/api/calendars`, `/api/template`, `/api/config/core/check_config`, `/api/intent/handle`, and service/event/state mutations. The WebSocket API adds state/event subscriptions, service calls, config validation, entity registry display, target extraction, triggers, and exposure management.
- Crowd-sniff was attempted as primary community discovery and returned no usable endpoints after npm extraction failures; docs and verified incumbent source remain the contract sources.

## Reachability Gate
- Decision: PASS (carve-out)
- Reason: lan-only-no-global-url
- Evidence: Home Assistant targets a user-owned LAN instance configured through `HASS_SERVER` (commonly `http://homeassistant.local:8123`); probing that hostname from the generation host would test this machine's network rather than the eventual user's server.

## Top Workflows
1. **Run a household mode safely:** Resolve friendly phrases such as “movie night” into a scene/script or a previewed bundle of service calls, show exactly which entities will change, apply it, then verify the resulting states.
2. **Answer “what is happening at home?”:** Search entities by friendly name, area, floor, label, device class, or state; answer questions such as which windows are open, which lights are on, and which batteries are low.
3. **Debug yesterday's failures:** Correlate automation traces, history, logbook, system logs, and entity changes around a time window to explain why a routine did not fire or did the wrong thing.
4. **Maintain the installation:** Inspect integrations, updates, backups, HACS/add-ons, system health, devices, orphaned entities, and stale/unavailable sensors; make reversible, previewed changes.
5. **Organize the model:** Bulk assign areas/floors/labels/categories, rename or disable registry entries, and validate references in automations/scenes/scripts before applying.

## Table Stakes
- From `home-assistant-ecosystem/home-assistant-cli` (`hass-cli`, 575 stars): discovery; config/system info; state list/get/edit/history; service list/call; event watch; areas/devices/entities/integrations; raw REST/WebSocket access; templates; map; supervisor/HAOS operations; JSON/YAML/table/NDJSON; custom columns, sorting, completions, TLS/client-certificate support.
- From `homeassistant-ai/ha-mcp` (4,019 stars, 87 published tools): smart search/overview; verified bulk control; operation status; services/events; floors/areas/devices/entities/labels/categories/zones; automations/scripts/scenes/helpers/groups/dashboards/blueprints; traces/history/logbook/logs; integrations/config flows; calendar/todo; cameras; energy; updates/backups/add-ons/HACS; radio networks; voice/Assist pipelines and exposure; system health; files/YAML/themes; templates; reload/restart; guarded destructive operations.
- Open incumbent pain points: entity deletion runtime failure (#468); incomplete service-argument completion (#431); missing labels/floors/categories (#424); bulk rename request (#407); state history escaping failure (#394); missing entity types (#341); unclear scene invocation (#338); lint/sanity-check request (#96).
- Agent baseline: deterministic JSON, `--agent`, `--select`, typed exit codes, dry-run, idempotency keys where possible, explicit destructive confirmation, no secrets in output, MCP mirror, and local SQLite sync/search/analytics.

## Data Layer
- Primary entities: areas, floors, devices, entities/states, labels, categories, services, scenes, scripts, automations, integrations, traces, history, logbook, updates, backups, calendars, todo items, system health.
- Relationships: floor → areas; area → devices/entities; device → entities; labels → entities/devices/areas; automations/scenes/scripts → referenced entity/service IDs; entity → state/history/logbook; automation/script → traces.
- Sync cursor: snapshot timestamp for registries/services/config; event/history timestamps for incremental history and logbook sync; trace run IDs for execution history.
- FTS/search: friendly names, entity IDs, aliases, area/floor/label names, integration titles, automation/script/scene names, service descriptions, log messages, and configuration references.

## Codebase Intelligence
- Source: DeepWiki and source analysis of `homeassistant-ai/ha-mcp` plus official Home Assistant docs.
- Auth: Long-lived tokens use `Authorization: Bearer <token>`; WebSocket authenticates with an access-token message. The MCP supports a stateless OAuth proxy but direct token auth is sufficient for a local CLI.
- Data model: REST handles state queries, history, logbook, templates, service calls, and common configuration; a persistent WebSocket handles subscriptions, real-time verification, registry/config-entry flows, and richer admin surfaces.
- Rate limiting: Official docs do not publish a general fixed limit; the leading MCP defaults to 30-second requests and three retries. The CLI should use bounded retries only for safe reads and never replay uncertain writes.
- Error handling: distinguish transport/DNS/timeout, 401 auth, 400 validation, 404 entity/route, 409/conflict-like state drift, and 5xx server errors; include the server-provided message without leaking tokens.
- Architecture: high-value operations span both REST and WebSocket. A one-shot REST wrapper cannot match registry/config flows or post-action verification; the CLI needs a shared client that can open a temporary WebSocket when required.

## User Vision
- Build a fun, personal-use CLI that lets an OpenClaw agent do things people normally enjoy asking for: “turn the apartment into movie-night mode,” “which windows are open?”, and “lower the lights when playback starts.”
- Use the rigorous Printing Press formula even though public GitHub alternatives exist: absorb their useful capabilities, preserve scriptable CLI ergonomics, and beat them with safe agent workflows and local memory.

## Product Thesis
- Name: Home Assistant Printing Press CLI (`home-assistant-pp-cli`).
- Why it should exist: `hass-cli` is a capable low-level operator CLI and `ha-mcp` is a broad agent server, but neither gives a portable, auditable Go CLI with offline household memory, agent-shaped output, preview/apply/verify semantics, and a generated MCP mirror from the same command tree.

## Build Priorities
1. Entity/service/event/state primitives plus search, areas/floors/devices/entities/labels/categories and JSON-safe output.
2. Scenes/scripts/automations, traces/history/logbook, integrations/system health/updates/backups, and WebSocket verification.
3. Local sync/search/analytics and five or more household-specific compound commands that are safer and more useful than one endpoint at a time.
4. Make every mutation previewable; resolve friendly names deterministically and reject ambiguity instead of guessing.
5. Preserve a clear boundary: this is Home Assistant, not Apple's Home app; Apple Home devices are controlled only when bridged into Home Assistant.
