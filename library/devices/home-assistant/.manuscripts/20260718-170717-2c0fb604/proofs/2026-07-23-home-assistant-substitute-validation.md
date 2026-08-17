# Home Assistant substitute validation — 2026-07-23

## Live boundary

The validation host has neither a reachable Home Assistant instance nor a
Long-Lived Access Token. Home Assistant is a user-owned LAN service, so this
report does not claim live Phase 5 acceptance and does not substitute an
unrelated public server.

## Current-head canonical shipcheck

Printing Press shipcheck was rerun against PR head
`eba7bcb8d2f43a86da0b37d50bdf0718ab831471`, the archived OpenAPI document,
and run `20260718-170717-2c0fb604`.

```text
shipcheck: PASS (7/7 legs, exit 0)
verify: PASS (86/86, 100%)
validate-narrative --strict --full-examples: PASS
structural dogfood: PASS
workflow-verify: PASS
apify-audit: PASS
verify-skill: PASS
scorecard: PASS (92%, Grade A)
```

Live scorecard sampling was disabled because no real installation was
available. The checked binary was rebuilt from the exact PR source before the
run.

## Behavioral substitute

The full Go suite passes together with `go vet ./...` and `go build ./...`.
The tree contains 590 named test functions and 15 local HTTP/WebSocket test
servers. The service-shaped regression coverage includes:

- Home Assistant WebSocket authentication, event subscription, event receipt,
  unknown commands, and cancellation;
- mode application that is reported as verified only after a state refresh;
- rejection of unverifiable service responses and partial import failures;
- persistence and reconciliation of changed or deleted entity state;
- mutation confirmation before client creation;
- generated REST client request, redirect, retry, and verification behavior;
- MCP command-tree and bound-tool validation.

This exercises the actual request/response and WebSocket wire paths without
requiring or mutating a contributor's private home.

## Commands

```text
go test ./...: PASS
go vet ./...: PASS
go build ./...: PASS
```

No token, private hostname, entity identifier, or live household payload is
included.
