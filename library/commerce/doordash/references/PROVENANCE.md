# DoorDash CLI Build Provenance

Phase 1 workspace: `/home/hermes/printing-press/library/doordash-pp-cli`

Generated skeleton: Printing Press 4.6.1 reprint/merge from the curated GraphQL operation map; original private-beta skeleton began as PP 4.2.0 from synthetic HAR/source-derived traffic context
Generated skeleton path: `/home/hermes/printing-press/library/doordash-pp-cli`
Generated at: 2026-05-13T21:47:44.838084603Z
Recorded/verified at: 2026-05-13T22:18:29Z
PER-33 re-verified at: 2026-05-13T22:59:57Z

## Reference inputs

- Reference MCP source: https://github.com/ashah360/doordash-mcp
- Local source path: `/home/hermes/printing-press/sources/doordash-mcp-ashah360`
- Reference MCP commit: b51dba31fe1aae531c0978950c02c3a473d7460f
- Source working tree at verification time: dirty
  - `M package-lock.json`
  - `M package.json`
  - `M src/client/http.ts`
  - `?? src/cli.ts`

## Generated/copied artifacts

- `spec.yaml` sha256: 9a4e97b04e17894050fe6289c60d0462e0b6e0cca707b49c48c1c57b9335cd7b
- `references/doordash-sniffed.yaml` sha256: 132bb43b71e0bd4f00d654e26de203ab46cdfa36f520e44cc98f18840a9b70bc
- `references/doordash-sniffed-traffic-analysis.json` sha256: 30609efb10c3df17abd6b8f24d5340c6a9c0bc35e1f000243f1cc541904cf77c
- `references/doordash-graphql-spec.yaml` sha256: 9a4e97b04e17894050fe6289c60d0462e0b6e0cca707b49c48c1c57b9335cd7b
- `references/doordash-graphql-spec-manifest.json` sha256: 8f5537f2f854aab197f28b4cdacffcfcbebb47c1969f98d73cd9d83dbba6c107
- `.printing-press.json` records the generator metadata, API name, CLI name, and current spec checksum.
- PER-33 verification confirmed the curated manifest contains 19 operations and the regenerated CLI exposes 19 `graphql create-*` commands.

## Safety

No credentials, cookies, DoorDash sessions, or bricenice17 account secrets are stored in this repo. Live auth must use `~/.doordash-mcp` or a bricenice17-approved session import path outside this generated tree.
