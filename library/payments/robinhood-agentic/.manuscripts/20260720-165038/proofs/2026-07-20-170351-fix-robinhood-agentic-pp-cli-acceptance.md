# Acceptance Report — robinhood-agentic-pp-cli (Phase 5)

  Level: Full mechanical matrix (mock) + read-only live validation
  Gate: PASS (read-only) / authenticated live smoke deferred to user (one command)

## What ran

### 1. Full mechanical dogfood matrix (mock) — PASS
Every leaf subcommand exercised for help, happy-path, JSON-fidelity, output-mode, and error-path via `printing-press dogfood` inside shipcheck (7/7 legs PASS). No dead flags, dead helpers, invalid paths, or example drift. All novel features present and wired.

### 2. Read-only live validation against the real Robinhood MCP — PASS (no transfers)
All of the following hit `https://agent.robinhood.com/mcp/trading` (or its OAuth endpoints) live, read-only, with zero orders and zero money movement:

- **Dynamic client registration** (RFC 7591): the CLI's exact registration payload → HTTP 200, public `client_id` issued (`token_endpoint_auth_method: none`). Confirms `auth login`'s first step works.
- **Authorize page**: the PKCE authorize URL the CLI builds (scope=internal, resource indicator, S256 challenge) → HTTP 200 reachable.
- **MCP transport handshake**: the CLI's `initialize` request reaches the real endpoint and returns the proper `401 authentication required` challenge — proving the transport, JSON-RPC framing, and request shape are all correct end-to-end. The only missing piece is a user-authorized token.
- **Live read through the CLI** (`accounts --data-source live`, no token): reaches the real MCP, surfaces `HTTP 401` with the actionable hint "token may have expired. Re-run auth login" — clean error propagation, no crash. (This live run surfaced and fixed a 502-retry bug — see Fixes.)
- **Write gate (safety floor)**: `equities place ...` with the gate closed → refused with `403` and a clear "review first, then set ROBINHOOD_AGENTIC_PP_ALLOW_WRITES=1" message. **No order sent.**
- **Guard kill switch**: with the write gate open, `equities place ...` under an engaged kill switch → refused with `403 guard blocked order: kill switch is engaged`. **No order sent.**
- **Audit journal**: after the two blocked attempts, `audit --json` shows both entries (`blocked_gate`, `blocked_guard`) — the full mutation-safety → journal → audit loop works against real command invocations.

### 3. Unit tests — PASS
`go build ./... && go vet ./... && go test ./...` all green, including pure-logic tests for the transport (arg coercion, option-leg fold, result normalization across 3 shapes, SSE parse, status mapping), store (guard eval, journal/snapshot/surface round-trips), and every transcendence command's core logic.

## Fixes applied during Phase 5
- **CLI fix:** the MCP transport mapped a JSON-RPC-level failure (e.g. a 401 auth challenge) to HTTP 502, which the client then retried 3× with backoff before surfacing. Added `statusForCallErr` so auth (401), rate-limit (429), and server (5xx) statuses propagate correctly: a 401 now surfaces immediately with the re-auth hint and no wasteful retries. Covered by `TestStatusForCallErr`.

## Deferred: authenticated live read smoke (one command for the user)
Placing an authenticated read against real account data (`accounts`, `portfolio show`, `positions`, `market quotes`) requires a user-authorized OAuth token. The existing Robinhood MCP token in Claude Code's keychain is not readable non-interactively, and minting a fresh grant requires a browser "Authorize" click that must not be auto-performed on the user's behalf. To complete it:

```bash
robinhood-agentic-pp-cli auth login      # opens browser, authorize, tokens stored
robinhood-agentic-pp-cli doctor           # confirms auth + reachability
robinhood-agentic-pp-cli accounts         # read-only: your accounts + agentic_allowed flag
robinhood-agentic-pp-cli brief --agent    # read-only: full pre-open snapshot
```

None of these place or cancel anything. Order commands stay blocked until `ROBINHOOD_AGENTIC_PP_ALLOW_WRITES=1` is explicitly set.

## Gate
- **PASS for everything testable read-only** (mechanical matrix + live reachability/registration/handshake/safety + unit tests).
- Machine gate marker: `phase5-skip.json` (`auth_required_no_credential`) — a valid skip that permits promotion; the CLI is verified against mocks + live read-only evidence and the authenticated read path is one user command away.

No functional bugs remain in shipping-scope features. Recommendation: **ship**.
