# Bambu Reprint Absorb Manifest

## Approved Product Boundary

- Normal local access observation and automation only.
- No Developer Mode, LAN Only Mode, Bambu Cloud, printer-control, dispatch, camera-control, or farm-management commands or stubs.
- Single-printer default with optional named `--printer` selection for multi-printer homes.
- Upfront documentation identifies agent/code monitoring and notification payloads as the audience and job.

## Shipping Scope

| Feature | Command | Buildability | Status |
| --- | --- | --- | --- |
| Provider-neutral lifecycle payloads with exact weight and preview | `events watch` | hand-code | shipping |
| Calibrated completion forecast | `job eta` | hand-code | shipping |
| Filament runway | `ams runway` | hand-code | shipping |
| Failure correlation matrix | `history failure-correlations` | hand-code | shipping |
| Print-stage timeline | `job timeline` | hand-code | shipping |
| Firmware field drift | `printer field-diff` | hand-code | shipping |
| Repeat-job comparison | `job repeats` | hand-code | shipping |

Fleet attention was deliberately dropped. It conflicts with the approved personal automation focus and is not a deferred stub.

## Acceptance

- README and SKILL opening paragraphs state who the CLI is for before setup details.
- No privileged/control/fleet stub appears in Cobra, agent-context, or MCP tools.
- Attaching to an active job can emit a start payload with the actual plate-preview PNG.
- A real terminal transition emits and persists a clean finish payload.
- Full live dogfood passes using normal local access with Developer Mode and LAN Only Mode disabled.
