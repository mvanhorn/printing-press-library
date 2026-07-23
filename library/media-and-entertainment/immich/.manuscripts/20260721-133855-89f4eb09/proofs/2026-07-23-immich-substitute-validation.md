# Immich substitute validation — 2026-07-23

## Live boundary

The validation host has neither a reachable Immich deployment nor an
`IMMICH_API_KEY`. Immich is self-hosted, so this report does not claim live
Phase 5 acceptance and does not send private photos to an unrelated instance.

## Current-head canonical shipcheck

Printing Press shipcheck was rerun against PR head
`626167b15231a922fff90f407eb89ca66c256317`, the archived Immich OpenAPI
document, and run `20260721-133855-89f4eb09`.

```text
shipcheck: PASS (7/7 legs, exit 0)
verify: PASS
validate-narrative --strict --full-examples: PASS
structural dogfood: PASS (370/370 wiring; 8/8 novel coverage)
workflow-verify: PASS
apify-audit: PASS
verify-skill: PASS
scorecard: PASS (97%, Grade A)
```

Live scorecard sampling was disabled because no real installation was
available. The checked binary was rebuilt from the exact PR source before the
run.

## Behavioral substitute

The full Go suite passes together with `go vet ./...` and `go build ./...`.
The tree contains 762 named test functions and 33 local HTTP test servers.
Service-shaped tests cover:

- people lookup followed by a separate query for every requested July;
- event-album preview versus explicit mutation;
- native duplicate planning, explicit keeper evidence, stale-plan rejection,
  and changed-evidence rejection before resolution;
- stack-detail retrieval and classification;
- exact archived-OpenAPI request and response shapes for every novel route;
- checksum and multipart upload fields;
- partial upload, metadata, and album-assignment failure recovery;
- archive traversal rejection and takeout sidecar path handling;
- source-collection prevalidation before any album or tag mutation;
- paginated Immich-to-Immich transfer, collision-safe temporary files, cleanup,
  and partial mapping rejection;
- MCP command-tree and bound-tool validation.

This executes the actual client and workflow boundaries with deterministic
responses while keeping private photo libraries off the validation host.

## Commands

```text
go test ./...: PASS
go vet ./...: PASS
go build ./...: PASS
```

No API key, private server URL, account identifier, photo metadata, or image
payload is included.
