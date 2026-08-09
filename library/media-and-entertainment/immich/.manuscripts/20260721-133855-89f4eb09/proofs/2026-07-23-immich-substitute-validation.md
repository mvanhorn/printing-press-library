# Immich substitute validation — refreshed 2026-08-09

## Live boundary

The validation host has neither a reachable Immich deployment nor an
`IMMICH_API_KEY`. Immich is self-hosted, so this report does not claim live
Phase 5 acceptance and does not send private photos to an unrelated instance.

## Current-head canonical shipcheck

The full substitute suite was rerun against validated code commit
`af7c95d7ca2b3b8f137deb5fb610de4beee69df1`, the archived Immich OpenAPI
document, and run `20260721-133855-89f4eb09`. The follow-up evidence-only
commit does not change the validated binary.

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
The tree contains 763 named test functions and 35 local HTTP test servers.
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
- source-collection prevalidation before any album or tag mutation, plus
  idempotent resume after a destination membership failure without duplicate
  album or tag creation;
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
