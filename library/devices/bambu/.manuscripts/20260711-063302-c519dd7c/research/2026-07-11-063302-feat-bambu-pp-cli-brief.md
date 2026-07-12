# Bambu Reprint Brief

## User Vision

This CLI is for people wiring an agent or their own code to monitor a Bambu print and post useful updates to Discord, webhooks, or another automation. The first screen of the README and skill must say that plainly.

The supported runtime is normal local access only: SSDP discovery, MQTT observation, implicit FTPS for exact current-job 3MF metadata and the plate preview, local history, and provider-neutral lifecycle payloads.

Developer Mode, LAN Only Mode, Bambu Cloud, printer control, print dispatch, camera control, and farm management are intentionally unsupported. Do not expose refusal stubs for those operations. Control-oriented users should use Bambuddy or another established project.

Most users have one printer. Keep the zero-configuration default. Optional `--printer` profiles remain for homes with more than one printer, without fleet-management positioning.

## Primary Workflow

1. Observe a real print start over local MQTT.
2. Read exact plate weight and the model-preview PNG from the current 3MF over FTPS.
3. Emit a provider-neutral `print.started` payload with ETA and attachment metadata.
4. Observe the real terminal MQTT transition.
5. Emit and persist `print.finished`, `print.failed`, or `print.canceled` without stale ETA fields.

## Research Mode

Reuse the prior protocol research from run `20260711-005600-31ac3ca2`. It is less than one day old, the local protocol evidence was live-verified, and this reprint changes product scope rather than protocol assumptions.

## Reprint Machine Delta

The local Printing Press now emits bounded HTTP response reads, bounded/cancellable MCP search, symlink-safe delivery temp files, and capability-aware archive/analytics utilities. The reprint must demonstrate that those generated fixes survive in the Bambu output.
