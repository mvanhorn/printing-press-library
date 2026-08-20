---
title: Add OAuth login flow to GSC CLI
type: feat
status: active
date: 2026-05-11
---

# Add OAuth login flow to GSC CLI

## Overview

Replace the current "paste a 1-hour token from OAuth Playground" auth UX with a real `gsc auth login` command backed by a loopback OAuth 2.0 flow (PKCE S256), refresh-token persistence, and silent auto-refresh inside every existing command. End state: user logs in once, never sees a token again.

## Problem Statement

The published GSC CLI ships with a usable-but-painful auth path: `export GSC_ACCESS_TOKEN=ya29...` from the OAuth Playground. The token dies in 1 hour and the user re-pastes it. The Printing Press already emitted half the refresh machinery (`RefreshToken` field, `refreshAccessToken()` function, `SaveTokens()`/`ClearTokens()` methods) but left the token endpoint URL hardcoded empty (`internal/client/client.go:357-360`), so refresh is a silent no-op. We're finishing the half-built flow and adding the interactive login + supporting commands.

## Proposed Solution

Add three new auth subcommands (`login`, `set-client`, `forget`), expand one (`status`), wire the refresh-token machinery to Google's real token endpoint, and make every existing command silently refresh expired access tokens before each request.

### Auth command surface (final shape)

```
gsc auth set-client <client_id> [<client_secret>]   # one-time, persists OAuth client to config
gsc auth login [--scope readonly|write] [--no-browser] [--port N]
gsc auth status [--json] [--show-token]
gsc auth set-token <token>                           # existing, keep for env-equivalent UX
gsc auth logout [--revoke]                           # clear tokens, keep client; --revoke hits Google's /revoke
gsc auth forget                                      # nuke everything (tokens + client)
```

### OAuth flow

1. Bind `net.Listen("tcp", "127.0.0.1:0")` for OS-assigned random loopback port (literal `127.0.0.1`, **not** `localhost` — Claude Code shipped a bug from this exact mistake, cli/cli#42765).
2. Generate PKCE verifier via `oauth2.GenerateVerifier()`, S256 method.
3. Generate random 32-byte state via `crypto/rand`.
4. Build auth URL with `oauth2.AccessTypeOffline` + `prompt=consent` + `S256ChallengeOption(verifier)`.
5. Open browser via `wslview` (WSL) / `xdg-open` (Linux) / `open` (macOS) / `cmd /c start` (Windows). On failure, print URL plainly and keep listening.
6. Loopback receives `/callback`, verifies state, rejects mismatches with 404. Single-shot the listener so a second callback is ignored.
7. Exchange code via `oauth2.Config.Exchange()` with `VerifierOption(verifier)`.
8. Atomically save `(access_token, refresh_token, expiry, client_id, client_secret)` via `os.Rename` from `.tmp`.
9. Render success page; auto-shutdown listener after 3s grace.

### Refresh wiring

`internal/client/client.go:357` fix: set `tokenURL = "https://oauth2.googleapis.com/token"`. The rest of `refreshAccessToken()` is correct as written — it persists rotated refresh tokens when Google returns one, retains the old one otherwise. Only addition needed: classify `invalid_grant` error responses as terminal (refresh token revoked/expired) and clear tokens with an actionable message: `Run 'gsc auth login' to re-authenticate.`

### Token storage

Existing `~/.config/google-search-console-pp-cli/config.toml` at mode 0600. Atomic save (rename from `config.toml.tmp`). Verify file mode on every read; warn if not 0600. Detect `/mnt/c/` style non-POSIX filesystems and warn during login that file mode can't be enforced there.

## Technical Approach

### File-level changes

| File | Change |
|---|---|
| `internal/cli/auth.go` | Add `login`, `set-client`, `forget` subcommands. Expand `status`. |
| `internal/cli/auth_login.go` (new) | Loopback server, PKCE, state handling, browser open helpers. |
| `internal/cli/auth_browser.go` (new) | Cross-platform browser open (WSL/Linux/macOS/Windows). |
| `internal/client/client.go:357` | Fix empty `tokenURL`; add `invalid_grant` terminal handling. |
| `internal/config/config.go:123` | Make `save()` atomic via tmp + `os.Rename`. |
| `internal/config/config.go:78-91` | Fix dead-code branch; document env-var > file precedence explicitly. |
| `internal/cli/doctor.go` | Surface refresh-token presence and expiry countdown. |
| `internal/mcp/tools.go` | Auth-error responses include `auth_required: true` hint when refresh fails in MCP context. |
| `README.md` | Auth section rewrite: lead with `gsc auth login`, demote `GSC_ACCESS_TOKEN` to "for CI / scripted use". |
| `SKILL.md` | Same — login-first, env-var fallback. |
| `go.mod` | Add `golang.org/x/oauth2` (and `golang.org/x/oauth2/google` for the endpoint constant). |
| `.printing-press-patches.json` | New file; catalogs this customization per AGENTS.md contract. |
| `internal/cli/auth_login_test.go` (new) | Unit tests: state/PKCE generation determinism (with seeded RNG), callback URL parsing, error param handling. |

### Why user-provided client credentials (not embedded)

Considered embedding. The argument for embed: every major CLI does it (`gh`, `gcloud`, `firebase`, `wrangler`, `vercel`), Google's docs say native-app client secrets are explicitly not secrets. The argument against: this CLI ships in a public library under Matt's name; embedding ties the publisher to OAuth verification submission (homepage, privacy policy, demo video, ~4-6 weeks) once usage crosses 100 users, plus the brand-impersonation phishing risk. **Decision: user-provided in v1**, with `set-client` as a one-time onboarding step. Embed remains a Phase 2 option if usage justifies the verification commitment. Document the tradeoff in README.

### Why loopback (not device flow)

Google deprecated OOB / device flow for installed apps. Loopback is the official replacement per Google's [Loopback IP Migration Guide](https://developers.google.com/identity/protocols/oauth2/resources/loopback-migration). Device flow (RFC 8628) only matters for truly headless environments and adds complexity — the `--no-browser` fallback (print URL, wait for callback) handles SSH/headless cases without a second flow.

### Why PKCE S256

RFC 8252 says public clients MUST use PKCE. Google supports both S256 and plain; plain is discouraged. The `golang.org/x/oauth2` package added native PKCE helpers (`GenerateVerifier`, `S256ChallengeOption`, `VerifierOption`) in v0.10. Use them.

## System-Wide Impact

### Interaction graph

`gsc <any-command>` → `rootFlags.newClient()` → `client.New()` → first API call → `c.authHeader()` → refresh check at `client.go:341` → if `RefreshToken != "" && expired`, `c.refreshAccessToken()` → POST to token endpoint → `cfg.SaveTokens()` (atomic) → request proceeds with fresh Bearer. **New invariant:** every API call may now do a token refresh before the actual request. Confirm timeout context propagation: `refreshAccessToken` uses `c.HTTPClient` which carries the user's `--timeout`. Long sync operations may need their own auth refresh at the loop boundary to avoid mid-loop refresh under tight per-request timeouts.

### Error propagation

- `invalid_grant` from token endpoint = terminal. Wipe tokens, return `authErr("Refresh token rejected — run 'gsc auth login' again")`. Exit code already mapped via `authErr`.
- Network error during refresh = transient. Bubble up as-is; user retries.
- Callback `?error=access_denied` (user clicked Cancel) = friendly message, exit 1.
- Callback state mismatch = 404 + log + ignore (don't surface to user — could be CSRF).
- Bind failure (port firewalled) = try IPv6 `[::1]:0` fallback, then exit with explicit `--port` hint.

### State lifecycle risks

The current `save()` (`config.go:123`) is non-atomic — `os.WriteFile` truncates then writes. SIGKILL between truncate and write = silent loss of refresh token. **Fix is non-negotiable:** write to `config.toml.tmp` (mode 0600), then `os.Rename` to `config.toml`. On Windows this is also atomic per Go's `os.Rename` contract since Go 1.5.

### API surface parity

The MCP server (`internal/mcp/tools.go`) shares `internal/client`, so silent refresh comes for free once `tokenURL` is set. The only MCP-specific change: when refresh fails inside MCP (which has no interactive path), return a structured error `{auth_required: true, hint: "..."}` instead of a generic 401 so Claude Desktop / other MCP clients can surface the message cleanly.

### Integration test scenarios

1. **Fresh user, no client set**: `gsc auth login` → friendly "run `gsc auth set-client` first" message + link to Google Cloud Console.
2. **Login → expire → command**: simulate expired access token in test, run any read command, assert refresh happened and command succeeded (`httptest.Server` standing in for both token endpoint and API).
3. **Refresh token revoked**: token endpoint returns `invalid_grant`, assert tokens cleared and error mentions `gsc auth login`.
4. **Rotated refresh token**: token endpoint returns new `refresh_token`, assert new value persisted, old value gone.
5. **Concurrent commands**: two `gsc` invocations both noticing expiry, both POST refresh, assert file isn't corrupted (covered by atomic save).
6. **Env-var + file mismatch**: `GSC_ACCESS_TOKEN` set + file has different account's token. `auth status` warns of identity divergence (if id_token decode reveals different `email`/`sub`).

## Acceptance Criteria

### Functional

- [ ] `gsc auth set-client <id> <secret>` persists OAuth client to config (mode 0600 verified).
- [ ] `gsc auth login` opens browser, completes loopback flow, persists access + refresh + expiry.
- [ ] `gsc auth login --scope write` requests write scope; default is `webmasters.readonly`.
- [ ] `gsc auth login --no-browser` prints URL and waits for callback without trying to open a browser.
- [ ] `gsc auth login --port <N>` overrides random port.
- [ ] After `auth login`, ALL existing commands work without `GSC_ACCESS_TOKEN`.
- [ ] When the access token expires, the next command silently refreshes; user sees no error.
- [ ] `gsc auth status` shows: account email (from id_token), scopes, expiry with relative time, refresh-token presence, config path, file mode.
- [ ] `gsc auth logout` clears access + refresh tokens, keeps client_id/secret.
- [ ] `gsc auth logout --revoke` additionally POSTs to `https://oauth2.googleapis.com/revoke`.
- [ ] `gsc auth forget` clears everything (tokens + client).
- [ ] `gsc auth login` without `set-client` first prints actionable error with Google Cloud Console link.
- [ ] Existing `GSC_ACCESS_TOKEN` env var path still works (backward compat).
- [ ] Existing `gsc auth set-token <token>` still works.
- [ ] When env-var token expires and no file refresh token exists, error explicitly says "set up `gsc auth login` for auto-refresh".

### Non-functional

- [ ] All token-handling paths use `maskToken()` for any log output.
- [ ] Atomic file save (no partial-write window).
- [ ] PKCE S256 verified end-to-end (verifier 43-128 chars, S256 challenge, `VerifierOption` on exchange).
- [ ] State param verified on callback; mismatches return 404 without leaking info.
- [ ] No client_secret or refresh_token ever appears in `--verbose` output or error messages.
- [ ] WSL2 path: `wslview` preferred over `xdg-open`, URL-print fallback if both fail.

### Quality gates

- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `go test ./...` clean (existing tests + new unit tests).
- [ ] Dogfood verify (`make verify` or equivalent) still passes for non-auth commands.
- [ ] README + SKILL.md updated.
- [ ] `.printing-press-patches.json` documents the change.

## Edge Cases (from SpecFlow analysis)

**Must handle:**

- `invalid_grant` on refresh → wipe + actionable error (NOT raw HTTP 400).
- `?error=access_denied` callback → friendly message ("Login cancelled. Run `gsc auth login` to try again").
- State mismatch on callback → 404 silent ignore.
- Process killed mid-flow → atomic save protects refresh token.
- WSL2 `xdg-open` flakiness → prefer `wslview`, print URL fallback.
- Port firewalled → try `[::1]:0`, then surface `--port` hint.
- Browser-open success but user never completes → 5-min listener timeout with clear message.
- Loopback bind on `localhost` → forbidden; use `127.0.0.1` literal (cli/cli#42765 precedent).
- Concurrent `auth login` → file lock on save; second invocation still gets its own port so listeners don't collide.
- Scope mismatch on write command (403 from API) → detect, print `gsc auth login --scope write` hint, don't auto-retry.
- MCP client hits expired token, refresh fails → structured `auth_required` error, not generic 401.
- Non-POSIX filesystem (e.g. `/mnt/c/...`) → warn at login, refuse to save tokens there without `--allow-insecure-store`.
- `set-client <id> <secret>` leaking secret to shell history / `ps` → support `--secret-stdin` mode, document the risk for the positional form.
- Env-var token + file token identity mismatch → `auth status` decodes id_tokens (if present) and warns.

**Nice to have (defer if scope creeps):**

- Clock skew detection (compare server `Date:` header to local).
- Multi-account profiles (single-account v1; users can swap configs via `GOOGLE_SEARCH_CONSOLE_CONFIG` env var).
- Keychain integration (TOML 0600 is median peer behavior; defensible).
- 100-refresh-token Google eviction warning.

## Implementation Phases

### Phase 1: Fix the existing stub + atomic save (foundation, ~30 min)

- Set `tokenURL = "https://oauth2.googleapis.com/token"` at `client.go:357`.
- Make `cfg.save()` atomic (tmp + rename) at `config.go:123`.
- Fix `AuthHeader()` dead-code branch at `config.go:87-91`.
- Add `invalid_grant` terminal-error classification in `refreshAccessToken()`.
- Build + unit test on these.

### Phase 2: Add login command (core, ~2 hr)

- Add `golang.org/x/oauth2` + `golang.org/x/oauth2/google` to go.mod (`go get`).
- New `internal/cli/auth_login.go`: loopback server, PKCE, state, exchange, save.
- New `internal/cli/auth_browser.go`: WSL/Linux/macOS/Windows browser open.
- New `set-client` subcommand in `auth.go`.
- New `forget` subcommand in `auth.go`.
- Expand `status` subcommand to surface refresh + expiry + identity.
- Update `logout` subcommand: `--revoke` flag, document keep-client behavior.

### Phase 3: Wire-up + polish (~1 hr)

- README + SKILL.md auth-section rewrite (login-first).
- Update MCP tools.go auth-error responses to include `auth_required` hint.
- Update doctor.go to surface refresh-token presence and expiry.
- `.printing-press-patches.json` entry.
- Inline `// PATCH:` comments on every modified line per AGENTS.md contract.

### Phase 4: Tests + verification (~1 hr)

- Unit tests for state/PKCE generation, callback parsing, atomic save, env-vs-file precedence.
- Manual smoke: `go build`, `gsc auth set-client <test-id> <test-secret>`, mock the OAuth flow if practical.
- `go vet`, `go test ./...`, `go build ./...` all green.

### Phase 5: Multi-agent review (`/workflows:review`) + handoff (~30 min)

- Trigger compound engineering review.
- Address findings.
- Write the Printing Press retro (`/printing-press-retro`) so future CLIs get this wired by default.

## Risks & Mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Google changes OAuth API mid-flight | Low | Pin to stable endpoint constants; `oauth2/google.Endpoint` tracks changes. |
| Refresh-token rotation race (two terminals refreshing simultaneously) | Low | Atomic save means worst case is one of them writes a now-stale rotated token; next refresh fixes it. File lock for v2 if reports come in. |
| Unverified-app warning scares users | Medium | README explicitly walks through the "Advanced → Go to GSC CLI (unsafe)" path with a screenshot. Frame as Google's standard interstitial, not a CLI bug. |
| User saves tokens on Windows-mounted FS (`/mnt/c/`) where 0600 doesn't apply | Medium | Detect at login, warn, require `--allow-insecure-store` flag to proceed. |
| WSL2 browser open doesn't work | High | Print URL as fallback, document up front, `--no-browser` flag for reliability. |
| Spec-flow surfaced 100-refresh-token Google eviction (silent) | Low | Out of scope for v1; document in troubleshooting. |

## Out of Scope (Phase 2+)

- Service account auth (different code path; relevant for fully automated CI).
- Multi-account profiles.
- Embedded client_id (revisit if usage > 50 users).
- OAuth verification submission to Google (only matters past 100 users).
- Linux keychain integration via `secret-service`.
- Refresh-token revocation on logout by default (revoke is opt-in via `--revoke`).

## Sources & References

### Internal

- `internal/client/client.go:341-409` — existing refresh stub.
- `internal/config/config.go:107-133` — existing token save (needs atomicity).
- `internal/cli/auth.go` — existing status/set-token/logout subcommands.
- `internal/cli/root.go:187` — where `newAuthCmd` attaches to root.
- `AGENTS.md` — local-customization contract; requires `// PATCH:` comments and `.printing-press-patches.json`.

### External

- [Google OAuth 2.0 for iOS & Desktop Apps](https://developers.google.com/identity/protocols/oauth2/native-app)
- [Loopback IP Address flow Migration Guide](https://developers.google.com/identity/protocols/oauth2/resources/loopback-migration)
- [RFC 8252 — OAuth 2.0 for Native Apps](https://datatracker.ietf.org/doc/html/rfc8252)
- [golang.org/x/oauth2 docs](https://pkg.go.dev/golang.org/x/oauth2) — `GenerateVerifier`, `S256ChallengeOption`, `VerifierOption`.
- [Google API Scopes — Search Console](https://developers.google.com/identity/protocols/oauth2/scopes#searchconsole).

### Related precedents

- `gh auth login` — model for status output (cli/cli#13330 issue: gh doesn't surface expiry; we will).
- `gcloud auth login` — model for loopback flow.
- Claude Code #42765 — bug from using `localhost` instead of `127.0.0.1`; we avoid it.
